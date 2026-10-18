// phase: 08

// Large values through select: arrays and structs with big arrays, received into case variables and
// into existing variables; the zero value of a closed channel is all zeros.
package main

import "fmt"

type blob struct {
	data [1000]int
	name string
}

func main() {
	c := make(chan blob, 1)
	var b blob
	for i := range b.data {
		b.data[i] = i
	}
	b.name = "first"
	select {
	case c <- b:
	}
	b.data[0] = -1
	select {
	case got := <-c:
		sum := 0
		for _, x := range got.data {
			sum += x
		}
		fmt.Println(got.name, sum, got.data[0], b.data[0])
	}
	var dst blob
	select {
	case c <- b:
	}
	select {
	case dst = <-c:
	}
	fmt.Println(dst.name, dst.data[0], dst.data[999])
	close(c)
	select {
	case z, ok := <-c:
		zero := true
		for _, x := range z.data {
			zero = zero && x == 0
		}
		fmt.Println(zero, z.name == "", ok)
	}
}

// Output:
// first 499500 0 -1
// first -1 999
// true true false
