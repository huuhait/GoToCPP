// phase: 06

// Multi-pkg is an app made of four packages in one module (see gocpp.mod):
// main, catalog, geom and codes, and codes is partly written in C. Each
// package prints a line in init, which shows Go's order: the packages are
// taken in the order of their import paths, each one as soon as the packages
// it imports are done. So codes starts first, although main imports catalog
// first and catalog imports geom.
package main

import (
	"fmt"

	"example.com/multi-pkg/catalog"
	"example.com/multi-pkg/codes"
	"example.com/multi-pkg/geom"
)

func init() { fmt.Println("init main") }

func main() {
	items := []catalog.Item{
		{Name: "tent", Size: geom.Rect{W: 50, H: 30}},
		{Name: "kayak", Size: geom.Rect{W: 35, H: 90}},
		{Name: "crate", Size: geom.Rect{W: 55, H: 55}},
		{Name: "lamp", Size: geom.Rect{W: 20, H: 20}},
		{Name: "sofa", Size: geom.Rect{W: 120, H: 80}},
	}
	shelves := catalog.DefaultShelves()
	for i, it := range items {
		code, err := codes.Make(fmt.Sprint(4000 + i))
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		s, err := catalog.Place(shelves, it)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		fmt.Println(it.Name, it.Size, "-> shelf", s.Label, "code", code, codes.Valid(code))
	}
	for _, s := range shelves {
		fmt.Println("shelf", s.Label, s.Rect, len(s.Items), "items,", s.Free(), "cm2 free")
	}

	// *catalog.Shelf is a geom.Shape: its Area comes from the embedded geom.Rect.
	var shapes []geom.Shape
	for _, s := range shelves {
		shapes = append(shapes, s)
	}
	fmt.Println("shelf area:", geom.TotalArea(shapes...))

	fmt.Println(codes.Valid("79927398713"), codes.Valid("79927398710"))
	_, err := codes.Make("12a4")
	fmt.Println("error:", err)
}
