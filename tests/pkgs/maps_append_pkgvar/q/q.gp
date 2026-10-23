// Package q has the package variables that main's map appends name as q.M, q.V.M, q.P.M and q.S.
package q

var M = map[int][]int{1: {1}}

type T struct{ M map[int][]int }

var V = T{M: map[int][]int{1: {2}}}
var P = &V

//go:noinline
func Change() int { M = map[int][]int{1: {9}}; return 2 }

var S = "a"

//go:noinline
func ChS() int { S = "b"; return 5 }
