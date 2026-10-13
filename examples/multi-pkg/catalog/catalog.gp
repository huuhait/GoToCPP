// Package catalog has the items of a warehouse and the shelves they go on.
package catalog

import (
	"fmt"

	"example.com/multi-pkg/geom"
)

func init() { fmt.Println("init catalog") }

// Item is something to store. Its size is a type of package geom.
type Item struct {
	Name string
	Size geom.Rect
}

// Shelf embeds geom.Rect, so the methods of geom.Rect (Area, Fits) are
// methods of Shelf too, across the package boundary.
type Shelf struct {
	geom.Rect
	Label string
	Items []Item
}

// Used returns the area that the items on the shelf take.
func (s *Shelf) Used() int {
	used := 0
	for _, it := range s.Items {
		used += it.Size.Area()
	}
	return used
}

func (s *Shelf) Free() int { return s.Area() - s.Used() }

// DefaultShelves returns three empty shelves.
func DefaultShelves() []*Shelf {
	return []*Shelf{
		{Rect: geom.Rect{W: 100, H: 40}, Label: "A"},
		{Rect: geom.Rect{W: 100, H: 60}, Label: "B"},
		{Rect: geom.Rect{W: 60, H: 60}, Label: "C"},
	}
}

// Place puts the item on the first shelf that it fits on and that has room
// for it.
func Place(shelves []*Shelf, it Item) (*Shelf, error) {
	for _, s := range shelves {
		if it.Size.Fits(s.Rect) && s.Free() >= it.Size.Area() {
			s.Items = append(s.Items, it)
			return s, nil
		}
	}
	return nil, fmt.Errorf("no shelf for %s (%v)", it.Name, it.Size)
}
