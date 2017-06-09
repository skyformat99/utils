package utils

import (
	"sync"
	"time"
)

type AsyncRunner struct {
	lock    sync.Mutex
	started bool
	funcs   chan func()
}

func (a *AsyncRunner) worker() {
	defer func() {
		a.lock.Lock()
		a.started = false
		a.lock.Unlock()
	}()
	for {
		select {
		case f := <-a.funcs:
			if f != nil {
				f()
			}
		case <-time.After(time.Second * 15):
			return
		}
	}
}

func (a *AsyncRunner) Run(f func()) {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.funcs == nil {
		a.funcs = make(chan func(), 1024)
	}
	if !a.started {
		a.started = true
		go a.worker()
	}
	select {
	case a.funcs <- f:
	case <-time.After(time.Millisecond * 10):
		go f()
	}
}
