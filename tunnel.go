package main

import (
	"log"
	"net"
	"time"
)

const TUNNEL_TIMEOUT = 4 * time.Second

func tunnelClientServe(address string, dest string) {
	laddr, _ := net.ResolveUDPAddr("udp", address)
	daddr, _ := net.ResolveUDPAddr("udp", dest)

	sConn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		log.Println(err)
		return
	}
	defer sConn.Close()

	for {
		buff := make([]byte, 2048)
		n, addr, err := sConn.ReadFromUDP(buff)
		if err != nil {
			log.Println(err)
			continue
		}

		debug("handle new udp conn", addr)
		go tunnel(sConn, addr, daddr, buff, n)
	}
}

func tunnel(sConn *net.UDPConn, addr, dest *net.UDPAddr, buff []byte, n int) {
	rConn, err := net.DialUDP("udp", nil, dest)

	if err != nil {
		log.Println("udp connection fail", err)
		return
	}

	defer rConn.Close()

	entype(buff[:n])

	rConn.SetWriteDeadline(time.Now().Add(TUNNEL_TIMEOUT))
	_, err = rConn.Write(buff[:n])
	if err != nil {
		log.Println("udp remote write timeout", err)
		return
	}

	rConn.SetReadDeadline(time.Now().Add(TUNNEL_TIMEOUT))
	n, err = rConn.Read(buff)

	if err != nil {
		log.Println("udp remote read timeout", err)
		return
	}

	entype(buff[:n])
	sConn.WriteToUDP(buff[:n], addr)
}

func entype(buff []byte) {
	length := len(buff)
	passLength := len(TunnelPassword)

	for i := 0; i < length; i++ {
		buff[i] ^= TunnelPassword[i%passLength]
	}
}

func tunnelServerServe(address string, dest string) {
	laddr, _ := net.ResolveUDPAddr("udp", address)
	daddr, _ := net.ResolveUDPAddr("udp", dest)

	sConn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		log.Println(err)
		return
	}
	defer sConn.Close()

	for {
		buff := make([]byte, 2048)
		n, addr, err := sConn.ReadFromUDP(buff)

		if err != nil {
			log.Println(err)
			continue
		}

		go tunnel(sConn, addr, daddr, buff, n)
	}
}
