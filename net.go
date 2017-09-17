package utils

import (
	"log"
	"net"
	"sync"
	"time"
)

// AddrCtx carries a ctx inteface and can sotre some control message
type AddrCtx struct {
	net.Addr
	Ctx interface{}
}

func RunUDPServer(conn net.PacketConn, create func(*SubConn) (net.Conn, net.Conn, error), mtu int, expires int) {
	defer conn.Close()
	die := make(chan bool)
	defer close(die)
	connsMap := &sync.Map{}
	buf := make([]byte, mtu)

	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Println(conn.LocalAddr(), err)
			return
		}
		if addr == nil {
			continue
		}
		addrstr := addr.String()
		b := buf[:n]
		v, ok := connsMap.Load(addrstr)
		if !ok {
			subconn := newSubConn(conn, die, connsMap, addr)
			connsMap.Store(addrstr, subconn)
			v = subconn
			go func(subconn *SubConn) {
				defer subconn.Close()
				c1, c2, err := create(subconn)
				if err != nil {
					return
				}
				defer c1.Close()
				defer c2.Close()
				PipeForUDPServer(c1, c2, mtu, expires)
			}(subconn)
		}
		subconn := v.(*SubConn)
		subconn.input(b)
	}
}

func NewUDPListener(address string) (conn *net.UDPConn, err error) {
	laddr, err := net.ResolveUDPAddr("udp", address)
	if err == nil {
		conn, err = net.ListenUDP("udp", laddr)
	}
	return
}

func PipeForUDPServer(c1, c2 net.Conn, mtu int, expires int) {
	c1die := make(chan bool)
	c2die := make(chan bool)
	f := func(dst, src net.Conn, die chan bool) {
		defer close(die)
		var n int
		var err error
		buf := make([]byte, mtu)
		for err == nil {
			src.SetReadDeadline(time.Now().Add(time.Second * time.Duration(expires)))
			n, err = src.Read(buf)
			if n > 0 || err == nil {
				_, err = dst.Write(buf[:n])
			}
		}
	}
	go f(c1, c2, c1die)
	go f(c2, c1, c2die)
	select {
	case <-c1die:
	case <-c2die:
	}
}
