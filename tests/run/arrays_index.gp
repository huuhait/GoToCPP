// phase: 03

// a[i] reads and writes (=, op=, ++, --), with indexes of any integer type, through a pointer
// to an array (p[i] and (*p)[i]), and len and cap, also of a nil pointer to an array.
package main

import "fmt"

var counts [5]int

func bump(i uint8) {
	counts[i]++
}

func get() *[3]string {
	fmt.Println("get")
	return &[3]string{"x", "y", "z"}
}

func main() {
	for _, c := range "hello" {
		bump(uint8(c % 5))
	}
	fmt.Println(counts[0], counts[1], counts[2], counts[3], counts[4])
	var hist [4]float64
	var j int8 = 3
	hist[j] = 1.5
	hist[j] *= 4
	hist[0]--
	fmt.Println(hist[0], hist[3])

	a := [3]int{1, 2, 3}
	p := &a
	p[0] = 10
	(*p)[1] += 5
	p[2]++
	fmt.Println(a[0], a[1], a[2], p[0] == a[0])
	q := p
	q[0] = -1
	fmt.Println(a[0], len(p), cap(q))
	var np *[7]int
	fmt.Println(len(np), cap(np), np == nil)
	fmt.Println(len(get()), get()[1])
	m := [2][2]int{}
	for i := range 2 {
		for k := range 2 {
			m[i][k] = i*2 + k
		}
	}
	m[1][0] <<= 3
	fmt.Println(m[0][0], m[0][1], m[1][0], m[1][1])
}

// Output:
// 0 2 0 2 1
// -1 6
// 10 7 4 true
// -1 3 3
// 7 7 true
// get
// get
// 3 y
// 0 1 16 3
