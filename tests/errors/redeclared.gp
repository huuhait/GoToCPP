// phase: 01

package main

func f() {}

func f() {} // ERROR "f redeclared in this block"

func main() {
	var s string = 1 // ERROR "declared and not used: s" "cannot use 1 \(untyped int constant\) as string value"
}
