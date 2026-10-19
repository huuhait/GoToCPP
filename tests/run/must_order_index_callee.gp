// phase: 05

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

// say prints a trace and returns the length of the name.
func say(name string) int {
	fmt.Println("say", name)
	return len(name)
}

func get(name string, n int, fail bool) (int, error) {
	fmt.Println("get", name, n)
	if fail {
		return 0, fmt.Errorf("get %s: %w", name, ErrBad)
	}
	return n, nil
}

func idx(i int) int {
	fmt.Println("idx", i)
	return i
}

func table() []func(int) int {
	fmt.Println("table")
	return []func(int) int{func(x int) int { return x + 1 }, func(x int) int { return x * 10 }}
}

func lookup() map[string]func(int) func(int) int {
	fmt.Println("lookup")
	return map[string]func(int) func(int) int{"k": func(a int) func(int) int { return func(b int) int { return a + b } }}
}

func key() string {
	fmt.Println("key")
	return "k"
}

func f(fail bool) (int, error) {
	fns := []func(int) int{func(x int) int { return x + 1 }, func(x int) int { return x * 10 }}
	v := fns[idx(1)](get!("a", 5, fail))
	w := table()[idx(0)](get!("b", 6, false))
	m := lookup()[key()](get!("c", 1, false))(get!("d", 2, false))
	return v + w + m, nil
}

func main() {
	fmt.Println(f(false))
	fmt.Println(f(true))
}

// Output:
// idx 1
// get a 5
// table
// idx 0
// get b 6
// lookup
// key
// get c 1
// get d 2
// 60 <nil>
// idx 1
// get a 5
// 0 get a: bad
