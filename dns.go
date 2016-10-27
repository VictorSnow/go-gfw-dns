package main

import (
	"errors"
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

const DNS_TIMEOUT = 5 * time.Second
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

func resolve(server string, req *dns.Msg) (*dnsRecord, error) {
	c := &dns.Client{
		Net:          "udp",
		ReadTimeout:  DNS_TIMEOUT,
		WriteTimeout: DNS_TIMEOUT,
	}

	qname := req.Question[0].Name

	r, _, err := c.Exchange(req, server)

	if err != nil {
		return nil, err
	}

	if r != nil && r.Rcode != dns.RcodeSuccess {
		return nil, errors.New("dns结果错误")
	}

	// just use ipv4 address
	for _, v := range r.Answer {
		if a, ok := v.(*dns.A); ok {
			if ip := a.A.To4(); ip != nil {
				expire := time.Now().Add(time.Duration(v.Header().Ttl+3600) * time.Second)
				return &dnsRecord{qname, ip, expire}, nil
			}
		}
	}

	// only ipv6
	for _, v := range r.Answer {
		if a, ok := v.(*dns.A); ok {
			if ip := a.A.To16(); ip != nil {
				expire := time.Now().Add(time.Duration(v.Header().Ttl+3600) * time.Second)
				return &dnsRecord{qname, ip, expire}, nil
			}
		}
	}

	return nil, errors.New("ip地址错误:" + qname)
}

func doResolve(server string, req *dns.Msg, recvChan chan<- dnsRecord, wg *sync.WaitGroup) {
	defer wg.Done()

	r, err := resolve(server, req)

	if err != nil {
		log.Println(err)
	}

	// try send or ignore
	if r != nil {
		select {
		case recvChan <- *r:
		default:
		}
	}
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

func dnsHandle(w dns.ResponseWriter, req *dns.Msg) {
	qname := req.Question[0].Name

	servers := inDoorServers
	if inHost(qname) {
		servers = bypassServers
	}

	// only handle  A record and hit cache
	if req.Question[0].Qtype == dns.TypeA {
		if record, ok := getRecord(qname); ok {
			if record.Expire.After(time.Now()) {
				responseRecord(w, req, record)
			}
		}
	} else {
		// 不处理其他类型的查询
		c := &dns.Client{
			Net:          "udp",
			ReadTimeout:  DNS_TIMEOUT,
			WriteTimeout: DNS_TIMEOUT,
		}

		resp, _, _ := c.Exchange(req, servers[0])
		if resp != nil {
			w.WriteMsg(resp)
		}
		return
	}

	recvChan := make(chan dnsRecord, 1)
	defer close(recvChan)

	var wg = &sync.WaitGroup{}

	for _, server := range servers {
		wg.Add(1)
		go doResolve(server, req, recvChan, wg)
	}

	select {
	case r := <-recvChan:
		addRecord(qname, r)
		responseRecord(w, req, r)
	case <-time.After(DNS_TIMEOUT):
		if record, ok := getRecord(qname); ok {
			responseRecord(w, req, record)
		}
		break
	}

	// do we need to wait all go routines exit ???
	wg.Wait()
}
