// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct{ X, Y int }

func main() {
	s := reflect.ValueOf([]int{1, 2})
	s2 := reflect.Append(s, reflect.ValueOf(3), reflect.ValueOf(4))
	fmt.Println(s.Len(), s2.Len(), s2.Interface(), s2.CanSet(), s2.CanAddr())
	var nilS []string
	r := reflect.Append(reflect.ValueOf(nilS), reflect.ValueOf("a"))
	fmt.Println(r.Interface(), r.Len())
	pts := []Pt{{1, 1}}
	pv := reflect.Append(reflect.ValueOf(pts), reflect.ValueOf(Pt{2, 2}))
	fmt.Println(pv.Interface())
	// append sharing: room in the array
	base := make([]int, 2, 10)
	b2 := reflect.Append(reflect.ValueOf(base), reflect.ValueOf(7))
	b3 := reflect.Append(reflect.ValueOf(base), reflect.ValueOf(8))
	fmt.Println(b2.Index(2), b3.Index(2), b2.Cap())
	ptr := reflect.ValueOf(&base).Elem()
	ptr.Set(reflect.Append(ptr, reflect.ValueOf(1), reflect.ValueOf(2)))
	fmt.Println(base)
	as := reflect.AppendSlice(reflect.ValueOf([]string{"a"}), reflect.ValueOf([]string{"b", "c"}))
	fmt.Println(as.Interface(), as.Len())
	grown := reflect.ValueOf([]int(nil))
	for i := 0; i < 100; i++ {
		grown = reflect.Append(grown, reflect.ValueOf(i))
	}
	fmt.Println(grown.Len(), grown.Index(99), grown.Cap() >= 100)
	ifs := reflect.Append(reflect.ValueOf([]any{}), reflect.ValueOf(1), reflect.ValueOf("x"), reflect.Zero(reflect.TypeOf((*any)(nil)).Elem()))
	fmt.Println(ifs.Len(), ifs.Index(0).Elem().Kind(), ifs.Index(2).IsNil())
	for _, f := range []func(){
		func() { reflect.Append(reflect.ValueOf(1)) },
		func() { reflect.Append(s, reflect.ValueOf("x")) },
		func() { reflect.AppendSlice(s, reflect.ValueOf([]string{"x"})) },
	} {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			f()
		}()
	}
}

// Output:
// 2 4 [1 2 3 4] false false
// [a] 1
// [{1 1} {2 2}]
// 8 8 10
// [0 0 1 2]
// [a b c] 3
// 100 99 true
// 3 int true
// recovered: reflect: call of unknown method on int Value
// recovered: reflect.Set: value of type string is not assignable to type int
// recovered: reflect.AppendSlice: int != string
