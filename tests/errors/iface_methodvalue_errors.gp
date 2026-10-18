// phase: 05

// Errors in method values and method expressions of interfaces (step 5.9).
package main

type Shape interface {
	Area() int
	Scale(k int) Shape
}

type Sq struct{ s int }

func (q Sq) Area() int         { return q.s }
func (q Sq) Scale(k int) Shape { return Sq{q.s * k} }

type Pt struct{ x int }

func (p *Pt) Area() int         { return p.x }
func (p *Pt) Scale(k int) Shape { return p }

func main() {
	var s Shape = Sq{1}
	_ = s.Perimeter              // ERROR "s.Perimeter undefined \(type Shape has no field or method Perimeter\)"
	_ = Shape.Perimeter          // ERROR "Shape.Perimeter undefined \(type Shape has no field or method Perimeter\)"
	_ = (*Shape).Area            // ERROR "\(\*Shape\).Area undefined \(type \*Shape is pointer to interface, not interface\)"
	_ = Pt.Area                  // ERROR "invalid method expression Pt.Area \(needs pointer receiver \(\*Pt\).Area\)"
	var f func() string = s.Area // ERROR "cannot use s.Area \(value of type func\(\) int\) as func\(\) string value in variable declaration"
	g := s.Scale
	_ = g("x") // ERROR "cannot use .x. \(untyped string constant\) as int value in argument to g"
	_ = g()    // ERROR "not enough arguments in call to g"
	h := Shape.Scale
	_ = h(s)       // ERROR "not enough arguments in call to h"
	_ = h(Pt{}, 2) // ERROR "cannot use Pt\{\} \(value of .*type Pt\) as Shape value in argument to h: Pt does not implement Shape \(method Area has pointer receiver\)"
	s.Area = nil   // ERROR "cannot assign to s.Area \(neither addressable nor a map index expression\)"
	_ = f
}
