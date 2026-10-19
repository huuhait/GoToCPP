package main

import "fmt"

type Point struct{ X, Y int }

func main() {
	var big int64 = 5
	p := Point{1, 2}
	fmt.Println(p, []int{1}, big, min(3, big))
	fmt.Printf("%5d|%v\n", big, p)
}
