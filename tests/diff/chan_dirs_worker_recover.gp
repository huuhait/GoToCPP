// phase: 08

// A worker pool whose workers recover panics into errors, and the directions of channel types in type switches and %T.
package main

import (
	"errors"
	"fmt"
)

type result struct {
	id  int
	val int
	err error
}

func worker(id int, in <-chan int, out chan<- result, fin chan<- struct{}) {
	defer close(fin)
	for {
		n, ok := <-in
		if !ok {
			return
		}
		out <- compute(id, n)
	}
}

func compute(id, n int) (r result) {
	r.id = id
	defer func() {
		if x := recover(); x != nil {
			switch e := x.(type) {
			case error:
				r.err = fmt.Errorf("worker: %w", e)
			default:
				r.err = fmt.Errorf("worker: %v", e)
			}
		}
	}()
	switch {
	case n%7 == 0:
		panic(fmt.Sprintf("seven %d", n))
	case n%5 == 0:
		var a []int
		_ = a[n]
	case n%3 == 0:
		panic(errors.New("three"))
	}
	r.val = n * n
	return
}

func main() {
	in := make(chan int)
	out := make(chan result)
	fin := make(chan struct{})
	go func() {
		for i := 1; i <= 12; i++ {
			in <- i
		}
		close(in)
	}()
	go worker(1, in, out, fin)
	go func() {
		<-fin
		close(out)
	}()
	sum, errs := 0, 0
	byN := map[int]result{}
	for {
		r, ok := <-out
		if !ok {
			break
		}
		if r.err != nil {
			errs++
		} else {
			sum += r.val
		}
		byN[r.val] = r
	}
	fmt.Println(sum, errs)
	var dirs []any
	var c chan int
	var ro <-chan int = c
	var so chan<- int = c
	dirs = append(dirs, c, ro, so, in, out, (<-chan result)(out))
	for _, d := range dirs {
		switch d.(type) {
		case chan int:
			fmt.Print("chan int; ")
		case <-chan int:
			fmt.Print("<-chan int; ")
		case chan<- int:
			fmt.Print("chan<- int; ")
		case chan result:
			fmt.Print("chan result; ")
		case <-chan result:
			fmt.Print("<-chan result; ")
		}
		fmt.Printf("%T\n", d)
	}
	defer func() {
		e := recover().(error)
		fmt.Println(e)
	}()
	_ = dirs[0].(<-chan int)
}
