// phase: 05

package main

import "fmt"

type Big [40000]int

func sum(b Big) int {
	t := 0
	for _, v := range b {
		t += v
	}
	return t
}

func f() (n int) {
	var b Big
	for i := range b {
		b[i] = i
	}
	defer func() {
		if r := recover(); r != nil {
			n = -n
		}
	}()
	defer fmt.Println("sum at defer:", sum(b))
	b[0] = 1000000
	n = sum(b)
	panic("big")
}

func main() {
	fmt.Println(f())
}
