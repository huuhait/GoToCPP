// phase: 05

// Method values of promoted interface methods (step 5.9): a struct embedding an interface (by
// value, through a pointer), an interface embedding another, and the method value taken from
// the embedded field or from the outer value.
package main

import "fmt"

type Speaker interface{ Speak() string }
type Named interface{ Name() string }
type Both interface {
	Speaker
	Named
}

type Dog struct{ name string }

func (d Dog) Speak() string { return d.name + " barks" }
func (d *Dog) Name() string { return d.name }

type Wrapper struct {
	Speaker
	tag string
}

type Twice struct{ *Wrapper }

type Deep struct{ Twice }

// A named pointer type has no methods and no struct to forward through.
type P *[]int

type Both2 struct {
	Both
}

func main() {
	d := &Dog{"rex"}
	var b Both = d
	speak := b.Speak
	name := b.Name
	d.name = "max"
	fmt.Println(speak(), name())

	w := Wrapper{Speaker: Dog{"fido"}, tag: "w"}
	f := w.Speak
	g := w.Speaker.Speak
	w.Speaker = Dog{"other"}
	fmt.Println(f(), g(), w.Speak())

	t := Twice{&w}
	h := t.Speak
	w.Speaker = Dog{"third"}
	fmt.Println(h(), t.Speak())

	dp := Deep{t}
	k := dp.Speak
	t.Wrapper = &Wrapper{Speaker: Dog{"swapped"}}
	fmt.Println(k(), dp.Speak())

	b2 := Both2{Both: d}
	n := b2.Name
	s := b2.Speak
	d.name = "last"
	fmt.Println(n(), s())

	// Through an interface that holds the struct.
	var pp any = P(nil)
	_, isP := pp.(P)
	fmt.Println(isP, pp != nil)
	var sp Speaker = w
	m := sp.Speak
	w.Speaker = Dog{"after"}
	fmt.Println(m(), sp.Speak(), w.Speak())
	sp = &dp
	m2 := sp.Speak
	fmt.Println(m2())
}

// Output:
// max barks max
// fido barks fido barks other barks
// other barks third barks
// third barks third barks
// last last barks
// true true
// third barks third barks after barks
// after barks
