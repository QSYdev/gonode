package main

import (
	"log"
	"net"
	"time"

	"github.com/qsydev/goterm/pkg/qsy"
)

const (
	MulticastAddr   = "224.0.0.12:3000"
	UDPVersion      = "udp4"
	TCPVersion      = "tcp4"
	HelloPeriod     = 500
	KeepAlivePeriod = 500
)

type node struct {
	uaddr   *net.UDPAddr
	ticker  *time.Ticker
	doneUDP chan bool
	i       *net.Interface
}

func main() {
	n := &node{}
	uaddr, err := net.ResolveUDPAddr(UDPVersion, MulticastAddr)
	if err != nil {
		log.Fatalf("invalid udp addr: %s", err)
	}
	i, err := net.InterfaceByName("en0")
	if err != nil {
		log.Fatalf("invalid network interface: %s", err)
	}
	n.i = i
	n.uaddr = uaddr
	n.doneUDP = make(chan bool, 1)
	go n.advertiseUDP()

	laddr, err := net.ResolveTCPAddr(TCPVersion, "10.0.0.2:3000")
	if err != nil {
		log.Fatalf("invalid tcp addr: %v", err)
	}
	ln, err := net.ListenTCP(TCPVersion, laddr)
	if err != nil {
		log.Fatalf("failed to announce on local TCP network: %s", err)
	}
	for {
		tconn, err := ln.Accept()
		if err != nil {
			log.Fatalf("failed to accept tcp connection: %s", err)
		}
		n.doneUDP <- true
		for {
			b, err := qsy.NewPacket(qsy.KeepAliveT, uint16(1), "", uint32(0), uint16(1), false, false).Encode()
			if err != nil {
				log.Printf("failed to encode packet, closing conn: %s", err)
				tconn.Close()
				break
			}
			tconn.Write(b)
			log.Printf("sent keep alive packet")
			time.Sleep(KeepAlivePeriod * time.Millisecond)
		}
		go n.advertiseUDP()
	}
}

func (n *node) advertiseUDP() {
	c, err := net.ListenMulticastUDP("udp4", n.i, n.uaddr)
	if err != nil {
		log.Fatalf("failed to listen UDP: %v", err)
	}
	n.ticker = time.NewTicker(HelloPeriod * time.Millisecond)
	for {
		select {
		case <-n.ticker.C:
			b, err := qsy.NewPacket(qsy.HelloT, uint16(1), "", uint32(0), uint16(1), false, false).Encode()
			if err != nil {
				log.Printf("failed to encode packet: %s", err)
				break
			}
			c.WriteTo(b, n.uaddr)
			log.Printf("sent hello packet")
		case <-n.doneUDP:
			n.ticker.Stop()
			return
		}
	}
}
