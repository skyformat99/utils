package utils

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// Conn represents a net.Conn that implement WriteBuffers method
// WriteBuffers can write serveral buffers at a time
type Conn interface {
	net.Conn
	WriteBuffers([][]byte) (int, error)
}

// UtilsConn is a net.Conn that implement the interface Conn
type UtilsConn struct {
	net.Conn
}

// GetTCPConn try to get the underlying TCPConn
func (conn *UtilsConn) GetTCPConn() (t *net.TCPConn, ok bool) {
	t, ok = conn.Conn.(*net.TCPConn)
	return
}

// WriteBuffers can send serveral buffers at a time
func (conn *UtilsConn) WriteBuffers(bufs [][]byte) (n int, err error) {
	buffers := net.Buffers(bufs)
	var n2 int64
	n2, err = buffers.WriteTo(conn.Conn)
	n = int(n2)
	return
}

// DialTCP calls net.DialTCP and returns *UtilsConn
func DialTCP(network string, laddr, raddr *net.TCPAddr) (conn *UtilsConn, err error) {
	netconn, err := net.DialTCP(network, laddr, raddr)
	if err == nil {
		conn = &UtilsConn{Conn: netconn}
	}
	return
}

// NewConn returns *UtilsConn from net.Conn
func NewConn(conn net.Conn) *UtilsConn {
	return &UtilsConn{Conn: conn}
}

type tmpBuf struct {
	buf []byte
	off int
}

// SubConn is the child connection of a net.PacketConn
type SubConn struct {
	die     chan bool
	pdie    chan bool
	lock    sync.Mutex
	sigch   chan int
	rbuf    []byte
	tmpbufs []tmpBuf
	net.PacketConn
	connsMap *sync.Map
	bufPool  *sync.Pool
	raddr    net.Addr
	rtime    time.Time
}

func newSubConn(c net.PacketConn, ctx *UDPServerCtx, raddr net.Addr) *SubConn {
	return &SubConn{
		die:        make(chan bool),
		pdie:       ctx.die,
		sigch:      make(chan int),
		PacketConn: c,
		connsMap:   ctx.connsMap,
		bufPool:    ctx.bufPool,
		raddr:      raddr,
	}
}

func (conn *SubConn) input(b []byte) {
	conn.lock.Lock()
	defer conn.lock.Unlock()
	var n int
	if conn.rbuf != nil {
		n = copy(conn.rbuf, b)
		conn.rbuf = nil
		conn.sigch <- n
		return
	}
	var tmpbuf tmpBuf
	tmpbuf.buf = conn.bufPool.Get().([]byte)
	tmpbuf.off = copy(tmpbuf.buf, b)
	conn.tmpbufs = append(conn.tmpbufs, tmpbuf)
}

// Close close the connection and delete it from connsMap
func (conn *SubConn) Close() error {
	conn.lock.Lock()
	defer conn.lock.Unlock()
	select {
	case <-conn.die:
	default:
		close(conn.die)
	}
	if conn.connsMap != nil && conn.raddr != nil {
		conn.connsMap.Delete(conn.raddr.String())
	}
	if len(conn.tmpbufs) != 0 {
		for _, tmpbuf := range conn.tmpbufs {
			conn.bufPool.Put(tmpbuf.buf)
		}
		conn.tmpbufs = nil
	}
	return nil
}

// RemoteAddr return the address of peer
func (conn *SubConn) RemoteAddr() net.Addr {
	return conn.raddr
}

func (conn *SubConn) Read(b []byte) (n int, err error) {
	conn.lock.Lock()
	if len(conn.tmpbufs) != 0 {
		tmpbuf := conn.tmpbufs[0]
		conn.tmpbufs = conn.tmpbufs[1:]
		n = copy(b, tmpbuf.buf[:tmpbuf.off])
		conn.lock.Unlock()
		conn.bufPool.Put(tmpbuf.buf)
		return
	}
	conn.rbuf = b
	conn.lock.Unlock()
	var rtch <-chan time.Time
	now := time.Now()
	if conn.rtime.Equal(time.Time{}) {
		if now.After(conn.rtime) {
			err = fmt.Errorf("timeout")
			return
		}
		rtimer := time.NewTimer(conn.rtime.Sub(now))
		rtch = rtimer.C
		defer rtimer.Stop()
	}
	select {
	case <-rtch:
		err = fmt.Errorf("timeout")
		return
	case <-conn.die:
		err = fmt.Errorf("closed connection")
		return
	case <-conn.pdie:
		err = fmt.Errorf("closed PacketConn")
		return
	case n = <-conn.sigch:
	}
	return
}

func (conn *SubConn) Write(b []byte) (n int, err error) {
	return conn.PacketConn.WriteTo(b, conn.raddr)
}

// SetReadDeadline set the dealdine of read
func (conn *SubConn) SetReadDeadline(t time.Time) error {
	conn.lock.Lock()
	defer conn.lock.Unlock()
	conn.rtime = t
	return nil
}
