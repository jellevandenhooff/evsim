package evsim

import (
	"log"
	"testing"
)

func TestCoro(t *testing.T) {
	c := newCoroutine[struct{}]()
	for run := range 10 {
		value := -1

		c.run(func(struct{}) {
			log.Print(run)
			for i := range run {
				value = i
				log.Print(i)
				c.yield()
			}
		}, struct{}{})

		for i := range run {
			if !c.step() {
				t.Error("huh")
			}
			if value != i {
				t.Error("huh")
			}
		}
		if c.step() {
			t.Error("huh")
		}
	}
}

func TestCoroStop(t *testing.T) {
	c := newCoroutine[struct{}]()

	value := -1

	c.run(func(struct{}) {
		for i := range 3 {
			value = i
			c.yield()
		}
	}, struct{}{})

	if value != -1 {
		t.Error(value)
	}
	if !c.step() {
		t.Error("huh")
	}
	if value != 0 {
		t.Error("huh")
	}
	if !c.step() {
		t.Error("huh")
	}
	if value != 1 {
		t.Error("huh")
	}

	c.stop()

	value = -1

	c.run(func(struct{}) {
		for i := range 2 {
			value = i
			c.yield()
		}
	}, struct{}{})

	if value != -1 {
		t.Error(value)
	}
	if !c.step() {
		t.Error("huh")
	}
	if value != 0 {
		t.Error("huh")
	}
	if !c.step() {
		t.Error("huh")
	}
	if value != 1 {
		t.Error("huh")
	}
	if c.step() {
		t.Error("huh")
	}
}
