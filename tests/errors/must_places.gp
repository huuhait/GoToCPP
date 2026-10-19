// phase: 05

// Places where a must call cannot be rewritten (step 10.13): a constant, an array length and a type are
// computed by the compiler, so no call can be there, with or without ! (a select case: must_select_case.gp).

package main

func two(a int) (int, error) { return a * 2, nil }

const k = two!(1) // ERROR "two!\(...\): a must call can't be used in a constant, a type or an array length"

var arr [two!(1)]int // ERROR "two!\(...\): a must call can't be used in a constant, a type or an array length"

func main() {
	var local [two!(4)]int // ERROR "two!\(...\): a must call can't be used in a constant, a type or an array length"
	_ = local
}

func sized(a [two!(3)]int) {} // ERROR "two!\(...\): a must call can't be used in a constant, a type or an array length"
