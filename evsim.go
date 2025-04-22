package evsim

import (
	"container/heap"
	"log"
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

type Process struct {
	simulation               *Simulation
	runnableIdx              int
	waitingPrev, waitingNext *Process
	fn                       func(p *Process)
	coroutine                *coroutine[*Process]
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
	p.runnableIdx = -1
	p.waitingPrev = nil
	p.waitingNext = nil
	p.fn = nil
}

func (p *Process) step() {
	ok := p.coroutine.step()
	if !ok {
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
	if p.runnableIdx != -1 {
		panic("already runnable")
	}
	// log.Printf("adding runnable... %p", p)
	p.runnableIdx = len(s.runnable)
	s.runnable = append(s.runnable, p)
	s.check()
}

func (s *Simulation) check() {
	for i := range len(s.runnable) {
		if s.runnable[i].runnableIdx != i {
			log.Fatalf("%d bad: %d", i, s.runnable[i].runnableIdx)
		}
	}
}

func (s *Simulation) removeRunnable(p *Process) {
	if p.runnableIdx == -1 {
		panic("already stopped")
	}
	if s.runnable[p.runnableIdx] != p {
		panic("uh oh")
	}
	// s.check()
	a := p.runnableIdx
	b := len(s.runnable) - 1
	s.runnable[a] = s.runnable[b]
	s.runnable[a].runnableIdx = a
	s.runnable = s.runnable[:b]
	p.runnableIdx = -1
	// s.check()
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
}

func (s *Simulation) Spawn(f func(p *Process)) {
	p := s.processPool.Get()
	p.simulation = s
	p.fn = f
	p.coroutine.run(f, p)
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
