// phase: 05

package main

import (
	"errors"
	"fmt"
)

var trace []string

func step(name string, n int, fail bool) (int, error) {
	trace = append(trace, name)
	if fail {
		return 0, errors.New("fail " + name)
	}
	return n, nil
}

func two(name string, a, b int, fail bool) (int, int, error) {
	trace = append(trace, name)
	if fail {
		return 0, 0, errors.New("fail " + name)
	}
	return a, b, nil
}

// f declares four variables in one group, two of them with a ! call: the specs run in order,
// a later spec sees the names of the earlier ones, and the first failure ends the function.
func f(failAt string) (string, error) {
	var (
		a        = step!("a", 1, failAt == "a")
		plain    = len(trace) // no ! call: it runs after a and before b
		b, c     = two!("b", a+10, plain, failAt == "b")
		d    int = step!("d", b+c, failAt == "d")
	)
	return fmt.Sprint(a, plain, b, c, d, trace), nil
}

// g has the group inside a block and uses a ! call in a later spec only.
func g(fail bool) (int, error) {
	if true {
		var (
			x    = 5
			y, z = two!("g", x, x*2, fail)
		)
		return x + y + z, nil
	}
	return 0, nil
}

func main() {
	for _, at := range []string{"", "a", "b", "d"} {
		trace = nil
		s, err := f(at)
		fmt.Printf("%q %v %v\n", s, err, trace)
	}
	fmt.Println(g(false))
	fmt.Println(g(true))
}

// Output:
// "1 1 11 1 12 [a b d]" <nil> [a b d]
// "" fail a [a]
// "" fail b [a b]
// "" fail d [a b d]
// 20 <nil>
// 0 fail g
