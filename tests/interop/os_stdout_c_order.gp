// phase: 09

// os.Stdout.Write and C's stdio share descriptor 1, but C's stdout has a buffer of its own. A
// write by package os to descriptor 1 or 2 flushes C's stdio first (sys_write), as fmt.Print and
// the builtin print do, so what C printf'd before comes out before: the order of the program.
// The first part writes to the harness's pipe, the second to a file (descriptor 1 duplicated).
package main

import (
	"<cstdio>"
	"fmt"
	"os"
	"syscall"
)

func main() {
	cstdio.printf("c1\n")
	os.Stdout.WriteString("go1\n")
	cstdio.printf("c2\n")
	cstdio.printf("c3\n")
	os.Stdout.Write([]byte("go2\n"))

	dir, err := os.MkdirTemp("", "gpos")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	f, err := os.Create(dir + "/out")
	if err != nil {
		panic(err)
	}
	saved, _ := syscall.Dup(1)
	syscall.Dup2(int(f.Fd()), 1)
	cstdio.printf("file c1\n")
	os.Stdout.WriteString("file go1\n")
	cstdio.printf("file c2\n")
	os.Stdout.WriteString("file go2\n")
	cstdio.fflush(cstdio.stdout)
	syscall.Dup2(saved, 1)
	syscall.Close(saved)
	f.Close()
	b, _ := os.ReadFile(dir + "/out")
	fmt.Printf("%q\n", b)
}

// Output:
// c1
// go1
// c2
// c3
// go2
// "file c1\nfile go1\nfile c2\nfile go2\n"
