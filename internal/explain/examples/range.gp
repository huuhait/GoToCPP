package main

import "fmt"

func count(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := 0; i < n; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

func main() {
	for x := range count(3) {
		fmt.Println(x)
	}
	s := "héllo"
	xs := []int{1, 2, 3}
	m := map[string]int{"a": 1}
	for i, r := range s {
		fmt.Println(i, r)
	}
	for _, v := range xs {
		xs = append(xs, v)
	}
	for k, v := range m {
		fmt.Println(k, v)
	}
	for i := range 2 {
		fmt.Println(i)
	}
}
