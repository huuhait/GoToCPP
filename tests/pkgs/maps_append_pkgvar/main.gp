// phase: 09

// m[k] = append(m[k], ...) of a map that is a variable of another package (q.M, a field of one,
// q.V.M and q.P.M) is gc's mapAppend: q.M is a name to gc, read where the entry is taken, after
// the arguments (q.Change() replaces q.M first), and so is a key q.S; append(m[k], make([]T,
// n)...) keeps its make there ("makeslice: len out of range", not growslice).
package main

import (
	"fmt"
	"math"

	"example.com/mapvar/q"
)

func try(name string, f func()) {
	defer func() { fmt.Println(name, recover(), q.M, q.V.M[1], q.P.M[1]) }()
	f()
}

func main() {
	huge := math.MaxInt
	try("pkg huge", func() { q.M[1] = append(q.M[1], make([]int, huge)...) })
	try("pkg field huge", func() { q.V.M[1] = append(q.V.M[1], make([]int, huge)...) })
	try("pkg ptr huge", func() { q.P.M[1] = append(q.P.M[1], make([]int, huge)...) })
	try("pkg order", func() { q.M[1] = append(q.M[1], q.Change()) })
	try("pkg order make", func() { q.M[1] = append(q.M[1], make([]int, q.Change())...) })
	ms := map[string][]int{}
	ms[q.S] = append(ms[q.S], q.ChS())
	fmt.Println("key", ms)
	try("pkg ok", func() { q.M[3] = append(q.M[3], make([]int, 2)...) })
}

// Output:
// pkg huge runtime error: makeslice: len out of range map[1:[1]] [2] [2]
// pkg field huge runtime error: makeslice: len out of range map[1:[1]] [2] [2]
// pkg ptr huge runtime error: makeslice: len out of range map[1:[1]] [2] [2]
// pkg order <nil> map[1:[9 2]] [2] [2]
// pkg order make <nil> map[1:[9 0 0]] [2] [2]
// key map[b:[5]]
// pkg ok <nil> map[1:[9 0 0] 3:[0 0]] [2] [2]
