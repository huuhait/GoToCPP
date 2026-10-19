// phase: 07

// Constraint methods reached by promotion through several embedded levels, pointers and interfaces.
package main

import "fmt"

type Base struct{ id int }

func (b Base) ID() int      { return b.id }
func (b *Base) SetID(i int) { b.id = i }

type Mid struct{ Base }
type Top struct{ *Mid }
type Wrap struct{ Top }

type Ider interface{ ID() int }
type Setter interface{ SetID(int) }

type Holder struct{ Ider }

func ID[T Ider](x T) int { return x.ID() }

func Rename[T Setter](x T, i int) T {
	x.SetID(i)
	return x
}

func Both[T interface {
	Ider
	Setter
}](x T) int {
	x.SetID(x.ID() * 2)
	return x.ID()
}

func main() {
	b := Base{1}
	m := Mid{Base{2}}
	t := Top{&Mid{Base{3}}}
	w := Wrap{Top{&Mid{Base{4}}}}
	fmt.Println(ID(b), ID(&b), ID(m), ID(&m), ID(t), ID(w), ID(Holder{b}), ID[Ider](m))
	Rename(&b, 10)
	Rename(&m, 20)
	Rename(t, 30)
	Rename(w, 40)
	Rename(&w, 50)
	fmt.Println(b.id, m.id, t.id, w.id)
	fmt.Println(Both(&b), Both(&m), Both(t), Both(&w))
	fmt.Println(b.id, m.id, t.id, w.id)
	fmt.Println(ID(Holder{Mid{Base{9}}}))
}

// Output:
// 1 1 2 2 3 4 1 2
// 10 20 30 50
// 20 40 60 100
// 20 40 60 100
// 9
