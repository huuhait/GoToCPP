package main

import (
	"fmt"
	"sync"
)

type Shape interface {
	Area() float64
}

type Rect struct{ W, H float64 }

func (r Rect) Area() float64 { return r.W * r.H }

type Circle struct{ R float64 }

func (c Circle) Area() float64 { return 3.14 * c.R * c.R }

func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func main() {
	shapes := []Shape{Rect{2, 3}, Circle{1}}
	areas := Map(shapes, func(s Shape) float64 { return s.Area() })
	fmt.Println(areas)

	defer fmt.Println("bye")

	results := make(chan int)
	var wg sync.WaitGroup
	for _, n := range []int{1, 2, 3} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- n * n
		}()
	}
	go func() { wg.Wait(); close(results) }()

	sum := 0
	for r := range results {
		sum += r
	}
	fmt.Println("sum:", sum)
}
