package evsim

import (
	"log"
	"testing"
)

func TestCoro(t *testing.T) {
	for run := 0; run < 10; run++ {
		c := allocCoroutine()

		c.run(func() {
			log.Print(run)
			for i := 0; i < run; i++ {
				log.Print(i)
				c.yield()
			}
		})

		for c.step() {
		}

		freeCoroutine(c)
	}
}
