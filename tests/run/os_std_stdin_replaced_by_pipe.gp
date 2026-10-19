// phase: 09

package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	old := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.WriteString("line one\nline two\n")
		w.Close()
	}()
	b, err := io.ReadAll(os.Stdin)
	fmt.Printf("%q %v\n", b, err)
	os.Stdin = old
	fmt.Println(os.Stdin == old)
}

// Output:
// "line one\nline two\n" <nil>
// true
