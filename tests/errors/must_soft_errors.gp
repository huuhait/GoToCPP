// phase: 10

// The soft errors of go/types (no new variables, declared and not used) are errors with `!` too,
// and their messages say nothing about the code that the compiler writes for `!`.
package main

func good() int           { return 1 }
func two() (int, error)   { return 1, nil }
func count() (int, error)  { return 3, nil }

func f() (int, error) {
	x := 1
	x := two!() // ERROR "no new variables on left side of :="
	_ = x
	for i, j := range count!() { // ERROR "range over count!\(\) \(value of type int\) permits only one iteration variable"
		_, _ = i, j
	}
	for i := 0; i < 3; j := two!() { // ERROR "cannot declare in post statement"
	}
	k := two!() // ERROR "declared and not used: k"
	return 0, nil
}

func g!() int {
	y := 1
	y := good() // ERROR "no new variables on left side of :="
	return y
}

func main() {}
