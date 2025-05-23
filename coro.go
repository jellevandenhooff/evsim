package evsim

import (
	"iter"
	"log"
	"runtime"
)

type coroutine[T any] struct {
	abort bool
	f     func(T)
	arg   T

	yieldFn func(bool) bool
	nextFn  func() (bool, bool)
	stopFn  func()
}

func newCoroutine[T any]() *coroutine[T] {
	c := &coroutine[T]{}
	next, stop := iter.Pull(c.entrypoint)
	c.nextFn = next
	c.stopFn = stop
	return c
}

func (c *coroutine[T]) entrypoint(yield func(bool) bool) {
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
			f, arg := c.f, c.arg
			var emptyT T
			c.f, c.arg = nil, emptyT
			f(arg)
		}()
		c.yieldFn(false)
	}
}

func (c *coroutine[T]) run(f func(T), arg T) {
	c.f = f
	c.arg = arg
}

func (c *coroutine[T]) step() bool {
	more, _ := c.nextFn()
	return more
}

func (c *coroutine[T]) stop() {
	if c.f != nil {
		var emptyT T
		c.f, c.arg = nil, emptyT
	} else {
		c.abort = true
		more, _ := c.nextFn()
		if more != false {
			panic("did not abort")
		}
		c.abort = false
	}
}

func (c *coroutine[T]) yield() {
	c.yieldFn(true)
	if c.abort {
		panic(stopValue)
	}
}
