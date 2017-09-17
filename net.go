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

// UDPServerCtx is the control centor of the udp server
type UDPServerCtx struct {
	Mtu     int
	Expires int

	once     sync.Once
	connsMap *sync.Map
	bufPool  *sync.Pool
	die      chan bool
}

func (ctx *UDPServerCtx) init() {
	ctx.once.Do(func() {
		ctx.die = make(chan bool)
		ctx.connsMap = &sync.Map{}
		ctx.bufPool = &sync.Pool{New: func() interface{} {
			return make([]byte, ctx.Mtu)
		}}
	})
}

func (ctx *UDPServerCtx) close() {
	close(ctx.die)
}

// RunUDPServer runs the udp server
func (ctx *UDPServerCtx) RunUDPServer(conn net.PacketConn, create func(*SubConn) (net.Conn, net.Conn, error)) {
	defer conn.Close()
	ctx.init()
	defer ctx.close()
	buf := make([]byte, ctx.Mtu)

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
		v, ok := ctx.connsMap.Load(addrstr)
		if !ok {
			subconn := newSubConn(conn, ctx, addr)
			ctx.connsMap.Store(addrstr, subconn)
			v = subconn
			go func(subconn *SubConn) {
				defer subconn.Close()
				c1, c2, err := create(subconn)
				if err != nil {
					return
				}
				defer c1.Close()
				defer c2.Close()
				PipeForUDPServer(c1, c2, ctx)
			}(subconn)
		}
		subconn := v.(*SubConn)
		subconn.input(b)
	}
}

// NewUDPListener simplely calls the net.ListenUDP and create a udp listener
func NewUDPListener(address string) (conn *net.UDPConn, err error) {
	laddr, err := net.ResolveUDPAddr("udp", address)
	if err == nil {
		conn, err = net.ListenUDP("udp", laddr)
	}
	return
}

// PipeForUDPServer is a simple pipe loop for udp server
func PipeForUDPServer(c1, c2 net.Conn, ctx *UDPServerCtx) {
	c1die := make(chan bool)
	c2die := make(chan bool)
	f := func(dst, src net.Conn, die chan bool) {
		defer close(die)
		var n int
		var err error
		buf := make([]byte, ctx.Mtu)
		for err == nil {
			src.SetReadDeadline(time.Now().Add(time.Second * time.Duration(ctx.Expires)))
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
