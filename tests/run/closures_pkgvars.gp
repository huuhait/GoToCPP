// phase: 03

// Closures in the initializers of package variables: a counter made by a func literal called
// at once (its variable is boxed, by the same analysis as a function's), a func value that
// reads another package variable when it runs, and package variables of func type assigned
// later.
package main

import "fmt"

var nextID = func() func() int {
	id := 0
	return func() int {
		id++
		return id
	}
}()

var prefix = "item"

var label = func(n int) string { return prefix + "-" + string(rune('0'+n)) }

var hook func(string)

var squares = func() []int {
	var s []int
	for i := range 4 {
		s = append(s, i*i)
	}
	return s
}()

func main() {
	fmt.Println(nextID(), nextID(), nextID())
	prefix = "thing"
	fmt.Println(hook == nil, len(squares), squares[3])
	hook = func(s string) { fmt.Println("hook:", s) }
	hook(label(nextID()))
}

// Output:
// 1 2 3
// true 4 9
// hook: thing-4
