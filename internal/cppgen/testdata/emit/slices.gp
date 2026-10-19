// Slices (Phase 03 step 3.5): gp::slice<T> for elements of every type the emitter has (numbers,
// bools, strings, pointers, nested slices), nil, make, literals (also keyed), s[i] read and
// written (=, op=, ++), s[a:b] and s[a:b:c], len and cap (Go's capacities),
// append (single, several, s..., []byte + string...), copy (also from a string), clear,
// s == nil, range (the range expression evaluated once, its length too), &s[i] (the owner
// table), the conversions []byte(s), string(b), []rune(s), string(runes), slices as
// parameters, results and package variables, and a boxed slice variable (&s). s = append(s,
// x) moves s into the call. The slices print through show, which the fmt shim doesn't need to
// know (step 3.15 prints them).
package main

import "fmt"

var gnil []int
var gzero []string = nil
var glit = []int{1, 2, 3}
var gbytes = []byte("hey")

func show(s []int) {
	fmt.Print("[")
	for i, v := range s {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Println("]", len(s), cap(s) >= len(s), s == nil)
}

func showStrs(s []string) {
	fmt.Print(len(s), ":")
	for _, v := range s {
		fmt.Print(" ", v)
	}
	fmt.Println()
}

// sum takes a slice and returns a new one: the caller's slice is shared, not copied.
func sum(s []int) (int, []int) {
	t := 0
	for _, v := range s {
		t += v
	}
	s[0] = -1
	return t, append(s[:1:1], t)
}

func build(n int) []int {
	var s []int
	for i := range n {
		s = append(s, i*i)
	}
	return s
}

func main() {
	show(gnil)
	showStrs(gzero)
	show(glit)
	fmt.Println(string(gbytes), len(gbytes))

	// make, literals, nil and empty.
	var n []int
	e := []int{}
	m := make([]int, 3)
	mc := make([]int, 2, 10)
	show(n)
	show(e)
	show(m)
	show(mc)
	show([]int{5: 1, 2, 1: 7})
	fmt.Println(n == nil, e == nil, nil != m, len(mc), cap(mc))

	// Index reads and writes, op=, ++ and --, on several element types.
	b := []byte("abc")
	b[0]++
	b[1] += 10
	b[2] = b[2] << 1
	fmt.Println(b[0], b[1], b[2], string(b))
	f := []float64{1.5, 2}
	f[1] /= 4
	f[0]--
	fmt.Println(f[0], f[1])
	flags := []bool{true, false}
	flags[1] = !flags[0]
	fmt.Println(flags[0], flags[1])
	small := []int8{127, -128}
	small[0]++
	small[1]--
	fmt.Println(small[0], small[1])
	strs := []string{"a", "b"}
	strs[0] += strs[1] + "!"
	showStrs(strs)

	// Sub-slices share the array; s[a:b:c] limits the capacity, so append copies.
	s := []int{0, 1, 2, 3, 4, 5}
	t := s[1:4]
	t[0] = 10
	show(s)
	show(t)
	u := s[2:3:4]
	show(u)
	u = append(u, 30)
	u = append(u, 40)
	u[0] = 20
	show(s)
	show(u)
	show(s[:2])
	show(s[4:])
	show(s[:])
	show(t[:cap(t)])

	// append: one element, several, a slice, a string into bytes, nothing.
	var a []int
	a = append(a, 1)
	a = append(a, 2, 3, 4)
	a = append(a, s[:2]...)
	a = append(a)
	show(a)
	a = append(a[:1], a[2:]...)
	show(a)
	a = append(a, a...)
	show(a)
	bs := append([]byte("go"), "pher"...)
	fmt.Println(string(bs))
	ss := append([]string(nil), "x", "y")
	ss = append(ss, ss...)
	showStrs(ss)

	// copy (overlapping too, and from a string) and clear.
	c := []int{1, 2, 3, 4, 5}
	k := copy(c[1:], c)
	show(c)
	fmt.Println(k, copy(c, []int{9}), copy(b, "xy"))
	fmt.Println(string(b))
	clear(c[3:])
	show(c)

	// Nested slices and pointers.
	grid := make([][]int, 3)
	for i := range grid {
		grid[i] = make([]int, i+1)
		for j := range grid[i] {
			grid[i][j] = i * j
		}
	}
	grid[2] = append(grid[2], 100)
	grid[0] = grid[2][1:]
	for _, row := range grid {
		show(row)
	}
	nested := [][]string{{"a"}, nil, {"b", "c"}}
	nested[1] = append(nested[1], nested[2]...)
	nested[2][0] = "z"
	for _, row := range nested {
		showStrs(row)
	}
	x, y := 1, 2
	ptrs := []*int{&x, &y, nil}
	*ptrs[0] += 10
	ptrs[2] = ptrs[1]
	*ptrs[2] = 7
	fmt.Println(x, y, ptrs[2] == &y, ptrs[0] != nil)

	// &s[i] shares the array: it outlives the slice, and sees (and makes) its writes.
	p := &s[5]
	*p = 50
	q := &build(4)[3]
	s = nil
	*p++
	fmt.Println(*p, *q)
	pg := &glit[0]
	*pg = 100
	show(glit)

	// range: the expression once, the original length, a copy of the counters.
	r := []int{1, 2, 3}
	for i, v := range r {
		if i == 0 {
			r = append(r, 4)
			r[1] = 20
		}
		i = 10
		fmt.Print(i, v, " ")
	}
	fmt.Println(len(r))
	for i := range build(3) {
		fmt.Print(i)
	}
	for range r {
		fmt.Print(".")
	}
	var i, v int
	for i, v = range r {
	}
	fmt.Println(i, v)
	vp := []*int{}
	for _, v := range r[:2] {
		vp = append(vp, &v)
	}
	*vp[0] = 9
	fmt.Println(*vp[0], *vp[1])

	// Conversions copy.
	str := "héllo, 世界\xff"
	runes := []rune(str)
	runes[1] = 'e'
	fmt.Println(len(runes), string(runes), string(runes[7:9]), str)
	raw := []byte(str)
	raw[0] = 'H'
	fmt.Println(len(raw), string(raw[:6]), string(raw) == str, string([]byte(nil)) == "")

	// Parameters and results; a boxed slice variable.
	total, rest := sum([]int{4, 5})
	fmt.Println(total, len(rest))
	sh := []int{1, 2, 3}
	total, rest = sum(sh)
	fmt.Println(total)
	show(sh)
	show(rest)
	bx := []int{1}
	pb := &bx
	*pb = append(*pb, 2)
	(*pb)[0] = 5
	bx = append(bx, 3)
	show(bx)
	show(*pb)
}
