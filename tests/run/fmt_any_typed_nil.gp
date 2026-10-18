// phase: 05

// A typed nil pointer in an interface is not a nil interface: %v prints <nil> for it either way,
// %T tells them apart, a nil pointer with a value-receiver String method prints <nil>; %p of nil.
package main

import "fmt"

type T struct{ n int }

func (t *T) String() string {
	if t == nil {
		return "nil-T"
	}
	return "T"
}

type V struct{ n int }

func (v V) String() string { return "V" }

type E struct{}

func (e *E) Error() string { return "E" }

func main() {
	var t *T
	var v *V
	var e *E
	var a any = t
	var b any = v
	var c any
	var err error = e
	fmt.Println(t, v, a, b, c, err)
	fmt.Printf("%T %T %T %T\n", a, b, c, err)
	fmt.Println(a == nil, b == nil, c == nil, err == nil)
	fmt.Printf("%v %s %d %p %p\n", a, b, c, t, c)
	fmt.Printf("%+v %#v %#v %#v\n", a, a, b, c)
	var s fmt.Stringer = t
	fmt.Println(s, s == nil)
	var np *int
	fmt.Printf("%v %d %x %p %t\n", np, np, np, np, np)
	fmt.Printf("%v %p %#v %v\n", []*int{nil}, []int(nil), (*int)(nil), map[string]*int{"a": nil})
}

// Output:
// nil-T <nil> nil-T <nil> <nil> E
// *main.T *main.V <nil> *main.E
// false false true false
// nil-T <nil> %!d(<nil>) 0x0 %!p(<nil>)
// nil-T (*main.T)(nil) (*main.V)(nil) <nil>
// nil-T false
// <nil> 0 0 0x0 %!t(*int=<nil>)
// [<nil>] 0x0 (*int)(nil) map[a:<nil>]
