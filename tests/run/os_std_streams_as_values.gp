// phase: 09

package main

import (
	"fmt"
	"io"
	"os"
)

func emit(w io.Writer, s string) {
	n, err := w.Write([]byte(s))
	fmt.Println(n, err)
}

func main() {
	var w io.Writer = os.Stdout
	emit(w, "via io.Writer\n")
	f := os.Stdout
	fmt.Fprintln(f, "via *os.File")
	g := f
	g.WriteString("via WriteString\n")
	fmt.Fprintf(os.Stdout, "%s %d\n", "printf", 7)
	io.WriteString(os.Stdout, "io.WriteString\n")
	fmt.Println(os.Stdout == f, os.Stdin != nil, os.Stderr != os.Stdout)
	var iface interface{} = os.Stdout
	_, ok := iface.(io.Writer)
	_, ok2 := iface.(io.Reader)
	_, ok3 := iface.(io.Closer)
	_, ok4 := iface.(io.Seeker)
	_, ok5 := iface.(io.ReaderAt)
	_, ok6 := iface.(io.StringWriter)
	fmt.Println(ok, ok2, ok3, ok4, ok5, ok6)
}

// Output:
// via io.Writer
// 14 <nil>
// via *os.File
// via WriteString
// printf 7
// io.WriteString
// true true true
// true true true true true true
