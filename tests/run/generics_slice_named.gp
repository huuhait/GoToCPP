// phase: 07

// A defined slice type stays itself through a generic function: the type argument is S, not []E.
package main

import "fmt"

type IDs []int

func (ids IDs) Sum() int {
	s := 0
	for _, i := range ids {
		s += i
	}
	return s
}

type Names []string

func Clone[S ~[]E, E any](s S) S { return append(S(nil), s...) }

func Filter[S ~[]E, E any](s S, keep func(E) bool) S {
	out := S{}
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

func Make[S ~[]E, E any](n int) S { return make(S, n) }

func Append[S ~[]E, E any](s S, vs ...E) S { return append(s, vs...) }

func First[S ~[]E, E any](s S) E { return s[0] }

func main() {
	ids := IDs{3, 4, 5}
	c := Clone(ids)
	fmt.Printf("%T %v %d\n", c, c, c.Sum())
	f := Filter(ids, func(i int) bool { return i != 4 })
	fmt.Printf("%T %v %d\n", f, f, f.Sum())
	m := Make[Names](2)
	fmt.Printf("%T %q\n", m, m)
	m = Append(m, "x", "y")
	fmt.Printf("%T %q %d\n", m, m, len(m))
	fmt.Println(Append(IDs(nil), 1, 2).Sum(), First(Names{"a", "b"}), First(ids))
	var plain []int = Clone([]int(ids))
	fmt.Printf("%T\n", plain)
}

// Output:
// main.IDs [3 4 5] 12
// main.IDs [3 5] 8
// main.Names ["" ""]
// main.Names ["" "" "x" "y"] 4
// 3 a 3
// []int
