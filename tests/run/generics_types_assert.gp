// phase: 07

// Type assertions and type switches on instances: *Stack[int] and *Stack[string] are different types.
package main

import "fmt"

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

type Box[T any] struct{ v T }

func Kind(v any) string {
	switch x := v.(type) {
	case *Stack[int]:
		return fmt.Sprint("stack of int ", len(x.items))
	case *Stack[string]:
		return fmt.Sprint("stack of string ", len(x.items))
	case Box[int]:
		return fmt.Sprint("box of int ", x.v)
	case Box[string], Box[bool]:
		return fmt.Sprint("box of string or bool ", x)
	case Stack[int]:
		return "stack value"
	case nil:
		return "nil"
	}
	return fmt.Sprintf("other %T", v)
}

func main() {
	s := &Stack[int]{}
	s.Push(1)
	t := &Stack[string]{}
	fmt.Println(Kind(s), Kind(t), Kind(Box[int]{5}), Kind(Box[string]{"b"}), Kind(Box[bool]{true}))
	fmt.Println(Kind(Stack[int]{}), Kind(Box[float64]{}), Kind(nil), Kind(&Stack[bool]{}))
	var a any = s
	_, ok1 := a.(*Stack[int])
	_, ok2 := a.(*Stack[string])
	_, ok3 := a.(Stack[int])
	fmt.Println(ok1, ok2, ok3)
	st := a.(*Stack[int])
	st.Push(2)
	fmt.Println(len(s.items))
	defer func() { fmt.Println(recover()) }()
	_ = a.(*Stack[string])
}

// Output:
// stack of int 1 stack of string 0 box of int 5 box of string or bool {b} box of string or bool {true}
// stack value other main.Box[float64] nil other *main.Stack[bool]
// true false false
// 2
// interface conversion: interface {} is *main.Stack[int], not *main.Stack[string]
