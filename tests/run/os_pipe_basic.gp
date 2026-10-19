// phase: 09

package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	r, w, err := os.Pipe()
	fmt.Println(err, r.Name(), w.Name())
	go func() {
		w.WriteString("through the pipe")
		w.Close()
	}()
	b, err := io.ReadAll(r)
	fmt.Printf("%q %v\n", b, err)
	n, err := r.Read(make([]byte, 4))
	fmt.Println(n, err)
	fmt.Println(r.Close(), w.Close() != nil)
	st, _ := os.Stdout.Stat()
	_ = st
}

// Output:
// <nil> |0 |1
// "through the pipe" <nil>
// 0 EOF
// <nil> true
