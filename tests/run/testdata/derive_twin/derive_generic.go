// #Derive on generic types and embedded fields (step 10.10). The methods of a generic type are
// written for its type parameters: a field of a type parameter is compared with == when the
// parameter is comparable, with its Equal method when the constraint has one, and as any(a) ==
// any(b) otherwise (which panics for a value that cannot be compared, like == on interfaces); it
// is copied as it is.

package main

import "fmt"

type Pair[K comparable, V any] struct {
	Key K
	Val V
	Ext []V
}

type Box[T interface{ Equal(T) bool }] struct {
	Items []T
	Best  *T
}

type Money struct {
	Cents int
	Unit  string
}

func (m Money) Equal(o Money) bool { return m.Cents == o.Cents }

type Base struct {
	ID   int
	Tags []string
}

type Derived struct {
	Base
	*Money
	Name   string
	hidden int
	_      int
}

type Wrapper struct {
	Inner  Derived
	Ptr    *Base
	List   []Base
	Map    map[string]Base
	Err    error
	Any    any
	Level  Level
	F      float64
	Bytes  []byte
	Nested struct{ A, B int }
}

type Level int

const (
	Low Level = iota
	High
)

func main() {
	p := Pair[string, []int]{"k", []int{1}, [][]int{{2}, {3}}}
	q := p.Clone()
	q.Val[0] = 9 // the copy of a field of a type parameter is the value itself
	q.Ext[0] = nil
	fmt.Println(p, q)
	e1 := Pair[string, string]{"k", "v", []string{"a"}}
	fmt.Println(e1.Equal(e1.Clone()), e1.Equal(Pair[string, string]{"k", "v", []string{"b"}}), Pair[int, int]{1, 2, nil}.Equal(Pair[int, int]{1, 2, []int{}}), Pair[int, int]{1, 2, nil}.Equal(Pair[int, int]{1, 3, nil}))
	fmt.Println(Pair[int, string]{1, "a", nil}, Pair[string, Level]{"x", High, []Level{Low, High}})
	fmt.Println(Pair[string, any]{"k", 1, nil}.Equal(Pair[string, any]{"k", 1, nil}), Pair[string, any]{"k", 1, nil}.Equal(Pair[string, any]{"k", "1", nil}))

	best := Money{5, "a"}
	b1 := Box[Money]{[]Money{{1, "x"}, {2, "y"}}, &best}
	b2 := b1.Clone()
	b2.Items[0].Cents = 7
	best.Cents = 6
	fmt.Println(b1.Items, *b1.Best, b2.Items, *b2.Best)
	fmt.Println(b1.Equal(b1.Clone()), b1.Equal(b2), Box[Money]{[]Money{{1, "u"}}, nil}.Equal(Box[Money]{[]Money{{1, "v"}}, nil}))

	d := Derived{Base{1, []string{"a"}}, &Money{3, "usd"}, "n", 4, 5}
	e := d.Clone()
	e.Tags[0] = "z"
	e.Money.Cents = 99
	fmt.Println(d.Base, *d.Money, d.Name, d.hidden)
	fmt.Println(e.Base, *e.Money, e.Name, e.hidden)
	fmt.Println(d.Equal(d.Clone()), d.Equal(e), d.Base.Equal(e.Base), d.Money.Equal(Money{3, "other"}))
	e.hidden = 1
	d.Money = nil
	e.Money = nil
	e.Tags[0] = "a"
	fmt.Println(d.Equal(e))
	fmt.Println(Wrapper{Inner: d, Ptr: nil, List: []Base{{2, nil}}, Map: map[string]Base{"m": {3, []string{"t"}}}, Err: fmt.Errorf("boom"), Any: 3, Level: High, F: 2.5, Bytes: []byte("hi")})
	fmt.Printf("%v|%v|%+v\n", Base{1, nil}, Base{2, []string{"x"}}, &Base{3, nil})
}

// The methods, written the way a Go programmer would.

func (p Pair[K, V]) Equal(o Pair[K, V]) bool {
	if p.Key != o.Key || any(p.Val) != any(o.Val) || len(p.Ext) != len(o.Ext) {
		return false
	}
	for i := range p.Ext {
		if any(p.Ext[i]) != any(o.Ext[i]) {
			return false
		}
	}
	return true
}

func (p Pair[K, V]) Clone() Pair[K, V] {
	out := p
	if p.Ext != nil {
		out.Ext = append([]V{}, p.Ext...)
	}
	return out
}

func (b Box[T]) Equal(o Box[T]) bool {
	if len(b.Items) != len(o.Items) {
		return false
	}
	for i := range b.Items {
		if !b.Items[i].Equal(o.Items[i]) {
			return false
		}
	}
	if b.Best != o.Best {
		if b.Best == nil || o.Best == nil || !(*b.Best).Equal(*o.Best) {
			return false
		}
	}
	return true
}

func (b Box[T]) Clone() Box[T] {
	out := b
	if b.Items != nil {
		out.Items = append([]T{}, b.Items...)
	}
	if b.Best != nil {
		v := *b.Best
		out.Best = &v
	}
	return out
}

func (m Money) Clone() Money { return m }

func (b Base) Equal(o Base) bool {
	if b.ID != o.ID || len(b.Tags) != len(o.Tags) {
		return false
	}
	for i := range b.Tags {
		if b.Tags[i] != o.Tags[i] {
			return false
		}
	}
	return true
}

func (b Base) Clone() Base {
	out := b
	if b.Tags != nil {
		out.Tags = append([]string{}, b.Tags...)
	}
	return out
}

func (d Derived) Equal(o Derived) bool {
	if !d.Base.Equal(o.Base) || d.Name != o.Name || d.hidden != o.hidden {
		return false
	}
	if d.Money != o.Money {
		if d.Money == nil || o.Money == nil || !d.Money.Equal(*o.Money) {
			return false
		}
	}
	return true
}

func (d Derived) Clone() Derived {
	out := d
	out.Base = d.Base.Clone()
	if d.Money != nil {
		m := d.Money.Clone()
		out.Money = &m
	}
	return out
}

func (l Level) String() string {
	switch l {
	case Low:
		return "Low"
	case High:
		return "High"
	}
	return "Level(" + fmt.Sprint(int(l)) + ")"
}
