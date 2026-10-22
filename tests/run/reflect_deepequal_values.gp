// phase: 09

// DeepEqual of two reflect.Value arguments (step 9.3 lows). Go compares the Value struct: its pointer
// field is the pointer itself for a pointer, a map, a channel and a func, so two Values of the same
// one are deeply equal, and the Elem or the Field of the same object has the same storage; a number,
// a string, a struct or a slice computed at run time is boxed anew by every ValueOf, so the
// storage differs. Go shares the box of a constant (and of a number below 256), so there
// DeepEqual(ValueOf(1), ValueOf(1)) is true, and gocpp says false: the rows of such constants are
// not here (ADR-035, item 10).
package main

import (
	"fmt"
	"os"
	"reflect"
)

type P struct{ X, Y int }

func eq(name string, a, b reflect.Value) {
	fmt.Println(name, reflect.DeepEqual(a, b))
}

func main() {
	n := len(os.Args) + 999
	s := fmt.Sprint(n)
	f := float64(n) + 0.5
	x := n
	var nilmap map[int]int
	m := map[int]int{1: 2}
	fn := func() int { return n }
	ch := make(chan int)
	p := P{n, 2}
	sl := []int{1, 2}
	st := &struct{ A, B int }{n, n}

	eq("nil map const", reflect.ValueOf(map[int]int(nil)), reflect.ValueOf(map[int]int(nil)))
	eq("run-time big int", reflect.ValueOf(n), reflect.ValueOf(n))
	eq("run-time string", reflect.ValueOf(s), reflect.ValueOf(s))
	eq("run-time float", reflect.ValueOf(f), reflect.ValueOf(f))
	eq("run-time struct", reflect.ValueOf(p), reflect.ValueOf(p))
	eq("run-time slice", reflect.ValueOf(sl), reflect.ValueOf(sl))
	eq("same pointer", reflect.ValueOf(&x), reflect.ValueOf(&x))
	eq("same map", reflect.ValueOf(m), reflect.ValueOf(m))
	eq("nil map", reflect.ValueOf(nilmap), reflect.ValueOf(nilmap))
	eq("same func", reflect.ValueOf(fn), reflect.ValueOf(fn))
	eq("same chan", reflect.ValueOf(ch), reflect.ValueOf(ch))
	v := reflect.ValueOf(n)
	eq("copy of v", v, v)
	eq("pointer elem", reflect.ValueOf(&x).Elem(), reflect.ValueOf(&x).Elem())
	eq("struct field", reflect.ValueOf(st).Elem().Field(0), reflect.ValueOf(st).Elem().Field(0))
	eq("other pointer", reflect.ValueOf(&x), reflect.ValueOf(&n))
	eq("other map", reflect.ValueOf(m), reflect.ValueOf(map[int]int{1: 2}))
	eq("other chan", reflect.ValueOf(ch), reflect.ValueOf(make(chan int)))
	eq("other kind", reflect.ValueOf(&x), reflect.ValueOf(m))
	eq("zero values", reflect.Value{}, reflect.Value{})
	eq("zero and one", reflect.Value{}, reflect.ValueOf(&x))
	eq("pointer elem and field of other", reflect.ValueOf(&x).Elem(), reflect.ValueOf(&n).Elem())
}

// Output:
// nil map const true
// run-time big int false
// run-time string false
// run-time float false
// run-time struct false
// run-time slice false
// same pointer true
// same map true
// nil map true
// same func true
// same chan true
// copy of v true
// pointer elem true
// struct field true
// other pointer false
// other map false
// other chan false
// other kind false
// zero values true
// zero and one false
// pointer elem and field of other false
