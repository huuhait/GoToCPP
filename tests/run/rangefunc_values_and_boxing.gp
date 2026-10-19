// phase: 07

package main

import "fmt"

type Big struct {
	A [300]int
	S string
}

type Point struct{ X, Y int }

func Points(n int) func(yield func(Point) bool) {
	return func(yield func(Point) bool) {
		for i := range n {
			if !yield(Point{i, i * i}) {
				return
			}
		}
	}
}

func Bigs(n int) func(yield func(int, Big) bool) {
	return func(yield func(int, Big) bool) {
		for i := range n {
			var b Big
			b.A[299] = i
			b.S = fmt.Sprint("big", i)
			if !yield(i, b) {
				return
			}
		}
	}
}

func Ifaces(yield func(any) bool) {
	_ = yield(1) && yield("two") && yield(3.5) && yield([]int{4}) && yield(nil) && yield(Point{5, 6})
}

func Ptrs(yield func(*Point) bool) {
	p := &Point{}
	for i := range 3 {
		p.X = i
		if !yield(p) {
			return
		}
	}
}

func main() {
	for p := range Points(3) {
		p.X += 100
		fmt.Println(p)
	}
	var keep []Big
	for i, b := range Bigs(3) {
		b.A[0] = 1000 + i
		keep = append(keep, b)
	}
	fmt.Println(len(keep), keep[2].A[0], keep[2].A[299], keep[2].S)
	for v := range Ifaces {
		fmt.Printf("%T %v\n", v, v)
	}
	var seen []*Point
	for p := range Ptrs {
		seen = append(seen, p)
	}
	fmt.Println(seen[0] == seen[2], *seen[0])
	// variables the body changes and takes the address of
	count := 0
	var arr [3]int
	var pt Point
	ptr := &count
	for x := range Points(4) {
		count++
		*ptr += 10
		arr[x.X%3] += x.Y
		pt.X += x.X
		pt.Y = x.Y
	}
	fmt.Println(count, arr, pt)
	s := []int{}
	m := map[string]int{}
	for x := range Points(4) {
		s = append(s, x.Y)
		m[fmt.Sprint(x.X)] = x.Y
	}
	fmt.Println(s, len(m), m["3"])
	str := ""
	for p := range Points(3) {
		str += fmt.Sprint(p.X)
	}
	fmt.Println(str)
}

// Output:
// {100 0}
// {101 1}
// {102 4}
// 3 1002 2 big2
// int 1
// string two
// float64 3.5
// []int [4]
// <nil> <nil>
// main.Point {5 6}
// true {2 0}
// 44 [9 1 4] {6 9}
// [0 1 4 9] 4 9
// 012
