package utils

import (
	"crypto/rand"
	"encoding/binary"
	"reflect"
	"strings"
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

var domainNodePool = &sync.Pool{New: func() interface{} {
	return &domainNode{
		nodes: make(map[string]*domainNode),
	}
}}

// DomainRoot is a tree
type DomainRoot struct {
	nodes     map[string]*domainNode
	nodesPool sync.Pool
}

func NewDomainRoot() *DomainRoot {
	return &DomainRoot{
		nodes: make(map[string]*domainNode),
	}
}

func reverse(ss []string) {
	last := len(ss) - 1
	for i := 0; i < len(ss)/2; i++ {
		ss[i], ss[last-i] = ss[last-i], ss[i]
	}
}

func (root *DomainRoot) Put(host string) {
	domains := strings.Split(host, ".")
	if len(domains) < 2 {
		return
	}
	reverse(domains)
	nodes := root.nodes
	var depth int
	for _, domain := range domains {
		if len(domain) == 0 {
			continue
		}
		depth++
		v, ok := nodes[domain]
		if ok {
			if v.fold {
				break
			} else if len(v.nodes) > 10 && depth > 1 {
				v.fold = true
				v.nodes = nil
				break
			}
			nodes = v.nodes
			continue
		}
		v = &domainNode{
			depth:  depth,
			domain: domain,
			nodes:  make(map[string]*(domainNode)),
		}
		nodes[domain] = v
		nodes = v.nodes
	}
}

func (root *DomainRoot) Test(host string) bool {
	domains := strings.Split(host, ".")
	if len(domains) < 2 {
		return false
	}
	reverse(domains)
	nodes := root.nodes
	depth := 0
	for _, domain := range domains {
		if len(domain) == 0 {
			continue
		}
		v, ok := nodes[domain]
		if !ok {
			return false
		}
		if v.fold {
			return true
		}
		depth++
		nodes = v.nodes
	}
	if len(domains) == depth {
		return true
	}
	return false
}

func (root *DomainRoot) Get() (foldHosts []string, hosts []string) {
	var domains []string
	var f func(map[string]*domainNode, bool)
	f = func(nodes map[string]*domainNode, fold bool) {
		for _, node := range nodes {
			domains = append([]string{node.domain}, domains...)
			f(node.nodes, node.fold)
			domains = domains[1:]
		}
		if len(nodes) == 0 {
			host := strings.Join(domains, ".")
			if fold {
				foldHosts = append(foldHosts, host)
			} else {
				hosts = append(hosts, host)
			}
		}
	}

	f(root.nodes, false)

	return 
}

type domainNode struct {
	domain string
	depth  int
	fold   bool
	nodes  map[string]*domainNode
}
