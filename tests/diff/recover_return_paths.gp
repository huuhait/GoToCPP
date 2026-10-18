// phase: 05

package main

import "fmt"

var log []string

func note(s string) { log = append(log, s) }

// Every return statement sets the results and goes through the deferred calls.
func paths(n int) (res int, tag string) {
	defer func() {
		note(fmt.Sprint("defer sees ", res, " ", tag))
		if r := recover(); r != nil {
			res, tag = -res, "recovered"
		}
	}()
	switch {
	case n < 0:
		return -1, "negative"
	case n == 0:
		panic("zero")
	case n == 1:
		res = 100
		return
	case n == 2:
		for i := 0; ; i++ {
			if i == 3 {
				return i * 10, "loop"
			}
		}
	}
	if n > 100 {
		res = n
		panic("big")
	}
	return n, "plain"
}

func main() {
	for _, n := range []int{-5, 0, 1, 2, 7, 500} {
		r, t := paths(n)
		fmt.Println(n, r, t)
	}
	for _, l := range log {
		fmt.Println(l)
	}
}
