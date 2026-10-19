// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type T struct{ A int }

func (t *T) Inc()    { t.A++ }
func (t T) Get() int { return t.A }

type I interface{ Get() int }

func main() {
	t := &T{1}
	var i I = t
	v := reflect.ValueOf(i)
	fmt.Println(v.Kind(), v.Type(), v.Elem().Kind(), v.Elem().Field(0).Int())
	v.Elem().Field(0).SetInt(10)
	fmt.Println(t.A, i.Get())
	var j I = T{5}
	w := reflect.ValueOf(j)
	fmt.Println(w.Kind(), w.Type(), w.CanAddr(), w.CanSet(), w.Field(0).CanSet())
	var np *T
	var k I = np
	nv := reflect.ValueOf(k)
	fmt.Println(nv.Kind(), nv.IsNil(), nv.Type(), k == nil)
	var arr any = &[3]int{1, 2, 3}
	av := reflect.ValueOf(arr)
	fmt.Println(av.Kind(), av.Elem().Len(), av.Elem().Index(2), av.Len())
	av.Elem().Index(0).SetInt(100)
	fmt.Println(*arr.(*[3]int))
	var m any = map[string]int{"a": 1}
	reflect.ValueOf(m).SetMapIndex(reflect.ValueOf("b"), reflect.ValueOf(2))
	fmt.Println(len(m.(map[string]int)))
}

// Output:
// ptr *main.T struct 1
// 10 10
// struct main.T false false false
// ptr true *main.T false
// ptr 3 3 3
// [100 2 3]
// 2
