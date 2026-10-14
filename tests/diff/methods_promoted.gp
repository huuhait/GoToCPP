// phase: 05

// Promoted methods (step 3.10 writes them; the checker allows embedding from Phase 05): value and
// pointer methods through an embedded value and through embedded pointers (o.Bump() is
// o.Mid.Deep.Bump()), on a package variable, through a pointer and deferred, and fmt's hooks of
// promoted String and Error methods (Error first, also through two embedded pointers, and a
// pointer method reached through an embedded pointer followed by an embedded value, also with
// that pointer nil).
package main

import "fmt"

type Inner struct{ v int }

func (i Inner) Get() int     { return i.v }
func (i *Inner) Set(v int)   { i.v = v }
func (i Inner) String() string { return "in" + string(rune('0'+i.v)) }

type Deep struct{ d int }

func (d *Deep) Bump() int  { d.d++; return d.d }
func (d Deep) Val() int    { return d.d }
func (d *Deep) Error() string { return "deep" + string(rune('0'+d.d)) }

type Mid struct {
	*Deep
	m int
}

type Outer struct {
	Inner
	*Mid
	x int
}

type OnlyPtr struct {
	Deep
}

// In has a pointer String; Out's value method set gets it through the pointer *Mid2 followed by
// the embedded value In (the hook takes In's address from the pointer), and so do Top (Out by
// value) and PTop (*Top).
type In struct{ v int }

func (i *In) String() string {
	if i == nil {
		return "nil In"
	}
	return "in" + string(rune('0'+i.v))
}

type Mid2 struct{ In }
type Out struct{ *Mid2 }
type Top struct{ Out }
type PTop struct{ *Top }

var g = Outer{Inner{1}, &Mid{&Deep{2}, 3}, 4}

func main() {
	o := Outer{Inner{1}, &Mid{&Deep{2}, 3}, 4}
	fmt.Println(o.Get(), o.Val(), o.Bump(), o.Bump())
	o.Set(5)
	fmt.Println(o.Get(), o.Inner)
	p := &o
	p.Set(6)
	fmt.Println(p.Get(), p.Val(), p.Bump())
	fmt.Println(o)
	g.Set(7)
	fmt.Println(g.Get(), g.Bump(), g.Mid.Deep.Error())
	var op OnlyPtr
	op.Bump()
	fmt.Println(op.Val(), op)
	m := Mid{&Deep{1}, 0}
	fmt.Println(m)
	fmt.Println(Out{&Mid2{In{1}}}, Top{Out{&Mid2{In{2}}}}, PTop{&Top{Out{&Mid2{In{3}}}}})
	fmt.Println(Out{}, Top{}) // a nil *Mid2: the String method panics, fmt prints PANIC=
	out := Out{&Mid2{In{4}}}
	fmt.Println(out.String(), Top{out}.String())
	defer o.Set(8)
	defer fmt.Println(o.Get())
	defer o.Bump()
}
