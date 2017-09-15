package utils

import "net"

// Conn represents a net.Conn that implement WriteBuffers method
// WriteBuffers can write serveral buffers at a time
type Conn interface {
	net.Conn
	WriteBuffers([][]byte) (int, error)
}

// TCPConn is a *net.TCPConn that implement the interface Conn
type TCPConn struct {
	*net.TCPConn
}

// WriteBuffers can send serveral buffers at a time
func (conn *TCPConn) WriteBuffers(bufs [][]byte) (n int, err error) {
	buffers := net.Buffers(bufs)
	var n2 int64
	n2, err = buffers.WriteTo(conn.TCPConn)
	n = int(n2)
	return
}

// DialTCP calls net.DialTCP and returns *TCPConn
func DialTCP(network string, laddr, raddr *net.TCPAddr) (conn *TCPConn, err error) {
	netconn, err := net.DialTCP(network, laddr, raddr)
	if err == nil {
		conn = &TCPConn{TCPConn: netconn}
	}
	return
}
