// Package-level declarations: consts (with iota and an expression that depends on it), vars,
// a named type, a func and func init, whose ordering Go guarantees. It imports nothing, so it
// belongs to the first 16.1f job; the import construct is covered by 17_list, 21_unicode and
// the package programs.
package main

const (
	a = iota
	b
	c = a + 10
	d
)

const greeting = "hello"

var counter = 1

type count int

func init() {
	counter = 2
	println("init ran")
}

func main() {
	println(a, b, c, d)
	println(greeting, counter)
	var n count = 3
	println(n == 3)
	println(greeting + "!")
}
