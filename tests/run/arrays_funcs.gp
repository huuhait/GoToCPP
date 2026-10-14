// phase: 03

// Arrays as package variables (constants are set at compile time, the others at
// initialization), parameters, results (also several), elements of slices, and large arrays
// (over 128 KiB: boxed, and ranged over a copy on the heap).
package main

import "fmt"

var primes = [5]int{2, 3, 5, 7, 11}
var names = [3]string{"a", "b", "c"}
var sq = squares()
var big [1 << 17]int64

func squares() [4]int {
	var s [4]int
	for i := range s {
		s[i] = i * i
	}
	return s
}

func swap(a [2]string) ([2]string, [2]string) {
	b := a
	b[0], b[1] = a[1], a[0]
	return a, b
}

func total(m [3][3]int) int {
	t := 0
	for i := range m {
		t += m[i][i]
	}
	return t
}

func main() {
	fmt.Println(primes[4], names[2], sq[3], len(big))
	x, y := swap([2]string{"l", "r"})
	fmt.Println(x[0], y[0])
	fmt.Println(total([3][3]int{{1}, {0, 2}, {0, 0, 3}}))
	list := [][2]int{}
	for i := range 3 {
		list = append(list, [2]int{i, -i})
	}
	list[1][1] = 100
	fmt.Println(len(list), list[1][1], list[2][1])

	var local [1 << 17]int64 // 1 MiB: boxed
	local[5] = 5
	big[7] = 7
	sum := int64(0)
	for i, v := range local {
		if i == 0 {
			local[5] = 500 // not seen: the copy is on the heap
		}
		sum += v
	}
	for i, v := range big {
		if i == 0 {
			big[7] = 700
		}
		sum += v
	}
	cp := local
	cp[1] = 1
	fmt.Println(sum, local[5], big[7], cp[1], local[1], cp == local)
}

// Output:
// 11 c 9 131072
// l r
// 6
// 3 100 -2
// 12 500 700 1 0 false
