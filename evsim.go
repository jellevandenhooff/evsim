package evsim

import (
	"container/heap"
	"math/rand"
)

var stopValue = new(int)

type timer struct {
	at          float64
	thenUnpause *Process
}

type timers []timer

func (t timers) Len() int {
	return len(t)
}

func (t timers) Less(i, j int) bool {
	return t[i].at < t[j].at
}

func (t timers) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t *timers) Push(x any) {
	panic("no")
}

func (t *timers) push(x timer) {
	*t = append(*t, x)
	heap.Fix(t, len(*t)-1)
}

func (t *timers) Pop() any {
	panic("no")
}

func (t *timers) pop() timer {
	x := (*t)[0]
	n := len(*t)
	(*t)[0] = (*t)[n-1]
	*t = (*t)[:n-1]
	heap.Fix(t, 0)
	return x
}

type Simulation struct {
	stopped bool

	now float64

	current *Process

	timers   timers
	runnable []*Process
	all      []*Process

	processPool Pool[*Process]
}

var SimulationsPool = SyncPool[*Simulation]{
	new:   NewSimulation,
	reset: (*Simulation).reset,
}

func (s *Simulation) reset() {
	s.stopped = false
	s.now = 0.0
	s.current = nil
	s.timers = s.timers[:0]
	s.runnable = s.runnable[:0]
}

func NewSimulation() *Simulation {
	return &Simulation{
		now: 0.0,
		processPool: Pool[*Process]{
			new:   newProcess,
			reset: (*Process).reset,
		},
	}
}

func intrusiveAppend[T any](list *[]T, idx func(elem T) *int, elem T) {
	n := len(*list)
	*list = append(*list, elem)
	i := *idx(elem)
	if i != -1 {
		panic("bad index")
	}
	*idx(elem) = n
}

func intrusiveRemove[T any](list *[]T, idx func(elem T) *int, elem T) {
	a := *idx(elem)
	if a == -1 {
		panic("bad index")
	}
	b := len(*list) - 1
	if a != b {
		other := (*list)[b]
		(*list)[a] = other
		*idx(other) = a
	}
	*list = (*list)[:b]
	*idx(elem) = -1
}

type Process struct {
	simulation               *Simulation
	allIdx, runnableIdx      int
	waitingPrev, waitingNext *Process
	coroutine                *coroutine[*Process]

	pendingItem any
}

func (p *Process) runnableIdxPtr() *int {
	return &p.runnableIdx
}

func (p *Process) allIdxPtr() *int {
	return &p.allIdx
}

func newProcess() *Process {
	p := &Process{
		coroutine: newCoroutine[*Process](),
	}
	p.reset()
	return p
}

func (p *Process) reset() {
	p.simulation = nil
	p.allIdx = -1
	p.runnableIdx = -1
	p.waitingPrev = nil
	p.waitingNext = nil
}

func (p *Process) step() {
	ok := p.coroutine.step()
	if !ok {
		p.simulation.removeAll(p)
		p.simulation.removeRunnable(p)
		p.simulation.processPool.Put(p)
	}
}

func (p *Process) yield() {
	if p.simulation.current != p {
		panic("help")
	}
	p.coroutine.yield()
}

func (p *Process) Simulation() *Simulation {
	return p.simulation
}

func (p *Process) Sleep(duration float64) {
	if duration <= 0 {
		return
	}
	s := p.simulation
	s.addTimer(s.now+duration, p)
	s.removeRunnable(p)
	p.yield()
}

type waitingList struct {
	first, last *Process
}

func (w *waitingList) addLast(p *Process) {
	p.waitingPrev = w.last
	if w.last != nil {
		w.last.waitingNext = p
	} else {
		w.first = p
	}
	w.last = p
}

func (w *waitingList) empty() bool {
	return w.first == nil
}

func (w *waitingList) removeFirst() *Process {
	p := w.first
	w.first = p.waitingNext
	p.waitingNext = nil
	if w.first != nil {
		w.first.waitingPrev = nil
	} else {
		w.last = nil
	}
	return p
}

func (s *Simulation) addTimer(at float64, thenUnpause *Process) {
	if at <= s.now {
		panic("bad time")
	}

	s.timers.push(timer{
		at:          at,
		thenUnpause: thenUnpause,
	})
}

func (s *Simulation) addRunnable(p *Process) {
	intrusiveAppend(&s.runnable, (*Process).runnableIdxPtr, p)
}

func (s *Simulation) removeRunnable(p *Process) {
	intrusiveRemove(&s.runnable, (*Process).runnableIdxPtr, p)
}

func (s *Simulation) addAll(p *Process) {
	intrusiveAppend(&s.all, (*Process).allIdxPtr, p)
}

func (s *Simulation) removeAll(p *Process) {
	intrusiveRemove(&s.all, (*Process).allIdxPtr, p)
}

func (s *Simulation) run() {
	for !s.stopped {
		if len(s.runnable) == 0 {
			if len(s.timers) > 0 {
				next := s.timers.pop()
				if next.at < s.now {
					panic("huh")
				}
				if next.at > s.now {
					// log.Printf("advancing time to %s", s.now.Format(time.TimeOnly))
					s.now = next.at
				}
				s.addRunnable(next.thenUnpause)
				for len(s.timers) > 0 && s.timers[0].at == s.now {
					s.addRunnable(s.timers[0].thenUnpause)
					s.timers.pop()
				}
				continue
			}
			// log.Printf("nothing runnable anymore; stopping")
			break
		}

		idx := rand.Intn(len(s.runnable))
		p := s.runnable[idx]
		s.current = p
		p.step()
		s.current = nil
	}

	for len(s.all) > 0 {
		p := s.all[0]
		p.coroutine.stop()
		s.removeAll(p)
		if p.runnableIdx != -1 {
			s.removeRunnable(p)
		}
		s.processPool.Put(p)
	}
}

func (s *Simulation) Spawn(f func(p *Process)) {
	p := s.processPool.Get()
	p.simulation = s
	p.coroutine.run(f, p)
	s.addAll(p)
	s.addRunnable(p)
}

func (s *Simulation) Start() {
	s.run()
}

func (s *Simulation) Stop() {
	s.stopped = true
}

func (s *Simulation) Now() float64 {
	return s.now
}

type Event struct {
	triggered bool
	waiting   waitingList
}

func (s *Simulation) NewEvent() *Event {
	return &Event{
		triggered: false,
	}
}

func (e *Event) Trigger() {
	if e.triggered {
		return
	}
	e.triggered = true

	for p := e.waiting.first; p != nil; p = p.waitingNext {
		p.simulation.addRunnable(p)
		p.waitingPrev = nil
		p.waitingNext = nil
	}
	e.waiting = waitingList{}
}

func (e *Event) Wait(p *Process) {
	if e.triggered {
		return
	}

	p.simulation.removeRunnable(p)
	e.waiting.addLast(p)
	p.yield()
}

func (e *Event) Triggered() bool {
	return e.triggered
}

type Semaphore struct {
	simulation *Simulation

	available int

	limit int

	waiting waitingList
}

func NewSemaphore(s *Simulation, limit int) *Semaphore {
	return &Semaphore{
		simulation: s,
		available:  limit,
		limit:      limit,
	}
}

func (r *Semaphore) Limit() int {
	return r.limit
}

func (r *Semaphore) Acquire(p *Process) {
	if r.available > 0 {
		r.available -= 1
		return
	}

	r.simulation.removeRunnable(p)
	r.waiting.addLast(p)
	p.yield()
}

func (r *Semaphore) Release(p *Process) {
	if !r.waiting.empty() {
		p := r.waiting.removeFirst()
		r.simulation.addRunnable(p)
	} else {
		r.available += 1
	}
}

type Channel[T any] struct {
	simulation Simulation

	items []T
	start int
	used  int

	readers, writers waitingList
}

func NewChannel[T any](capacity int) *Channel[T] {
	if capacity <= 0 {
		panic("bad capacity")
	}
	return &Channel[T]{
		items: make([]T, capacity),
	}
}

func (c *Channel[T]) Write(p *Process, item T) {
	if c.used < len(c.items) {
		if !c.readers.empty() {
			reader := c.readers.removeFirst()
			reader.pendingItem = item
			p.simulation.addRunnable(reader)
			return
		}
		pos := c.start + c.used
		if pos >= len(c.items) {
			pos -= len(c.items)
		}
		c.items[pos] = item
		c.used++
		return
	}
	p.pendingItem = item
	p.simulation.removeRunnable(p)
	c.writers.addLast(p)
	p.yield()
}

func (c *Channel[T]) Read(p *Process) T {
	if c.used > 0 {
		item := c.items[c.start]
		if !c.writers.empty() {
			writer := c.writers.removeFirst()
			c.items[c.start] = writer.pendingItem.(T)
			writer.pendingItem = nil
			p.simulation.addRunnable(writer)
			c.start++
			if c.start >= len(c.items) {
				c.start -= len(c.items)
			}
		} else {
			var empty T
			c.items[c.start] = empty
			c.start++
			if c.start >= len(c.items) {
				c.start -= len(c.items)
			}
			c.used--
		}
		return item
	}
	p.simulation.removeRunnable(p)
	c.readers.addLast(p)
	p.yield()
	item := p.pendingItem.(T)
	p.pendingItem = nil
	return item
}
