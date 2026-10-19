// phase: 09

package main

import (
	"errors"
	"fmt"
	"reflect"
)

type Basic struct {
	x int
	y float32
}
type NotBasic Basic
type DeepEqualTest struct {
	a, b any
	eq   bool
}

type MyInt int
type MyStr string
type Inner struct{ I *int }
type Wrap struct {
	In Inner
	S  []Inner
}

var (
	fn1 func()
	fn2 func()
	fn3 = func() { fmt.Println("test") }
	i1  = 5
	i2  = 5
	i3  = 6
)

var table = []DeepEqualTest{
	// equal
	{1, 1, true}, {int32(1), int32(1), true}, {0.5, 0.5, true}, {float32(0.5), float32(0.5), true}, {"hello", "hello", true},
	{make([]int, 10), make([]int, 10), true}, {&[3]int{1, 2, 3}, &[3]int{1, 2, 3}, true},
	{Basic{1, 0.5}, Basic{1, 0.5}, true}, {error(nil), error(nil), true},
	{map[int]string{1: "one", 2: "two"}, map[int]string{2: "two", 1: "one"}, true},
	{fn1, fn2, true}, {[]byte{1, 2, 3}, []byte{1, 2, 3}, true}, {MyStr("a"), MyStr("a"), true},
	{[]int{}, []int{}, true}, {map[string]int{}, map[string]int{}, true},
	{&i1, &i2, true}, {[]*int{&i1}, []*int{&i2}, true}, {Wrap{Inner{&i1}, []Inner{{&i1}}}, Wrap{Inner{&i2}, []Inner{{&i2}}}, true},
	{[]any{1, "a", nil}, []any{1, "a", nil}, true}, {[2][]int{{1}, {2}}, [2][]int{{1}, {2}}, true},
	{map[string][]int{"a": {1, 2}}, map[string][]int{"a": {1, 2}}, true}, {struct{}{}, struct{}{}, true},
	{[0]int{}, [0]int{}, true}, {complex64(1 + 2i), complex64(1 + 2i), true}, {uintptr(3), uintptr(3), true},
	{errors.New("x"), errors.New("x"), true}, {[]error{nil}, []error{nil}, true}, {'a', 'a', true},
	{map[int]map[int]int{1: {2: 3}}, map[int]map[int]int{1: {2: 3}}, true},
	{[]map[string]int{{"a": 1}}, []map[string]int{{"a": 1}}, true}, {&struct{ A []int }{[]int{1}}, &struct{ A []int }{[]int{1}}, true},
	// not equal
	{1, 2, false}, {int32(1), int64(1), false}, {0.5, "hello", false}, {[]int{1, 2, 3}, [3]int{1, 2, 3}, false},
	{&[3]any{1, 2, 3}, &[3]any{1, 2, [3]int{4, 5, 6}}, false}, {Basic{1, 0.5}, Basic{1, 0.6}, false}, {Basic{1, 0}, Basic{2, 0}, false},
	{Basic{1, 0.5}, NotBasic{1, 0.5}, false}, {map[uint]string{1: "one", 2: "two"}, map[int]string{2: "two", 1: "one"}, false},
	{map[int]string{1: "one", 3: "two"}, map[int]string{2: "two", 1: "one"}, false}, {map[int]string{1: "one", 2: "txo"}, map[int]string{2: "two", 1: "one"}, false},
	{map[int]string{1: "one"}, map[int]string{2: "two", 1: "one"}, false}, {map[int]string{2: "two", 1: "one"}, map[int]string{1: "one"}, false},
	{nil, 1, false}, {1, nil, false}, {fn1, fn3, false}, {fn3, fn3, false}, {[][]int{{1}}, [][]int{{2}}, false},
	{&struct{ a int }{1}, &struct{ a int }{2}, false}, {Basic{0, 0.5}, Basic{0, 0.5000001}, false},
	{[]int{}, []int(nil), false}, {map[string]int{}, map[string]int(nil), false}, {[]byte{}, []byte(nil), false}, {&i1, &i3, false},
	{[]any{1, 2}, []any{1, int64(2)}, false}, {MyInt(1), 1, false}, {MyStr("a"), "a", false}, {[]MyInt{1}, []int{1}, false},
	{[3]int{1, 2, 3}, [3]int{1, 2, 4}, false}, {[]int{1, 2, 3}, []int{1, 2}, false},
	{Wrap{Inner{&i1}, nil}, Wrap{Inner{&i3}, nil}, false}, {[]any{[]int{1}}, []any{[]int{2}}, false},
	{map[string][]int{"a": {1}}, map[string][]int{"a": {2}}, false}, {&[]int{1}, &[]int{2}, false},
	{[2]bool{true, false}, [2]bool{true, true}, false}, {complex(1, 2), complex(1, 3), false}, {"a", "b", false},
	{struct{ A int }{1}, struct{ B int }{1}, false}, {int8(1), uint8(1), false}, {1.0, float32(1), false},
}

func main() {
	bad := 0
	for i, tt := range table {
		got := reflect.DeepEqual(tt.a, tt.b)
		if got != tt.eq {
			fmt.Println("mismatch", i, got, tt.eq)
			bad++
		}
	}
	fmt.Println(len(table), "cases,", bad, "mismatches")
	for i, tt := range table {
		fmt.Print(reflect.DeepEqual(tt.a, tt.b), ";")
		if i%20 == 19 {
			fmt.Println()
		}
	}
	fmt.Println()
}

// Output:
// 71 cases, 0 mismatches
// true;true;true;true;true;true;true;true;true;true;true;true;true;true;true;true;true;true;true;true;
// true;true;true;true;true;true;true;true;true;true;true;false;false;false;false;false;false;false;false;false;
// false;false;false;false;false;false;false;false;false;false;false;false;false;false;false;false;false;false;false;false;
// false;false;false;false;false;false;false;false;false;false;false;
