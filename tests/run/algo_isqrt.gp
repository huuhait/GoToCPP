// phase: 02

// Integer square root by Newton's method and by binary search, perfect
// squares, and a float square root by Newton's method.
package main

import "fmt"

func isqrtNewton(n uint64) uint64 {
	if n < 2 {
		return n
	}
	x := n
	y := x>>1 + x&1
	for y < x {
		x = y
		y = (x + n/x) / 2
	}
	return x
}

func isqrtBinary(n uint64) uint64 {
	lo, hi := uint64(0), uint64(4294967295)
	for lo < hi {
		mid := lo + (hi-lo+1)/2
		if mid*mid <= n && mid <= 4294967295 {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func sqrtFloat(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for range 60 {
		z -= (z*z - x) / (2 * z)
	}
	return z
}

func main() {
	values := 0
	for n := uint64(0); n < 50; n += 7 {
		fmt.Print(isqrtNewton(n), ":", isqrtBinary(n), " ")
		values++
	}
	fmt.Println(values)
	fmt.Println(isqrtNewton(1000000), isqrtNewton(999999), isqrtNewton(18446744073709551615))
	fmt.Println(isqrtBinary(1000000), isqrtBinary(999999), isqrtBinary(18446744073709551615))
	squares := 0
	for n := uint64(1); n <= 10000; n++ {
		r := isqrtNewton(n)
		if r*r == n {
			squares++
		}
	}
	fmt.Println("squares up to 10000:", squares)
	fmt.Println(sqrtFloat(2), sqrtFloat(9), sqrtFloat(1e10), sqrtFloat(0.25))
}

// Output:
// 0:0 2:2 3:3 4:4 5:5 5:5 6:6 7:7 8
// 1000 999 4294967295
// 1000 999 4294967295
// squares up to 10000: 100
// 1.414213562373095 3 100000 0.5
