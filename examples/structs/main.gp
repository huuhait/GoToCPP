// phase: 03

// Structs shows struct values, value and pointer receivers, pointers to
// structs, struct comparison, method values and a named non-struct type with
// methods.
package main

import "fmt"

type Point struct {
	X, Y int
}

// Add has a value receiver: p is a copy, so the caller's Point never changes.
func (p Point) Add(q Point) Point {
	p.X += q.X
	p.Y += q.Y
	return p
}

// Move has a pointer receiver: it changes the caller's Point.
func (p *Point) Move(dx, dy int) {
	p.X += dx
	p.Y += dy
}

type Rect struct {
	Min, Max Point
}

// NewRect returns a pointer to a new Rect; it stays alive as long as it is used.
func NewRect(x, y, w, h int) *Rect {
	return &Rect{Min: Point{x, y}, Max: Point{X: x + w, Y: y + h}}
}

func (r Rect) Width() int  { return r.Max.X - r.Min.X }
func (r Rect) Height() int { return r.Max.Y - r.Min.Y }
func (r Rect) Area() int   { return r.Width() * r.Height() }

// Contains reports whether p is inside r (Max is outside).
func (r Rect) Contains(p Point) bool {
	return p.X >= r.Min.X && p.X < r.Max.X && p.Y >= r.Min.Y && p.Y < r.Max.Y
}

// Translate moves both corners. r.Min.Move(...) is (&r.Min).Move(...).
func (r *Rect) Translate(dx, dy int) {
	r.Min.Move(dx, dy)
	r.Max.Move(dx, dy)
}

// Celsius is a named type that is not a struct; it can have methods too.
type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

func (c Celsius) Fahrenheit() float64 { return float64(c)*9/5 + 32 }

type Account struct {
	Owner   string
	Balance int
	History []int
}

// Deposit changes the account through the pointer, including its slice field.
func (a *Account) Deposit(amounts ...int) {
	for _, n := range amounts {
		a.Balance += n
		a.History = append(a.History, n)
	}
}

func main() {
	p := Point{1, 2}
	q := p.Add(Point{10, 20})
	fmt.Println(p, q)
	p.Move(5, 5) // p is addressable, so this is (&p).Move(5, 5)
	fmt.Println(p)

	pp := &p
	pp.Move(1, 1)                     // a pointer method through a pointer
	fmt.Println(*pp, pp.Add(Point{})) // a value method through a pointer
	fmt.Println(pp, p == *pp)

	r := NewRect(0, 0, 4, 3)
	fmt.Println(r.Width(), r.Height(), r.Area())
	fmt.Println(r.Contains(Point{2, 2}), r.Contains(Point{4, 1}))
	r.Translate(10, 10)
	fmt.Printf("%v %+v\n", *r, r.Min)

	// Struct values are copied when assigned, and compared field by field.
	r2 := *r
	r2.Max.X = 100
	var zero Rect
	fmt.Println(r.Max.X, r2.Max.X, r2 == *r, zero == Rect{})

	// A method value binds its receiver when it is made: area keeps a copy
	// of *r, move keeps &p.
	area := r.Area
	move := p.Move
	r.Translate(1, 1)
	r.Max.X += 6
	move(100, 0)
	fmt.Println(area(), r.Area(), p)

	// A slice of structs, changed through an index and through a pointer.
	pts := []Point{{1, 1}, {2, 4}, {3, 9}}
	pts[0].X = 7
	last := &pts[2]
	last.Move(0, 1)
	fmt.Println(pts, len(pts))

	for _, c := range []Celsius{-40, 0, 37, 100} {
		fmt.Println(c, c.Fahrenheit())
	}

	acct := Account{Owner: "ann"}
	acct.Deposit(50, 25)
	acct.Deposit()
	acct.Deposit(-30)
	fmt.Println(acct.Owner, acct.Balance, acct.History)
	fmt.Println(acct)
}
