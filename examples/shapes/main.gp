// phase: 05

// Shapes shows interfaces (satisfied without "implements"), type switches and
// assertions, errors as values (errors.New, a custom error type, %w wrapping,
// errors.Is and errors.As), struct embedding with promoted fields and methods,
// and recover.
package main

import (
	"errors"
	"fmt"
)

const pi = 3.14159

type Shape interface {
	Area() float64
	Perimeter() float64
}

// A Solid is a Shape with a volume: an interface can embed another one.
type Solid interface {
	Shape
	Volume() float64
}

type Scaler interface {
	Scale(k float64)
}

type Labeler interface {
	Label() string
}

// Named is embedded in every shape, so its field and method are promoted.
type Named struct {
	Name string
}

func (n Named) Label() string { return "<" + n.Name + ">" }

type Rect struct {
	Named
	W, H float64
}

func (r Rect) Area() float64      { return r.W * r.H }
func (r Rect) Perimeter() float64 { return 2 * (r.W + r.H) }

// Circle's methods have pointer receivers, so only *Circle is a Shape.
type Circle struct {
	Named
	R float64
}

func (c *Circle) Area() float64      { return pi * c.R * c.R }
func (c *Circle) Perimeter() float64 { return 2 * pi * c.R }
func (c *Circle) Scale(k float64)    { c.R *= k }

// Square embeds Rect and gets Area, Perimeter, Label, W, H and Name from it.
type Square struct {
	Rect
}

func NewSquare(name string, side float64) Square {
	return Square{Rect{Named{name}, side, side}}
}

// Cube embeds Square, two levels of promotion, and adds Volume and String.
type Cube struct {
	Square
}

func (c Cube) Volume() float64 { return c.W * c.W * c.W }
func (c Cube) String() string  { return "cube " + c.Name }

// ShapeError is an error type of our own.
type ShapeError struct {
	Shape string
	Size  float64
}

func (e *ShapeError) Error() string {
	return fmt.Sprintf("bad %s size %v", e.Shape, e.Size)
}

var ErrNoName = errors.New("shape has no name")

func NewCircle(name string, r float64) (*Circle, error) {
	if name == "" {
		return nil, ErrNoName
	}
	if r <= 0 {
		return nil, &ShapeError{"circle", r}
	}
	return &Circle{Named{name}, r}, nil
}

func describe(s Shape) string {
	switch v := s.(type) {
	case nil:
		return "nothing"
	case Rect:
		return fmt.Sprintf("rect %vx%v", v.W, v.H)
	case *Circle:
		return fmt.Sprintf("circle r=%v", v.R)
	case Square, Cube:
		return fmt.Sprintf("%T with area %v", v, v.Area()) // v is still a Shape here
	default:
		return "unknown"
	}
}

// asRect asserts that s holds a Rect. A failed assertion panics, and the
// deferred recover turns the panic into an error.
func asRect(s Shape) (r Rect, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("recovered: %v", p)
		}
	}()
	return s.(Rect), nil
}

func main() {
	wheel, _ := NewCircle("wheel", 1)
	shapes := []Shape{
		Rect{Named{"door"}, 2, 3},
		wheel,
		NewSquare("tile", 2),
		Cube{NewSquare("box", 3)},
	}
	for _, s := range shapes {
		fmt.Println(describe(s))
		fmt.Printf("  %T area=%v perimeter=%v\n", s, s.Area(), s.Perimeter())
	}
	fmt.Println(describe(nil))

	// Promoted fields and methods, and the full path to them.
	sq := NewSquare("tile", 2)
	fmt.Println(sq.Name, sq.Label(), sq.W, sq.Rect.Named.Name)
	fmt.Printf("%v %+v\n", sq, sq)
	fmt.Println(shapes[3]) // Cube has a String method

	// Interface to interface: checked at run time.
	for _, s := range shapes {
		if sc, ok := s.(Scaler); ok {
			sc.Scale(2) // changes wheel: the interface holds the same pointer
		}
		if so, ok := s.(Solid); ok {
			fmt.Println("solid", so.(Labeler).Label(), "volume", so.Volume())
		}
	}
	fmt.Println("wheel radius now", wheel.R)

	// Sort by area by hand, smallest first.
	for i := 1; i < len(shapes); i++ {
		for j := i; j > 0 && shapes[j].Area() < shapes[j-1].Area(); j-- {
			shapes[j], shapes[j-1] = shapes[j-1], shapes[j]
		}
	}
	for _, s := range shapes {
		fmt.Println(s.(Labeler).Label(), s.Area())
	}

	// Errors are values: compare with errors.Is, extract with errors.As.
	for _, name := range []string{"", "coin", "ring"} {
		r := 0.5
		if name == "coin" {
			r = -1
		}
		c, err := NewCircle(name, r)
		var se *ShapeError
		switch {
		case errors.Is(err, ErrNoName):
			fmt.Println("no name:", err)
		case errors.As(err, &se):
			fmt.Println("shape error:", se.Shape, se.Size, "-", err)
		case err == nil:
			fmt.Println("ok:", c.Label(), c.Area())
		}
	}
	wrapped := fmt.Errorf("load shapes.txt: %w", &ShapeError{"rect", -2})
	var se *ShapeError
	fmt.Println(wrapped)
	fmt.Println(errors.As(wrapped, &se), se.Size, errors.Unwrap(wrapped) == error(se))

	// A nil *Circle inside an interface makes a non-nil interface (Go's
	// well-known trap, shown on purpose; staticcheck warns about it).
	var nc *Circle
	var s Shape = nc
	fmt.Println(nc == nil, s == nil)

	fmt.Println(asRect(Rect{Named{"ok"}, 1, 1}))
	fmt.Println(asRect(wheel))
}
