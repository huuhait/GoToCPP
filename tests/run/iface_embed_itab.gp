// phase: 05

// Promoted methods are in the method sets, so the itabs of the outer type find them: a value in
// an interface has the value methods (also those of an embedded pointer's type, pointer methods
// included), a pointer has all of them, with the receiver adjusted by the path (an address inside
// the owner for a pointer method of an embedded value).
package main

import "fmt"

type Getter interface{ Get() int }
type Setter interface{ Set(int) }
type GetSetter interface {
	Getter
	Setter
}

type Base struct{ ID int }

func (b Base) Get() int   { return b.ID }
func (b *Base) Set(v int) { b.ID = v }

type User struct {
	Base
	Name string
}

type PUser struct {
	*Base
	Name string
}

type Admin struct {
	User
	Level int
}

type PAdmin struct {
	*PUser
	Level int
}

func use(g Getter) int { return g.Get() }

func set(s Setter, v int) { s.Set(v) }

func main() {
	u := User{Base{1}, "u"}
	var g Getter = u // User has Get (not Set)
	u.ID = 2         // the interface holds a copy
	fmt.Println(g.Get(), u.Get(), use(u), use(&u))

	var gs GetSetter = &u // *User has both
	gs.Set(10)
	fmt.Println(u.ID, gs.Get(), g.Get())

	p := PUser{&Base{5}, "p"}
	var gs2 GetSetter = p // PUser has both: the embedded *Base brings Set
	gs2.Set(6)
	fmt.Println(p.ID, gs2.Get())
	var gs3 GetSetter = &p
	gs3.Set(7)
	fmt.Println(p.ID, gs3.Get(), gs2.Get()) // the value shares the *Base

	a := Admin{User{Base{20}, "a"}, 1}
	set(&a, 21)
	fmt.Println(a.ID, use(a), use(&a), a.User.Base.ID)

	pa := PAdmin{&PUser{&Base{30}, "pa"}, 2}
	set(pa, 31)
	fmt.Println(pa.ID, use(pa), use(&pa), pa.PUser.Base.ID)

	var any1 any = a
	if x, ok := any1.(Getter); ok {
		fmt.Println("Getter", x.Get())
	}
	if _, ok := any1.(Setter); !ok {
		fmt.Println("Admin value has no Set")
	}
	any1 = &a
	if x, ok := any1.(GetSetter); ok {
		x.Set(40)
		fmt.Println("GetSetter", x.Get(), a.ID)
	}
	switch v := any1.(type) {
	case Getter:
		fmt.Println("switch Getter", v.Get())
	}
}

// Output:
// 1 2 2 2
// 10 10 1
// 6 6
// 7 7 7
// 21 21 21 21
// 31 31 31 31
// Getter 21
// Admin value has no Set
// GetSetter 40 40
// switch Getter 40
