package main

var d = 3

func f(s string, n int) int { return len(s) + n }

var q = f(")", // )
	10/d)

var r = f("(", /* ) */
	20/d)

var t = f(`)
(`, 30/d)

func main() {
	println(q, r, t)
}
