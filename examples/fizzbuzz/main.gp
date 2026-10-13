// phase: 02

// Fizzbuzz prints the numbers 1 to 30, with Fizz for multiples of 3, Buzz for
// multiples of 5 and FizzBuzz for both, then counts the words it printed.
package main

import "fmt"

const limit = 30

// word returns the word for n, and false when n is printed as a number.
func word(n int) (string, bool) {
	switch {
	case n%15 == 0:
		return "FizzBuzz", true
	case n%3 == 0:
		return "Fizz", true
	case n%5 == 0:
		return "Buzz", true
	}
	return "", false
}

func main() {
	fizz, buzz, both, numbers := 0, 0, 0, 0
	for i := range limit {
		n := i + 1
		w, ok := word(n)
		if !ok {
			fmt.Println(n)
			numbers++
			continue
		}
		fmt.Println(w)
		switch w {
		case "Fizz":
			fizz++
		case "Buzz":
			buzz++
		default:
			both++
		}
	}
	fmt.Printf("Fizz: %d, Buzz: %d, FizzBuzz: %d, numbers: %d\n", fizz, buzz, both, numbers)

	// A 3-clause loop that counts down, and a loop with only a condition.
	// fmt.Print adds no spaces next to string operands.
	sep := ""
	for n := 15; n > 0; n -= 5 {
		w, _ := word(n)
		fmt.Print(sep, n, "=", w)
		sep = " "
	}
	fmt.Println()
	n := 1
	for n*3 < limit {
		n *= 3
	}
	fmt.Println("largest power of 3 below", limit, "is", n)
}
