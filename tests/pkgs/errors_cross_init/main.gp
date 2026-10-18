// phase: 06

// Errors made by package variable initializers and init functions in several packages, in dependency order.

package main

import (
	"errors"
	"fmt"

	"example.com/crossinit/lib"
	"example.com/crossinit/more"
)

func init() {
	lib.Order = append(lib.Order, "main init")
}

func main() {
	fmt.Println(lib.Order)
	fmt.Println(lib.ErrA.Error(), lib.ErrB.Error())
	fmt.Printf("%q\n", lib.All.Error())
	fmt.Println(errors.Is(lib.All, lib.ErrA), errors.Is(lib.All, lib.ErrB))
	fmt.Printf("%q\n", more.Combined.Error())
	fmt.Println(errors.Is(more.Combined, lib.ErrA), more.Same == lib.ErrB)
	r := lib.Reset()
	fmt.Printf("%q %v\n", r.Error(), errors.Is(r, lib.ErrA))
}

// Output:
// [lib init more init main init]
// lib: a replaced lib: b
// "lib: a\nlib: b"
// false true
// "lib: a replaced\nmore"
// true true
// "lib: a replaced\nlib: b" true
