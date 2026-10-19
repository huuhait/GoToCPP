// phase: 10

// ! goes on a name that a call uses. A call of an arbitrary expression (fns[0]!()) is not in v1:
// store the function in a variable first.
package main

func two() (int, error) { return 1, nil }

func main() {
	fns := []func() (int, error){two}
	_ = fns[0]!() // ERROR "expected ';', found '!'"
}
