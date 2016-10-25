package main

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const DNS_TIMEOUT = 2 * time.Second

type dnsRecord struct {
	name   string
	ip     net.IP
	expire time.Time
}

type dnsCache struct {
	items map[string]*dnsRecord
	lock  sync.Mutex
}

var DnsCache dnsCache
var bypassServers []string
var inDoorServers []string

func ListenAndServe(address string, inDoor []string, byPass []string) {
	inDoorServers = inDoor
	bypassServers = byPass

	DnsCache = dnsCache{make(map[string]*dnsRecord), sync.Mutex{}}

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

	// 清理过期的记录
	go func() {
		for {
			DnsCache.lock.Lock()
			for k, v := range DnsCache.items {
				if v.expire.Before(time.Now()) {
					delete(DnsCache.items, k)
				}
			}
			DnsCache.lock.Unlock()
			time.Sleep(120 * time.Second)
		}
	}()
	err := udpServer.ListenAndServe()
	if err != nil {
		log.Println("服务错误", err)
	}
}

func removeRecord(qname string) {
	DnsCache.lock.Lock()
	defer DnsCache.lock.Unlock()
	delete(DnsCache.items, qname)
}

func getRecord(qname string) *dnsRecord {
	if r, ok := DnsCache.items[qname]; ok {
		if r.expire.After(time.Now()) {
			return r
		} else {
			DnsCache.lock.Lock()
			defer DnsCache.lock.Unlock()
			delete(DnsCache.items, qname)
		}
	}
	return nil
}

func addRecord(qname string, record *dnsRecord) {
	DnsCache.lock.Lock()
	defer DnsCache.lock.Unlock()

	DnsCache.items[qname] = record
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
				expire := time.Now().Add(time.Duration(a.Hdr.Ttl) * time.Second)
				return &dnsRecord{qname, ip, expire}, nil
			}
		}
	}
	return nil, errors.New("ipv4地址错误")
}

func doResolve(server string, req *dns.Msg, recvChan chan<- *dnsRecord, wg *sync.WaitGroup) {
	defer wg.Done()

	r, _ := resolve(server, req)
	// try send or ignore
	if r != nil {
		select {
		case recvChan <- r:
		default:
		}
	}
}

func responseRecord(w dns.ResponseWriter, req *dns.Msg, record *dnsRecord) {
	ip := record.ip.To4()

	a := &dns.A{
		Hdr: dns.RR_Header{
			Name:     record.name,
			Rrtype:   dns.TypeA,
			Class:    dns.ClassINET,
			Rdlength: uint16(len(ip)),
			Ttl:      uint32(record.expire.Sub(time.Now()).Seconds()),
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

	// only handle  A record and hit cache
	if req.Question[0].Qtype == dns.TypeA {
		if record := getRecord(qname); record != nil {
			responseRecord(w, req, record)
		}
	}

	recvChan := make(chan *dnsRecord, 1)
	defer close(recvChan)

	var wg = &sync.WaitGroup{}

	servers := inDoorServers
	if inHost(qname) {
		servers = bypassServers
	}

	for _, server := range servers {
		wg.Add(1)
		go doResolve(server, req, recvChan, wg)
	}

	select {
	case r := <-recvChan:
		addRecord(qname, r)
		responseRecord(w, req, r)
	case <-time.After(DNS_TIMEOUT):
		break
	}

	// do we need to wait all go routines exit ???
	wg.Wait()
}
