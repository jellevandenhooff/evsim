package evsim_test

import (
	"log"
	"reflect"
	"runtime"
	"sync"
	"testing"

	"github.com/jellevandenhooff/evsim"
)

func TestBasic(t *testing.T) {
	s := evsim.NewSimulation()

	log.Println("spawning")

	for range 5 {
		s.Spawn(func(p *evsim.Process) {
			log.Println("starting ticker")
			for {
				log.Println("tick", p.Simulation().Now())
				p.Sleep(10)
			}
		})
	}

	s.Spawn(func(p *evsim.Process) {
		log.Println("starting background")
		p.Sleep(60)
		log.Println("stopping simulation")
		p.Simulation().Stop()
	})

	log.Println("starting s")
	s.Start()
}

func TestSemaphore(t *testing.T) {
	s := evsim.NewSimulation()

	log.Println("spawning")

	sema := evsim.NewSemaphore(s, 5)

	for range 20 {
		s.Spawn(func(p *evsim.Process) {
			log.Println("started at", p.Simulation().Now())
			log.Println("starting worker")
			sema.Acquire(p)
			log.Println("acquired; working")
			p.Sleep(10)
			sema.Release(p)
			log.Println("finished at", p.Simulation().Now())
			log.Println("done worker")
		})
	}

	log.Println("starting s")
	s.Start()
}

func runParallel(n int, f func()) {
	var wg sync.WaitGroup
	m := runtime.NumCPU()
	wg.Add(m)
	ch := make(chan struct{}, n)
	for range n {
		ch <- struct{}{}
	}
	close(ch)
	for range m {
		go func() {
			defer wg.Done()
			for range ch {
				f()
			}
		}()
	}
	wg.Wait()
}

func BenchmarkEvsim(b *testing.B) {
	runParallel(b.N, func() {
		sim := evsim.SimulationsPool.Get()
		defer evsim.SimulationsPool.Put(sim)

		sem := evsim.NewSemaphore(sim, 10)

		for range 1000 {
			sim.Spawn(func(p *evsim.Process) {
				for range 10 {
					sem.Acquire(p)
					p.Sleep(.01)
					sem.Release(p)
				}
			})
		}

		sim.Start()
	})
}

func TestChannelNoBlock(t *testing.T) {
	sim := evsim.NewSimulation()

	ch := evsim.NewChannel[int](1)

	var read []int

	sim.Spawn(func(p *evsim.Process) {
		ch.Write(p, 1)
	})

	sim.Spawn(func(p *evsim.Process) {
		p.Sleep(1)
		read = append(read, ch.Read(p))
	})

	sim.Start()

	log.Println(read)
	if !reflect.DeepEqual(read, []int{1}) {
		t.Error(read)
	}
}

func TestChannelBlockWrite(t *testing.T) {
	sim := evsim.NewSimulation()

	ch := evsim.NewChannel[int](1)

	var read []int

	sim.Spawn(func(p *evsim.Process) {
		ch.Write(p, 1)
		ch.Write(p, 2)
	})

	sim.Spawn(func(p *evsim.Process) {
		p.Sleep(1)
		read = append(read, ch.Read(p))
		read = append(read, ch.Read(p))
	})

	sim.Start()

	log.Println(read)
	if !reflect.DeepEqual(read, []int{1, 2}) {
		t.Error(read)
	}
}

func TestChannelBlockRead(t *testing.T) {
	sim := evsim.NewSimulation()

	ch := evsim.NewChannel[int](1)

	var read []int

	sim.Spawn(func(p *evsim.Process) {
		p.Sleep(1)
		ch.Write(p, 1)
		ch.Write(p, 2)
	})

	sim.Spawn(func(p *evsim.Process) {
		read = append(read, ch.Read(p))
		read = append(read, ch.Read(p))
	})

	sim.Start()

	log.Println(read)
	if !reflect.DeepEqual(read, []int{1, 2}) {
		t.Error(read)
	}
}

func TestProcessCleanup(t *testing.T) {
	sim := evsim.NewSimulation()

	aborted := 0
	read := 0
	slept := false

	ch := evsim.NewChannel[int](1)

	for range 5 {
		sim.Spawn(func(p *evsim.Process) {
			defer func() {
				aborted++
			}()
			ch.Read(p)
			read++
		})
	}

	sim.Spawn(func(p *evsim.Process) {
		p.Sleep(1)
		slept = true
	})

	sim.Start()

	if aborted != 5 {
		t.Error("expected abort")
	}
	if read != 0 {
		t.Error("did not expect read")
	}
	if !slept {
		t.Error("expected slept")
	}
}
