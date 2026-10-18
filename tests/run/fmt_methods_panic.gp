// phase: 05

// A panic in String, Error, GoString or Format is caught and printed as %!v(PANIC=String method:
// ...) with the panic value formatted by fmt; a nil pointer receiver prints <nil>; the value of a
// panic can be a string, an error, a float, a custom type.
package main

import "fmt"

type PS struct{}

func (PS) String() string { panic("boom") }

type PE struct{}

func (PE) Error() string { panic(fmt.Errorf("err %d", 7)) }

type PF struct{}

func (PF) String() string { panic(1.5) }

type PI struct{}

func (PI) String() string { panic(42) }

type PT struct{ a, b int }

func (PT) String() string { panic(struct{ a, b int }{1, 2}) }

type PN struct{}

func (PN) String() string {
	var m map[string]int
	m["a"] = 1
	return ""
}

type PX struct{}

func (PX) String() string {
	var a []int
	_ = a[3]
	return ""
}

type PG struct{}

func (PG) GoString() string { panic("gostring") }

type PFm struct{}

func (PFm) Format(f fmt.State, c rune) { panic("format") }

type NilRecv struct{ n int }

func (r *NilRecv) String() string { return fmt.Sprint(r.n) }

type ValRecv struct{ n int }

func (v ValRecv) String() string { return fmt.Sprint(v.n) }

type PD struct{}

func (PD) String() string {
	defer func() { recover() }()
	panic("recovered inside")
}

type Inner struct{ PS PS }

func main() {
	fmt.Println(PS{}, PE{}, PF{}, PI{}, PT{}, PN{}, PX{})
	fmt.Printf("%v|%s|%q|%x|%d|%10v|%-10s|\n", PS{}, PS{}, PS{}, PS{}, PS{}, PS{}, PS{})
	fmt.Printf("%#v|%+v|%v\n", PG{}, PG{}, PG{})
	fmt.Printf("%v|%d|%s\n", PFm{}, PFm{}, PFm{})
	fmt.Println([]PS{{}, {}}, map[string]PS{"k": {}}, Inner{}, &Inner{})
	var nr *NilRecv
	var vr *ValRecv
	fmt.Println(nr, vr)
	fmt.Printf("%v %s %d %+v|%10v|\n", nr, nr, nr, vr, nr)
	fmt.Println([]*NilRecv{nr}, []*ValRecv{vr, {3}})
	fmt.Println(PD{})
	fmt.Println(fmt.Sprint(PS{}), len(fmt.Sprintln(PS{})))
}

// Output:
// %!v(PANIC=String method: boom) %!v(PANIC=Error method: err 7) %!v(PANIC=String method: 1.5) %!v(PANIC=String method: 42) %!v(PANIC=String method: {1 2}) %!v(PANIC=String method: assignment to entry in nil map) %!v(PANIC=String method: runtime error: index out of range [3] with length 0)
// %!v(PANIC=String method: boom)|%!s(PANIC=String method: boom)|%!q(PANIC=String method: boom)|%!x(PANIC=String method: boom)|{}|%!v(PANIC=String method: boom)|%!s(PANIC=String method: boom)|
// %!v(PANIC=GoString method: gostring)|{}|{}
// %!v(PANIC=Format method: format)|%!d(PANIC=Format method: format)|%!s(PANIC=Format method: format)
// [%!v(PANIC=String method: boom) %!v(PANIC=String method: boom)] map[k:%!v(PANIC=String method: boom)] {%!v(PANIC=String method: boom)} &{%!v(PANIC=String method: boom)}
// <nil> <nil>
// <nil> <nil> 0 <nil>|<nil>|
// [<nil>] [<nil> 3]
//
// %!v(PANIC=String method: boom) 31
