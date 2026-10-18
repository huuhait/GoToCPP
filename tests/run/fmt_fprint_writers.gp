// phase: 05
// Stderr: .*

// Fprint, Fprintln and Fprintf write to os.Stdout and os.Stderr and return Go's (n int, err
// error); Print, Println and Printf return the same; Append, Appendf and Appendln append to a
// byte slice.
package main

import (
	"fmt"
	"os"
)

func main() {
	n, err := fmt.Fprintln(os.Stdout, "to stdout", 1, 2)
	fmt.Println(n, err)
	n, err = fmt.Fprintf(os.Stdout, "%d-%s\n", 7, "x")
	fmt.Println(n, err)
	n, err = fmt.Fprint(os.Stdout, "a", 1, 2, "b\n")
	fmt.Println(n, err)
	n, err = fmt.Fprintln(os.Stderr, "to stderr")
	fmt.Println(n, err)
	n, err = fmt.Fprintf(os.Stderr, "stderr %d\n", 1)
	n, err = fmt.Print("print ", 1, "\n")
	fmt.Println(n, err)
	n, err = fmt.Printf("%s\n", "printf")
	fmt.Println(n, err)
	n, err = fmt.Println("println", 3)
	fmt.Println(n, err == nil)
	_, _ = fmt.Println()
	b := []byte("x:")
	b = fmt.Append(b, 1, 2, "s")
	b = fmt.Appendf(b, " %05d", 42)
	b = fmt.Appendln(b, "end", 1)
	fmt.Printf("%q\n", b)
	fmt.Println(string(fmt.Append(nil, "only")), fmt.Appendf(nil, "%d", 12), len(fmt.Appendln(nil)))
	s := fmt.Sprintf("%v", 1)
	fmt.Println(s, fmt.Sprint(), fmt.Sprintln() == "\n")
}

// Output:
// to stdout 1 2
// 14 <nil>
// 7-x
// 4 <nil>
// a1 2b
// 6 <nil>
// 10 <nil>
// print 1
// 8 <nil>
// printf
// 7 <nil>
// println 3
// 10 true
//
// "x:1 2s 00042end 1\n"
// only [49 50] 1
// 1  true
