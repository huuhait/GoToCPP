// phase: 08

// The default clause may be first, in the middle or last; it runs only when no other case is ready, and
// the cases after it still run.
package main

import "fmt"

func first(c chan int) string {
	select {
	default:
		return "default"
	case v := <-c:
		return fmt.Sprint("got ", v)
	}
}

func middle(a, b chan int) string {
	select {
	case v := <-a:
		return fmt.Sprint("a ", v)
	default:
		return "default"
	case v := <-b:
		return fmt.Sprint("b ", v)
	}
}

func main() {
	c := make(chan int, 1)
	fmt.Println(first(c))
	c <- 1
	fmt.Println(first(c))
	fmt.Println(first(c))

	a, b := make(chan int, 1), make(chan int, 1)
	fmt.Println(middle(a, b))
	b <- 2
	fmt.Println(middle(a, b))
	a <- 3
	fmt.Println(middle(a, b))
	fmt.Println(middle(a, b))

	// only a default, and a default with an empty body
	select {
	default:
		fmt.Println("only default")
	}
	n := 0
	for i := 0; i < 5; i++ {
		select {
		default:
			n++
		}
	}
	select {
	case <-a:
	default:
	}
	fmt.Println(n)
}

// Output:
// default
// got 1
// default
// default
// b 2
// a 3
// default
// only default
// 5
