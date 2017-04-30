package utils

import "net"

type Conn interface {
	net.Conn
	WriteBuffers([][]byte) (int, error)
}
