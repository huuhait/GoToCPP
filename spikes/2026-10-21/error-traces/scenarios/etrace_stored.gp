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

type task struct {
	err error
}

func parse(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty input")
	}
	return len(s), nil
}

func step!(s string) int {
	n := parse!(s)
	return n
}

func (t *task) run() {
	_, err := step("")
	t.err = err
}

func (t *task) runTraced() {
	_, err := step("")
	t.err = errors.Traced(err)
}

func finish(t *task) error {
	return t.err
}

func report!(t *task) {
	finish!(t)
}

func main() {
	a := &task{}
	a.run()
	e := report(a)
	show("stored, then returned later: the trace begins at the late hop", e)

	b := &task{}
	b.runTraced()
	step("")
	e = report(b)
	show("stored with errors.Traced: the history comes with it", e)
}
