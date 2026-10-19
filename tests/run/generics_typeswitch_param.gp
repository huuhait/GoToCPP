// phase: 08

// Type switches on values of generic type: switch any(v).(type) with the type argument's own case,
// defined types, instances of generic types, composite types made of T, and the difference between
// int and int64 and between byte and uint8 (the same type) and rune and int32 (the same type).
package main

import "fmt"

type Box[T any] struct{ V T }
type Pair[A, B any] struct {
	A A
	B B
}
type MyInt int
type Celsius float64

func Kind[T any](v T) string {
	switch x := any(v).(type) {
	case nil:
		return "nil"
	case int:
		return fmt.Sprint("int ", x)
	case int64:
		return fmt.Sprint("int64 ", x)
	case int8:
		return fmt.Sprint("int8 ", x)
	case uint8:
		return fmt.Sprint("uint8/byte ", x)
	case int32:
		return fmt.Sprint("int32/rune ", x)
	case float64:
		return fmt.Sprint("float64 ", x)
	case string:
		return fmt.Sprint("string ", x)
	case bool:
		return fmt.Sprint("bool ", x)
	case MyInt:
		return fmt.Sprint("MyInt ", int(x))
	case Celsius:
		return fmt.Sprint("Celsius ", float64(x))
	case []int:
		return fmt.Sprint("[]int ", len(x))
	case []T:
		return fmt.Sprint("[]T ", len(x))
	case *T:
		return fmt.Sprint("*T ", x != nil)
	case Box[int]:
		return fmt.Sprint("Box[int] ", x.V)
	case Box[string], Box[bool]:
		return fmt.Sprintf("Box[string|bool] %v", x)
	case *Box[int]:
		return fmt.Sprint("*Box[int] ", x.V)
	case Pair[int, string]:
		return fmt.Sprint("Pair[int,string] ", x.A, x.B)
	case Pair[string, int]:
		return fmt.Sprint("Pair[string,int] ", x.A, x.B)
	case map[string]T:
		return fmt.Sprint("map[string]T ", len(x))
	case func(T) T:
		return "func(T) T"
	case chan T:
		return "chan T"
	case error:
		return "error " + x.Error()
	case fmt.Stringer:
		return "Stringer " + x.String()
	default:
		return fmt.Sprintf("other %T", x)
	}
}

func Same[A, B any](a A, b B) bool {
	_, ok := any(a).(B)
	return ok
}

func Is[T any](v any) bool {
	_, ok := v.(T)
	return ok
}

func main() {
	fmt.Println(Kind(1), Kind(int64(2)), Kind(int8(3)), Kind(uint8(4)), Kind(byte(5)), Kind('x'), Kind(rune(6)), Kind(int32(7)))
	fmt.Println(Kind(1.5), Kind("s"), Kind(true), Kind(MyInt(8)), Kind(Celsius(36.6)), Kind([]int{1, 2}))
	fmt.Println(Kind([]string{"a"}), Kind(Box[int]{9}), Kind(Box[string]{"b"}), Kind(Box[bool]{true}), Kind(Box[float64]{1}))
	fmt.Println(Kind(&Box[int]{10}), Kind(Pair[int, string]{1, "a"}), Kind(Pair[string, int]{"b", 2}), Kind(Pair[int, int]{}))
	fmt.Println(Kind(map[string]int{"a": 1}), Kind(map[string]string{}), Kind(func(int) int { return 0 }), Kind(make(chan int)), Kind(make(chan string)))
	fmt.Println(Kind(fmt.Errorf("oops")), Kind[any](nil), Kind[error](nil), Kind[any](3), Kind[any](MyInt(3)), Kind[fmt.Stringer](nil))
	var p *int
	fmt.Println(Kind(p), Kind(new(string)), Kind([3]int{}), Kind(struct{}{}), Kind(uint(1)), Kind(uintptr(1)), Kind(float32(1)))
	fmt.Println(Same(1, 2), Same(1, int64(2)), Same(byte(1), uint8(2)), Same('a', int32(1)), Same(MyInt(1), 2), Same(Box[int]{}, Box[int]{}), Same(Box[int]{}, Box[int8]{}))
	fmt.Println(Is[int](1), Is[int64](1), Is[Box[int]](Box[int]{}), Is[Box[any]](Box[int]{}), Is[any](nil), Is[fmt.Stringer](MyInt(1)), Is[error](fmt.Errorf("x")))
}

// Output:
// int 1 int64 2 int8 3 uint8/byte 4 uint8/byte 5 int32/rune 120 int32/rune 6 int32/rune 7
// float64 1.5 string s bool true MyInt 8 Celsius 36.6 []int 2
// other []string Box[int] 9 Box[string|bool] {b} Box[string|bool] {true} other main.Box[float64]
// *Box[int] 10 Pair[int,string] 1a Pair[string,int] b2 other main.Pair[int,int]
// other map[string]int other map[string]string other func(int) int other chan int other chan string
// error oops nil nil int 3 MyInt 3 nil
// other *int other *string other [3]int other struct {} other uint other uintptr other float32
// true false true true false true false
// true false true false false false true
