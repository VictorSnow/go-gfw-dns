package main

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/pmylund/go-cache"
)

const DNS_TIMEOUT = 5 * time.Second
const DNS_CACHE_INTERVAL = 24 * 365 * 60 * 60
const DNS_SAVE_INTERVAL = 60 * 60

type dnsRecord struct {
	Name   string
	msg    *dns.Msg
	Expire time.Time
}

var bypassServers []string
var inDoorServers []string
var Cdns *cache.Cache

func ListenAndServe(address string, inDoor []string, byPass []string) {
	inDoorServers = inDoor
	bypassServers = byPass

	// 缓存文件
	Cdns = cache.New(time.Second*time.Duration(DNS_CACHE_INTERVAL), time.Second*60)

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
	Cdns.Delete(qname)
}

func getRecord(qname string) (dnsRecord, bool) {
	r, ok := Cdns.Get(qname)
	if ok {
		rd, ok := r.(dnsRecord)
		return rd, ok
	}
	return dnsRecord{}, false
}

func addRecord(qname string, record dnsRecord) {
	Cdns.Add(qname, record, DNS_CACHE_INTERVAL*time.Second)
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

			if err != nil {
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

func responseRecord(w dns.ResponseWriter, req *dns.Msg, record dnsRecord) {
	// 修改id
	record.msg.Id = req.MsgHdr.Id

	// 修改ttl
	for _, a := range record.msg.Answer {
		if a.Header().Ttl < 600 {
			a.Header().Ttl = 600
		}
	}

	for _, a := range record.msg.Extra {
		if a.Header().Ttl < 600 {
			a.Header().Ttl = 600
		}
	}

	for _, a := range record.msg.Ns {
		if a.Header().Ttl < 600 {
			a.Header().Ttl = 600
		}
	}

	w.WriteMsg(record.msg)
	return
}

func inBlackIpList(ip net.IP) bool {
	str := ip.String()
	_, ok := BlackIpList[str]
	return ok
}

func dnsHandle(w dns.ResponseWriter, req *dns.Msg) {
	qname := req.Question[0].Name
	qtype, _ := dns.TypeToString[req.Question[0].Qtype]
	qclass, _ := dns.ClassToString[req.Question[0].Qclass]
	cacheKey := qname + qtype + qclass

	if record, ok := getRecord(cacheKey); ok {
		if record.Expire.After(time.Now()) {
			responseRecord(w, req, record)
			return
		}
	}

	servers := inDoorServers
	if inHost(qname) || ServerConfig.ForceRemote {
		servers = bypassServers
	}

	recvChan := make(chan *dns.Msg, 1)
	go mutilResolve(servers, req, recvChan)

	select {
	case msg := <-recvChan:
		if msg != nil {
			r := &dnsRecord{qname, msg, time.Now().Add(time.Duration(3600 * time.Second))}
			addRecord(cacheKey, *r)
			responseRecord(w, req, *r)
		}
	case <-time.After(DNS_TIMEOUT):
		if record, ok := getRecord(cacheKey); ok {
			responseRecord(w, req, record)
		}
		break
	}
}
