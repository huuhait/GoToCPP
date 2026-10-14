// phase: 02

// Digit algorithms: digit sums, reversing a number, palindromes, the
// digital root, Armstrong numbers and bases up to 10 printed digit by
// digit.
package main

import "fmt"

func digitSum(n int) int {
	s := 0
	for n > 0 {
		s += n % 10
		n /= 10
	}
	return s
}

func reverse(n int64) int64 {
	var r int64
	for n != 0 {
		r = r*10 + n%10
		n /= 10
	}
	return r
}

func isPalindrome(n int64) bool {
	return n >= 0 && reverse(n) == n
}

func digitalRoot(n int) int {
	for n >= 10 {
		n = digitSum(n)
	}
	return n
}

func digits(n int) int {
	if n == 0 {
		return 1
	}
	c := 0
	for ; n > 0; n /= 10 {
		c++
	}
	return c
}

func ipow(b, e int) int {
	r := 1
	for range e {
		r *= b
	}
	return r
}

func isArmstrong(n int) bool {
	k := digits(n)
	s := 0
	for m := n; m > 0; m /= 10 {
		s += ipow(m%10, k)
	}
	return s == n
}

func printBase(n uint64, base uint64) {
	if n >= base {
		printBase(n/base, base)
	}
	fmt.Print(n % base)
}

func main() {
	fmt.Println(digitSum(0), digitSum(12345), digitSum(999999999))
	fmt.Println(reverse(12345), reverse(-120), reverse(1000000))
	fmt.Println(isPalindrome(12321), isPalindrome(12345), isPalindrome(-121))
	fmt.Println(digitalRoot(493193), digitalRoot(9875))
	for i := 1; i < 10000; i++ {
		if isArmstrong(i) {
			fmt.Print(i, " ")
		}
	}
	fmt.Println("|")
	fmt.Println(digits(0), digits(7), digits(1234567890))
	printBase(255, 2)
	fmt.Println("|")
	printBase(255, 8)
	fmt.Println("|")
	printBase(1000, 3)
	fmt.Println("|")
	printBase(18446744073709551615, 10)
	fmt.Println("|")
}

// Output:
// 0 15 81
// 54321 -21 1
// true false false
// 2 2
// 1 2 3 4 5 6 7 8 9 153 370 371 407 1634 8208 9474 |
// 1 1 10
// 11111111|
// 377|
// 1101001|
// 18446744073709551615|
