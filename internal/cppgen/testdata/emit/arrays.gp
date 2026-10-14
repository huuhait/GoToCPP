// Arrays (Phase 03 step 3.6): gp::array<T, N> for elements of every type the emitter has
// (numbers, bools, strings, pointers, slices, nested arrays), zero values, literals (positional,
// keyed, [...]T, nested), a[i] read and written (=, op=, ++), through a pointer (p[i]), len and
// cap (constants, also of a nil pointer), == and !=, value semantics (assignment, parameters,
// results copy), a[lo:hi] and a[lo:hi:max] of boxed and package arrays and p[lo:hi], &a[i]
// (the owner table, also nested and through calls: gp::elem_ptr), &[N]T{...}, the conversions
// [N]T(s) and (*[N]T)(s), range (over a copy only when the body could change the array; through
// a pointer; the forms without a value count to N without evaluating the array), constinit
// package arrays of constants, and arrays over 128 KiB (built in their boxes; parameters by
// reference). The arrays print through show, which the fmt shim doesn't need
// to know (step 3.15 prints them).
package main

import "fmt"

var gzero [3]int
var glit = [3]int{1, 2, 3}
var gkeyed = [5]int{4: 40, 1: 10}
var gnested = [2][2]int32{{1, 2}, {3, 4}}
var gfloats = [...]float64{0.5, 2: -1.25}
var gstrs = [2]string{"a", "b"}
var gsl = glit[1:]
var gp3 = &glit

func show(a [3]int) {
	fmt.Println(a[0], a[1], a[2])
}

func showStrs(a [2]string) {
	fmt.Println(len(a), a[0], a[1])
}

// double changes its copy and returns it.
func double(a [3]int) [3]int {
	for i := range a {
		a[i] *= 2
	}
	return a
}

func pair(x, y int) ([2]int, bool) {
	return [2]int{x, y}, x == y
}

func mk() *[3]int {
	fmt.Print("mk ")
	return &[3]int{7, 8, 9}
}

// sumBig takes an array over 128 KiB: by reference, copied into a box at the entry.
func sumBig(a [1 << 15]int, k int) int {
	a[0] += k
	t := 0
	for _, v := range a {
		t += v
	}
	return t
}

func rows() [][3]int {
	return [][3]int{{1, 2, 3}, {4, 5, 6}}
}

func main() {
	show(gzero)
	show(glit)
	fmt.Println(gkeyed[0], gkeyed[1], gkeyed[4], len(gkeyed), gnested[1][0], gfloats[2], len(gfloats))
	showStrs(gstrs)

	// Zero values, literals, value semantics.
	var z [3]int
	a := [3]int{7, 8, 9}
	b := a
	b[0] = 70
	show(z)
	show(a)
	show(b)
	show(double(a))
	show(a)
	k := [...]string{2: "c", "d"}
	fmt.Println(len(k), k[0] == "", k[3])
	part := [4]int{1, 2}
	fmt.Println(part[1], part[3])
	two, same := pair(3, 3)
	fmt.Println(two[1], same)
	var empty [0]int
	fmt.Println(len(empty), empty == [0]int{})

	// Index reads and writes, op=, ++, on several element types; comparisons.
	i := 2
	a[i] += 5
	a[i]++
	a[0] = a[1] * 2
	show(a)
	fl := [2]float64{1.5, 2}
	fl[1] /= 4
	fl[0]--
	fmt.Println(fl[0], fl[1])
	small := [2]int8{127, -128}
	small[0]++
	small[1]--
	fmt.Println(small[0], small[1])
	strs := [2]string{"x", "y"}
	strs[0] += strs[1] + "!"
	showStrs(strs)
	fmt.Println(a == b, a != b, a == [3]int{16, 8, 15}, strs == [2]string{"xy!", "y"})
	x, y := 1, 2
	ptrs := [3]*int{&x, &y}
	*ptrs[0] += 10
	ptrs[2] = ptrs[1]
	fmt.Println(x, ptrs[2] == &y, ptrs[0] != nil, ptrs == [3]*int{&x, &y, &y})
	sls := [2][]int{{1}, nil}
	sls[1] = append(sls[1], sls[0]...)
	fmt.Println(len(sls[0]), sls[1][0], sls[0] == nil)

	// Nested arrays, slices of arrays.
	grid := [2][3]int{{1, 2, 3}, {4, 5, 6}}
	row := grid[1]
	row[0] = 40
	grid[0][2] *= 10
	fmt.Println(grid[1][0], row[0], grid[0][2])
	rs := rows()
	rs[1][2] = 60
	fmt.Println(rs[1][2], len(rs), rs[0] == [3]int{1, 2, 3})

	// Pointers to arrays: p[i], len and cap of a nil one, &[N]T{...}.
	p := &a
	p[1] = 200
	show(a)
	var np *[3]int
	fmt.Println(len(p), cap(p), len(np), cap(mk()))
	lp := &[2]string{"p", "q"}
	lp[0] = "r"
	showStrs(*lp)
	gp3[2] = 33

	// Slicing shares the array: a boxed local, a package array, through a pointer, a row.
	s := a[:]
	s[0] = 100
	t := a[1:2]
	u := a[0:1:2]
	u = append(u, 30)
	u = append(u, 40)
	u[0] = 50
	show(a)
	fmt.Println(len(t), cap(t), cap(u), len(s))
	gsl[0] = 20
	g2 := glit[:2]
	g2 = append(g2, 300)
	show(glit)
	ps := p[1:]
	ps[0] = -1
	r2 := grid[1][:]
	r2[1] = 50
	fmt.Println(a[1], grid[1][1], len(mk()[1:]))

	// &a[i]: the pointer shares the owner of the array.
	q := &a[2]
	*q = 3
	qg := &glit[0]
	*qg = 1
	qq := &grid[1][2]
	*qq = 66
	qp := &p[0]
	*qp = 5
	qr := &rs[0][1]
	*qr = 22
	qm := &mk()[1]
	qn := &rows()[1][0]
	*qm++
	show(a)
	show(glit)
	fmt.Println(grid[1][2], rs[0][1], *qm, *qn)

	// Conversions from slices.
	sl := []int{1, 2, 3, 4}
	arr := [3]int(sl)
	arr[0] = 10
	ap := (*[2]int)(sl)
	ap[1] = 22
	var nilsl []int
	fmt.Println(arr[0], sl[0], sl[1], len(ap), (*[0]int)(nilsl) == nil, [0]int(nilsl) == empty)

	// range: a copy where the body could change the array, else the array itself.
	for i, v := range a {
		if i == 0 {
			a[2] = -1
		}
		fmt.Print(v, " ")
	}
	fmt.Println(a[2])
	for i, v := range p {
		if i == 0 {
			a[2] = -2
		}
		fmt.Print(v, " ")
	}
	fmt.Println()
	for _, v := range b {
		fmt.Print(v, " ")
	}
	for i, v := range [2]string{"m", "n"} {
		i = 10
		fmt.Print(i, v, " ")
	}
	for _, r := range grid {
		for _, v := range r {
			fmt.Print(v, ",")
		}
	}
	fmt.Println()
	for i := range a {
		fmt.Print(i)
	}
	for range *np { // not evaluated: no nil dereference
		fmt.Print(".")
	}
	for i, _ := range np {
		fmt.Print(i)
	}
	for i := range mk() {
		fmt.Print(i)
	}
	fmt.Println()
	var j, w int
	for j, w = range glit {
	}
	fmt.Println(j, w)
	vp := []*int{}
	for _, v := range b {
		vp = append(vp, &v)
	}
	*vp[0] = 9
	fmt.Println(*vp[0], *vp[1], b[0])

	// Arrays over 128 KiB: built in their boxes; a parameter by reference.
	big := [1 << 15]int{2: 5}
	big2 := [1 << 15]int(make([]int, 1<<15))
	fmt.Println(sumBig(big, 1), big[0], sumBig(big2, 0))

	// Assignments that evaluate the target's operands first.
	c := [2]int{1, 2}
	c, c[0] = [2]int{9, 9}, 5
	n := 0
	n, c[n] = 1, 7
	fmt.Println(c[0], c[1], n)
}
