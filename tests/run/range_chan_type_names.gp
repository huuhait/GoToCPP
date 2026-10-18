// phase: 08

// Range over named channel types and channels of named types.
package main

import "fmt"

type Celsius float64

type Pipe chan int

func (p Pipe) fill(n int) {
	for i := 0; i < n; i++ {
		p <- i
	}
	close(p)
}

func main() {
	p := make(Pipe, 4)
	p.fill(4)
	for v := range p {
		fmt.Println(v)
	}
	temps := make(chan Celsius, 3)
	temps <- 20.5
	temps <- -3
	temps <- 100
	close(temps)
	var total Celsius
	for t := range temps {
		total += t
	}
	fmt.Println(total)
}

// Output:
// 0
// 1
// 2
// 3
// 117.5
