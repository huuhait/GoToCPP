// phase: 08

// A receive is evaluated where Go evaluates calls and receives: left to right. The channel
// operand and the value of a send are evaluated before the communication.
package main

import "fmt"

var log []string

func ch(name string, c chan int) chan int {
	log = append(log, "chan "+name)
	return c
}

func val(name string, v int) int {
	log = append(log, "val "+name)
	return v
}

func main() {
	a, b := make(chan int, 3), make(chan int, 3)
	for i := 1; i <= 3; i++ {
		a <- i
		b <- i * 10
	}
	fmt.Println(<-a, <-a, <-a)
	fmt.Println(<-b - <-b, <-b)

	a <- 1
	a <- 2
	x := <-a*10 + <-a
	fmt.Println(x)

	ch("first", a) <- val("one", 1)
	ch("second", b) <- val("two", 2)
	fmt.Println(log, len(a), len(b))
	fmt.Println(<-ch("third", a), <-ch("fourth", b), log[len(log)-2:])

	arr := []int{0, 0, 0, 0}
	a <- 3
	b <- 1
	arr[<-b] = <-a
	fmt.Println(arr)

	m := map[int]int{}
	a <- 7
	a <- 8
	m[<-a] = <-a
	fmt.Println(m)

	n := 0
	f := func() int { n++; return n }
	a <- 100
	fmt.Println(f(), <-a, f(), len(a))
}

// Output:
// 1 2 3
// -10 30
// 12
// [chan first val one chan second val two] 1 1
// 1 2 [chan third chan fourth]
// [0 3 0 0]
// map[7:8]
// 1 100 2 0
