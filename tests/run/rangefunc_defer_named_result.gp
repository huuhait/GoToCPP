// phase: 07

package main

import (
	"errors"
	"fmt"
)

func Cnt(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func f() (n int, err error) {
	defer func() {
		if err != nil {
			n = -n
		}
		fmt.Println("deferred sees", n, err)
	}()
	for x := range Cnt(10) {
		n += x
		if x == 3 {
			return n * 2, errors.New("stopped")
		}
	}
	return n, nil
}

func g() (s string) {
	defer func() { s += " (deferred)" }()
	for x := range Cnt(3) {
		for y := range Cnt(3) {
			if x == 1 && y == 1 {
				return fmt.Sprint("returned ", x, y)
			}
		}
	}
	return "end"
}

func h() int {
	x := 1
	defer func() { x = 100 }()
	for i := range Cnt(3) {
		x += i
		if i == 2 {
			return x
		}
	}
	return 0
}

func main() {
	fmt.Println(f())
	fmt.Println(g())
	fmt.Println(h())
}

// Output:
// deferred sees -12 stopped
// -12 stopped
// returned 1 1 (deferred)
// 4
