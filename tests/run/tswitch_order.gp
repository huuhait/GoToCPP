// phase: 05

// The first matching case wins: an interface case before a concrete one hides it (step 5.5).
package main

import "fmt"

type Sh interface{ Area() int }
type Sq int
type Re struct{ w, h int }

func (s Sq) Area() int  { return int(s * s) }
func (r *Re) Area() int { return r.w * r.h }

func first(x any) string {
	switch x.(type) {
	case Sh:
		return "Sh"
	case Sq:
		return "Sq"
	case *Re:
		return "*Re"
	case any:
		return "any"
	}
	return "unreachable"
}

func second(x any) string {
	switch x.(type) {
	case Sq:
		return "Sq"
	case *Re:
		return "*Re"
	case Sh:
		return "Sh"
	}
	return "none"
}

func main() {
	fmt.Println(first(Sq(2)), first(&Re{1, 2}), first(Re{}), first(1), first(nil))
	fmt.Println(second(Sq(2)), second(&Re{1, 2}), second(Re{}), second(1), second(nil))
}

// Output:
// Sh Sh any any unreachable
// Sq *Re none none none
