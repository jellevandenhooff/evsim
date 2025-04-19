package evsim

import (
	"container/heap"
	"iter"
	"log"
	"math/rand"
	"runtime"
	"time"
)

var stopValue = new(int)

type timer struct {
	at          time.Time
	thenUnpause *Process
}

type timers []timer

func (t timers) Len() int {
	return len(t)
}

func (t timers) Less(i, j int) bool {
	return t[i].at.Before(t[j].at)
}

func (t timers) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t *timers) Push(x any) {
	*t = append(*t, x.(timer))
}

func (t *timers) Pop() any {
	n := len(*t)
	last := (*t)[n-1]
	*t = (*t)[:n-1]
	return last
}

type Simulation struct {
	stopped bool

	now time.Time

	current *Process

	timers   timers
	runnable []*Process
}

func NewSimulation() *Simulation {
	return &Simulation{
		now: time.Date(2020, 1, 1, 10, 15, 10, 0, time.UTC),
	}
}

type Process struct {
	simulation *Simulation

	runnableIdx int

	waitingPrev, waitingNext *Process

	fn func(p *Process)

	iterYieldFn func(struct{}) bool
	iterNextFn  func() (struct{}, bool)
	iterStopFn  func()
}

func (p *Process) run(yield func(struct{}) bool) {
	p.iterYieldFn = yield
	defer func() {
		if p := recover(); p != nil {
			if p != stopValue {
				var buf [4096]byte
				n := runtime.Stack(buf[:], false)
				log.Fatalf("unexpected panic\n%s\n%s", p, string(buf[:n]))
			}
		}
	}()
	p.fn(p)
}

func (p *Process) step() {
	_, ok := p.iterNextFn()
	if !ok {
		p.simulation.removeRunnable(p)
	}
}

func (p *Process) yield() {
	if p.simulation.current != p {
		panic("help")
	}
	if !p.iterYieldFn(struct{}{}) {
		panic(stopValue)
	}
}

func (p *Process) Simulation() *Simulation {
	return p.simulation
}

func (p *Process) Sleep(duration time.Duration) {
	if duration <= 0 {
		return
	}
	s := p.simulation
	s.addTimer(s.now.Add(duration), p)
	s.removeRunnable(p)
	p.yield()
}

func (s *Simulation) addTimer(at time.Time, thenUnpause *Process) {
	if !at.After(s.now) {
		panic("bad time")
	}

	heap.Push(&s.timers, timer{
		at:          at,
		thenUnpause: thenUnpause,
	})
}

func (s *Simulation) addRunnable(p *Process) {
	if p.runnableIdx != -1 {
		panic("already runnable")
	}
	p.runnableIdx = len(s.runnable)
	s.runnable = append(s.runnable, p)
}

func (s *Simulation) removeRunnable(p *Process) {
	if p.runnableIdx == -1 {
		panic("already stopped")
	}
	a := p.runnableIdx
	b := len(s.runnable) - 1
	s.runnable[a] = s.runnable[b]
	s.runnable[a].runnableIdx = a
	s.runnable = s.runnable[:b]
	p.runnableIdx = -1
}

func (s *Simulation) run() {
	for !s.stopped {
		if len(s.runnable) == 0 {
			if len(s.timers) > 0 {
				next := heap.Pop(&s.timers).(timer)
				if !next.at.After(s.now) {
					panic("huh")
				}
				if next.at.After(s.now) {
					log.Printf("advancing time to %s", s.now.Format(time.TimeOnly))
					s.now = next.at
				}
				s.addRunnable(next.thenUnpause)
				for len(s.timers) > 0 && s.timers[0].at.Equal(s.now) {
					s.addRunnable(s.timers[0].thenUnpause)
					heap.Pop(&s.timers)
				}
				continue
			}
			log.Printf("nothing runnable anymore; stopping")
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
	p := &Process{
		fn:          f,
		runnableIdx: -1,
		simulation:  s,
	}
	next, stop := iter.Pull(p.run)
	p.iterNextFn = next
	p.iterStopFn = stop
	s.addRunnable(p)
}

func (s *Simulation) Start() {
	s.run()
}

func (s *Simulation) Stop() {
	s.stopped = true
}

func (s *Simulation) Now() time.Time {
	return s.now
}

type Semaphore struct {
	simulation *Simulation

	available int

	waitingOrig []*Process
	waiting     []*Process
}

func NewSemaphore(s *Simulation, limit int) *Semaphore {
	return &Semaphore{
		simulation: s,
		available:  limit,
	}
}

func (r *Semaphore) Acquire(p *Process) {
	if r.available > 0 {
		r.available -= 1
		return
	}

	r.simulation.removeRunnable(p)
	r.waiting = append(r.waiting, p)
	p.yield()
}

func (r *Semaphore) Release(p *Process) {
	if len(r.waiting) > 0 {
		// ugh...
		p := r.waiting[0]
		r.simulation.addRunnable(p)
		r.waiting = r.waiting[1:]
	} else {
		r.available += 1
	}
}
