package main

import "fmt"

type node struct{ next *node }

func main() {
	xs := []int{1, 2, 3}
	i := len(xs) - 1
	n := &node{}
	m := n.next
	fmt.Println(xs[i], xs[1]/i, 10/3, m == nil, 1<<i)
}
