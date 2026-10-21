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

type conn struct {
	name string
}

func (c *conn) Close() error {
	return errors.New("close " + c.name)
}

func use!(name string) (err error) {
	c := &conn{name}
	defer c.Close!()
	return nil
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

func mustStep(s string) int {
	n := step!(s)
	return n
}

func safe(s string) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
			show("inside the handler", err)
		}
	}()
	return mustStep(s), nil
}

func outer!(s string) int {
	n := safe!(s)
	return n
}

func useUp!(name string) {
	use!(name)
}

func main() {
	show("a deferred Close! that becomes the error", use("db"))
	show("the deferred hop is linked to the hop of the caller", useUp("db"))

	parseAll := func(s string) (int, error) {
		n := step!(s)
		return n, nil
	}
	_, err := parseAll("")
	show("a ! inside a closure leaves through the closure's result", err)

	_, err = outer("")
	show("after the recover: the next ! starts from the frame it is in", err)
}
