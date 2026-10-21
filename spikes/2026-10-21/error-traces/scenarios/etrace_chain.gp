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

func write(n int) (int, error) {
	if n > 2 {
		return 0, errors.New("disk full")
	}
	return n, nil
}

func save!(n int) int {
	x := write!(n)
	return x * 2
}

func flush!(n int) int {
	y := save!(n)
	return y + 1
}

func main() {
	_, err := flush(5)
	fmt.Println(err)
	show("a fresh error through two must functions", err)
	v, err := flush(1)
	fmt.Println(v, err)
	show("no error, no trace", err)
}
