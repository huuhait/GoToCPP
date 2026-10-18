// phase: 05

// A String method that changes what is being printed: an operand is a copy, but through a pointer
// fmt reads the slice again for each element (Stamp changes a global printed after it, MapEdit
// deletes from the map that holds it, Clear and Drop empty the slice or the slice of slices being
// printed).
package main

import "fmt"

type Stamp int

var later = []int{1, 2}

func (s Stamp) String() string {
	later = nil
	return "stamp"
}

type MapEdit int

var edited map[int]MapEdit

func (m MapEdit) String() string {
	for k := range edited {
		delete(edited, k)
	}
	return "edit" + fmt.Sprint(int(m))
}

type Clear int

type Holder struct{ L []Clear }

var held Holder

func (Clear) String() string {
	held.L = nil
	return "c"
}

type Drop int

var drops [][]Drop

func (Drop) String() string {
	drops = nil
	return "d"
}

type Pair struct{ A, B Clear }

var pp *Pair

func main() {
	fmt.Println(Stamp(1), later)
	edited = map[int]MapEdit{1: 1, 2: 2, 3: 3}
	fmt.Println(edited, len(edited))
	edited = map[int]MapEdit{1: 1, 2: 2, 3: 3}
	fmt.Printf("%v %d\n", &edited, len(edited))
	held = Holder{[]Clear{1, 2, 3}}
	fmt.Println(held, held.L == nil)
	held = Holder{[]Clear{1, 2, 3}}
	fmt.Println(&held, held.L == nil)
	drops = [][]Drop{{1, 2}, {3}}
	fmt.Println(&drops, drops == nil)
	drops = [][]Drop{{1, 2}, {3}}
	fmt.Println(drops, drops == nil)
	held = Holder{[]Clear{1, 2, 3}}
	fmt.Printf("%v %+v %s\n", &held, &held, &held)
}

// Output:
// stamp [1 2]
// map[1:edit1 2:edit2 3:edit3] 3
// &map[1:edit1 2:edit2 3:edit3] 3
// {[c c c]} false
// &{[c]} false
// &[[d d]] false
// [[d d] [d]] false
// &{[c]} &{L:[]} &{[]}
