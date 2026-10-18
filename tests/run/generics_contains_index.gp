// phase: 07

// Contains and Index on comparable elements: ints, strings, structs, arrays, pointers and interfaces.
package main

import "fmt"

func Index[T comparable](xs []T, v T) int {
	for i, x := range xs {
		if x == v {
			return i
		}
	}
	return -1
}

func Contains[T comparable](xs []T, v T) bool { return Index(xs, v) >= 0 }

type Point struct{ X, Y int }

type Shape interface{ Area() int }
type Sq struct{ s int }

func (q Sq) Area() int { return q.s * q.s }

func main() {
	fmt.Println(Index([]int{5, 6, 7}, 7), Index([]string{"a"}, "b"))
	fmt.Println(Contains([]Point{{1, 2}, {3, 4}}, Point{3, 4}), Contains([]Point{{1, 2}}, Point{2, 1}))
	fmt.Println(Contains([][2]int{{1, 2}}, [2]int{1, 2}))
	a, b := new(int), new(int)
	fmt.Println(Contains([]*int{a}, a), Contains([]*int{a}, b))
	fmt.Println(Index([]any{1, "x", 2.5, nil}, any("x")), Index([]any{1, "x", nil}, nil))
	fmt.Println(Index([]Shape{Sq{1}, Sq{2}}, Shape(Sq{2})), Contains([]Shape{Sq{1}}, Shape(nil)))
	fmt.Println(Index([]error{nil}, nil), Contains([]float64{1.5}, 1.5), Contains([]bool{false}, true))
	fmt.Println(Index([]rune("héllo"), 'l'), Index([]byte("abc"), 'c'))
}

// Output:
// 2 -1
// true false
// true
// true false
// 1 2
// 1 false
// 0 true false
// 2 2
