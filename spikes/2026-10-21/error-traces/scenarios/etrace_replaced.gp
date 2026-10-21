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

func main() {
	_, err1 := step("")
	show("err1 just after the call", err1)

	_, err2 := step("")
	show("err2 just after its call", err2)

	show("err1 again: err2 is in flight now", err1)
	fmt.Println(err1 != err2, err1.Error() == err2.Error())
}
