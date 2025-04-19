package evsim_test

import (
	"log"
	"testing"
	"time"

	"github.com/jellevandenhooff/evsim"
)

func TestBasic(t *testing.T) {
	s := evsim.NewSimulation()

	log.Println("spawnning")

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
