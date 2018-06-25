package main

import (
	"context"
	"flag"
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
	ctx     context.Context
	uaddr   *net.UDPAddr
	doneUDP chan bool
	id      uint16
}

func main() {
	var (
		ip = flag.String("ip", "", "node ip")
		id = flag.Int("id", 0, "node id")
	)
	flag.Parse()
	n := &node{}
	n.id = uint16(*id)
	// run this for 120 seconds
	n.ctx = context.TODO()
	uaddr, err := net.ResolveUDPAddr(UDPVersion, MulticastAddr)
	if err != nil {
		log.Fatalf("invalid udp addr: %s", err)
	}
	n.uaddr = uaddr
	n.doneUDP = make(chan bool, 1)
	go n.advertiseUDP()

	laddr, err := net.ResolveTCPAddr(TCPVersion, net.JoinHostPort(*ip, "3000"))
	if err != nil {
		log.Fatalf("invalid tcp addr: %v", err)
	}
	ln, err := net.ListenTCP(TCPVersion, laddr)
	if err != nil {
		log.Fatalf("failed to announce on local TCP network: %s", err)
	}

	go func() {
		for {
			tconn, err := ln.AcceptTCP()
			if err != nil {
				log.Printf("failed to accept tcp connection: %s", err)
				continue
			}
			n.doneUDP <- true
			// listen blocks until connection is closed
			// or context is done.
			n.listen(tconn)
			go n.advertiseUDP()
		}
	}()
	<-n.ctx.Done()
	ln.Close()
}

func (n *node) listen(tconn *net.TCPConn) {
	ticker := time.NewTicker(KeepAlivePeriod * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			b, err := qsy.NewPacket(qsy.KeepAliveT, n.id, qsy.NoColor, uint32(0), uint16(1), false, false).Encode()
			if err != nil {
				log.Printf("failed to encode packet, closing conn: %s", err)
				tconn.Close()
				break
			}
			if _, err := tconn.Write(b); err != nil {
				log.Printf("failed to write tconn: %v", err)
				tconn.Close()
				ticker.Stop()
				return
			}
			log.Printf("sent keep alive packet")
		case <-n.ctx.Done():
			log.Printf("closing tconn")
			tconn.Close()
			ticker.Stop()
			return
		}
	}
}

func (n *node) advertiseUDP() {
	c, err := net.DialUDP(UDPVersion, nil, n.uaddr)
	if err != nil {
		log.Fatalf("failed to listen UDP: %v", err)
	}
	ticker := time.NewTicker(HelloPeriod * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			b, err := qsy.NewPacket(qsy.HelloT, n.id, qsy.NoColor, uint32(0), uint16(1), false, false).Encode()
			if err != nil {
				log.Printf("failed to encode packet: %s", err)
				break
			}
			c.Write(b)
			log.Printf("sent hello packet")
		case <-n.doneUDP:
			ticker.Stop()
			c.Close()
			return
		case <-n.ctx.Done():
			log.Printf("closing udp conn")
			ticker.Stop()
			c.Close()
			return
		}
	}
}
