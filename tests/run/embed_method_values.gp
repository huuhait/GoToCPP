// phase: 05

// Method values and method expressions of promoted methods: the receiver is evaluated when the
// value is made (a copy for a value method, the address for a pointer method), through the
// embedded fields and pointers; also deferred and called later.
package main

import "fmt"

type Inner struct{ v int }

func (i Inner) Get() int   { return i.v }
func (i *Inner) Set(v int) { i.v = v }
func (i *Inner) Bump() int { i.v++; return i.v }

type Deep struct{ d int }

func (d *Deep) Bump() int { d.d++; return d.d }
func (d Deep) Val() int   { return d.d }

type Mid struct {
	*Deep
	m int
}

type Outer struct {
	Inner
	*Mid
	x int
}

func main() {
	o := Outer{Inner{1}, &Mid{&Deep{2}, 3}, 4}
	get, set, bump := o.Get, o.Set, o.Inner.Bump
	dbump, dval := o.Bump, o.Val
	o.Inner.v = 10
	o.Mid.Deep.d = 20
	set(5)
	fmt.Println(get(), o.Get(), bump(), o.v, dbump(), dval(), o.Val())

	fmt.Println(Outer.Get(o), (*Outer).Bump(&o), Outer.Val(o), (*Outer).Get(&o))
	(*Outer).Set(&o, 8)
	fmt.Println(o.v, Mid.Val(*o.Mid), (*Mid).Bump(o.Mid))

	f := (*Outer).Bump
	g := Outer.Get
	h := (*Mid).Val
	fmt.Println(f(&o), g(o), h(o.Mid), o.Inner.v, o.d)

	defer o.Set(100)
	defer fmt.Println("deferred get", o.Get())
	defer fmt.Println("deferred bump", o.Bump())
	o.Inner.v = 50
}

// Output:
// 1 5 6 7 7 2 20
// 7 8 20 8
// 8 20 21
// 9 9 21 9 21
// deferred bump 10
// deferred get 9
