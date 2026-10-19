// The hand-written Go twin of umacro_start.gp: the macros are functions with the place of the use
// as arguments, and each use on top of a function is the first statement of the function (a func()
// result is deferred unless it is nil).

package main

import (
	"fmt"
	"sync"
)

func Trace(gp_file string, gp_line int, gp_func string) func() {
	fmt.Println("enter", gp_func)
	return func() { fmt.Println("exit ", gp_func) }
}

func Locked(gp_file string, gp_line int, gp_func string, mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}

func Maybe(gp_file string, gp_line int, gp_func string, on bool) func() {
	if !on {
		return nil
	}
	fmt.Println("maybe on at line", gp_line)
	return func() { fmt.Println("maybe off in", gp_func) }
}

func Note(gp_file string, gp_line int, gp_func string, msg string) {
	fmt.Println("note:", msg, gp_file, gp_line)
}

func Safe(gp_file string, gp_line int, gp_func string) func() {
	return func() {
		if r := recover(); r != nil {
			fmt.Println("safe recovered:", r, "in", gp_func)
		}
	}
}

type Store struct {
	mu sync.Mutex
	m  map[string]int
}

func (s *Store) Put(k string, v int) {
	if gp_defer1 := Locked("umacro_start.gp", 56, "main.(*Store).Put", &s.mu); gp_defer1 != nil {
		defer gp_defer1()
	}
	s.m[k] += v
}

func plain() string {
	if gp_defer1 := Trace("umacro_start.gp", 61, "main.plain"); gp_defer1 != nil {
		defer gp_defer1()
	}
	fmt.Println("in plain")
	return "plain"
}

func panics() {
	if gp_defer1 := Trace("umacro_start.gp", 67, "main.panics"); gp_defer1 != nil {
		defer gp_defer1()
	}
	defer fmt.Println("own defer of panics")
	panic("boom")
}

func maybe(n int) int {
	if gp_defer1 := Maybe("umacro_start.gp", 73, "main.maybe", n > 0); gp_defer1 != nil {
		defer gp_defer1()
	}
	fmt.Println("in maybe", n)
	return n * 2
}

func stacked() (res int) {
	if gp_defer1 := Trace("umacro_start.gp", 79, "main.stacked"); gp_defer1 != nil {
		defer gp_defer1()
	}
	Note("umacro_start.gp", 80, "main.stacked", "stacked")
	if gp_defer2 := Maybe("umacro_start.gp", 81, "main.stacked", true); gp_defer2 != nil {
		defer gp_defer2()
	}
	defer func() { res++ }()
	fmt.Println("in stacked")
	return 10
}

func safe(n int) (res int, err error) {
	if gp_defer1 := Safe("umacro_start.gp", 88, "main.safe"); gp_defer1 != nil {
		defer gp_defer1()
	}
	res = n
	if n > 1 {
		panic(fmt.Sprint("n too big: ", n))
	}
	return res, nil
}

func Id[T any](v T) T {
	Note("umacro_start.gp", 97, "main.Id[...]", "generic")
	return v
}

func main() {
	fmt.Println(plain())

	func() {
		defer func() { fmt.Println("main recovered:", recover()) }()
		panics()
	}()

	fmt.Println(maybe(2))
	fmt.Println(maybe(-1))

	fmt.Println(stacked())

	fmt.Println(safe(1))
	fmt.Println(safe(5))
	fmt.Println(Id("id"), Id(3))

	s := &Store{m: map[string]int{}}
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() { s.Put(fmt.Sprint("k", i%3), i) })
	}
	wg.Wait()
	fmt.Println(s.m["k0"], s.m["k1"], s.m["k2"])
}
