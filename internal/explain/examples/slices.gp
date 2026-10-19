package main

import "fmt"

func main() {
	xs := []int{1, 2, 3, 4}
	ys := xs[1:3] // shares the array
	zs := make([]int, 2)
	n := copy(zs, ys)
	ys = append(ys, 9)
	clear(zs)
	fmt.Println(xs, ys, zs, n)
}
