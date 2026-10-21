// phase: 10
// sim: skip (macros: the plain-Go simulation of run.sh cannot expand them; the names are those of ADR-124, section 3)
package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
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

macro Parse(s string) int {
	n := strconv.Atoi!(s)
	return n
}

macro Around() {
	#body()
}

func load!(s string) int {
	n := strconv.Atoi!(s)
	return n
}

#Around()
func viaWrap(s string) (int, error) {
	f := func() (int, error) {
		n := load!(s)
		return n, nil
	}
	return f()
}

func guarded(s string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
			show("a ! in a value macro: no error result, so it panics; read in the handler", err)
		}
	}()
	fmt.Println(#Parse(s))
	return nil
}

func main() {
	_, err := viaWrap("x")
	show("a ! in a closure inside the body of a wrap macro", err)
	guarded("y")
}
