// #Derive(Equal) (step 10.10): deep equality that follows pointers, slices, maps and arrays and
// calls the Equal method of a field's type when it has one (time.Time, other derived types). A
// nil slice or map equals an empty one; pointers to the same value are equal; two nil pointers are
// equal and a nil pointer equals no other. The twin (testdata/derive_twin/derive_equal.go) has the
// methods written by hand.

package main

import (
	"fmt"
	"time"
)

type Point struct{ X, Y int }

type Plain struct {
	A int
	B string
	C [2]bool
	P Point
}

type Money struct {
	Cents    int64
	Currency string
}

type Item struct {
	Name   string
	Price  Money
	Tags   []string
	Counts map[string]int
}

type Order struct {
	ID     int
	Items  []Item
	Index  map[string][]Item
	Grid   [2][]int
	Owner  *Item
	Paid   *Money
	When   time.Time
	Note   any
	Err    error
	Ratio  float64
	Raw    []byte
	Anon   struct{ Lo, Hi []int }
	Pt     Point
	Plain  Plain
	Cancel chan int
}

type Names []string

type Tree struct {
	Val         int
	Left, Right *Tree
}

type Matrix [2][2]float64

type Lookup map[string][]Money

func item(name string, cents int64, tags ...string) Item {
	return Item{Name: name, Price: Money{cents, "EUR"}, Tags: tags, Counts: map[string]int{"n": len(tags)}}
}

func main() {
	a := Order{ID: 1, Items: []Item{item("x", 5, "a", "b"), item("y", 7)}, Owner: &Item{Name: "o"}, When: time.Unix(1000, 0)}
	b := Order{ID: 1, Items: []Item{item("x", 5, "a", "b"), item("y", 7)}, Owner: &Item{Name: "o"}, When: time.Unix(1000, 0)}
	fmt.Println("same:", a.Equal(b), b.Equal(a), a.Equal(a))

	// each change makes the orders different
	var diffs []string
	check := func(name string, change func(o *Order)) {
		c := Order{ID: 1, Items: []Item{item("x", 5, "a", "b"), item("y", 7)}, Owner: &Item{Name: "o"}, When: time.Unix(1000, 0)}
		change(&c)
		if a.Equal(c) || c.Equal(a) {
			diffs = append(diffs, name+" still equal")
		}
	}
	check("id", func(o *Order) { o.ID = 2 })
	check("item name", func(o *Order) { o.Items[1].Name = "z" })
	check("item price", func(o *Order) { o.Items[0].Price.Cents++ })
	check("item tag", func(o *Order) { o.Items[0].Tags[1] = "c" })
	check("item tag len", func(o *Order) { o.Items[0].Tags = o.Items[0].Tags[:1] })
	check("item count", func(o *Order) { o.Items[0].Counts["n"] = 9 })
	check("item count key", func(o *Order) { delete(o.Items[0].Counts, "n"); o.Items[0].Counts["m"] = 2 })
	check("items len", func(o *Order) { o.Items = o.Items[:1] })
	check("index", func(o *Order) { o.Index = map[string][]Item{"k": nil} })
	check("grid", func(o *Order) { o.Grid[1] = []int{1} })
	check("owner", func(o *Order) { o.Owner.Name = "p" })
	check("owner nil", func(o *Order) { o.Owner = nil })
	check("paid", func(o *Order) { o.Paid = &Money{} })
	check("when", func(o *Order) { o.When = time.Unix(1001, 0) })
	check("note", func(o *Order) { o.Note = 3 })
	check("err", func(o *Order) { o.Err = fmt.Errorf("x") })
	check("ratio", func(o *Order) { o.Ratio = 0.5 })
	check("raw", func(o *Order) { o.Raw = []byte{1} })
	check("anon", func(o *Order) { o.Anon.Hi = []int{4} })
	check("pt", func(o *Order) { o.Pt.Y = 1 })
	check("plain", func(o *Order) { o.Plain.C[1] = true })
	check("chan", func(o *Order) { o.Cancel = make(chan int) })
	fmt.Println("changes that were not seen:", diffs)

	// nil and empty are equal; pointers are followed
	c := Order{ID: 1, Items: []Item{item("x", 5, "a", "b"), item("y", 7)}, Owner: &Item{Name: "o"}, When: time.Unix(1000, 0)}
	c.Raw = []byte{}
	c.Index = map[string][]Item{}
	c.Anon.Lo = []int{}
	c.Items[1].Tags = []string{}
	c.Items[1].Counts = map[string]int{"n": 0}
	fmt.Println("nil and empty:", a.Equal(c), c.Equal(a))
	c.Paid = &Money{5, "USD"}
	b.Paid = &Money{5, "USD"}
	fmt.Println("paid:", b.Equal(c), a.Equal(c))
	var same Order
	fmt.Println("zero:", same.Equal(Order{}), same.Equal(a))

	// the same time instant in another zone is equal (time.Time.Equal)
	d := Order{When: time.Unix(5, 0).UTC()}
	e := Order{When: time.Unix(5, 0).In(time.FixedZone("x", 3600))}
	fmt.Println("zones:", d.Equal(e), d.When == e.When)

	// other types
	fmt.Println("names:", Names{"a", "b"}.Equal(Names{"a", "b"}), Names{"a"}.Equal(Names{"b"}), Names(nil).Equal(Names{}), Names{"a"}.Equal(Names{}))
	t1 := &Tree{1, &Tree{2, nil, nil}, &Tree{3, &Tree{4, nil, nil}, nil}}
	t2 := &Tree{1, &Tree{2, nil, nil}, &Tree{3, &Tree{4, nil, nil}, nil}}
	fmt.Println("tree:", t1.Equal(*t2), t1.Left.Equal(*t2.Left), t1.Right.Equal(*t2.Left))
	t2.Right.Left.Val = 5
	fmt.Println("tree changed:", t1.Equal(*t2))
	fmt.Println("matrix:", Matrix{{1, 2}, {3, 4}}.Equal(Matrix{{1, 2}, {3, 4}}), Matrix{{1, 2}, {3, 4}}.Equal(Matrix{{1, 2}, {3, 5}}))
	l1 := Lookup{"a": {{1, "x"}}, "b": nil}
	l2 := Lookup{"a": {{1, "x"}}, "b": {}}
	fmt.Println("lookup:", l1.Equal(l2), l1.Equal(Lookup{"a": {{1, "x"}}}), l1.Equal(Lookup{"a": {{1, "y"}}, "b": nil}))
	fmt.Println("money:", Money{1, "a"}.Equal(Money{1, "a"}), Money{1, "a"}.Equal(Money{1, "b"}))
}

// The methods, written the way a Go programmer would.

func (m Money) Equal(o Money) bool { return m.Cents == o.Cents && m.Currency == o.Currency }

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func eqInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (it Item) Equal(o Item) bool {
	if it.Name != o.Name || !it.Price.Equal(o.Price) || !eqStrings(it.Tags, o.Tags) {
		return false
	}
	if len(it.Counts) != len(o.Counts) {
		return false
	}
	for k, v := range it.Counts {
		if w, ok := o.Counts[k]; !ok || w != v {
			return false
		}
	}
	return true
}

func eqItems(a, b []Item) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equal(b[i]) {
			return false
		}
	}
	return true
}

func (r Order) Equal(o Order) bool {
	if r.ID != o.ID || !eqItems(r.Items, o.Items) {
		return false
	}
	if len(r.Index) != len(o.Index) {
		return false
	}
	for k, v := range r.Index {
		w, ok := o.Index[k]
		if !ok || !eqItems(v, w) {
			return false
		}
	}
	for i := range r.Grid {
		if !eqInts(r.Grid[i], o.Grid[i]) {
			return false
		}
	}
	if (r.Owner == nil) != (o.Owner == nil) || r.Owner != nil && r.Owner != o.Owner && !r.Owner.Equal(*o.Owner) {
		return false
	}
	if (r.Paid == nil) != (o.Paid == nil) || r.Paid != nil && r.Paid != o.Paid && !r.Paid.Equal(*o.Paid) {
		return false
	}
	if !r.When.Equal(o.When) || r.Note != o.Note || r.Err != o.Err || r.Ratio != o.Ratio {
		return false
	}
	if len(r.Raw) != len(o.Raw) {
		return false
	}
	for i := range r.Raw {
		if r.Raw[i] != o.Raw[i] {
			return false
		}
	}
	return eqInts(r.Anon.Lo, o.Anon.Lo) && eqInts(r.Anon.Hi, o.Anon.Hi) && r.Pt == o.Pt && r.Plain == o.Plain && r.Cancel == o.Cancel
}

func (n Names) Equal(o Names) bool { return eqStrings(n, o) }

func (t Tree) Equal(o Tree) bool {
	if t.Val != o.Val {
		return false
	}
	for _, p := range [][2]*Tree{{t.Left, o.Left}, {t.Right, o.Right}} {
		a, b := p[0], p[1]
		if a == b {
			continue
		}
		if a == nil || b == nil || !a.Equal(*b) {
			return false
		}
	}
	return true
}

func (m Matrix) Equal(o Matrix) bool { return m == o }

func (l Lookup) Equal(o Lookup) bool {
	if len(l) != len(o) {
		return false
	}
	for k, v := range l {
		w, ok := o[k]
		if !ok || len(v) != len(w) {
			return false
		}
		for i := range v {
			if !v[i].Equal(w[i]) {
				return false
			}
		}
	}
	return true
}
