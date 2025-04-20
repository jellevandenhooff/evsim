package evsim

import (
	"container/heap"
	"log"
	"math/rand"
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

	now time.Time

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
	s.now = time.Date(2020, 1, 1, 10, 15, 10, 0, time.UTC)
	s.current = nil
	s.timers = s.timers[:0]
	s.runnable = s.runnable[:0]
}

func NewSimulation() *Simulation {
	return &Simulation{
		now: time.Date(2020, 1, 1, 10, 15, 10, 0, time.UTC),
		processPool: Pool[*Process]{
			new:   newProcess,
			reset: (*Process).reset,
		},
	}
}

type Process struct {
	simulation *Simulation

	runnableIdx int

	waitingPrev, waitingNext *Process

	fn func(p *Process)

	coroutine *coroutine[*Process]
}

func newProcess() *Process {
	p := &Process{
		coroutine: newCoro[*Process](),
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
	for i := 0; i < len(s.runnable); i++ {
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
				if !next.at.After(s.now) {
					panic("huh")
				}
				if next.at.After(s.now) {
					// log.Printf("advancing time to %s", s.now.Format(time.TimeOnly))
					s.now = next.at
				}
				s.addRunnable(next.thenUnpause)
				for len(s.timers) > 0 && s.timers[0].at.Equal(s.now) {
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

func (s *Simulation) Now() time.Time {
	return s.now
}

type Semaphore struct {
	simulation *Simulation

	available int

	waitingFirst, waitingLast *Process
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
	p.waitingPrev = r.waitingLast
	if r.waitingLast != nil {
		r.waitingLast.waitingNext = p
	} else {
		r.waitingFirst = p
	}
	r.waitingLast = p
	p.yield()
}

func (r *Semaphore) Release(p *Process) {
	if r.waitingFirst != nil {
		p := r.waitingFirst
		r.waitingFirst = p.waitingNext
		p.waitingNext = nil
		if r.waitingFirst != nil {
			r.waitingFirst.waitingPrev = nil
		} else {
			r.waitingLast = nil
		}
		r.simulation.addRunnable(p)
	} else {
		r.available += 1
	}
}
