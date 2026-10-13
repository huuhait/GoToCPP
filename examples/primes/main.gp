// phase: 02

// Primes finds prime numbers by trial division. It uses no arrays or slices,
// only loops, if and switch (the Phase 02 subset).
package main

import "fmt"

const limit = 100

// isPrime reports whether n is a prime number.
func isPrime(n int) bool {
	if n < 2 {
		return false
	}
	for d := 2; d*d <= n; d++ {
		if n%d == 0 {
			return false
		}
	}
	return true
}

// kind says whether n is prime, composite or neither.
func kind(n int) string {
	switch {
	case n < 2:
		return "neither"
	case isPrime(n):
		return "prime"
	default:
		return "composite"
	}
}

func main() {
	// The primes below limit, ten per line. The inner loop goes on with the
	// next n as soon as it finds a divisor: continue with a label.
	count, col := 0, 0
next:
	for n := 2; n < limit; n++ {
		for d := 2; d*d <= n; d++ {
			if n%d == 0 {
				continue next
			}
		}
		count++
		if col > 0 {
			fmt.Print(" ")
		}
		fmt.Print(n)
		col++
		if col == 10 {
			fmt.Println()
			col = 0
		}
	}
	if col > 0 {
		fmt.Println()
	}
	fmt.Printf("%d primes below %d\n", count, limit)

	// Twin primes: p and p+2 are both prime.
	sep := "twin primes:"
	for p := 3; p+2 < limit; p += 2 {
		if isPrime(p) && isPrime(p+2) {
			fmt.Printf("%s %d-%d", sep, p, p+2)
			sep = ","
		}
	}
	fmt.Println()

	// The largest gap between two primes in a row.
	prev, gapFrom, gap := 2, 0, 0
	for n := 3; n < limit; n += 2 {
		if !isPrime(n) {
			continue
		}
		if n-prev > gap {
			gapFrom, gap = prev, n-prev
		}
		prev = n
	}
	fmt.Println("largest gap:", gap, "after", gapFrom)

	// Goldbach: every even number from 4 to 20 is the sum of two primes.
	for n := 4; n <= 20; n += 2 {
		for p := 2; p <= n/2; p++ {
			if isPrime(p) && isPrime(n-p) {
				fmt.Printf("%d = %d + %d\n", n, p, n-p)
				break
			}
		}
	}

	// Count the kinds of the numbers 0 to 19.
	primes, composites, neither := 0, 0, 0
	for n := range 20 {
		switch kind(n) {
		case "prime":
			primes++
		case "composite":
			composites++
		default:
			neither++
		}
	}
	fmt.Println("0 to 19:", primes, "prime,", composites, "composite,", neither, "neither")
}
