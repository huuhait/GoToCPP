// phase: 09
// Stderr: (?s)^four on stderr\nfive on stderr\n$

//go:build windows

package main

import (
	"fmt"
	"os"
)

// Writes to the standard handles keep their order whichever way they are made, and the handles
// are what the parent gave (a pipe, a file or a console: the test works with any), and Stat
// works on them.
func main() {
	fmt.Println("one")
	os.Stdout.WriteString("two\n")
	fmt.Fprintln(os.Stdout, "three")
	println("four on stderr")
	os.Stderr.WriteString("five on stderr\n")
	fmt.Print("six\n")
	fi, err := os.Stdout.Stat()
	fmt.Println(err, fi.Mode().IsDir())
	fmt.Println(os.Stdin.Name(), os.Stdout.Name(), os.Stderr.Name())
	fmt.Println(os.Stdout.Fd() != os.Stderr.Fd(), os.Stdin.Fd() != os.Stdout.Fd())
	n, err := os.Stdout.Write(nil)
	fmt.Println(n, err)
	fmt.Println("done")
}

// Output:
// one
// two
// three
// six
// <nil> false
// /dev/stdin /dev/stdout /dev/stderr
// true true
// 0 <nil>
// done
