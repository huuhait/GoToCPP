// phase: 09

//go:build linux

// Scan, Scanln and Scanf of a program that does not import os read file descriptor 0 themselves, a
// byte at a time: the program puts a pipe onto descriptor 0 (dup2), with the text in it. The newline
// rules of the three, the end of the input, a closed descriptor and a directory.
package main

import (
	"fmt"
	"syscall"
)

// feed makes descriptor 0 the read end of a pipe that holds s and is closed at the write end.
func feed(s string) {
	var p [2]int
	if err := syscall.Pipe(p[:]); err != nil {
		panic(err)
	}
	syscall.Write(p[1], []byte(s))
	syscall.Close(p[1])
	if err := syscall.Dup2(p[0], 0); err != nil {
		panic(err)
	}
	syscall.Close(p[0])
}

func main() {
	var a, b int
	var s string
	var f float64

	feed("5 6\nseven\n8.5\n")
	n, err := fmt.Scan(&a, &b)
	fmt.Println(n, err, a, b)
	n, err = fmt.Scanln(&s)
	fmt.Println(n, err, s)
	n, err = fmt.Scanf("%f", &f)
	fmt.Println(n, err, f)
	n, err = fmt.Scan(&a)
	fmt.Println(n, err)

	// Scanln stops at the newline; Scan reads over it.
	feed("1\n2\n3 4 5\n6\n")
	n, err = fmt.Scanln(&a, &b)
	fmt.Println(n, err, a)
	n, err = fmt.Scan(&a, &b)
	fmt.Println(n, err, a, b)
	n, err = fmt.Scanln(&a, &b)
	fmt.Println(n, err, a, b)
	n, err = fmt.Scanln(&a)
	fmt.Println(n, err, a)

	// Scanf needs the newlines of its format.
	feed("10 20\n30\n40")
	n, err = fmt.Scanf("%d %d\n", &a, &b)
	fmt.Println(n, err, a, b)
	n, err = fmt.Scanf("%d", &a)
	fmt.Println(n, err, a)
	n, err = fmt.Scanf("%d", &b)
	fmt.Println(n, err, b)
	n, err = fmt.Scanf("%d", &b)
	fmt.Println(n, err)

	// Bytes are read one at a time, so the next call sees what the last one left.
	feed("12345 678\n")
	n, err = fmt.Scanf("%3d", &a)
	fmt.Println(n, err, a)
	n, err = fmt.Scanf("%d", &b)
	fmt.Println(n, err, b)

	// A descriptor that is a directory, and one that is closed: the errors of the read.
	d, err := syscall.Open(".", syscall.O_RDONLY, 0)
	if err != nil {
		panic(err)
	}
	syscall.Dup2(d, 0)
	syscall.Close(d)
	n, err = fmt.Scan(&a)
	fmt.Println(n, err)
	syscall.Close(0)
	n, err = fmt.Scan(&a)
	fmt.Println(n, err)
	n, err = fmt.Scanln(&a)
	fmt.Println(n, err)
}

// Output:
// 2 <nil> 5 6
// 1 <nil> seven
// 1 <nil> 8.5
// 0 EOF
// 1 unexpected newline 1
// 2 <nil> 2 3
// 2 <nil> 4 5
// 1 <nil> 6
// 2 <nil> 10 20
// 1 <nil> 30
// 1 <nil> 40
// 0 EOF
// 1 <nil> 123
// 1 <nil> 45
// 0 read /dev/stdin: is a directory
// 0 read /dev/stdin: bad file descriptor
// 0 read /dev/stdin: bad file descriptor
