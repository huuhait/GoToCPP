// phase: 07

// A generic type declared inside a function can refer to itself (a list, a tree) and to the other generic types of the same
// function.
package main

import "fmt"

func main() {
	type List[T any] struct {
		head T
		tail *List[T]
	}
	type Tree[K, V any] struct {
		key         K
		val         V
		left, right *Tree[K, V]
		items       List[V]
	}

	var l *List[int]
	for i := 1; i <= 3; i++ {
		l = &List[int]{i, l}
	}
	for p := l; p != nil; p = p.tail {
		fmt.Print(p.head, ";")
	}
	fmt.Println()

	t := &Tree[string, int]{key: "m", val: 1}
	t.left = &Tree[string, int]{key: "a", val: 2}
	t.right = &Tree[string, int]{key: "z", val: 3, items: List[int]{9, nil}}
	fmt.Println(t.left.key, t.right.key, t.right.items.head, t.items.tail == nil)
}

// Output:
// 3;2;1;
// a z 9 true
