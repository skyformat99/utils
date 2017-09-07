package utils

import (
	"sync"
	"time"
)

type AsyncRunner struct {
	once  sync.Once
	funcs chan func()
}

func (a *AsyncRunner) worker() {
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
	default:
		go a.worker()
	}
}
