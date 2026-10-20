// phase: 09
// Stderr: (?m)^scanrange 43 1 <nil>$
// Stderr: (?m)^file got ""$

// The first store of the program to os.Stdin is the variable of a range: fmt.Scan reads the new file (ADR-100).
package main

import (
	"fmt"
	"io"
	"os"
)

var _ = io.EOF

func tmp() *os.File {
	f, err := os.CreateTemp("", "form")
	if err != nil {
		panic(err)
	}
	return f
}

type T struct{ Stdout *os.File }



func main() {
	f := tmp()
	defer os.Remove(f.Name())
	f.WriteString("43"); f.Seek(0, 0); for _, os.Stdin = range []*os.File{f} { var x int; n, err := fmt.Scan(&x); fmt.Fprintln(os.Stderr, "scanrange", x, n, err) }; f.Truncate(0)
	f.Seek(0, 0)
	b, _ := io.ReadAll(f)
	fmt.Fprintf(os.Stderr, "file got %q\n", b)
}

// Output:
