package utils

import (
	"sync"
	"sync/atomic"
	"time"
)

type AsyncRunner struct {
	once    sync.Once
	funcs   chan func()
	workers int32
}

func (a *AsyncRunner) worker() {
	atomic.AddInt32(&a.workers, 1)
	defer atomic.AddInt32(&a.workers, -1)
	for {
		select {
		case f := <-a.funcs:
			if f != nil {
				f()
			}
		case <-time.After(time.Second * 5):
			return
		}
	}
}

func (a *AsyncRunner) Run(f func()) {
	a.once.Do(func() {
		a.funcs = make(chan func(), 16)
	})
	select {
	case a.funcs <- f:
		if atomic.LoadInt32(&a.workers) == 0 {
			go a.worker()
		}
	default:
		go a.worker()
		a.funcs <- f
	}
}
