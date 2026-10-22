// phase: 05

// A parenthesized callee is the same call (the audit row L74): (g)(1), (fmt.Println)(1), a method
// value, a func variable, a func literal, several levels of parentheses, defer and a conversion.
package main

import "fmt"

type T struct{ n int }

func (t T) Add(d int) int { return t.n + d }

func g(x int) { fmt.Println("g", x) }

func two(a, b int) (int, int) { return b, a }

func main() {
	(g)(1)
	((g))(2)
	(fmt.Println)("hi", 3)
	t := T{10}
	fmt.Println((t.Add)(5))
	fmt.Println((T.Add)(t, 6))
	f := g
	(f)(4)
	(func(s string) { fmt.Println("lit", s) })("x")
	defer (g)(9)
	defer (fmt.Println)("deferred")
	a, b := (two)(1, 2)
	fmt.Println(a, b)
	fmt.Println((float64)(3), (string)([]byte("ab")), (*int)(nil) == nil)
}

// Output:
// g 1
// g 2
// hi 3
// 15
// 16
// g 4
// lit x
// 2 1
// 3 ab true
// deferred
// g 9
