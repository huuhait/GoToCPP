// phase: 03

// Arrays are values: zero values, literals (positional, keyed, [...]T, nested), assignment,
// parameters and results copy, and == compares the elements.
package main

import "fmt"

func show(a [4]int) {
	fmt.Println(a[0], a[1], a[2], a[3])
}

func inc(a [4]int) [4]int {
	for i := range a {
		a[i]++
	}
	return a
}

func minmax(a [4]int) ([2]int, int) {
	lo, hi := a[0], a[0]
	for _, v := range a {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return [2]int{lo, hi}, hi - lo
}

func main() {
	var z [4]int
	show(z)
	a := [4]int{3, 1, 4}
	show(a)
	b := a
	b[3] = 9
	show(a)
	show(b)
	c := inc(b)
	show(b)
	show(c)
	mm, span := minmax(c)
	fmt.Println(mm[0], mm[1], span)

	k := [6]string{5: "f", 1: "b", "c"}
	fmt.Println(len(k), k[1], k[2], k[5], k[0] == "")
	auto := [...]float64{1.5, 3: 2.5}
	fmt.Println(len(auto), auto[0], auto[1], auto[3])
	grid := [2][3]int{{1, 2, 3}, {4, 5, 6}}
	grid[1][2] *= 10
	r := grid[0]
	r[0] = 100
	fmt.Println(grid[0][0], grid[1][2], r[0])
	words := [2][2]string{{"a", "b"}, {"c"}}
	fmt.Println(words[1][0], words[1][1] == "")

	fmt.Println(a == b, a != b, c == [4]int{4, 2, 5, 10})
	fmt.Println(grid == [2][3]int{{1, 2, 3}, {4, 5, 60}}, k == [6]string{1: "b", 2: "c", 5: "f"})
	var e1, e2 [0]int
	fmt.Println(e1 == e2, len(e1))
	x, y := 1, 2
	ps := [2]*int{&x, &y}
	qs := ps
	qs[1] = &x
	fmt.Println(ps == qs, ps[0] == qs[1], *qs[1])
	fs := [2]bool{true}
	fs[1] = !fs[0]
	fmt.Println(fs[0], fs[1], fs == [2]bool{true, false})
	small := [3]uint8{250, 5}
	small[0] += 10
	small[1] -= 10
	small[2]--
	fmt.Println(small[0], small[1], small[2])
}

// Output:
// 0 0 0 0
// 3 1 4 0
// 3 1 4 0
// 3 1 4 9
// 3 1 4 9
// 4 2 5 10
// 2 10 8
// 6 b c f true
// 4 1.5 0 2.5
// 1 60 100
// c true
// false true true
// true true
// true 0
// false true 1
// true false true
// 4 251 255
