// phase: 10

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

// num parses one digit.
func num(s string) (int, error) {
	if len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
		return int(s[0] - '0'), nil
	}
	return 0, fmt.Errorf("num %q: %w", s, ErrBad)
}

// ok fails for n < 0, with a trace.
func ok(n int) (int, error) {
	fmt.Println("ok", n)
	if n < 0 {
		return 0, fmt.Errorf("ok %d: %w", n, ErrBad)
	}
	return n * 2, nil
}

func seq(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		defer fmt.Println("seq done")
		for i := 0; i < n; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

func f(a string, limit, bias int) (total int, err error) {
	for x := range seq(num!(a)) {
		v := ok!(x - bias)
		if v > limit {
			break
		}
		if x == 1 {
			continue
		}
		total += v
	}
	return total, nil
}

func g!(a string) (int, string) {
	for x := range seq(num(a)) {
		if err := errors.New("shadow"); err != nil {
			fmt.Println("shadowed", err)
		}
		if x == 2 {
			return ok(x), "found"
		}
	}
	return 0, "none"
}

func main() {
	fmt.Println(f("4", 100, 0))
	fmt.Println(f("4", 2, 0))
	fmt.Println(f("x", 2, 0))
	fmt.Println(f("4", 100, 2))
	fmt.Println(f("3", 100, 0))
	fmt.Println(g("4"))
	fmt.Println(g("1"))
	fmt.Println(g("x"))
}

// Output:
// ok 0
// ok 1
// ok 2
// ok 3
// seq done
// 10 <nil>
// ok 0
// ok 1
// ok 2
// seq done
// 0 <nil>
// 0 num "x": bad
// ok -2
// seq done
// 0 ok -2: bad
// ok 0
// ok 1
// ok 2
// seq done
// 4 <nil>
// shadowed shadow
// shadowed shadow
// shadowed shadow
// ok 2
// seq done
// 4 found <nil>
// shadowed shadow
// seq done
// 0 none <nil>
// 0  num "x": bad
