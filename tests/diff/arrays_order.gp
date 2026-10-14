// phase: 03

// The order of evaluation with arrays, as gc does it: an element of a package array, of a
// boxed local array and through a pointer is read after the calls of its statement (which can
// change it); a slice of an array is taken in its place among the calls; the operands of an
// assignment's targets are evaluated before the stores, the array itself is not copied
// (a, a[0] = b, 5 stores into the new a); [N]T(s) reads s after the calls.
package main

import "fmt"

var ga = [4]int{1, 2, 3, 4}
var pa = &ga
var gs = []int{1, 2, 3}

func f(tag int) int {
	fmt.Print("f", tag, " ")
	ga[tag&3] += 10
	gs[0] += tag
	return tag
}

func set(p *[3]int, v int) int {
	fmt.Print("set", v, " ")
	p[1] = v
	return v
}

func main() {
	fmt.Println(ga[1], f(1), ga[1])
	fmt.Println(pa[2]+f(2), (*pa)[2])
	x := ga[3] * f(3)
	fmt.Println(x)
	y, z := ga[0], f(0)
	fmt.Println(y, z)

	b := [3]int{1, 2, 3}
	pb := &b
	fmt.Println(b[1], set(pb, 20), b[1])
	fmt.Println(b == [3]int{1, 20, 3}, set(pb, 30), b == [3]int{1, 30, 3})
	l := [2]int{5, 6}
	fmt.Println(l[0], f(1), l[1])

	s := ga[1:3]
	fmt.Println(s[0], f(1), s[0])
	fmt.Println(len(ga[:f(2)]), ga[2])
	fmt.Println([3]int(gs)[0], f(5))
	fmt.Println((*[2]int)(gs)[0], f(6))

	ga[f(0)] = f(3)
	fmt.Println(ga[0], ga[1], ga[2], ga[3])
	ga[f(1)] += f(2)
	fmt.Println(ga[1])

	c := [2]int{1, 2}
	c, c[0] = [2]int{9, 9}, 5
	fmt.Println(c[0], c[1])
	i := 0
	i, c[i] = 1, 7
	fmt.Println(c[0], c[1], i)
	q := &[2]int{4, 5}
	pc := &c
	pc, pc[0] = q, 10
	fmt.Println(c[0], q[0], pc[0])
	g := [2][2]int{}
	j := 1
	j, g[j][j-1] = 0, 8
	fmt.Println(g[1][0], g[0][0], j)
	sl := [][2]int{{1, 2}}
	old := sl
	sl, sl[0][1] = nil, 20
	fmt.Println(old[0][1], sl == nil)
	k := 0
	arr := [3]int{}
	arr[k], k = f(1), f(2)
	fmt.Println(arr[0], arr[2], k)
}
