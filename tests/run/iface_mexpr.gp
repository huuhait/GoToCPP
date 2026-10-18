// phase: 05

// Method expressions on interface types (step 5.9): I.M is func(i I, args...), also for
// methods that an interface gets from an embedded one, with several results and a variadic
// method, and called directly, stored or passed on.
package main

import "fmt"

type Shape interface {
	Area() int
	Scale(k int) Shape
	Info(tag string, more ...int) (string, int)
}

type Named interface{ Name() string }

type Both interface {
	Shape
	Named
}

type Sq struct{ s int }

func (q Sq) Area() int         { return q.s * q.s }
func (q Sq) Scale(k int) Shape { return Sq{q.s * k} }
func (q Sq) Name() string      { return "sq" }
func (q Sq) Info(tag string, more ...int) (string, int) {
	return fmt.Sprint(tag, q.s, more), len(more)
}

type Rc struct{ w, h int }

func (r *Rc) Area() int         { return r.w * r.h }
func (r *Rc) Scale(k int) Shape { return &Rc{r.w * k, r.h * k} }
func (r *Rc) Name() string      { return "rc" }
func (r *Rc) Info(tag string, more ...int) (string, int) {
	return fmt.Sprint(tag, r.w, r.h), 0
}

func apply(f func(Shape) int, s Shape) int { return f(s) }

func main() {
	area := Shape.Area
	fmt.Println(area(Sq{3}), area(&Rc{2, 5}))
	fmt.Println(apply(Shape.Area, Sq{4}), apply(Shape.Area, &Rc{1, 1}))

	scale := Shape.Scale
	fmt.Println(scale(Sq{2}, 3).Area(), scale(&Rc{1, 2}, 4).Area())

	info := Shape.Info
	fmt.Println(info(Sq{7}, "t"))
	fmt.Println(info(Sq{7}, "t", 1, 2, 3))
	fmt.Println(Shape.Info(&Rc{2, 3}, "r"))

	name := Both.Name
	fmt.Println(name(Sq{1}), name(&Rc{}), Named.Name(Sq{1}))
	fmt.Println(Both.Area(Sq{5}), Both.Scale(Sq{5}, 2).Area())

	fs := []func(Shape) int{Shape.Area, func(s Shape) int { return s.Area() + 1 }}
	for _, f := range fs {
		fmt.Println(f(Sq{6}))
	}
	var asNamed Named = Sq{1}
	fmt.Println(Named.Name(asNamed))

	// Method expressions of concrete types still work next to them.
	fmt.Println(Sq.Area(Sq{8}), (*Rc).Area(&Rc{3, 3}), Sq.Name(Sq{}))
	g := (*Rc).Scale
	fmt.Println(g(&Rc{2, 2}, 2).Area())
}

// Output:
// 9 10
// 16 1
// 36 32
// t7 [] 0
// t7 [1 2 3] 3
// r2 3 0
// sq rc sq
// 25 100
// 36
// 37
// sq
// 64 9 sq
// 16
