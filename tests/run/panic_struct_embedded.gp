// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// A promoted field through a nil embedded pointer panics with Go's message.
//go:build linux

package main

import "fmt"

type Base struct{ ID int }

type User struct {
	*Base
	Name string
}

func main() {
	u := User{Name: "ann"}
	fmt.Println(u.Name, u.Base == nil)
	fmt.Println(u.ID)
}

// Output:
// ann true
