package evsim_test

import (
	"log"
	"runtime"
	"sync"
	"testing"
	"time"

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
				p.Sleep(10 * time.Second)
			}
		})
	}

	s.Spawn(func(p *evsim.Process) {
		log.Println("starting background")
		p.Sleep(1 * time.Minute)
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
			log.Println("started at", p.Simulation().Now().Format(time.TimeOnly))
			log.Println("starting worker")
			sema.Acquire(p)
			log.Println("acquired; working")
			p.Sleep(10 * time.Second)
			sema.Release(p)
			log.Println("finished at", p.Simulation().Now().Format(time.TimeOnly))
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
		sim := evsim.NewSimulation()
		sem := evsim.NewSemaphore(sim, 10)

		for j := 0; j < 1000; j++ {
			sim.Spawn(func(p *evsim.Process) {
				for k := 0; k < 10; k++ {
					sem.Acquire(p)
					p.Sleep(1 * time.Microsecond)
					sem.Release(p)
				}
			})
		}

		sim.Start()
	})
}

// before: BenchmarkEvsim-10            633           1802171 ns/op         1317468 B/op      34047 allocs/op
// after timers: BenchmarkEvsim-10           1383            751415 ns/op          677227 B/op      14046 allocs/op
// after alloc: BenchmarkEvsim-10           1732            686324 ns/op          450715 B/op      14018 allocs/op
// after pool: BenchmarkEvsim-10           1479            771514 ns/op          109718 B/op       3019 allocs/op
