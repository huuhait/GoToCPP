// Package image is the one type of Go's image package that Go's tests of encoding/json embed.
package image

import "strconv"

// A Point is an X, Y coordinate pair.
type Point struct {
	X, Y int
}

// String returns a string representation of p like "(3,4)".
func (p Point) String() string {
	return "(" + strconv.Itoa(p.X) + "," + strconv.Itoa(p.Y) + ")"
}
