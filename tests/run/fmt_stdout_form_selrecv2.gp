// phase: 09
// Stderr: ^file got "selrecv2 true\\n"\n$

// The first store of the program to os.Stdout is a selrecv2 form (not a plain assignment): Println goes to the new file, nothing to
// standard output (ADR-100).
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
	ch := make(chan *os.File, 1); ch <- f; var ok bool; select { case os.Stdout, ok = <-ch: fmt.Println("selrecv2", ok) }
	f.Seek(0, 0)
	b, _ := io.ReadAll(f)
	fmt.Fprintf(os.Stderr, "file got %q\n", b)
}

// Output:
