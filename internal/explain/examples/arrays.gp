package main

import "fmt"

func sum(a [3]int) int {
	t := 0
	for _, v := range a {
		t += v
	}
	return t
}

func main() {
	a := [3]int{1, 2, 3}
	b := a
	b[0] = 10
	fmt.Println(sum(a), sum(b), a == b, a[1:])
}
