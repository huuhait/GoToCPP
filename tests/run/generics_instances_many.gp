// phase: 07

// One generic function instantiated with many different types, and the same instance used again and again.
package main

import "fmt"

type Celsius float64
type ID int
type Point struct{ X, Y int }

func Echo[T any](v T) T { return v }

func Dup[T any](v T) [2]T { return [2]T{v, v} }

func Wrap[T any](v T) []T { return []T{v} }

func Fmt[T any](v T) string { return fmt.Sprintf("%v:%T", v, v) }

func main() {
	fmt.Println(Echo(1), Echo(int8(2)), Echo(int16(3)), Echo(int32(4)), Echo(int64(5)), Echo(uint(6)), Echo(uint8(7)))
	fmt.Println(Echo(uint16(8)), Echo(uint32(9)), Echo(uint64(10)), Echo(1.5), Echo(float32(2.5)), Echo(true), Echo("s"))
	fmt.Println(Echo(Celsius(1)), Echo(ID(2)), Echo(Point{1, 2}), Echo([]int{1}), Echo(map[string]int{"a": 1}), Echo(&Point{3, 4}) != nil)
	fmt.Println(Echo([2]int{1, 2}), Echo(struct{ A int }{5}), Echo('x'), Echo(1i), Echo[any](nil), Echo[error](nil))
	fmt.Println(Dup(1), Dup("a"), Dup(Point{}), Dup([]int(nil)), Dup(Dup(1.5)))
	fmt.Println(Wrap(1), Wrap(Wrap("x")), Wrap(Wrap(Wrap(true))), len(Wrap[any](nil)))
	for i := 0; i < 3; i++ {
		fmt.Print(Echo(i), Echo(i*2), ";")
	}
	fmt.Println()
	fmt.Println(Fmt(1), Fmt(int64(1)), Fmt(ID(1)), Fmt(Celsius(1)), Fmt(Dup(1)), Fmt(Wrap(Point{})))
}

// Output:
// 1 2 3 4 5 6 7
// 8 9 10 1.5 2.5 true s
// 1 2 {1 2} [1] map[a:1] true
// [1 2] {5} 120 (0+1i) <nil> <nil>
// [1 1] [a a] [{0 0} {0 0}] [[] []] [[1.5 1.5] [1.5 1.5]]
// [1] [[x]] [[[true]]] 1
// 0 0;1 2;2 4;
// 1:int 1:int64 1:main.ID 1:main.Celsius [1 1]:[2]int [{0 0}]:[]main.Point
