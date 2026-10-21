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

var ErrDB = errors.New("db down")

func query(n int) (int, error) {
	return 0, ErrDB
}

func lower!(n int) int {
	v := query!(n)
	return v
}

func adapter(n int) (int, error) {
	return lower(n)
}

func upper!(n int) int {
	v := adapter!(n)
	return v
}

func adapterBang!(n int) int {
	v := lower!(n)
	return v
}

func upper2!(n int) int {
	v := adapterBang!(n)
	return v
}

func main() {
	_, err := upper(1)
	show("a function without ! in the middle: the trace begins above it", err)
	_, err = upper2(1)
	show("the same chain with ! at every level", err)
}
