// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct{ X, Y int }
type Celsius float64
type IDs []int

func (c Celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

func main() {
	vals := []any{1, int8(2), uint(3), 4.5, "s", true, 'x', byte(1), Pt{1, 2}, &Pt{3, 4}, []int{1, 2}, [2]bool{true, false}, map[string]int{"a": 1},
		Celsius(36.6), IDs{7, 8}, complex(1, -1), struct{ A string }{"x"}, [][]int{{1}, {2, 3}}, map[Pt]string{{1, 2}: "p"}}
	for _, x := range vals {
		v := reflect.ValueOf(x)
		y := v.Interface()
		fmt.Printf("%T %v | %T %v | %v\n", x, x, y, y, reflect.DeepEqual(x, y))
	}
	p := &Pt{1, 2}
	pv := reflect.ValueOf(p)
	fmt.Println(pv.Interface().(*Pt) == p)
	e := reflect.ValueOf(p).Elem()
	q := e.Interface().(Pt)
	q.X = 99
	fmt.Println(p.X, q.X)
	e.Field(0).SetInt(5)
	fmt.Println(q.X, p.X, e.Interface())
	var any1 any = Pt{1, 2}
	var any2 any = reflect.ValueOf(any1).Interface()
	fmt.Println(any1 == any2)
}

// Output:
// int 1 | int 1 | true
// int8 2 | int8 2 | true
// uint 3 | uint 3 | true
// float64 4.5 | float64 4.5 | true
// string s | string s | true
// bool true | bool true | true
// int32 120 | int32 120 | true
// uint8 1 | uint8 1 | true
// main.Pt {1 2} | main.Pt {1 2} | true
// *main.Pt &{3 4} | *main.Pt &{3 4} | true
// []int [1 2] | []int [1 2] | true
// [2]bool [true false] | [2]bool [true false] | true
// map[string]int map[a:1] | map[string]int map[a:1] | true
// main.Celsius 36.6°C | main.Celsius 36.6°C | true
// main.IDs [7 8] | main.IDs [7 8] | true
// complex128 (1-1i) | complex128 (1-1i) | true
// struct { A string } {x} | struct { A string } {x} | true
// [][]int [[1] [2 3]] | [][]int [[1] [2 3]] | true
// map[main.Pt]string map[{1 2}:p] | map[main.Pt]string map[{1 2}:p] | true
// true
// 1 99
// 99 5 {5 2}
// true
