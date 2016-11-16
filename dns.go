package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"github.com/pmylund/go-cache"
)

const DNS_TIMEOUT = 20 * time.Second
const DNS_CACHE_INTERVAL = 24 * 365 * 60 * 60
const DNS_SAVE_INTERVAL = 60 * 60

type dnsRecord struct {
	Name   string
	Ip     net.IP
	Expire time.Time
}

var bypassServers []string
var inDoorServers []string
var cdns *cache.Cache

func ListenAndServe(address string, inDoor []string, byPass []string) {
	inDoorServers = inDoor
	bypassServers = byPass

	// 缓存文件
	cdns = cache.New(time.Second*time.Duration(DNS_CACHE_INTERVAL), time.Second*60)
	cdns.LoadFile("data.txt")
	defer func() {
		cdns.SaveFile("data.txt")
	}()

	// catch exit
	saveSig := make(chan os.Signal)
	go func() {
		select {
		case <-saveSig:
			cdns.SaveFile("data.txt")
		}
	}()
	signal.Notify(saveSig, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL, syscall.SIGABRT)

	udpHandler := dns.NewServeMux()
	udpHandler.HandleFunc(".", dnsHandle)

	udpServer := &dns.Server{
		Addr:         address,
		Net:          "udp",
		Handler:      udpHandler,
		UDPSize:      65535,
		ReadTimeout:  DNS_TIMEOUT,
		WriteTimeout: DNS_TIMEOUT,
	}

	err := udpServer.ListenAndServe()
	if err != nil {
		log.Println("服务错误", err)
	}
}

func removeRecord(qname string) {
	cdns.Delete(qname)
}

func getRecord(qname string) (dnsRecord, bool) {
	r, ok := cdns.Get(qname)
	if ok {
		rd, ok := r.(dnsRecord)
		return rd, ok
	}
	return dnsRecord{}, false
}

func addRecord(qname string, record dnsRecord) {
	cdns.Add(qname, record, DNS_CACHE_INTERVAL*time.Second)
}

func mutilResolve(server []string, req *dns.Msg, recvChan chan<- *dns.Msg) {
	defer close(recvChan)

	wg := &sync.WaitGroup{}
	for _, s := range server {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := &dns.Client{
				Net:          "udp",
				ReadTimeout:  DNS_TIMEOUT,
				WriteTimeout: DNS_TIMEOUT,
				DialTimeout:  DNS_TIMEOUT,
			}

			r, _, err := c.Exchange(req, s)

			if err != nil || (r != nil && r.Rcode != dns.RcodeSuccess) {
				return
			}
			select {
			case recvChan <- r:
			default:
			}
		}()
	}
	wg.Wait()
	// force always have one msg
	select {
	case recvChan <- nil:
	default:
	}
}

func parseDnsMsg(r *dns.Msg) *dnsRecord {
	if r == nil {
		return nil
	}

	qname := r.Question[0].Name
	// just use ipv4 address
	for _, v := range r.Answer {
		if a, ok := v.(*dns.A); ok {
			if ip := a.A.To4(); ip != nil {
				expire := time.Now().Add(time.Duration(v.Header().Ttl+3600) * time.Second)
				return &dnsRecord{qname, ip, expire}
			}
		}
	}

	// only ipv6
	for _, v := range r.Answer {
		if a, ok := v.(*dns.A); ok {
			if ip := a.A.To16(); ip != nil {
				expire := time.Now().Add(time.Duration(v.Header().Ttl+3600) * time.Second)
				return &dnsRecord{qname, ip, expire}
			}
		}
	}
	return nil
}

func responseRecord(w dns.ResponseWriter, req *dns.Msg, record dnsRecord) {
	ip := record.Ip

	a := &dns.A{
		Hdr: dns.RR_Header{
			Name:     record.Name,
			Rrtype:   dns.TypeA,
			Class:    dns.ClassINET,
			Rdlength: uint16(len(ip)),
			Ttl:      uint32(record.Expire.Sub(time.Now()).Seconds()),
		},
		A: ip,
	}

	q := req.Question[0]

	res := &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 req.MsgHdr.Id,
			Response:           true,
			Opcode:             0,
			Authoritative:      false,
			Truncated:          false,
			RecursionDesired:   true,
			RecursionAvailable: true,
			Zero:               false,
			AuthenticatedData:  false,
			CheckingDisabled:   false,
			Rcode:              0,
		},
		Compress: false,
		Question: []dns.Question{q},
		Answer:   []dns.RR{a},
		Ns:       []dns.RR{},
		Extra:    []dns.RR{},
	}

	w.WriteMsg(res)
	return
}

func inBlackIpList(ip net.IP) bool {
	str := ip.String()
	_, ok := BlackIpList[str]
	return ok
}

func dnsHandle(w dns.ResponseWriter, req *dns.Msg) {
	qname := req.Question[0].Name

	log.Println("解析域名", qname)

	servers := inDoorServers
	mode := "normal"

	if inHost(qname) {
		mode = "bypass"
		servers = bypassServers
	}

	// only handle  A record and AAAA record
	if req.Question[0].Qtype == dns.TypeA || req.Question[0].Qtype == dns.TypeAAAA {
		if record, ok := getRecord(qname); ok {
			if record.Expire.After(time.Now()) {
				responseRecord(w, req, record)
			}
		}
	} else {
		recvChan := make(chan *dns.Msg, 1)
		go mutilResolve(servers, req, recvChan)

		select {
		case r := <-recvChan:
			if r != nil {
				w.WriteMsg(r)
			}
		}
		return
	}

	recvChan := make(chan *dns.Msg, 1)
	go mutilResolve(servers, req, recvChan)

	select {
	case msg := <-recvChan:
		if msg != nil {
			r := parseDnsMsg(msg)
			if r != nil {
				// 国内的dns返回的是污染的ip
				if mode == "normal" && inBlackIpList(r.Ip) {
					// 重新解析qname
					log.Println("受污染的域名", qname)
					addHost(qname)
					dnsHandle(w, req)
				} else {
					addRecord(qname, *r)
					responseRecord(w, req, *r)
				}
			}
		}
	case <-time.After(DNS_TIMEOUT):
		if record, ok := getRecord(qname); ok {
			responseRecord(w, req, record)
		}
		break
	}
}
