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

var ErrBusy = errors.New("busy")

func try(n int) (int, error) {
	if n < 0 {
		return 0, ErrBusy
	}
	return n, nil
}

func inner!(n int) int {
	v := try!(n)
	return v
}

func plainPass(n int) (int, error) {
	return try(n)
}

func caller!(n int) int {
	_, err := inner(n)
	_ = err
	v := plainPass!(n)
	return v
}

func rec!(n int) int {
	if n == 0 {
		v := try!(-1)
		return v
	}
	v := rec!(n - 1)
	return v
}

func lookupB!(n int) int {
	v := try!(n)
	return v
}

func handle!(n int, again bool) int {
	if again {
		v := try!(n)
		return v
	}
	_, err := lookupB(n)
	_ = err
	return 0
}

func main() {
	_, err := caller(-1)
	show("LIMIT: the handled propagation of a shared value is still there", err)

	handle(-1, false)
	_, err = handle(-1, true)
	show("LIMIT: the same function again, in another call", err)

	_, err = rec(40)
	frames := errors.Trace(err)
	fmt.Println(len(frames), frames[0].Function, frames[len(frames)-1].Function)
	for i, f := range frames {
		if f.File == "" {
			fmt.Printf("frame %d: ... %d hops omitted\n", i, f.Line)
		}
	}
}
