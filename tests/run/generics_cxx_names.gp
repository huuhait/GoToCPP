// phase: 07

// Names that are keywords or macros in C++: functions, type parameters and variables of generic code.
package main

import "fmt"

func new_[T any](v T) *T { return &v }

func class[T any](v T) T { return v }

func template[namespace any](typename namespace) namespace { return typename }

func delete_[K comparable, V any](m map[K]V, key K) { delete(m, key) }

func operator[auto any](this auto, int_ int) []auto { return make([]auto, int_) }

func NULL[T any](EOF T) T {
	errno := EOF
	return errno
}

func max_[T int | float64](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func friend[public, private any](virtual public, static private) (public, private) { return virtual, static }

func main() {
	fmt.Println(*new_(1), class("c"), template(2.5), NULL("n"), max_(3, 4), max_(1.5, 0.5))
	m := map[string]int{"a": 1, "b": 2}
	delete_(m, "a")
	fmt.Println(m, operator("x", 2), len(operator(1, 3)))
	fmt.Println(friend(1, "static"))
	volatile, register := friend(true, 'r')
	fmt.Println(volatile, register)
}

// Output:
// 1 c 2.5 n 4 1.5
// map[b:2] [ ] 3
// 1 static
// true 114
