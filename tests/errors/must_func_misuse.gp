// phase: 05

// What the compiler says about a must function that cannot work (step 10.4): main! and init!
// have no parameters and results (an error is printed and the program exits), and the returns of
// a must function have one value less than the signature that callers see.
package main

func main!(x int) { // ERROR "func main! must have no parameters and no results" // ERROR "func main must have no arguments and no return values"
}

func init!() int { // ERROR "func init! must have no parameters and no results" // ERROR "func init must have no arguments and no return values"
	return 1
}

func ret!() (int, error) {
	return 1 // ERROR "not enough return values"
}

func many!() int {
	return 1, 2, 3 // ERROR "too many return values"
}

func bare!() int {
	return // ERROR "not enough return values"
}
