// phase: 05

// Interface idioms: compile-time assertions (var _ I = (*T)(nil)), package-level interface variables, comparing an interface with a concrete value, switch on error values, map[string]any updates.
package main

import (
	"errors"
	"fmt"
)

type Shape interface{ Area() float64 }
type Circle struct{ R float64 }
type Square struct{ S float64 }

func (c *Circle) Area() float64 { return 3 * c.R * c.R }
func (s Square) Area() float64  { return s.S * s.S }

// compile-time interface checks
var _ Shape = (*Circle)(nil)
var _ Shape = Square{}
var _ Shape = &Square{}
var _ error = (*MyErr)(nil)
var _ fmt.Stringer = Level(0)

type MyErr struct{ op string }

func (e *MyErr) Error() string { return "myerr: " + e.op }

type Level int

func (l Level) String() string { return [...]string{"debug", "info", "warn"}[l] }

var (
	ErrA = errors.New("a")
	ErrB = errors.New("b")
)

var registry = map[string]func() Shape{
	"circle": func() Shape { return &Circle{1} },
	"square": func() Shape { return Square{2} },
}

var defaultShape Shape = registry["circle"]()
var counter = initCounter()

func initCounter() Shape {
	fmt.Println("initCounter called, default area", defaultShape.Area())
	return Square{3}
}

func classify(err error) string {
	switch err {
	case nil:
		return "nil"
	case ErrA:
		return "A"
	case ErrB:
		return "B"
	}
	return "other:" + err.Error()
}

func find(err error) string {
	var me *MyErr
	switch {
	case errors.As(err, &me):
		return "myerr/" + me.op
	case errors.Is(err, ErrA):
		return "isA"
	}
	return "?"
}

func main() {
	fmt.Println(defaultShape.Area(), counter.Area(), len(registry))
	for _, k := range []string{"circle", "square", "tri"} {
		if mk, ok := registry[k]; ok {
			fmt.Printf("%s %.1f\n", k, mk().Area())
		} else {
			fmt.Println(k, "missing", mk == nil)
		}
	}
	var s Shape = Square{2}
	fmt.Println(s == Square{2}, s == Square{3}, s != nil)
	var c Shape = &Circle{1}
	cc := &Circle{1}
	fmt.Println(c == cc, c == Shape(cc), c == &Circle{1})
	var e error = ErrA
	fmt.Println(e == ErrA, e == ErrB, e != nil, ErrA == e)
	for _, err := range []error{nil, ErrA, ErrB, errors.New("zzz"), fmt.Errorf("w: %w", ErrA), &MyErr{"x"}} {
		fmt.Print(classify(err), " ", find(err), "; ")
	}
	fmt.Println()
	var st fmt.Stringer = Level(1)
	fmt.Println(st == Level(1), st == Level(2), st)
	var a any = Level(2)
	fmt.Println(a == Level(2), a == 2, a == any(Level(2)), a.(fmt.Stringer).String())
	m := map[string]any{"a": map[string]any{"b": []any{1, "x", nil}}, "n": 3}
	m["a"].(map[string]any)["c"] = true
	inner := m["a"].(map[string]any)
	inner["b"].([]any)[2] = "filled"
	fmt.Println(len(m["a"].(map[string]any)), m["a"].(map[string]any)["b"], m["a"].(map[string]any)["c"], m["n"].(int)+1)
	if v, ok := m["zzz"].(int); !ok {
		fmt.Println("missing int", v)
	}
	_, ok := m["a"].(map[string]int)
	fmt.Println(ok)
	var fns []func() string
	for _, l := range []Level{0, 1, 2} {
		var s fmt.Stringer = l
		fns = append(fns, s.String)
	}
	for _, f := range fns {
		fmt.Print(f(), " ")
	}
	fmt.Println()
}
