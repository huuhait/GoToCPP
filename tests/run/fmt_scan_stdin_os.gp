// phase: 09

//go:build linux

// Scan, Scanln and Scanf of a program that imports os read the file that the variable os.Stdin holds
// when the program has assigned another one (or closed the original), also through a func value, in a
// map, in a defer or go statement; otherwise they read the standard input (descriptor 0).
package main

import (
	"fmt"
	"os"
	"syscall"
)

func pipe(s string) {
	r, w, _ := os.Pipe()
	w.WriteString(s)
	w.Close()
	os.Stdin = r
}

func main() {
	var a, b int
	orig := os.Stdin

	// Not assigned: descriptor 0.
	var p [2]int
	syscall.Pipe(p[:])
	syscall.Write(p[1], []byte("1 2\n"))
	syscall.Close(p[1])
	syscall.Dup2(p[0], 0)
	syscall.Close(p[0])
	n, err := fmt.Scan(&a, &b)
	fmt.Println("fd 0:", a, b, n, err)
	n, err = fmt.Scan(&a)
	fmt.Println("fd 0 end:", n, err)

	pipe("1 2\n")
	n, err = fmt.Scan(&a, &b)
	fmt.Println("direct:", a, b, n, err)
	pipe("3 4\n")
	g := fmt.Scan
	n, err = g(&a, &b)
	fmt.Println("funcval Scan:", a, b, n, err)
	pipe("5 6\n")
	gl := fmt.Scanln
	n, err = gl(&a, &b)
	fmt.Println("funcval Scanln:", a, b, n, err)
	pipe("7\n")
	gf := fmt.Scanf
	n, err = gf("%d", &a)
	fmt.Println("funcval Scanf:", a, n, err)
	pipe("8\n")
	func() {
		defer fmt.Scan(&a)
	}()
	fmt.Println("defer:", a)
	pipe("9\n")
	done := make(chan bool)
	go func() {
		defer close(done)
		fmt.Scan(&b)
	}()
	<-done
	fmt.Println("go:", b)
	pipe("10\n")
	fns := map[string]func(...any) (int, error){"scan": fmt.Scan}
	n, err = fns["scan"](&a)
	fmt.Println("map funcval:", a, n, err)

	// A copy of the contents of a file: *os.Stdin = *r.
	r, w, _ := os.Pipe()
	w.WriteString("11\n")
	w.Close()
	saved := os.Stdin
	copyOf := *saved
	*os.Stdin = *r
	n, err = fmt.Scan(&a)
	fmt.Println("struct copy:", a, n, err)
	*os.Stdin = copyOf

	// The closed original and a nil variable.
	os.Stdin = orig
	orig.Close()
	n, err = fmt.Scan(&a)
	fmt.Println("closed:", n, err)
	os.Stdin = nil
	n, err = fmt.Scanln(&a)
	fmt.Println("nil:", n, err)
}

// Output:
// fd 0: 1 2 2 <nil>
// fd 0 end: 0 EOF
// direct: 1 2 2 <nil>
// funcval Scan: 3 4 2 <nil>
// funcval Scanln: 5 6 2 <nil>
// funcval Scanf: 7 1 <nil>
// defer: 8
// go: 9
// map funcval: 10 1 <nil>
// struct copy: 11 1 <nil>
// closed: 0 read /dev/stdin: file already closed
// nil: 0 invalid argument
