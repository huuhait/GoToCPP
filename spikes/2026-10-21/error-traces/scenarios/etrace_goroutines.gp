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
	if s == "" {
		return 0, errors.New("empty input")
	}
	return len(s), nil
}

func step!(s string) int {
	n := parse!(s)
	return n
}

func job!(s string) int {
	n := step!(s)
	return n
}

func check(err error) error {
	return err
}

func report!(err error) {
	check!(err)
}

func main() {
	plainCh := make(chan error)
	tracedCh := make(chan error)
	go func() {
		_, err := job("")
		plainCh <- err
	}()
	go func() {
		_, err := job("")
		tracedCh <- errors.Traced(err)
	}()
	e1 := <-plainCh
	e2 := <-tracedCh
	show("received over a channel as it is", e1)
	show("received as errors.Traced(err)", e2)
	report(e2)
	show("after a ! in the receiving goroutine", e2)
	fmt.Println(errors.Unwrap(e2) != nil, e2.Error())

	e3 := errors.Traced(e2)
	show("Traced of a traced error: each hop once", e3)

	_, err := job("")
	w := errors.Traced(err)
	show("Traced(err) read in the goroutine that made it: each hop once", w)
	show("err itself: the trace moved to the wrapper", err)
}
