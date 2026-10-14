// phase: 03

// Named types whose names are those of gp::newtype's base classes (step 3.9): in the C++ struct
// of a named type, the injected names of its bases (newtype, slice, array, map, ptr, string)
// would hide the package's types of those names, so these are written qualified there.
package main

import "fmt"

type array [2]int
type Grid [2]array

type ptr *int
type PP *ptr

type newtype int
type slice []newtype
type Ss []slice

type string int
type Strs []string

type Pair struct {
	a array
	s slice
}

func main() {
	g := Grid{{1, 2}, {3, 4}}
	fmt.Println(g[1][0], len(g))
	x := 5
	p := ptr(&x)
	pp := PP(&p)
	**pp = 7
	fmt.Println(x)
	ss := Ss{slice{1, 2}, {newtype(3)}}
	ss = append(ss, slice{9})
	fmt.Println(len(ss), ss[0][1], ss[2][0]*2)
	strs := Strs{1, 2}
	fmt.Println(strs[1] + 40)
	pr := Pair{array{5, 6}, slice{7}}
	fmt.Println(pr.a[1], pr.s[0])
}
