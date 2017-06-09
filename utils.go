package utils

import (
	"crypto/rand"
	"encoding/binary"
)

const defaultMethod = "aes-256-cfb"

func PutRandomBytes(b []byte) {
	binary.Read(rand.Reader, binary.BigEndian, b)
}

func GetRandomBytes(len int) []byte {
	if len <= 0 {
		return nil
	}
	data := make([]byte, len)
	PutRandomBytes(data)
	return data
}
