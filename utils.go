package utils

import (
	"crypto/rand"
	"encoding/binary"
	"reflect"
	"sync"
	"time"
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
	once   sync.Once
}

func (c *ExitCleaner) Push(f func()) int {
	c.lock.Lock()
	defer c.lock.Unlock()
	n := len(c.runner)
	c.runner = append(c.runner, f)
	return n
}

func (c *ExitCleaner) Exit() {
	flag := true
	c.once.Do(func() { flag = false })
	if flag {
		return
	}
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

type Die struct {
	RWLock
	ch  chan bool
	die bool
}

func (d *Die) Ch() (ch <-chan bool) {
	d.RunInLock(func() {
		if d.ch == nil {
			d.ch = make(chan bool)
		}
		ch = d.ch
	})
	return
}

func (d *Die) Die(f func()) {
	var die bool

	d.RunInLock(func() {
		die = d.die
		if !d.die {
			d.die = true
		}
		if d.ch == nil {
			d.ch = make(chan bool)
		}
	})

	if die {
		return
	}

	close(d.ch)

	if f != nil {
		f()
	}
}

func (d *Die) IsDead() (dead bool) {
	d.RunInRLock(func() {
		dead = d.die
	})
	return
}

type Lock struct {
	sync.Mutex
}

func (l *Lock) RunInLock(f func()) {
	l.Lock()
	defer l.Unlock()
	if f != nil {
		f()
	}
}

type RWLock struct {
	sync.RWMutex
}

func (l *RWLock) RunInLock(f func()) {
	l.Lock()
	defer l.Unlock()
	if f != nil {
		f()
	}
}

func (l *RWLock) RunInRLock(f func()) {
	l.RLock()
	defer l.RUnlock()
	if f != nil {
		f()
	}
}

type Locker interface {
	RunInLock(f func())
}

type RWLocker interface {
	Locker
	RunInRLock(f func())
}

type Expires struct {
	E time.Time
	L RWLock
}

func (e *Expires) isExpired() bool {
	return time.Now().After(e.E)
}

func (e *Expires) IsExpired() bool {
	e.L.RLock()
	defer e.L.RUnlock()
	return e.isExpired()
}

func (e *Expires) update(d time.Duration) {
	e.E = time.Now().Add(d)
}

func (e *Expires) Update(d time.Duration) {
	e.L.Lock()
	e.update(d)
	e.L.Unlock()
}

func (e *Expires) IsExpiredAndUpdate(d time.Duration) bool {
	expired := e.IsExpired()
	if !expired {
		return expired
	}
	e.L.Lock()
	defer e.L.Unlock()
	expired = e.isExpired()
	if expired {
		e.update(d)
		return true
	}
	return false
}
