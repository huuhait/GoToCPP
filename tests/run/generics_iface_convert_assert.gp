// phase: 07

// Converting a type parameter value to interfaces and asserting back, with constraint methods.
package main

import "fmt"

type Namer interface{ Name() string }
type Aged interface{ Age() int }

type Person struct {
	name string
	age  int
}

func (p Person) Name() string { return p.name }
func (p *Person) Age() int    { return p.age }

type Robot struct{ id int }

func (r Robot) Name() string { return fmt.Sprint("robot-", r.id) }

func ToNamer[T Namer](x T) Namer { return x }
func ToAny[T any](x T) any       { return x }
func Aged_[T Namer](x T) (int, bool) {
	a, ok := any(x).(Aged)
	if !ok {
		return -1, false
	}
	return a.Age(), true
}
func Names[T Namer](xs ...T) (out []Namer) {
	for _, x := range xs {
		out = append(out, x)
	}
	return
}
func Back[T any](x any) (T, bool) { t, ok := x.(T); return t, ok }
func Upcast[T Namer](x T) fmt.Stringer {
	s, _ := any(x).(fmt.Stringer)
	return s
}
func Dyn[T Namer](x T) string { return fmt.Sprintf("%T:%s", x, x.Name()) }

func main() {
	p := Person{"ann", 30}
	n1 := ToNamer(p)
	n2 := ToNamer(&p)
	n3 := ToNamer(Robot{7})
	fmt.Println(n1.Name(), n2.Name(), n3.Name())
	_, a1 := n1.(Aged)
	_, a2 := n2.(Aged)
	fmt.Println(a1, a2)
	fmt.Println(Aged_(p))
	fmt.Println(Aged_(&p))
	fmt.Println(Aged_(Robot{1}))
	fmt.Println(Aged_[Namer](&p))
	fmt.Println(Aged_[Namer](Robot{2}))
	fmt.Println(len(Names(p, Person{"bob", 1})), len(Names[Namer](p, Robot{1}, &p)), Names[Robot]() == nil)
	x := ToAny(p)
	y := ToAny(&p)
	z := ToAny[Namer](Robot{3})
	pp, ok := Back[Person](x)
	_, ok2 := Back[*Person](x)
	q, ok3 := Back[*Person](y)
	r, ok4 := Back[Robot](z)
	fmt.Println(pp.name, ok, ok2, q.name, ok3, r.id, ok4)
	fmt.Println(Upcast(p) == nil, Dyn(p), Dyn(&p), Dyn(Robot{4}), Dyn[Namer](Robot{5}), Dyn[Namer](n2))
}

// Output:
// ann ann robot-7
// false true
// -1 false
// 30 true
// -1 false
// 30 true
// -1 false
// 2 3 true
// ann true false ann true 3 true
// true main.Person:ann *main.Person:ann main.Robot:robot-4 main.Robot:robot-5 *main.Person:ann
