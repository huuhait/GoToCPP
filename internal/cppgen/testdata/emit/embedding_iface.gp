package main

import "fmt"

type Getter interface{ Get() int }
type Setter interface{ Set(int) }

type Base struct{ ID int }

func (b Base) Get() int   { return b.ID }
func (b *Base) Set(v int) { b.ID = v }

type User struct {
	Base
	Name string
}

type PUser struct{ *Base }

type Speaker interface{ Speak() string }

type Dog struct{}

func (Dog) Speak() string { return "woof" }

type Loud struct {
	Speaker
	n []int
}

func main() {
	u := User{Base{1}, "u"}
	var g Getter = u
	var s Setter = &u
	s.Set(5)
	p := PUser{&Base{7}}
	var s2 Setter = p
	s2.Set(8)
	var sp Speaker = Loud{Dog{}, nil}
	fmt.Println(g.Get(), u.ID, p.ID, sp.Speak())
	var a any = struct {
		Base
		tag string
	}{Base{3}, "a"}
	fmt.Println(a.(Getter).Get())
}
