// switch: the three shapes (a C++ switch on an integer tag, an if-else chain, a
// case index for fallthrough and break), cases evaluated lazily in source
// order, default anywhere, init statements, break in a switch in a loop, and
// fallthrough as a labeled statement or before empty statements.
package main

import "fmt"

var tagEvals = 0

func tag(s string) string {
	tagEvals++
	return s
}

func val(name string, v int) int {
	fmt.Print(name, " ")
	return v
}

func describe(n int) string {
	switch n {
	case 0:
		return "zero"
	case 1, 2, 3:
		return "small"
	case -1:
		return "minus one"
	default:
		return "other"
	case 100:
		return "hundred"
	}
}

// last ends with a switch whose cases end with a labeled fallthrough or one
// before empty statements: Go calls it a terminating statement.
func last(n int) int {
	switch n {
	case 0:
		if n == 0 {
			goto ft
		}
		n = 99
	ft:
		fallthrough
	case 1:
		fallthrough; ;
	default:
		return n + 1
	}
}

func main() {
	fmt.Println(last(0), last(1), last(5))
	// The same in an integer switch (a C++ switch on the tag) and in a tagless
	// one (a case index).
	for i := range 3 {
		switch i {
		case 0:
			if i < 5 {
				goto zero
			}
			fmt.Println("never")
		zero:
			fallthrough
		case 1:
			fmt.Println("one or after zero", i)
			fallthrough; ;
		case 2:
			fmt.Println("two or after one", i)
		}
		switch {
		case i == 0:
			if i < 5 {
				goto z
			}
			fmt.Println("never")
		z:
			fallthrough
		case i == 1:
			fmt.Println("tagless one or after zero", i)
			fallthrough; ;
		default:
			fmt.Println("tagless default", i)
		}
	}
	// break L to a switch from a loop inside it: still an if-else chain.
sw:
	switch s := "go"; {
	case len(s) == 2:
		for j := 0; ; j++ {
			if j == 1 {
				fmt.Println("break sw", j)
				break sw
			}
		}
	default:
		fmt.Println("never")
	}
	for n := -1; n <= 4; n++ {
		fmt.Println(n, describe(n))
	}
	fmt.Println(describe(100), describe(50))

	// Integer switch with fallthrough, and a default in the middle.
	for i := range 4 {
		switch i {
		case 0:
			fmt.Print("zero ")
			fallthrough
		case 1:
			fmt.Print("one-or-after-zero ")
		default:
			fmt.Print("default ")
			fallthrough
		case 2:
			fmt.Print("two-or-after-default ")
		}
		fmt.Println()
	}

	// The tag is evaluated once, the cases lazily in order, only until one matches.
	switch tag("b") {
	case "a":
		fmt.Println("a")
	case "b", "c":
		fmt.Println("b or c")
	}
	fmt.Println("tag evals", tagEvals)
	switch x := 2; x {
	case val("one", 1), val("two", 2), val("three", 3):
		fmt.Println("matched", x)
	case val("four", 4):
		fmt.Println("four")
	}
	switch 5 {
	case val("a", 1):
		fmt.Println("a")
	default:
		fmt.Println("default after all cases")
	case val("b", 2):
		fmt.Println("b")
	}

	// Tagless switches.
	for n := 0; n < 16; n += 5 {
		switch {
		case n%15 == 0:
			fmt.Println(n, "FizzBuzz")
		case n%3 == 0:
			fmt.Println(n, "Fizz")
		case n%5 == 0:
			fmt.Println(n, "Buzz")
		default:
			fmt.Println(n)
		}
	}
	switch y := val("y", 7); {
	case y > 5 && y < 10, y == 0:
		fmt.Println("y in range")
		fallthrough
	case y > 100:
		fmt.Println("y after fallthrough")
	case val("never", 1) > 0:
		fmt.Println("never")
	}

	// break in a switch leaves the switch, not the loop.
	for i := range 5 {
		switch {
		case i == 1:
			fmt.Println("one, break")
			break
		case i == 3:
			if i > 2 {
				break
			}
			fmt.Println("never")
		default:
			fmt.Println("i", i)
		}
	}
	var s string = "x"
	switch s {
	case "x":
		if len(s) == 1 {
			fmt.Println("break in string switch")
			break
		}
		fmt.Println("never")
	}

	// Switches with few clauses.
	switch tag("only default") {
	default:
		fmt.Println("only default")
	}
	switch {
	}
	switch tag("empty") {
	}
	fmt.Println("tag evals", tagEvals)
	var u8 uint8 = 200
	switch u8 + 100 {
	case 44:
		fmt.Println("uint8 wraps")
	case 255:
		fmt.Println("no")
	}
	f := 2.5
	switch f * 2 {
	case 5:
		fmt.Println("float 5")
	}
	b := true
	switch b {
	case false:
		fmt.Println("false")
	case true:
		fmt.Println("true")
	}
}
