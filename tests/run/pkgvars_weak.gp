// phase: 03

// Weak pointers in package variables: a zero one, one made in a variable's initializer from a
// pointer to another package variable (whose static owner never frees it), a registry map of
// weak pointers filled by init, and a struct variable with a weak field.
package main

import (
	"fmt"
	"weak"
)

type Svc struct{ name string }

var (
	main0   = Svc{"main"}
	zero    weak.Pointer[Svc]
	toMain  = weak.Make(&main0)
	reg     = map[string]weak.Pointer[Svc]{}
	keep    []*Svc
	wrapped = struct {
		w weak.Pointer[Svc]
		n int
	}{toMain, 1}
)

func init() {
	for _, n := range []string{"db", "cache"} {
		s := &Svc{n}
		keep = append(keep, s)
		reg[n] = weak.Make(s)
	}
	reg["main"] = toMain
}

func main() {
	fmt.Println(zero.Value() == nil, toMain.Value().name, wrapped.w == toMain, wrapped.n)
	for _, n := range []string{"cache", "db", "main", "none"} {
		if s := reg[n].Value(); s != nil {
			fmt.Println(n, s.name, s == &main0)
		} else {
			fmt.Println(n, "nil")
		}
	}
	main0.name = "renamed"
	fmt.Println(toMain.Value().name, len(keep))
}

// Output:
// true main true 1
// cache cache false
// db db false
// main main true
// none nil
// renamed 2
