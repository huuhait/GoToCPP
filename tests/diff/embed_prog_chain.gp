// phase: 05

// Embedding: pointer embedding, interface embedding, promoted methods through several levels, shadowing, method values bound early, nil embedded pointers.
package main

import "fmt"

type Logger interface{ Log(string) }
type Base struct {
	ID   int
	logs []string
}

func (b *Base) Log(s string)     { b.logs = append(b.logs, fmt.Sprintf("%d:%s", b.ID, s)) }
func (b Base) Describe() string  { return fmt.Sprint("base#", b.ID) }
func (b *Base) SetID(id int)     { b.ID = id }

type Mid struct {
	*Base
	Name string
}

func (m Mid) Describe() string { return "mid(" + m.Base.Describe() + ")" }

type Top struct {
	Mid
	Logger
	extra int
}

type Wrapper struct{ Logger }

func (w Wrapper) Log(s string) { w.Logger.Log("[" + s + "]") }

type Describer interface{ Describe() string }
type IDSetter interface{ SetID(int) }

type A struct{ X int }
type B struct{ X int }
type AB struct {
	A
	B
	Y int
}

func (a A) Hello() string { return "A.Hello" }
func (b B) Bye() string   { return "B.Bye" }

type Deep struct{ AB }

func main() {
	base := &Base{ID: 1}
	m := Mid{base, "m"}
	t := Top{Mid: m, Logger: Wrapper{base}}
	m.Log("a")
	t.Mid.Log("b")
	t.Logger.Log("c")
	t.SetID(7)
	t.Mid.Log("d")
	fmt.Println(base.logs, base.ID, t.ID, t.Mid.Base.ID)
	var d Describer = t
	fmt.Println(d.Describe(), t.Mid.Describe(), t.Base.Describe())
	var s IDSetter = t
	s.SetID(9)
	fmt.Println(base.ID)
	s = &t
	s.SetID(10)
	fmt.Println(t.ID)
	var l Logger = t.Mid
	l.Log("e")
	fmt.Println(len(base.logs), base.logs[len(base.logs)-1])
	f := t.Describe
	g := t.Mid.SetID
	t.Mid = Mid{&Base{ID: 100}, "other"}
	g(55)
	fmt.Println(f(), t.Describe(), base.ID)
	ab := AB{A{1}, B{2}, 3}
	fmt.Println(ab.A.X, ab.B.X, ab.Y, ab.Hello(), ab.Bye())
	dp := Deep{ab}
	fmt.Println(dp.Hello(), dp.AB.A.X, dp.Y)
	fmt.Printf("%v %+v\n", ab, dp)
	var nilmid Mid
	defer func() {
		fmt.Println("recovered:", recover())
		var tt Top
		defer func() { fmt.Println("recovered 2:", recover()) }()
		tt.Log("x")
	}()
	fmt.Println(nilmid.Name == "")
	fmt.Println(nilmid.ID)
}
