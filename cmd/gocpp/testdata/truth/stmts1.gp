package main

import "fmt"

type T struct {
	A int    `json:"a(b"`
	B string `json:"b)}]"`
	C int
}

func (t *T) M(x int) int { return x }

func id[X any](x X) X { return x }

func f(args ...int) int { return len(args) }

var d = 3

func main() {
	t := &T{}
	s1 := f(1, int(10/d),
		// a comment with ((( brackets
		20/d, /* block ( comment */ 30/d,
		len("str ( with ] brackets")+40/d,
		len(`raw ( string
spanning ) lines {{ `)+50/d,
		int('(')/d, int(')')/d, int('"')/d,
		t.M(60/d),
	)
	s2 := id[int](
		70 / d,
	) + id[int](f(
		80/d,
		f(
			90/d,
		),
	))
	go func() {
		fmt.Println(
			100 / d)
	}()
	defer func(
		a int,
	) {
		_ = 110 / d
	}(
		120 / d,
	)
	x := []int{
		1 / d,
		2 / d,
	}; y := 5 / d
	z := map[string]int{
		"a": 6 / d,
		"b}": 7 / d,
	}
	fmt.Println(s1, s2, x, y, z,
		8/d)
outer:
	for i := 0; i < 3; i++ {
		if i == 1 { continue outer }; w := 9 /
			d
		_ = w
	}
	fmt.Println(`a
b`, 11/d,
		12/d)
}
