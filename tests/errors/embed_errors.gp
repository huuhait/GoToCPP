// phase: 05

// Embedding errors are go/types' (the messages keep Go's words): the same name at the same depth
// is ambiguous when it is used (a selector, a method expression, a method set for an interface),
// an embedded type cannot be repeated or be a pointer to an interface, and a field and a method
// of one type cannot share a name; a method at a shallower depth than a promoted field is fine.
package main

type A struct{ X int }
type B struct{ X int }

func (A) M()  {}
func (B) M()  {}
func (A) AM() {}

type Both struct {
	A
	B
}

type Dup struct {
	A
	*A // ERROR "A redeclared"
}

type Clash struct {
	A
	AM int
}

func (Clash) AM() {} // ERROR "field and method with the same name AM"

type FieldMethod struct {
	A int
}

func (FieldMethod) A() {} // ERROR "field and method with the same name A"

type PtrIface struct {
	*error // ERROR "embedded field type cannot be a pointer to an interface"
}

type Ok struct{ A }

func (Ok) X() {}

func main() {
	var b Both
	_ = b.X // ERROR "ambiguous selector b.X"
	b.M()   // ERROR "ambiguous selector b.M"
	Both.M(b) // ERROR "ambiguous selector Both.M"
	_ = Both.X // ERROR "ambiguous selector Both.X"
	_ = b.A.X
	b.B.M()
	var o Ok
	_ = o.X
	_ = o.A.X
	var i interface{ M() } = b // ERROR "Both does not implement interface\{M\(\)\} \(ambiguous selector Both.M\)"
	_ = i
	var j interface{ AM() } = Both{}
	_ = j
}
