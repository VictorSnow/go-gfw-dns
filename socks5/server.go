package socks5

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

type Server struct {
	ListenAddr    string                   // 监听地址
	ProxyAddr     string                   // 远程转发地址
	ResolveAddr   func(addr string) string // 解析地址 hostname -> ip
	ResolveIpList func(ip string) bool     // ip是否在黑名单
}

func ServerStart(address string) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Panicln("socks5 服务器启动错误", err)
	}

	for {
		c, err := l.Accept()
		if err != nil {
			continue
		}
		go ServerHandleConnect(c)
	}
}

func ServerHandleConnect(c net.Conn) {
	defer c.Close()

	nodelay(c)

	r := bufio.NewReader(c)
	// ver
	r.ReadByte()
	// nmethods
	nmethods, _ := r.ReadByte()
	if nmethods > 0 {
		buff := make([]byte, nmethods)
		r.Read(buff)
	}

	c.Write([]byte{5, 0})

	// ver
	r.ReadByte()
	// cmd
	r.ReadByte()
	// rsv
	r.ReadByte()
	// atyp
	atyp, _ := r.ReadByte()

	port := 0
	ipaddr := ""

	switch int(atyp) {
	case 1: // ipv4
		ip := make([]byte, 4)
		n, err := r.Read(ip)
		if n != 4 {
			log.Println("ip地址错误", ip, err)
			return
		}
		// 读取端口
		l, _ := r.ReadByte()
		h, _ := r.ReadByte()
		port = int(l)<<8 + int(h)
		// 地址信息
		ipaddr = fmt.Sprintf("%d.%d.%d.%d:%d", ip[0], ip[1], ip[2], ip[3], port)
	case 3: // domain
		length, _ := r.ReadByte()
		buff := make([]byte, length)
		n, _ := r.Read(buff)
		if length != byte(n) {
			log.Println("解析domain错误", buff)
			return
		}

		// 读取端口
		l, _ := r.ReadByte()
		h, _ := r.ReadByte()
		port = int(l)<<8 + int(h)
		ipaddr = string(buff[:n]) + ":" + strconv.Itoa(port)
	case 4: // ipv6
		ip := make([]byte, net.IPv6len)
		n, _ := r.Read(ip)
		if n != net.IPv6len {
			return
		}
		// 读取端口
		l, _ := r.ReadByte()
		h, _ := r.ReadByte()
		port = int(l)<<8 + int(h)
		ipaddr = net.IP(ip).String() + ":" + strconv.Itoa(port)
	default:
		return
	}

	con, err := net.DialTimeout("tcp", ipaddr, 5*time.Second)
	if err != nil {
		log.Println("链接到远端错误", ipaddr, err)
		return
	}
	defer con.Close()
	nodelay(con)

	// 通知客户端 socks5链接成功
	c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})

	// 如果有buff在
	n := r.Buffered()
	if n > 0 {
		buff := make([]byte, n)
		n, _ = r.Read(buff[:n])
		con.Write(buff[:n])
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	closed := false

	flipIo := func(src, dest net.Conn) {
		defer func() {
			wg.Done()
			closed = true
		}()

		buff := make([]byte, 8192)
		for !closed {
			src.SetReadDeadline(time.Now().Add(5 * time.Second))
			n, err := src.Read(buff)

			if istimeout(err) {
				continue
			} else if err != nil {
				break
			}

			if n > 0 {
			retry:
				dest.SetWriteDeadline(time.Now().Add(5 * time.Second))
				m, err := dest.Write(buff[:n])
				if m != n {
					// 写入不完全
					break
				}

				if istimeout(err) {
					// 超时重发
					goto retry
				} else if err != nil {
					break
				}
			}
		}
	}

	go flipIo(c, con)
	go flipIo(con, c)

	wg.Wait()
}
