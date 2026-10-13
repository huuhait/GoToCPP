// Package geom has sizes, in centimeters, and the Shape interface.
package geom

import "fmt"

func init() { fmt.Println("init geom") }

// Rect is the size of something seen from above.
type Rect struct {
	W, H int
}

func (r Rect) Area() int { return r.W * r.H }

// Fits reports whether r fits into box, maybe turned by 90 degrees.
func (r Rect) Fits(box Rect) bool {
	return r.W <= box.W && r.H <= box.H || r.H <= box.W && r.W <= box.H
}

func (r Rect) String() string { return fmt.Sprintf("%dx%d", r.W, r.H) }

// A Shape has an area. Types of other packages implement it too.
type Shape interface {
	Area() int
}

// TotalArea adds up the areas of the shapes.
func TotalArea(shapes ...Shape) int {
	total := 0
	for _, s := range shapes {
		total += s.Area()
	}
	return total
}
