// phase: 07

// A generic type declared inside a method of a generic type takes the receiver's type parameters like a function's; the
// instances of the receiver make different types.
package main

import "fmt"

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Walk() string {
	type Cell[U any] struct {
		u U
		t T
	}
	cs := []Cell[string]{}
	for _, it := range s.items {
		cs = append(cs, Cell[string]{fmt.Sprint(it), it})
	}
	return fmt.Sprintf("%v %T", cs, cs)
}

func (s Stack[T]) Kind() any {
	type Tag[U any] struct{ u U }
	return Tag[int]{}
}

func main() {
	s := &Stack[int]{[]int{1, 2}}
	fmt.Println(s.Walk())
	s2 := &Stack[string]{[]string{"a"}}
	fmt.Println(s2.Walk())
	fmt.Println(s.Kind() == (&Stack[int]{}).Kind(), s.Kind() == s2.Kind())
}

// Output:
// [{1 1} {2 2}] []main.Cell[int;string]
// [{a a}] []main.Cell[string;string]
// true false
