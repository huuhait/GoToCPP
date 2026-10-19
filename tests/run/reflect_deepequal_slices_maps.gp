// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct{ X, Y int }

func main() {
	fmt.Println(reflect.DeepEqual([]int{1, 2, 3}, []int{1, 2, 3}), reflect.DeepEqual([]int{1, 2, 3}, []int{1, 2}), reflect.DeepEqual([]int{1, 2, 3}, []int{1, 2, 4}))
	fmt.Println(reflect.DeepEqual([]int(nil), []int{}), reflect.DeepEqual([]int{}, []int{}), reflect.DeepEqual([]int(nil), []int(nil)))
	fmt.Println(reflect.DeepEqual([]byte("ab"), []byte("ab")), reflect.DeepEqual([]byte(nil), []byte{}), reflect.DeepEqual([]byte("ab"), []byte("ac")))
	fmt.Println(reflect.DeepEqual([][]int{{1}, {2, 3}}, [][]int{{1}, {2, 3}}), reflect.DeepEqual([][]int{{1}, nil}, [][]int{{1}, {}}))
	fmt.Println(reflect.DeepEqual(map[string]int{"a": 1, "b": 2}, map[string]int{"b": 2, "a": 1}), reflect.DeepEqual(map[string]int{"a": 1}, map[string]int{"a": 2}), reflect.DeepEqual(map[string]int{"a": 1}, map[string]int{"b": 1}))
	fmt.Println(reflect.DeepEqual(map[string]int(nil), map[string]int{}), reflect.DeepEqual(map[string]int{}, map[string]int{}), reflect.DeepEqual(map[string]int(nil), map[string]int(nil)))
	fmt.Println(reflect.DeepEqual(map[string][]int{"a": {1}}, map[string][]int{"a": {1}}), reflect.DeepEqual(map[string][]int{"a": nil}, map[string][]int{"a": {}}))
	fmt.Println(reflect.DeepEqual(map[Pt]string{{1, 2}: "x"}, map[Pt]string{{1, 2}: "x"}), reflect.DeepEqual(map[Pt]*Pt{{1, 2}: {3, 4}}, map[Pt]*Pt{{1, 2}: {3, 4}}))
	fmt.Println(reflect.DeepEqual(map[any]int{1: 1, "a": 2}, map[any]int{"a": 2, 1: 1}), reflect.DeepEqual(map[any]int{1: 1}, map[any]int{int8(1): 1}))
	a := []int{1, 2, 3}
	fmt.Println(reflect.DeepEqual(a, a), reflect.DeepEqual(a[:2], a[:2]), reflect.DeepEqual(a[:2], a[1:]))
	m := map[string]int{"a": 1}
	fmt.Println(reflect.DeepEqual(m, m), reflect.DeepEqual([]any{1, "a", nil, 2.5}, []any{1, "a", nil, 2.5}), reflect.DeepEqual([]any{1}, []any{int64(1)}))
	fmt.Println(reflect.DeepEqual([]string{"a", "b"}, []string{"a", "b"}), reflect.DeepEqual([]string{"a"}, []any{"a"}))
}

// Output:
// true false false
// false true true
// true false false
// true false
// true false false
// false true true
// true false
// true true
// true false
// true true false
// true true false
// true false
