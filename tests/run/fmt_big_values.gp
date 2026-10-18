// phase: 05

// Large arrays and structs (more than 128 KiB) are printed from the heap copy that the emitter
// makes; a slice of a million elements; deep nesting.
package main

import "fmt"

type Big struct {
	A [20000]int
	S string
}

func main() {
	var a [30000]int
	a[0], a[29999] = 1, 2
	s := fmt.Sprint(a)
	fmt.Println(len(s), s[:10], s[len(s)-10:])
	var b Big
	b.A[19999] = 7
	b.S = "big"
	t := fmt.Sprintf("%+v", b)
	fmt.Println(len(t), t[:12], t[len(t)-14:])
	pb := &b
	u := fmt.Sprint(pb)
	fmt.Println(len(u), u[:3])
	sl := make([]int, 1000000)
	sl[999999] = 5
	v := fmt.Sprint(sl)
	fmt.Println(len(v), v[len(v)-6:])
	nest := [][][]int{{{1}, {2, 3}}, {{}}, nil}
	fmt.Println(nest, fmt.Sprintf("%v|%d|%+v", nest, nest, nest))
	deep := []any{[]any{[]any{[]any{[]any{1, "x"}}}}}
	fmt.Println(deep, fmt.Sprintf("%#v", deep))
}

// Output:
// 60001 [1 0 0 0 0 0 0 0 0 2]
// 40011 {A:[0 0 0 0   0 0 7] S:big}
// 40008 &{[
// 2000001 0 0 5]
// [[[1] [2 3]] [[]] []] [[[1] [2 3]] [[]] []]|[[[1] [2 3]] [[]] []]|[[[1] [2 3]] [[]] []]
// [[[[[1 x]]]]] []interface {}{[]interface {}{[]interface {}{[]interface {}{[]interface {}{1, "x"}}}}}
