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

func parse(s string) (int, error) {
	return 0, errors.New("empty input")
}

func step!(s string) (n int, err error) {
	defer func() {
		if err != nil {
			_ = fmt.Errorf("step failed: %w", err)
		}
	}()
	n = parse!(s)
	return n, nil
}

func top!(s string) int {
	n := step!(s)
	return n
}

func main() {
	_, err := top("")
	show("COST: a wrapper made only for a log line takes the trace; the real error restarts at top", err)
}
