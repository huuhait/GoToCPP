// phase: 07

// any(x) of a type parameter value boxes the Go type of the argument: int and int64 differ though both are 64 bits.
package main

import "fmt"

type MyInt int
type Celsius float64
type Pair struct{ a, b int }

func Box[T any](v T) any { return v }

func Type[T any](v T) string { return fmt.Sprintf("%T", v) }

func TypeOf[T any]() string {
	var z T
	return fmt.Sprintf("%T", z)
}

func Same[T, U any](a T, b U) bool { return any(a) == any(b) }

func Wrap[T any](vs ...T) []any {
	out := make([]any, len(vs))
	for i, v := range vs {
		out[i] = v
	}
	return out
}

func main() {
	fmt.Println(Type(1), Type(int64(1)), Type(int32(1)), Type(MyInt(1)), Type(uint8(1)), Type('a'), Type(byte(1)))
	fmt.Println(Type(1.5), Type(float32(1)), Type(Celsius(1)), Type("s"), Type(true), Type(1i), Type(Pair{}))
	fmt.Println(Type([]int{}), Type([]MyInt{}), Type(map[string][]int{}), Type([2]bool{}), Type(&Pair{}), Type((*int)(nil)))
	fmt.Println(Type(func(int) string { return "" }), Type(struct{ X int }{}), Type([]any{}), Type(any(nil)), Type[error](nil))
	fmt.Println(TypeOf[int](), TypeOf[MyInt](), TypeOf[*Pair](), TypeOf[[]string](), TypeOf[any](), TypeOf[error](), TypeOf[fmt.Stringer]())
	fmt.Println(Same(1, 1), Same(1, int64(1)), Same(MyInt(1), 1), Same("a", "a"), Same[any, any](1, 1), Same[any, any](1, "1"))
	fmt.Println(Box(1) == Box(1), Box(1) == Box(int64(1)), Box(Pair{1, 2}) == Box(Pair{1, 2}))
	fmt.Printf("%v %v %v\n", Box(MyInt(5)), Box(Celsius(2.5)), Box(Pair{1, 2}))
	fmt.Printf("%d %v %s\n", Box(7), Wrap(1, 2), Wrap("a", "b"))
	fmt.Printf("%T %T\n", Wrap(MyInt(1))[0], Wrap[any](nil)[0])
}

// Output:
// int int64 int32 main.MyInt uint8 int32 uint8
// float64 float32 main.Celsius string bool complex128 main.Pair
// []int []main.MyInt map[string][]int [2]bool *main.Pair *int
// func(int) string struct { X int } []interface {} <nil> <nil>
// int main.MyInt *main.Pair []string <nil> <nil> <nil>
// true false false true true false
// true false true
// 5 2.5 {1 2}
// 7 [1 2] [a b]
// main.MyInt <nil>
