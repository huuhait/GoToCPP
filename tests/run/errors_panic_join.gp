// phase: 05
// ExitCode: 2
// Stderr: (?m)^panic: first\n\tsecond\n\tthird$

// panic(errors.Join(...)): the text has newlines; Go indents the lines after the first one with a tab.

package main

import "errors"

func main() {
	panic(errors.Join(errors.New("first"), errors.New("second"), errors.New("third")))
}

// Output:
