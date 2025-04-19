package evsim

import (
	"iter"
	"log"
	"runtime"
	"sync"
)

type coroutine struct {
	f       func()
	yieldFn func(bool) bool
	nextFn  func() (bool, bool)
	stopFn  func()
}

func newCoro() *coroutine {
	c := &coroutine{}
	next, stop := iter.Pull(c.entrypoint)
	c.nextFn = next
	c.stopFn = stop
	return c
}

func (c *coroutine) entrypoint(yield func(bool) bool) {
	c.yieldFn = yield
	for {
		func() {
			defer func() {
				if p := recover(); p != nil {
					if p != stopValue {
						var buf [4096]byte
						n := runtime.Stack(buf[:], false)
						log.Fatalf("unexpected panic\n%s\n%s", p, string(buf[:n]))
					}
				}
			}()
			c.f()
		}()
		c.yieldFn(false)
	}
}

func (c *coroutine) run(f func()) {
	c.f = f
}

func (c *coroutine) step() bool {
	more, _ := c.nextFn()
	return more
}

func (c *coroutine) yield() {
	c.yieldFn(true)
}

var (
	freeCoroutinesMu sync.Mutex
	freeCoroutines   []*coroutine
)

func allocCoroutine() *coroutine {
	freeCoroutinesMu.Lock()
	defer freeCoroutinesMu.Unlock()

	n := len(freeCoroutines)
	if n > 0 {
		c := freeCoroutines[n-1]
		freeCoroutines = freeCoroutines[:n-1]
		return c
	}
	return newCoro()
}

func freeCoroutine(c *coroutine) {
	freeCoroutinesMu.Lock()
	defer freeCoroutinesMu.Unlock()

	freeCoroutines = append(freeCoroutines, c)
}
