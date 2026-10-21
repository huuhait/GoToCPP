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

var ErrPerm = errors.New("permission denied")

func open(path string) (int, error) {
	return 0, ErrPerm
}

func a!(path string) (n int, err error) {
	defer errors.Wrapf(&err, "a %s", path)
	n = open!(path)
	return n, nil
}

func g() (int, error) {
	return 0, fmt.Errorf("g: %w", ErrPerm)
}

func f2!() int {
	if _, err := a("x"); err != nil {
		fmt.Println("handled and dropped:", err)
	}
	n := g!()
	return n
}

func g2() (int, error) {
	return 0, ErrPerm
}

func f3!() int {
	if _, err := a("x"); err != nil {
		fmt.Println("handled and dropped:", err)
	}
	n := g2!()
	return n
}

func main() {
	_, err := f2()
	show("g wraps the sentinel and never called open: the trace must not say it did", err)

	_, err = f3()
	show("g2 returns the bare sentinel: the same", err)
}
