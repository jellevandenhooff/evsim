package evsim

import (
	"log"
	"testing"
)

func TestCoro(t *testing.T) {
	c := newCoroutine[struct{}]()
	for run := 0; run < 10; run++ {
		c.run(func(struct{}) {
			log.Print(run)
			for i := 0; i < run; i++ {
				log.Print(i)
				c.yield()
			}
		}, struct{}{})

		for c.step() {
		}
	}
}
