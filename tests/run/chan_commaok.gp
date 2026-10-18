// phase: 08

// The forms of a receive: v := <-c, v, ok := <-c, v, ok = <-c, var v, ok = <-c, _ = <-c, <-c, and a
// receive in a condition.
package main

import "fmt"

func main() {
	c := make(chan int, 10)
	for i := 1; i <= 8; i++ {
		c <- i * 10
	}
	close(c)

	a := <-c
	b, ok := <-c
	fmt.Println(a, b, ok)
	var d int
	var okd bool
	d, okd = <-c
	fmt.Println(d, okd)
	var e, oke = <-c
	fmt.Println(e, oke)
	_ = <-c
	<-c
	_, ok = <-c
	fmt.Println(ok)
	if v, ok := <-c; ok {
		fmt.Println("if:", v)
	}
	if v, ok := <-c; !ok {
		fmt.Println("closed and empty:", v)
	}
	for v, ok := <-c; ok; v, ok = <-c {
		fmt.Println("never", v)
	}
	switch v, ok := <-c; {
	case !ok:
		fmt.Println("switch on closed", v)
	}
}

// Output:
// 10 20 true
// 30 true
// 40 true
// true
// if: 80
// closed and empty: 0
// switch on closed 0
