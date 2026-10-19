// phase: 10

// User macros together with C and C++ values (step 10.9): the arguments of a line of macros on top of a
// func and of a use inside code can name header packages and namespaces (math.sqrt, std.string); a generic
// macro with a lazy parameter can be used with a C++ type argument (the type of s is std.string).

package main

import (
	"fmt"

	"<math.h>"
	"<string>"
)

macro Show(v float64) {
	fmt.Println("show", v)
}

macro Wrap[T any](v T, f lazy bool) T {
	if f() {
		fmt.Println("long")
	}
	return v
}

macro Plain[T any](v T) T { return v }

macro Twice(f lazy float64) float64 { return f() + f() }

#Show(float64(math.sqrt(16)))
func shown() { fmt.Println("shown") }

#Show(float64(math.pow(2, 10)))
#Show(float64(std.string("abc").size()))
func stacked() int { return 1 }

func main() {
	shown()
	stacked()
	s := std.string("hello")
	t := #Wrap(s, s.size() > 2)
	u := #Plain(s)
	fmt.Println(int(t.size()), int(u.size()))
	fmt.Println(#Twice(float64(math.sqrt(9))))
}

// Output:
// show 4
// shown
// show 1024
// show 3
// long
// 5 5
// 6
