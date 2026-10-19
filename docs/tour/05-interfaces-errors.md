# Interfaces and errors

An interface is satisfied by any type that has the methods; there is no `implements`. Errors are
values of the interface `error`, and `errors.Is` and `errors.As` look through wrapped errors.

<!-- program: 05 id=shapes go -->
```go
package main

import (
	"errors"
	"fmt"
)

type Shape interface {
	Area() float64
}

type Rect struct{ W, H float64 }
type Circle struct{ R float64 }

func (r Rect) Area() float64   { return r.W * r.H }
func (c Circle) Area() float64 { return 3 * c.R * c.R }

// ErrNegative is a sentinel error: callers compare against it with errors.Is.
var ErrNegative = errors.New("negative size")

func newRect(w, h float64) (Rect, error) {
	if w < 0 || h < 0 {
		return Rect{}, fmt.Errorf("rect %vx%v: %w", w, h, ErrNegative)
	}
	return Rect{w, h}, nil
}

func describe(s Shape) string {
	// A type switch looks at the dynamic type.
	switch v := s.(type) {
	case Rect:
		return fmt.Sprintf("rect %vx%v", v.W, v.H)
	case Circle:
		return fmt.Sprintf("circle r=%v", v.R)
	}
	return "unknown"
}

func main() {
	r, err := newRect(2, 3)
	shapes := []Shape{r, Circle{2}}
	for _, s := range shapes {
		fmt.Println(describe(s), "area", s.Area())
	}

	_, err = newRect(-1, 3)
	fmt.Println(err)
	fmt.Println(errors.Is(err, ErrNegative), errors.Unwrap(err) == ErrNegative)

	// The zero value of an interface is nil.
	var s Shape
	fmt.Println(s == nil)
	if c, ok := shapes[1].(Circle); ok {
		fmt.Println("the second shape is a circle of radius", c.R)
	}
}
```

<!-- output -->
```text
rect 2x3 area 6
circle r=2 area 12
rect -1x3: negative size
true true
true
the second shape is a circle of radius 2
```

## What to notice

- An interface value holds a type descriptor and the value, so `s.(type)` and `s.(Circle)` are
  checks at run time. A call through an interface goes through
  a method table and a generated thunk; in the benchmark suite (`iface_call`) it is about 5.8 times
  slower than the same call in Go, and the plan has devirtualization for it
  ([benchmarks](../benchmarks/README.md)).
- `fmt.Errorf` with `%w` wraps. `errors` is Go's own package, ported to gocpp.
- Everything here is valid Go and prints what Go prints.

Next: [Generics](06-generics.md).
