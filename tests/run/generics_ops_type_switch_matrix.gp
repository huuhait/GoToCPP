// phase: 07

// Type switches and assertions on values of a type parameter, for many type arguments.
package main

import "fmt"

type Celsius float64
type Pair struct{ A, B int }
type Shape interface{ Area() int }
type Sq int

func (s Sq) Area() int { return int(s * s) }

func Kind[T any](x T) string {
	switch v := any(x).(type) {
	case nil:
		return "nil"
	case int:
		return fmt.Sprint("int ", v+1)
	case int8, uint8:
		return fmt.Sprintf("small %v", v)
	case float64:
		return fmt.Sprint("float64 ", v*2)
	case Celsius:
		return fmt.Sprint("celsius ", v)
	case string:
		return "string " + v
	case []int:
		return fmt.Sprint("ints ", len(v))
	case map[string]int:
		return fmt.Sprint("map ", len(v))
	case Pair:
		return fmt.Sprint("pair ", v.A+v.B)
	case *Pair:
		return fmt.Sprint("ptr pair ", v.A)
	case func() int:
		return fmt.Sprint("func ", v())
	case error:
		return "error " + v.Error()
	case Shape:
		return fmt.Sprint("shape ", v.Area())
	case fmt.Stringer:
		return "stringer " + v.String()
	}
	return fmt.Sprintf("other %T", x)
}

func Is[T, U any](x T) bool {
	_, ok := any(x).(U)
	return ok
}

func Cast[U, T any](x T) (U, bool) {
	u, ok := any(x).(U)
	return u, ok
}

func MustCast[U, T any](x T) U { return any(x).(U) }

func main() {
	fmt.Println(Kind(1), Kind(int8(2)), Kind(uint8(3)), Kind(2.5), Kind(Celsius(7)), Kind("s"), Kind([]int{1, 2}), Kind(map[string]int{}))
	fmt.Println(Kind(Pair{1, 2}), Kind(&Pair{5, 0}), Kind(func() int { return 9 }), Kind(fmt.Errorf("boom")), Kind(Sq(4)), Kind[any](nil), Kind[error](nil))
	fmt.Println(Kind(true), Kind([2]int{}), Kind(int64(1)), Kind[Shape](Sq(3)), Kind[*int](nil), Kind[any](Sq(2)))
	fmt.Println(Is[int, int](1), Is[int, any](1), Is[int, int64](1), Is[Sq, Shape](1), Is[int, Shape](1), Is[any, int](1), Is[any, int]("x"), Is[Shape, Sq](Sq(1)), Is[Shape, Sq](nil))
	v, ok := Cast[string](any("x"))
	w, ok2 := Cast[int](any("x"))
	s, ok3 := Cast[Shape](Sq(3))
	fmt.Println(v, ok, w, ok2, s.Area(), ok3)
	defer func() { fmt.Println("recovered:", recover()) }()
	fmt.Println(MustCast[int](5))
	fmt.Println(MustCast[Shape](Sq(2)).Area())
	fmt.Println(MustCast[string](5))
}

// Output:
// int 2 small 2 small 3 float64 5 celsius 7 string s ints 2 map 0
// pair 3 ptr pair 5 func 9 error boom shape 16 nil nil
// other bool other [2]int other int64 shape 9 other *int shape 4
// true true false true false true false true false
// x true 0 false 9 true
// 5
// 4
// recovered: interface conversion: interface {} is int, not string
