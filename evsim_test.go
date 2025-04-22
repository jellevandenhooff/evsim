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

	for i := 0; i < 5; i++ {
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

	for i := 0; i < 20; i++ {
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
	for i := 0; i < n; i++ {
		ch <- struct{}{}
	}
	close(ch)
	for i := 0; i < m; i++ {
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

		for j := 0; j < 1000; j++ {
			sim.Spawn(func(p *evsim.Process) {
				for k := 0; k < 10; k++ {
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
