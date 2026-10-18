// phase: 05

// Type switch errors: fallthrough, a variable that no clause uses, a non-interface operand,
// x.(type) outside a switch (step 5.5).
package main

func main() {
	var x any
	switch x.(type) {
	case int:
		fallthrough // ERROR "cannot fallthrough in type switch"
	case string:
	}
	switch v := x.(type) { // ERROR "v declared and not used"
	case int:
	}
	n := 5
	switch n.(type) { // ERROR "n \(variable of type int\) is not an interface"
	}
	_ = x.(type) // ERROR "use of .\(type\) outside type switch"
}
