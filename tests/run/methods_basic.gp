// phase: 03

// Value and pointer receivers (step 3.10): a value method gets a copy, a pointer method changes
// the caller's value; x.M() takes x's address for a pointer method and p.M() dereferences p for
// a value method; methods of a named type that isn't a struct, and fmt calling String().
package main

import "fmt"

type Point struct {
	X, Y int
}

func (p Point) Add(q Point) Point {
	p.X += q.X
	p.Y += q.Y
	return p
}

func (p *Point) Move(dx, dy int) {
	p.X += dx
	p.Y += dy
}

type Rect struct {
	Min, Max Point
}

func NewRect(x, y, w, h int) *Rect {
	return &Rect{Min: Point{x, y}, Max: Point{X: x + w, Y: y + h}}
}

func (r Rect) Width() int  { return r.Max.X - r.Min.X }
func (r Rect) Height() int { return r.Max.Y - r.Min.Y }
func (r Rect) Area() int   { return r.Width() * r.Height() }

func (r Rect) Contains(p Point) bool {
	return p.X >= r.Min.X && p.X < r.Max.X && p.Y >= r.Min.Y && p.Y < r.Max.Y
}

func (r *Rect) Translate(dx, dy int) {
	r.Min.Move(dx, dy)
	r.Max.Move(dx, dy)
}

type Celsius float64

func (c Celsius) Fahrenheit() float64 { return float64(c)*9/5 + 32 }

func (c Celsius) String() string {
	if c < 0 {
		return "below zero"
	}
	return "not below zero"
}

func main() {
	p := Point{1, 2}
	q := p.Add(Point{10, 20})
	fmt.Println(p, q)
	p.Move(5, 5)
	fmt.Println(p)

	pp := &p
	pp.Move(1, 1)
	fmt.Println(*pp, pp.Add(Point{}), p == *pp)

	r := NewRect(0, 0, 4, 3)
	fmt.Println(r.Width(), r.Height(), r.Area())
	fmt.Println(r.Contains(Point{2, 2}), r.Contains(Point{4, 1}))
	r.Translate(10, 10)
	fmt.Println(*r, r.Min)

	pts := []Point{{1, 1}, {2, 4}, {3, 9}}
	pts[0].X = 7
	last := &pts[2]
	last.Move(0, 1)
	fmt.Println(pts[0], pts[2], len(pts))

	for _, c := range []Celsius{-40, 0, 37, 100} {
		fmt.Println(c, c.Fahrenheit())
	}
}

// Output:
// {1 2} {11 22}
// {6 7}
// {7 8} {7 8} true
// 4 3 12
// true false
// {{10 10} {14 13}} {10 10}
// {7 1} {3 10} 3
// below zero -40
// not below zero 32
// not below zero 98.6
// not below zero 212
