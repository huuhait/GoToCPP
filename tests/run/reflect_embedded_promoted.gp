// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type A struct{ X, Y int }
type B struct{ Y, Z int }
type Cc struct {
	A
	B
	W int
}
type D struct {
	Cc
	X string
}
type E struct {
	*A
	Q int
}

func main() {
	show := func(t reflect.Type, name string) {
		f, ok := t.FieldByName(name)
		fmt.Println(t, name, ok, f.Name, f.Index, f.Type)
	}
	tc := reflect.TypeOf(Cc{})
	for _, n := range []string{"X", "Y", "Z", "W", "A", "B", "Nope"} {
		show(tc, n)
	}
	td := reflect.TypeOf(D{})
	for _, n := range []string{"X", "Y", "Z", "W", "Cc", "A"} {
		show(td, n)
	}
	te := reflect.TypeOf(E{})
	for _, n := range []string{"X", "Y", "Q", "A"} {
		show(te, n)
	}
	d := D{Cc: Cc{A: A{1, 2}, B: B{3, 4}, W: 5}, X: "top"}
	v := reflect.ValueOf(d)
	fmt.Println(v.FieldByName("X"), v.FieldByName("Z"), v.FieldByName("W"), v.FieldByName("Y").IsValid())
	fmt.Println(v.FieldByIndex([]int{0, 0, 1}), v.FieldByIndex([]int{0, 1, 0}))
}

// Output:
// main.Cc X true X [0 0] int
// main.Cc Y false  [] <nil>
// main.Cc Z true Z [1 1] int
// main.Cc W true W [2] int
// main.Cc A true A [0] main.A
// main.Cc B true B [1] main.B
// main.Cc Nope false  [] <nil>
// main.D X true X [1] string
// main.D Y false  [] <nil>
// main.D Z true Z [0 1 1] int
// main.D W true W [0 2] int
// main.D Cc true Cc [0] main.Cc
// main.D A true A [0 0] main.A
// main.E X true X [0 0] int
// main.E Y true Y [0 1] int
// main.E Q true Q [1] int
// main.E A true A [0] *main.A
// top 4 5 false
// 2 3
