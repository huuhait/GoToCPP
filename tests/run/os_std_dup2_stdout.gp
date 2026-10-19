// phase: 09

//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"
)

func main() {
	d, _ := os.MkdirTemp("", "gpos")
	defer os.RemoveAll(d)
	f, _ := os.Create(d + "/out")
	saved, err := syscall.Dup(1)
	fmt.Println(err)
	fmt.Println(syscall.Dup2(int(f.Fd()), 1))
	fmt.Println("this goes to the file")
	os.Stdout.WriteString("and this\n")
	syscall.Dup2(saved, 1)
	syscall.Close(saved)
	f.Close()
	b, _ := os.ReadFile(d + "/out")
	fmt.Printf("%q\n", b)
}

// Output:
// <nil>
// "<nil>\nthis goes to the file\nand this\n"
