package main

import (
	"log"
	"net"
	"sync"
	"time"
)

const TUNNEL_TIMEOUT = 2 * time.Second

func tunnelClientServe(address string, dest string) {
	l, e := net.Listen("udp", address)

	if e != nil {
		log.Println(e)
		return
	}

	for {
		n, _ := l.Accept()
		go tunnelClientHandle(n, dest)
	}
}

func flip(src, dest net.Conn, wg *sync.WaitGroup) {
	buff := make([]byte, 2048)
	for {
		src.SetReadDeadline(time.Now().Add(TUNNEL_TIMEOUT))
		n, err := src.Read(buff)

		for i := 0; i < n; i++ {
			buff[i] ^= 0x71
		}

		if n > 0 {
			dest.SetWriteDeadline(time.Now().Add(TUNNEL_TIMEOUT))
			_, err2 := dest.Write(buff[:n])
			if err2 != nil {
				break
			}
		}

		if err != nil {
			break
		}
	}
	wg.Done()
}

func tunnelClientHandle(conn net.Conn, dest string) {
	remoteConn, err := net.DialTimeout("udp", dest, TUNNEL_TIMEOUT)
	// ignore
	if err != nil {
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go flip(conn, remoteConn, wg)
	go flip(remoteConn, conn, wg)
	wg.Wait()
}

func tunnelServerServe(address string, dest string) {
	l, e := net.Listen("udp", address)

	if e != nil {
		log.Println(e)
		return
	}

	for {
		n, _ := l.Accept()
		go tunnelClientHandle(n, dest)
	}
}
