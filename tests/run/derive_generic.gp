// phase: 10

// #Derive on generic types and embedded fields (step 10.10). The methods of a generic type are
// written for its type parameters: a field of a type parameter is compared with == when the
// parameter is comparable, with its Equal method when the constraint has one, and as any(a) ==
// any(b) otherwise (which panics for a value that cannot be compared, like == on interfaces); it
// is copied as it is. The twin (testdata/derive_twin/derive_generic.go) has the methods written
// by hand.

package main

import "fmt"

#Derive(Equal, Clone)
type Pair[K comparable, V any] struct {
	Key K
	Val V
	Ext []V
}

#Derive(Equal, Clone)
type Box[T interface{ Equal(T) bool }] struct {
	Items []T
	Best  *T
}

#Derive(Clone)
type Money struct {
	Cents int
	Unit  string
}

func (m Money) Equal(o Money) bool { return m.Cents == o.Cents }

#Derive(Equal, Clone)
type Base struct {
	ID   int
	Tags []string
}

#Derive(Equal, Clone)
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

#Derive(String)
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

// Output:
// {k [9] [[2] [3]]} {k [9] [[] [3]]}
// true false true false
// {1 a []} {x High [Low High]}
// true false
// [{1 x} {2 y}] {6 a} [{7 x} {2 y}] {5 a}
// true false true
// {1 [a]} {3 usd} n 4
// {1 [z]} {99 usd} n 4
// true false false true
// false
// {{{1 [a]} <nil> n 4 0} <nil> [{2 []}] map[m:{3 [t]}] boom 3 High 2.5 [104 105] {0 0}}
// {1 []}|{2 [x]}|&{ID:3 Tags:[]}
