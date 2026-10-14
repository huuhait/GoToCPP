// phase: 03

// Methods (step 3.10) are member functions of their type's struct: gdb stops in them, bt names
// them gpkg_main::Point::Add (a value method, a const member) and gpkg_main::Point::Move (a
// pointer method, a static member), next steps through their lines, and the value receiver is a
// local copy under its Go name (print p.X), made on the func's line; the pointer receiver is the
// parameter (print p->X). A receiver that the body can't change is a const reference to the
// caller's value (Len).
package main

import "fmt"

type Point struct{ X, Y int }

func (p Point) Add(q Point) Point {
	p.X += q.X // gdb: print p.X = 1; print q.X = 10; bt gpkg_main::Point::Add gpkg_main::main; next 17
	p.Y += q.Y
	return p
}

func (p *Point) Move(dx int) {
	p.X += dx // gdb: print p->X = 1; print dx = 5; bt gpkg_main::Point::Move gpkg_main::main; next 23
	fmt.Println("moved", p.X)
}

func (p Point) Len() int {
	return p.X + p.Y // gdb: print p ~ ^\(const gpkg_main::Point &\) @0x[0-9a-f]+: \{X = 6, Y = 2,; print p.Y = 2
}

func main() {
	p := Point{1, 2}
	q := p.Add(Point{10, 20})
	p.Move(5)
	fmt.Println(q, p.Len())
}
