package evsim

import "sync"

type Pool[T any] struct {
	available []T
	new       func() T
	reset     func(T)
}

func (p *Pool[T]) Get() T {
	n := len(p.available) - 1
	if n >= 0 {
		x := p.available[n]
		p.available = p.available[:n]
		return x
	}
	return p.new()
}

func (p *Pool[T]) Put(x T) {
	if p.reset != nil {
		p.reset(x)
	}
	p.available = append(p.available, x)
}

type SyncPool[T any] struct {
	mu        sync.Mutex
	pool      Pool[T]
	available []T
	new       func() T
	reset     func(T)
}

func (p *SyncPool[T]) Get() T {
	p.mu.Lock()
	n := len(p.available) - 1
	if n >= 0 {
		x := p.available[n]
		p.available = p.available[:n]
		p.mu.Unlock()
		return x
	}
	p.mu.Unlock()
	return p.new()
}

func (p *SyncPool[T]) Put(x T) {
	if p.reset != nil {
		p.reset(x)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.available = append(p.available, x)
}
