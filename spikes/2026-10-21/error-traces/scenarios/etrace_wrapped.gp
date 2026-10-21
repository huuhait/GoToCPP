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

func load!(path string) (n int, err error) {
	defer errors.Wrapf(&err, "load %s", path)
	n = open!(path)
	return n, nil
}

func boot!(path string) int {
	n := load!(path)
	return n
}

func main() {
	_, err := boot("/etc/shadow")
	fmt.Println(err)
	fmt.Println(errors.Is(err, ErrPerm), err == ErrPerm)
	show("through Wrapf: the wrapper carries on the trace", err)
	show("the sentinel alone is not the error in flight", ErrPerm)
	wrapped := fmt.Errorf("start: %w", err)
	show("fmt.Errorf with %w: the new wrapper has the trace", wrapped)
	show("the error it wrapped does not", err)
	joined := errors.Join(errors.New("other"), wrapped)
	show("errors.Join does the same", joined)
}
