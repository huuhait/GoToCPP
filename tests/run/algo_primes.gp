// phase: 02

// Primes by trial division (no slices in Phase 02): the primes below 100,
// the count below 10000, the 1000th prime and a factorization.
package main

import "fmt"

func isPrime(n int) bool {
	if n < 2 {
		return false
	}
	if n%2 == 0 {
		return n == 2
	}
	for d := 3; d*d <= n; d += 2 {
		if n%d == 0 {
			return false
		}
	}
	return true
}

func factor(n int) {
	fmt.Print(n, " =")
	sep := " "
	for d := 2; d*d <= n; d++ {
		for n%d == 0 {
			fmt.Print(sep, d)
			sep = " * "
			n /= d
		}
	}
	if n > 1 {
		fmt.Print(sep, n)
	}
	fmt.Println("|")
}

func main() {
	for i := range 100 {
		if isPrime(i) {
			fmt.Print(i, " ")
		}
	}
	fmt.Println("|")
	count := 0
	for i := range 10000 {
		if isPrime(i) {
			count++
		}
	}
	fmt.Println("below 10000:", count)
	nth, n := 0, 1
	for nth < 1000 {
		n++
		if isPrime(n) {
			nth++
		}
	}
	fmt.Println("prime 1000:", n)
	factor(360)
	factor(9973)
	factor(600851475143)
}

// Output:
// 2 3 5 7 11 13 17 19 23 29 31 37 41 43 47 53 59 61 67 71 73 79 83 89 97 |
// below 10000: 1229
// prime 1000: 7919
// 360 = 2 * 2 * 2 * 3 * 3 * 5|
// 9973 = 9973|
// 600851475143 = 71 * 839 * 1471 * 6857|
