// phase: 10

package main

import (
	"errors"
	"fmt"
	"path/filepath"
)

func show(label string, err error) {
	fmt.Println("--", label)
	frames := errors.Trace(err)
	if len(frames) == 0 {
		fmt.Println("  (no trace)")
	}
	for _, f := range frames {
		if f.File == "" {
			fmt.Printf("  ... %d hops omitted\n", f.Line)
			continue
		}
		fmt.Printf("  %s:%d %s\n", filepath.Base(f.File), f.Line, f.Function)
	}
}

var ErrX = errors.New("x")

func leaf() (int, error) {
	return 0, ErrX
}

type Svc interface {
	A() (int, error)
}

type S struct{}

// sim: thunk
func (S) A!() int {
	v := leaf!()
	return v
}

func run!(s Svc) int {
	v := s.A!()
	return v
}

func viaPlain(s Svc) (int, error) {
	return s.A()
}

func h2!() int {
	v := leaf!()
	return v
}

func main() {
	var s Svc = S{}
	_, err := run(s)
	show("a hop in a method called through an interface: the trace begins above it", err)

	viaPlain(s)
	_, err = h2()
	show("an unrelated propagation of the same value after it: no old hop in front", err)
}
