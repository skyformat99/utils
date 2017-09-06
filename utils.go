package utils

import (
	"crypto/rand"
	"encoding/binary"
	"reflect"
	"sync"
	"unsafe"
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

type ExitCleaner struct {
	lock   sync.Mutex
	runner []func()
}

func (c *ExitCleaner) Push(f func()) int {
	c.lock.Lock()
	defer c.lock.Unlock()
	n := len(c.runner)
	c.runner = append(c.runner, f)
	return n
}

func (c *ExitCleaner) Exit() {
	c.lock.Lock()
	defer c.lock.Unlock()
	for i := len(c.runner) - 1; i >= 0; i-- {
		f := c.runner[i]
		if f == nil {
			continue
		}
		f()
	}
}

func (c *ExitCleaner) Delete(index int) func() {
	c.lock.Lock()
	defer c.lock.Unlock()
	if index > len(c.runner) || index < 0 {
		return nil
	}
	f := c.runner[index]
	if index == 0 {
		c.runner = c.runner[1:]
	} else if index == len(c.runner)-1 {
		c.runner = c.runner[:index]
	} else {
		runner1 := c.runner[:index]
		runner2 := c.runner[index+1:]
		c.runner = append(runner1, runner2...)
	}
	return f
}

func SliceToString(b []byte) (s string) {
	pbytes := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	pstring := (*reflect.StringHeader)(unsafe.Pointer(&s))
	pstring.Data = pbytes.Data
	pstring.Len = pbytes.Len
	return
}

func StringToSlice(s string) (b []byte) {
	pbytes := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	pstring := (*reflect.StringHeader)(unsafe.Pointer(&s))
	pbytes.Data = pstring.Data
	pbytes.Len = pstring.Len
	pbytes.Cap = pstring.Len
	return
}
