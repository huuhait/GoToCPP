// phase: 08

// Values through select: structs, arrays, slices, strings, interfaces, pointers, funcs, nested
// channels. Received values are copies; a sent struct is copied when the select starts.
package main

import (
	"errors"
	"fmt"
)

type point struct{ x, y int }

type shape interface{ area() int }

type rect struct{ w, h int }

func (r rect) area() int { return r.w * r.h }

func main() {
	ps := make(chan point, 1)
	as := make(chan [3]int, 1)
	ss := make(chan []string, 1)
	is := make(chan shape, 1)
	es := make(chan error, 1)
	fs := make(chan func(int) int, 1)
	cs := make(chan chan int, 1)
	pp := make(chan *point, 1)
	anys := make(chan any, 1)

	p := point{1, 2}
	select {
	case ps <- p:
	}
	p.x = 100 // the channel has its own copy
	select {
	case q := <-ps:
		fmt.Println(q, p)
	}

	arr := [3]int{1, 2, 3}
	select {
	case as <- arr:
	}
	arr[0] = 9
	select {
	case b := <-as:
		fmt.Println(b, arr)
	}

	words := []string{"a", "b"}
	select {
	case ss <- words:
	}
	words[0] = "z" // slices share their array
	select {
	case w := <-ss:
		fmt.Println(w)
	}

	select {
	case is <- rect{3, 4}:
	}
	select {
	case s := <-is:
		fmt.Println(s.area())
	}

	select {
	case es <- errors.New("boom"):
	}
	select {
	case err := <-es:
		fmt.Println(err)
	}

	select {
	case fs <- func(i int) int { return i * 3 }:
	}
	select {
	case f := <-fs:
		fmt.Println(f(14))
	}

	inner := make(chan int, 1)
	inner <- 8
	select {
	case cs <- inner:
	}
	select {
	case c := <-cs:
		fmt.Println(<-c)
	}

	pt := &point{5, 6}
	select {
	case pp <- pt:
	}
	select {
	case q := <-pp:
		q.x++
		fmt.Println(*pt, q == pt)
	}

	for _, v := range []any{1, "two", 3.5, point{7, 8}, nil, []int{1}} {
		select {
		case anys <- v:
		}
		select {
		case got := <-anys:
			fmt.Printf("%v %T\n", got, got)
		}
	}
}

// Output:
// {1 2} {100 2}
// [1 2 3] [9 2 3]
// [z b]
// 12
// boom
// 42
// 8
// {6 6} true
// 1 int
// two string
// 3.5 float64
// {7 8} main.point
// <nil> <nil>
// [1] []int
