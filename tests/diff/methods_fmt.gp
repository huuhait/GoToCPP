// phase: 03

// fmt calls String() and Error() like Go (step 3.10, through the hooks of gocpp/fmtshim.hpp):
// of a value's method set (a value method), of a pointer's (a pointer method: only for a
// pointer, here in a field, also a nil one, which the method gets), Error before String, a value
// method through a nil pointer prints <nil>, a panic in the method prints fmt's PANIC= form, an
// unexported field is printed without its methods, and a String with another signature, or with
// a named string result, is not fmt's.
package main

import "fmt"

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + itoa(n%10)
}

type Celsius float64

func (c Celsius) String() string { return itoa(int(c)) + "°C" }

type P struct{ x int }

func (p *P) String() string {
	if p == nil {
		return "nil P"
	}
	return "P" + itoa(p.x)
}

type V struct{ x int }

func (v V) String() string { return "V" + itoa(v.x) }

type E struct{ code int }

func (e E) Error() string { return "E" + itoa(e.code) }

type Both struct{ n int }

func (Both) Error() string  { return "Error wins" }
func (Both) String() string { return "String" }

type Bad struct{ n int }

func (b Bad) String() string {
	if b.n > 0 {
		panic("bad String")
	}
	return "good"
}

type Name string

type Other struct{ n int }

func (Other) String() Name { return "not a Stringer" }

type Count struct{ n int }

func (Count) String(int) string { return "not a Stringer either" }

type Holder struct {
	P  *P
	V  V
	PV *V
	E  E
	h  V
	NP *P
	NV *V
	C  Celsius
	B  Both
}

type IDs []int

func (ids IDs) String() string { return "ids:" + itoa(len(ids)) }

func main() {
	c := Celsius(21.5)
	fmt.Println(c, Celsius(-40))
	fmt.Printf("%v %s %d|%f\n", c, c, 7, c)
	fmt.Println(V{1}, E{2}, Both{}, Bad{0}, Bad{1}, Other{3}, Count{4})
	fmt.Printf("%v|%s|%d\n", V{5}, E{6}, V{7})
	h := Holder{P: &P{1}, V: V{2}, PV: &V{3}, E: E{4}, h: V{5}, C: 6}
	fmt.Println(h)
	fmt.Printf("%v|%s\n", h, h)
	fmt.Println(struct{ A, B IDs }{IDs{1, 2, 3}, nil})
	fmt.Println([]V{{1}, {2}}[1], map[string]V{"a": {8}}["a"])
}
