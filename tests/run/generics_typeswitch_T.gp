// phase: 07

// Type switch cases that mention the type parameter: case T, []T, map[string]T, func() T.
package main

import (
	"errors"
	"fmt"
)

func Kind[T any](v any) string {
	switch x := v.(type) {
	case T:
		return fmt.Sprint("T:", x)
	case []T:
		return fmt.Sprint("[]T:", len(x))
	case map[string]T:
		return fmt.Sprint("map:", len(x))
	case func() T:
		return "func"
	case *T:
		return "ptr"
	case nil:
		return "nil"
	case error:
		return "error"
	}
	return "other"
}

func Is[T any](v any) bool {
	_, ok := v.(T)
	return ok
}

func Both[T any](v T) string {
	switch any(v).(type) {
	case T:
		return "always"
	}
	return "never"
}

func Dup[T any](v T) string {
	switch any(v).(type) {
	case int:
		return "int"
	case T:
		return "T"
	case string:
		return "string"
	}
	return "?"
}

func main() {
	fmt.Println(Kind[int](5), Kind[int]([]int{1, 2}), Kind[int](map[string]int{}), Kind[int](func() int { return 1 }))
	x := 3
	fmt.Println(Kind[int](&x), Kind[int](nil), Kind[int](errors.New("e")), Kind[int]("s"), Kind[int](int8(1)))
	fmt.Println(Kind[string]("s"), Kind[error](errors.New("e")), Kind[any](5), Kind[fmt.Stringer](5))
	fmt.Println(Is[int](1), Is[int8](1), Is[any](nil), Is[error](nil), Is[error](errors.New("e")), Is[[]int]([]int(nil)))
	fmt.Println(Both(1), Both("x"), Both[any](nil), Both[error](nil))
	fmt.Println(Dup(1), Dup("s"), Dup(2.5), Dup[any](1), Dup[any]("s"), Dup[any](2.5))
}

// Output:
// T:5 []T:2 map:0 func
// ptr nil error other other
// T:s T:e T:5 other
// true false false false true true
// always always never never
// int T T int T T
