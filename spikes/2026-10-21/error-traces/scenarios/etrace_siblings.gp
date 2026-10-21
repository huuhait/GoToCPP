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

func leaf(n int) (int, error) {
	return 0, ErrX
}

func a1!(n int) int {
	v := leaf!(n)
	return v
}

func a2!(n int) int {
	v := a1!(n)
	return v
}

func b1!(n int) int {
	v := leaf!(n)
	return v
}

func main() {
	_, err := a2(1)
	show("path 1: a2, a1, leaf", err)
	_, err = b1(1)
	show("path 2: b1, leaf, a sibling of a2 at the same depth", err)
	_, err = a2(1)
	show("path 1 again", err)
}
