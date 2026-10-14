// phase: 02

// Recursive functions: Fibonacci, gcd, Ackermann (small), the number of
// moves of the towers of Hanoi, fast power and mutual recursion.
package main

import "fmt"

func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

func gcd(a, b int) int {
	if b == 0 {
		return a
	}
	return gcd(b, a%b)
}

func ack(m, n int) int {
	switch {
	case m == 0:
		return n + 1
	case n == 0:
		return ack(m-1, 1)
	}
	return ack(m-1, ack(m, n-1))
}

func hanoi(n int, from, to, via string) int {
	if n == 0 {
		return 0
	}
	moves := hanoi(n-1, from, via, to)
	if n == 3 {
		fmt.Println("move disk 3 from", from, "to", to)
	}
	return moves + 1 + hanoi(n-1, via, to, from)
}

func pow(base, exp uint64) uint64 {
	if exp == 0 {
		return 1
	}
	half := pow(base, exp/2)
	if exp%2 == 1 {
		return half * half * base
	}
	return half * half
}

func isEven(n uint) bool {
	if n == 0 {
		return true
	}
	return isOdd(n - 1)
}

func isOdd(n uint) bool {
	if n == 0 {
		return false
	}
	return isEven(n - 1)
}

func sumDigits(n int) int {
	if n < 10 {
		return n
	}
	return n%10 + sumDigits(n/10)
}

func main() {
	for i := range 15 {
		fmt.Print(fib(i), " ")
	}
	fmt.Println("|")
	fmt.Println(fib(25))
	fmt.Println(gcd(1071, 462), gcd(17, 5), gcd(0, 9), gcd(48, 18))
	fmt.Println(ack(0, 0), ack(1, 2), ack(2, 3), ack(3, 3))
	fmt.Println(hanoi(4, "A", "C", "B"))
	fmt.Println(pow(2, 10), pow(3, 13), pow(2, 63), pow(2, 64), pow(7, 30))
	fmt.Println(isEven(10), isOdd(7), isEven(3))
	fmt.Println(sumDigits(987654321))
}

// Output:
// 0 1 1 2 3 5 8 13 21 34 55 89 144 233 377 |
// 75025
// 21 1 9 6
// 1 4 9 61
// move disk 3 from A to B
// move disk 3 from B to C
// 15
// 1024 1594323 9223372036854775808 0 1576789505350337489
// true true false
// 45
