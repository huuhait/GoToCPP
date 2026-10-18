// phase: 08

// panic and recover inside case bodies, and defer in a loop with select: the deferred calls run at
// the end of the function in reverse order, with the values of their iteration.
package main

import "fmt"

func safe(c chan int) (res string) {
	defer func() {
		if r := recover(); r != nil {
			res = fmt.Sprint("recovered ", r)
		}
	}()
	select {
	case v := <-c:
		if v == 0 {
			panic("zero")
		}
		return fmt.Sprint("value ", v)
	}
}

func loop(c chan int) {
	for i := 0; i < 3; i++ {
		select {
		case v := <-c:
			defer fmt.Println("deferred", i, v)
		}
	}
	fmt.Println("loop done")
}

func main() {
	c := make(chan int, 3)
	c <- 0
	c <- 5
	fmt.Println(safe(c))
	fmt.Println(safe(c))
	c <- 7
	c <- 8
	c <- 9
	loop(c)
}

// Output:
// recovered zero
// value 5
// loop done
// deferred 2 9
// deferred 1 8
// deferred 0 7
