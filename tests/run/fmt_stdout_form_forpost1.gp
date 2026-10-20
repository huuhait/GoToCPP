// phase: 09
// Stderr: ^file got "forpost1 1\\n"\n$

// The first store of the program to os.Stdout is the post statement of a for (the first iteration prints to standard
// output, the second to the new file; ADR-100).
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
	for i := 0; i < 2; os.Stdout = f { fmt.Println("forpost1", i); i++ }
	f.Seek(0, 0)
	b, _ := io.ReadAll(f)
	fmt.Fprintf(os.Stderr, "file got %q\n", b)
}

// Output:
// forpost1 0
