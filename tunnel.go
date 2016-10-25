package main

import (
	"log"
	"net"
	"time"
)

const TUNNEL_TIMEOUT = 2 * time.Second

func tunnelClientServe(address string, dest string) {
	laddr, _ := net.ResolveUDPAddr("udp", address)
	daddr, _ := net.ResolveUDPAddr("udp", dest)

	sConn, _ := net.ListenUDP("udp", laddr)

	for {
		buff := make([]byte, 2048)
		n, addr, _ := sConn.ReadFromUDP(buff)

		go tunnel(sConn, addr, daddr, buff, n)
	}
}

func tunnel(sConn *net.UDPConn, addr, dest *net.UDPAddr, buff []byte, n int) {
	rConn, err := net.DialUDP("udp", nil, dest)
	defer rConn.Close()

	if err != nil {
		log.Println(err)
		return
	}

	entype(buff[:n])

	_, err = rConn.Write(buff[:n])
	if err != nil {
		log.Println(err)
		return
	}

	n, err = rConn.Read(buff)

	entype(buff[:n])

	if err != nil {
		log.Println(err)
		return
	}
	sConn.WriteToUDP(buff[:n], addr)
}

func entype(buff []byte) {
	length := len(buff)
	for i := 0; i < length; i++ {
		buff[i] ^= 0x59
	}
}

func tunnelServerServe(address string, dest string) {
	laddr, _ := net.ResolveUDPAddr("udp", address)
	daddr, _ := net.ResolveUDPAddr("udp", dest)

	sConn, _ := net.ListenUDP("udp", laddr)

	for {
		buff := make([]byte, 2048)
		n, addr, _ := sConn.ReadFromUDP(buff)

		go tunnel(sConn, addr, daddr, buff, n)
	}
}
