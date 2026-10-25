// Interfaces: declarations, satisfaction by two types, dispatch, type assertions with and
// without comma-ok, a type switch, a nil interface, an empty interface, and a Stringer-style
// interface whose method is called through the interface.
package main

type Shape interface {
	Area() int
	Name() string
}

type Rect struct{ W, H int }

func (r Rect) Area() int    { return r.W * r.H }
func (r Rect) Name() string { return "rect" }

type Square int

func (s Square) Area() int    { return int(s) * int(s) }
func (s Square) Name() string { return "square" }

type Stringer interface{ String() string }

type Thing struct{ v int }

func (t Thing) String() string { return "thing" }

func describe(s Shape) {
	println(s.Name(), s.Area())
}

func main() {
	var s Shape = Rect{2, 3}
	describe(s)
	s = Square(4)
	describe(s)
	r, ok := s.(Rect)
	println(r.Area(), ok)
	sq, ok := s.(Square)
	println(int(sq), ok)
	switch v := s.(type) {
	case Rect:
		println("rect", v.W)
	case Square:
		println("square", int(v))
	default:
		println("other")
	}
	var empty interface{} = 42
	n, ok := empty.(int)
	println(n, ok)
	var none Shape
	println(none == nil)
	if none != nil {
		println("non-nil")
	}
	var st Stringer = Thing{7}
	println(st.String())
	var anyv any = "x"
	println(anyv.(string))
	shapes := []Shape{Rect{1, 2}, Square(3)}
	total := 0
	for _, sh := range shapes {
		total += sh.Area()
	}
	println(total)
}
