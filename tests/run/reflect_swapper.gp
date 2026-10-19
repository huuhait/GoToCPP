// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct {
	X int
	S string
}

func bubble(s any, less func(i, j int) bool) {
	n := reflect.ValueOf(s).Len()
	sw := reflect.Swapper(s)
	for i := 0; i < n; i++ {
		for j := 0; j+1 < n-i; j++ {
			if less(j+1, j) {
				sw(j, j+1)
			}
		}
	}
}

func main() {
	ints := []int{5, 2, 9, 1, 7}
	bubble(ints, func(i, j int) bool { return ints[i] < ints[j] })
	fmt.Println(ints)
	strs := []string{"pear", "apple", "fig", "banana"}
	bubble(strs, func(i, j int) bool { return strs[i] < strs[j] })
	fmt.Println(strs)
	pts := []Pt{{3, "c"}, {1, "a"}, {2, "b"}}
	bubble(pts, func(i, j int) bool { return pts[i].X < pts[j].X })
	fmt.Println(pts)
	ptrs := []*Pt{{3, "c"}, {1, "a"}}
	bubble(ptrs, func(i, j int) bool { return ptrs[i].X < ptrs[j].X })
	fmt.Println(*ptrs[0], *ptrs[1])
	anys := []any{3, "x", 2.5}
	reflect.Swapper(anys)(0, 2)
	fmt.Println(anys)
	big := make([][100]int, 2)
	big[0][0], big[1][0] = 1, 2
	reflect.Swapper(big)(0, 1)
	fmt.Println(big[0][0], big[1][0])
	bytes := []byte("abc")
	reflect.Swapper(bytes)(0, 2)
	fmt.Println(string(bytes))
	one := []int{1}
	reflect.Swapper(one)(0, 0)
	fmt.Println(one)
	for _, f := range []func(){
		func() { reflect.Swapper(ints)(0, 5) }, func() { reflect.Swapper(ints)(-1, 0) }, func() { reflect.Swapper([]int{})(0, 0) },
		func() { reflect.Swapper(1) }, func() { reflect.Swapper(nil) }, func() { reflect.Swapper([3]int{}) },
	} {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			f()
		}()
	}
}

// Output:
// [1 2 5 7 9]
// [apple banana fig pear]
// [{1 a} {2 b} {3 c}]
// {1 a} {3 c}
// [2.5 x 3]
// 2 1
// cba
// [1]
// recovered: runtime error: index out of range [5] with length 5
// recovered: runtime error: index out of range [-1]
// recovered: reflect: slice index out of range
// recovered: reflect: call of Swapper on int Value
// recovered: reflect: call of Swapper on zero Value
// recovered: reflect: call of Swapper on array Value
