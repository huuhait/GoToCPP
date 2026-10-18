// phase: 08

// The assignment form: for x = range ch, for arr[i] = range ch, for s.f = range ch, for *p = range ch.
package main

import "fmt"

type holder struct{ last int }

func main() {
	c := make(chan int, 6)
	for i := 1; i <= 6; i++ {
		c <- i
	}
	close(c)
	var x int
	n := 0
	for x = range c {
		n++
		if n == 3 {
			break
		}
	}
	fmt.Println(x, n)
	var h holder
	for h.last = range c {
	}
	fmt.Println(h)

	d := make(chan int, 3)
	d <- 7
	d <- 8
	d <- 9
	close(d)
	arr := []int{0, 0, 0}
	i := 0
	for arr[i] = range d {
		i++
		if i == len(arr) {
			break
		}
	}
	fmt.Println(arr, i)

	e := make(chan int, 2)
	e <- 5
	e <- 6
	close(e)
	var y int
	p := &y
	for *p = range e {
	}
	fmt.Println(y)

	// a different element type: assigned to an interface variable
	f := make(chan int, 2)
	f <- 1
	f <- 2
	close(f)
	var a any
	for a = range f {
	}
	fmt.Printf("%v %T\n", a, a)

	// m[k] target
	g := make(chan string, 2)
	g <- "one"
	g <- "two"
	close(g)
	m := map[int]string{}
	k := 0
	for m[k] = range g {
		k++
	}
	fmt.Println(m)
}

// Output:
// 3 3
// {6}
// [7 8 9] 3
// 6
// 2 int
// map[0:one 1:two]
