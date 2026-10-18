// phase: 07

// Generic functions that call generic functions, box their value in an interface, switch on its type and
// take func values, all with C++ type arguments that C++ deduces.
package main

import (
	"fmt"

	"<string>"
	"<vector>"
)

func Inner[T any](x T) T { return x }

func Outer[T any](x T) T { return Inner(x) }

func Outer2[T any](x T) T { return Inner[T](Inner(x)) }

func Box[T any](x T) any { return x }

func Name[T any](x T) string {
	switch any(x).(type) {
	case int:
		return "int"
	case std.string:
		return "string"
	}
	return "other"
}

func Make[T any]() T {
	var z T
	return z
}

// Fresh makes a value of its own type argument: T is not in its parameters of Make, so the call
// inside the instance names it, Make<T>().
func Fresh[T any](x T) T {
	y := Make[T]()
	_ = x
	return y
}

func Map[T, U any](xs []T, f func(T) U) []U {
	var out []U
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func main() {
	v := std.vector[int]{1, 2, 3}
	s := std.string("hello")
	fmt.Println(int(Outer(v).size()), int(Outer2(s).size()), int(Fresh(v).size()), int(Fresh(s).size()))
	b := Box(s)
	fmt.Println(b != nil, Name(s), Name(5), Name(v))
	var ls []int = Map([]std.string{s, std.string("ab")}, func(x std.string) int { return int(x.size()) })
	fmt.Println(ls)
}

// Output:
// 3 5 0 0
// true string int other
// [5 2]
