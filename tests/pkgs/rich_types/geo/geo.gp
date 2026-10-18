package geo

import "fmt"

type Point struct{ X, Y int }

func (p Point) Add(q Point) Point  { return Point{p.X + q.X, p.Y + q.Y} }
func (p *Point) Move(dx, dy int)   { p.X += dx; p.Y += dy }
func (p Point) String() string     { return fmt.Sprintf("<%d,%d>", p.X, p.Y) }

type Stack struct {
	items []Point
	top   int
}

func (s *Stack) Push(p Point) { s.items = append(s.items, p); s.top = len(s.items) }
func (s *Stack) Len() int     { return len(s.items) }
func (s Stack) Top() Point    { return s.items[s.top-1] }

type Points []Point

func (ps Points) Sum() (t Point) {
	for _, p := range ps {
		t = t.Add(p)
	}
	return
}

type ByName map[string]Point

func (m ByName) Get(k string) Point { return m[k] }

type Transform func(Point) Point

func (t Transform) Twice(p Point) Point { return t(t(p)) }

type Mover interface {
	Move(dx, dy int)
}

type Named interface {
	Name() string
}

type Both interface {
	Mover
	Named
}

type Pt3 struct {
	P Point
	Z int
}

func (p *Pt3) Move(dx, dy int) { p.P.Move(dx, dy) }

func (p *Pt3) Name() string { return fmt.Sprint("pt3-", p.Z) }

func Pair() (Point, Point) { return Point{1, 2}, Point{3, 4} }

var Origin, Unit = Pair()
