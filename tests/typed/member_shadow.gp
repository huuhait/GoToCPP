// phase: 12
// opaque: reject

// A class with an operator-> keeps its own members: x.m is the class's when the class has a member
// of that name, whatever kind it is (a using-declaration, an enumerator, a static, a member of a base,
// of an anonymous union, a bit-field, a method, a member template); x.m is the pointee's, x->m, only
// when the compiler says that the class has none.
package main

import (
	"fmt"

	"./shadow.hpp"
)

func main() {
	var a shadow.PrivBase
	var pub shadow.PubBase
	var b shadow.AnonU
	var c shadow.AnonS
	var d shadow.StaticX
	var e shadow.FromTBase
	var f shadow.BitX
	var g shadow.MethX
	var i shadow.EnumX
	var j shadow.MultiBase
	var k shadow.TemplX
	j.l2.x = 500
	fmt.Println(int(a.x), int(pub.x), int(b.x), int(c.x), int(d.x), int(e.x), int(f.x), int(g.x()), int(i.x), int(j.x))
	a.x = 5
	pub.x = 6
	b.x = 7
	c.x = 8
	e.x = 9
	f.x = 2
	j.x = 4
	fmt.Println(int(a.x), int(pub.x), int(b.x), int(c.x), int(e.x), int(f.x), int(j.x), int(a.l.x), int(j.l2.x))
	var p shadow.Plain
	p.x = 31
	// a.tm is the pointee's; a.get is the class's (a using-declaration), with an argument that is the
	// pointee's member of another class: one probe fails for p.x, and a.get stays a.get.
	fmt.Println(k.tm(1), a.tm(2), a.get(3), a.get(p.x))

	// resize and size are the class's, from a base that the declarations do not list; the argument
	// p.x is the pointee's, so the probe of the whole call fails for x, not for resize.
	var ps shadow.PrivStd
	ps.resize(p.x)
	fmt.Println(int(ps.size()))

	// No member of the name in the class: the pointee's, through operator->.
	var ch shadow.Chain
	ch.x = 32
	fmt.Println(int(p.x), int(p.l.x), p.get(), int(ch.x), ch.get())
}

// Output:
// 99 99 77 66 55 44 3 33 22 1
// 5 6 7 8 9 2 4 1 500
// 12 0 1003 1031
// 31
// 31 31 100 32 100
