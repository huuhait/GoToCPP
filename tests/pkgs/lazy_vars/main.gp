// phase: 06

// Package variables that hold a C++ class by value (step 6.4, ADR-093): the class has a constructor
// that C++ cannot run at compile time, so the variables are gp::lazy storage that gp_var_init builds
// (and a dynamic initializer builds when it did not), seen from another package through the
// extern reference of pkg.hpp. Each object is built once, a store in init() stays, and nothing is
// destroyed.
package main

import (
	"fmt"

	"example.com/lazyvars/store"
)

func main() {
	fmt.Println(store.Order)
	fmt.Println(store.Get())
	fmt.Println(store.Sum())
	store.SetL(8)
	fmt.Println(store.Get())
	fmt.Println(store.Bump(), store.H.N)
}

// Output:
// built 4
// 7 300 11
// 418
// 8 300 11
// 5 0
