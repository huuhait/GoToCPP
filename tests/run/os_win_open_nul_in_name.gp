// phase: 09

//go:build windows

package main

import (
	"fmt"
	"os"
)

func main() {
	_, err := os.Open("a\x00b")
	fmt.Printf("%q\n", err.Error())
	_, err = os.Stat("a\x00b")
	fmt.Printf("%q\n", err.Error())
	fmt.Printf("%q\n", os.Remove("a\x00b").Error())
	fmt.Printf("%q\n", os.Mkdir("a\x00b", 0o700).Error())
	fmt.Printf("%q\n", os.Rename("a\x00b", "c").Error())
	fmt.Printf("%q\n", os.Setenv("A\x00", "x").Error())
	fmt.Printf("%q\n", os.Setenv("A", "x\x00").Error())
	fmt.Printf("%q\n", os.Setenv("", "x").Error())
	fmt.Printf("%q\n", os.Setenv("A=B", "x").Error())
}

// Output:
// "open a\x00b: invalid argument"
// "Stat a\x00b: invalid argument"
// "remove a\x00b: invalid argument"
// "mkdir a\x00b: invalid argument"
// "rename a\x00b c: invalid argument"
// "setenv: invalid argument"
// "setenv: invalid argument"
// "setenv: The parameter is incorrect."
// "setenv: The parameter is incorrect."
