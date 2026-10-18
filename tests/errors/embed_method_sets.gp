// phase: 05

// Method sets through embedding follow Go: the method set of the outer value has the value methods
// of an embedded value and all the methods of an embedded pointer's type; *Outer has all of them.
// A pointer method of an embedded value is missing from the value, and says so.
package main

type Getter interface{ Get() int }
type Setter interface{ Set(int) }

type Base struct{ ID int }

func (b Base) Get() int   { return b.ID }
func (b *Base) Set(v int) { b.ID = v }

type User struct{ Base }
type PUser struct{ *Base }

type Speaker interface{ Speak() string }

type Quiet struct {
	Speaker
	Base
}

type Mute struct{ Quiet }

func main() {
	var u User
	var p PUser
	var g Getter = u
	var s Setter = u // ERROR "User does not implement Setter \(method Set has pointer receiver\)"
	g = &u
	s = &u
	g, s = p, p
	g, s = &p, &p
	s = Base{}  // ERROR "Base does not implement Setter \(method Set has pointer receiver\)"
	s = &Base{}
	var sp Speaker = Quiet{}
	sp = Mute{}
	sp = Base{} // ERROR "Base does not implement Speaker \(missing method Speak\)"
	_, _, _ = g, s, sp
	_ = User.Set     // ERROR "invalid method expression User.Set \(needs pointer receiver \(\*User\).Set\)"
	_ = (*User).Set
	_ = PUser.Set
}
