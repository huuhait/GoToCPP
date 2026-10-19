package main

import "fmt"

type T struct{ n int }

func (t *T) Inc() int { t.n++; return t.n }

func f(a, b int) int { return a + b }

func main() {
	t := &T{}
	x := f(
		t.Inc(),
		t.Inc()/
			f(1, 2),
	)
	m := map[string]int{
		"a": 1,
		"b": 2,
	}
	fmt.Println(
		x,
		m["a"],
		t.n,
	)
	y := []int{
		1, 2, 3,
	}[1]
	_ = y
}
