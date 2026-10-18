// phase: 05

// f := i.M binds a COPY of the interface value where it is evaluated: assigning to i later
// changes nothing (step 5.9). A pointer in the interface still shares its object; a value in
// it was copied into the interface by the conversion, so a method value of it never sees the
// variable it came from.
package main

import "fmt"

type Namer interface{ Name() string }

type V struct{ s string }

func (v V) Name() string { return "V:" + v.s }

type P struct{ s string }

func (p *P) Name() string { return "P:" + p.s }

type Grow struct{ n int }

func (g Grow) Name() string { g.n++; return fmt.Sprint("grow", g.n) }

func main() {
	var i Namer = V{"one"}
	f := i.Name
	i = V{"two"}
	fmt.Println(f(), i.Name())

	i = &P{"p1"}
	g := i.Name
	i = nil
	fmt.Println(g(), i == nil)

	p := &P{"shared"}
	i = p
	h := i.Name
	p.s = "changed"
	fmt.Println(h())
	i = &P{"other"}
	fmt.Println(h(), i.Name())

	v := V{"var"}
	i = v
	k := i.Name
	v.s = "var2"
	fmt.Println(k(), v.Name())

	// A value receiver is the method's own copy at every call.
	i = Grow{}
	w := i.Name
	fmt.Println(w(), w(), w())

	// The method values of two interface values are independent.
	a, b := Namer(V{"a"}), Namer(V{"b"})
	fa, fb := a.Name, b.Name
	a, b = b, a
	fmt.Println(fa(), fb(), a.Name(), b.Name())

	// Rebinding in a loop.
	var fs []func() string
	for _, n := range []Namer{V{"x"}, &P{"y"}, V{"z"}} {
		fs = append(fs, n.Name)
	}
	for _, f := range fs {
		fmt.Println(f())
	}
}

// Output:
// V:one V:two
// P:p1 true
// P:changed
// P:changed P:other
// V:var V:var2
// grow1 grow1 grow1
// V:a V:b V:b V:a
// V:x
// P:y
// V:z
