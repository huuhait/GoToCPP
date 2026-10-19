// phase: 09

//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func main() {
	for _, c := range [][2]string{{"", "v"}, {"A=B", "v"}, {"A\x00B", "v"}, {"AB", "v\x00"}, {"OK", "fine"}} {
		err := os.Setenv(c[0], c[1])
		fmt.Printf("%q %q -> %v\n", c[0], c[1], err)
		if err != nil {
			se := err.(*os.SyscallError)
			fmt.Println(se.Syscall, se.Err == syscall.EINVAL, errors.Is(err, syscall.EINVAL))
		}
	}
	fmt.Println(os.Unsetenv(""))
	fmt.Println(os.Unsetenv("A=B"))
}

// Output:
// "" "v" -> setenv: The parameter is incorrect.
// setenv false false
// "A=B" "v" -> setenv: The parameter is incorrect.
// setenv false false
// "A\x00B" "v" -> setenv: invalid argument
// setenv true true
// "AB" "v\x00" -> setenv: invalid argument
// setenv true true
// "OK" "fine" -> <nil>
// The parameter is incorrect.
// The parameter is incorrect.
