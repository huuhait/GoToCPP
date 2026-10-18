// phase: 05

// &o.Inner shares the owner of o: the pointer keeps the whole object alive, and writes through
// it are writes of o. An address taken through an embedded pointer shares the owner of what the
// pointer points to.
package main

import "fmt"

type Base struct {
	ID   int
	Tags []string
}

func (b *Base) Touch() int { b.ID++; return b.ID }
func (b Base) Get() int    { return b.ID }

type User struct {
	Base
	Name string
}

type Ref struct {
	*Base
	N int
}

func makeUser() *Base {
	u := &User{Base{1, nil}, "tmp"}
	u.Touch()
	return &u.Base // the User lives on, through this pointer
}

func makeField() *int {
	u := User{Base{40, nil}, "x"}
	return &u.ID // a local, boxed because its field's address escapes
}

func makeRef() *int {
	r := Ref{&Base{ID: 70}, 1}
	return &r.ID // points into the Base that r.Base points to
}

func main() {
	u := &User{Base{10, nil}, "ann"}
	pb := &u.Base
	pb.Touch()
	pb.Tags = append(pb.Tags, "a")
	fmt.Println(u.ID, u.Tags, u.Get(), pb.Get())
	pi := &u.ID
	*pi = 20
	u = nil // the pointers keep the object
	fmt.Println(pb.ID, *pi, pb.Touch())

	b := makeUser()
	fmt.Println(b.ID, b.Touch())
	p := makeField()
	*p += 2
	fmt.Println(*p)
	q := makeRef()
	*q += 5
	fmt.Println(*q)

	var v User
	pv := &v.Base
	pv.ID = 7
	fmt.Println(v.ID, v.Base.ID)
	v.Touch()
	fmt.Println(pv.ID, &v.Base == pv, &v.ID == &pv.ID)

	r := Ref{&Base{ID: 3}, 0}
	pr := &r.Base
	(*pr).ID = 4
	pd := &r.Base.ID
	r.Base = &Base{ID: 100}
	*pd++
	fmt.Println(r.ID, *pd, (*pr).ID)
}

// Output:
// 11 [a] 11 11
// 21 21 21
// 3 3
// 42
// 75
// 7 7
// 8 true true
// 100 5 100
