// phase: 03

// Arrays in maps: array keys (also elided in literals, where a constant array is a static entry
// that is stored first, -0 and +0 in a float array are one key, [0]T has one key) and array
// values (m[k] is a copy: changing it changes nothing in the map; m[k][i] reads an element, and
// a range over m[k] ranges over a copy), v, ok with an array value, a map of slices with an
// array key, and a pointer to an array as a key.
package main

import "fmt"

var gx = 1

func f() int {
	gx += 10
	return gx
}

func main() {
	pts := map[[2]int]string{{1, 2}: "a", {3, 4}: "b"}
	pts[[2]int{5, 6}] = "c"
	k := [2]int{1, 2}
	fmt.Println(pts[k], pts[[2]int{3, 4}], pts[[2]int{9, 9}] == "", len(pts))
	k[1] = 4
	_, ok := pts[k]
	fmt.Println(ok)
	delete(pts, [2]int{1, 2})
	fmt.Println(len(pts))
	sum := 0
	for key, v := range pts {
		sum += key[0]*10 + key[1] + len(v)
	}
	fmt.Println(sum)

	grid := map[string][3]int{"r": {1, 2, 3}, "s": {}}
	fmt.Println(grid["r"][1], grid["s"][2], grid["none"][0], len(grid["r"]))
	row := grid["r"]
	row[0] = 100
	fmt.Println(grid["r"][0], row[0])
	grid["t"] = [3]int{gx, f(), gx}
	fmt.Println(grid["t"][0], grid["t"][1], grid["t"][2])
	i := 2
	fmt.Println(grid["r"][i], grid["r"][i]+f())
	total := 0
	for _, v := range grid["r"] {
		total += v
	}
	for k, a := range grid {
		total += len(k) + a[0] + a[1] + a[2]
	}
	fmt.Println(total)
	v, ok2 := grid["s"]
	fmt.Println(v[0], ok2, v == [3]int{})
	nested := map[[2]string][]int{{"a", "b"}: {1}}
	nk := [2]string{"a", "b"}
	nested[nk] = append(nested[nk], 2)
	fmt.Println(len(nested[nk]), nested[[2]string{"a", "b"}][1])
	gx = 1
	lit := map[[1]int]int{{gx}: 1, {1}: 2}
	fmt.Println(len(lit), lit[[1]int{1}])
	var zero [0]int
	mz := map[[0]int]int{}
	mz[zero] = 5
	mz[[0]int{}]++
	fmt.Println(mz[zero], len(mz))
	fk := map[[2]float64]int{}
	z := 0.0
	fk[[2]float64{-z, 1}] = 1
	fk[[2]float64{z, 1}] = 2
	fmt.Println(len(fk), fk[[2]float64{0, 1}])
	pa := map[*[2]int]int{}
	arr := [2]int{}
	pa[&arr] = 3
	fmt.Println(pa[&arr])
	ag := map[int][2]int{}
	ag[1] = [2]int{f(), gx}
	fmt.Println(ag[1][0] > 0, ag[1][1] > 0)
}

// Output:
// a b true 3
// false
// 2
// 92
// 2 0 0 3
// 1 100
// 11 11 11
// 3 24
// 48
// 0 true true
// 2 2
// 1 1
// 6 1
// 1 2
// 3
// true true
