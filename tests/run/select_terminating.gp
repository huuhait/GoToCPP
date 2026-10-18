// phase: 08

// A select without default is a terminating statement when every case body ends in one: a function
// can end with it, whatever the shape of the bodies (if with else, for {}, switch with default, a
// block, panic, a nested select, a labeled statement). The harness fails a run that prints a C++
// compiler warning, so -Wreturn-type is checked too.
package main

import "fmt"

func ifElse(c chan int) int {
	select {
	case v := <-c:
		if v > 0 {
			return 1
		} else {
			return 2
		}
	}
}

func forever(c chan int, stop chan bool) int {
	select {
	case <-c:
		for {
			select {
			case <-stop:
				return 7
			}
		}
	}
}

func mixed(c, full chan int) int {
	select {
	case v := <-c:
		switch {
		case v > 0:
			return 1
		default:
			return 0
		}
	case full <- 1:
		{
			panic("x")
		}
	}
}

func nested(a, b chan int) int {
	select {
	case v := <-a:
		select {
		case w := <-b:
			return v + w
		case <-a:
			goto done
		}
	done:
		return -1
	}
}

func labeled(c chan int) (r int) {
	select {
	case v := <-c:
	L:
		for {
			if v > 5 {
				return 5
			}
			v++
			continue L
		}
	}
}

func typeSwitch(c chan any) string {
	select {
	case v := <-c:
		switch v.(type) {
		case int:
			return "int"
		default:
			return "other"
		}
	}
}

func main() {
	c := make(chan int, 2)
	c <- 5
	fmt.Println(ifElse(c))
	c <- -5
	fmt.Println(ifElse(c))
	stop := make(chan bool, 1)
	c <- 1
	stop <- true
	fmt.Println(forever(c, stop))
	c <- 3
	full := make(chan int)
	fmt.Println(mixed(c, full))
	a, b := make(chan int, 2), make(chan int, 2)
	a <- 1
	b <- 2
	r := nested(a, b)
	fmt.Println(r)
	c <- 2
	fmt.Println(labeled(c))
	d := make(chan any, 2)
	d <- 1
	fmt.Println(typeSwitch(d))
	d <- "s"
	fmt.Println(typeSwitch(d))
}


// Output:
// 1
// 2
// 7
// 1
// 3
// 5
// int
// other
