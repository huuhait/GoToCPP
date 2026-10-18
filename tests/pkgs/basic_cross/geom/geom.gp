package geom

import "fmt"

const (
	Max  = 10
	Name = "geom"
	Pi   = 3.14159
)

type Celsius float64

const Boiling Celsius = 100

type Point struct{ X, Y int }

func (p Point) Add(q Point) Point { return Point{p.X + q.X, p.Y + q.Y} }
func (p *Point) Scale(k int)      { p.X *= k; p.Y *= k }
func (p Point) String() string    { return fmt.Sprintf("(%d,%d)", p.X, p.Y) }

type Item struct{ ID int }

func (i Item) Describe() string { return fmt.Sprint("geom item ", i.ID) }

type hidden struct{ n int }

func NewHidden(n int) hidden { return hidden{n} }
func (h hidden) N() int      { return h.n }

var Origin = Point{0, 0}
var Counter int
var Registry = map[string]int{"a": 1}

func Next() int { Counter++; return Counter }

func New(x, y int) Point { return Point{x, y} }

func init() { fmt.Println("init geom", Counter) }
