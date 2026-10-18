// phase: 05

// Two embedded types with the same name at the same depth make that name ambiguous only if it
// is used; the outer type resolves it by declaring its own, or the path names one.
package main

import "fmt"

type A struct{ V int }

func (a A) Who() string { return "A" }
func (a *A) Set(v int)  { a.V = v }

type B struct{ V int }

func (b B) Who() string { return "B" }
func (b *B) Set(v int)  { b.V = v }

type Both struct {
	A
	B
	Own int
}

func (b Both) Who() string { return b.A.Who() + b.B.Who() }

type Pick struct {
	A
	B
}

func (p *Pick) Set(v int) { p.A.Set(v); p.B.Set(v * 10) }

type OnlyOne struct {
	A
	C struct{ V2 int }
}

type Deeper struct {
	Both
	A2 A
}

func main() {
	b := Both{A{1}, B{2}, 3}
	fmt.Println(b.Who(), b.A.V, b.B.V, b.Own)
	b.A.Set(5)
	b.B.Set(6)
	fmt.Println(b.A.V, b.B.V)
	p := Pick{A{1}, B{2}}
	p.Set(4)
	fmt.Println(p.A.V, p.B.V, p.A.Who(), p.B.Who())
	o := OnlyOne{A{7}, struct{ V2 int }{8}}
	o.Set(9)
	fmt.Println(o.V, o.Who(), o.C.V2)
	d := Deeper{Both{A{1}, B{2}, 3}, A{4}}
	fmt.Println(d.Who(), d.Own, d.A2.V, d.Both.A.V)
}

// Output:
// AB 1 2 3
// 5 6
// 4 40 A B
// 9 A 8
// AB 3 4 1
