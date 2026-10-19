// phase: 05

// The rules for declaring a macro (step 10.9). Each is checked where the macro is declared.

package main

import "fmt"

macro Twice() {}

macro Twice() {} // ERROR "macro Twice redeclared in this package"

macro file() {} // ERROR "file is a built-in macro"

macro Derive(x int) {} // ERROR "Derive is a built-in macro"

macro min() {} // ERROR "min is a predeclared name"

macro Reserved(gp_x int) {} // ERROR "the name gp_x is reserved"

macro Hides() {
	gp_file := "x" // ERROR "the name gp_file is reserved"
	fmt.Println(gp_file)
}

macro TypeParam[T any](v lazy T) {} // ERROR "lazy parameter v: the type T is a type parameter of the macro"

macro Unknown(v lazy Missing) {} // ERROR "lazy parameter v: unknown type Missing"

macro Fields(v lazy struct{ x int }) {} // ERROR "a struct type with fields can't be written at every use"

macro Methods(v lazy interface{ M() }) {} // ERROR "an interface type with methods can't be written at every use"

const n = 3

macro Length(v lazy [n]int) {} // ERROR "an array length that is not an integer literal can't be written at every use"

macro Local(v lazy fmt.Stringer, w lazy fmt.Missing) {}

func main() {}
