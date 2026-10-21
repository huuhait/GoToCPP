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

type opError struct {
	op  string
	err error
}

var unwrapCalls int

func (e *opError) Error() string {
	return e.op + ": " + e.err.Error()
}

func (e *opError) Unwrap() error {
	unwrapCalls++
	return e.err
}

type nilError struct{}

func (e *nilError) Error() string {
	return "never called"
}

func parse(s string) (int, error) {
	return 0, errors.New("empty input")
}

func step!(s string) int {
	n := parse!(s)
	return n
}

func decorate(s string) (int, error) {
	n, err := step(s)
	if err != nil {
		return 0, &opError{"decorate", err}
	}
	return n, nil
}

func run!(s string) int {
	n := decorate!(s)
	return n
}

func typedNil() (int, error) {
	var p *nilError
	return 0, p
}

func viaNil!() int {
	v := typedNil!()
	return v
}

func main() {
	_, err := run("")
	fmt.Println("Unwrap calls made by the hops:", unwrapCalls)
	show("a wrapper that gocpp did not make: the trace restarts at the first ! that sees it", err)
	var op *opError
	fmt.Println(errors.As(err, &op), op.op)

	_, err = viaNil()
	fmt.Println(err != nil)
	show("a typed nil pointer has no identity: nothing is claimed for it", err)
}
