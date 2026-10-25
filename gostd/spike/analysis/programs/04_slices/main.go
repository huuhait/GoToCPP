// Slices and arrays: make with len and cap, append with several values, copy, two- and
// three-index slicing, array values and keyed array literals, range over slice/array/int, and
// nil slices.
package main

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

func main() {
	a := []int{3, 1, 4}
	println(len(a), cap(a))
	a = append(a, 5, 9)
	println(len(a))
	b := make([]int, 2, 4)
	println(len(b), cap(b), b[0])
	c := make([]string, 2)
	println(len(c), c[0] == "")
	copy(b, a)
	println(b[0], b[1])
	println(sum(a))
	s := a[1:3]
	println(len(s), cap(s), s[0])
	s2 := a[:2:2]
	println(len(s2), cap(s2))
	arr := [4]int{1, 2, 3, 4}
	arr2 := arr
	arr2[0] = 9
	println(arr[0], arr2[0], len(arr))
	var keyed [256]uint8
	keyed['a'] = 1
	println(keyed['a'], keyed['b'])
	var grid [2][3]int
	grid[1][2] = 7
	println(grid[0][0], grid[1][2], len(grid), len(grid[0]))
	var nilSlice []int
	println(nilSlice == nil, len(nilSlice), cap(nilSlice))
	for i := range 3 {
		println("i", i)
	}
	for i, v := range []int{10, 20} {
		println(i, v)
	}
	for i, v := range arr {
		println("arr", i, v)
	}
	ls := [][]int{{1}, {2, 3}}
	println(len(ls), len(ls[1]), ls[1][1])
}
