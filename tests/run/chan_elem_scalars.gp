// phase: 08

// Channels of the basic types, of named types and of arrays and structs: values are copied.
package main

import "fmt"

type Celsius float64
type ID int
type Point struct{ X, Y int }
type Rec struct {
	Name string
	Tags []string
	P    Point
	Arr  [3]int
}

func main() {
	ci := make(chan int8, 1)
	ci <- -5
	fmt.Println(<-ci)
	cu := make(chan uint8, 1)
	cu <- 250
	fmt.Println(<-cu + 10) // wraps
	cf := make(chan float64, 1)
	cf <- 2.5
	fmt.Println(<-cf * 2)
	cc := make(chan complex128, 1)
	cc <- complex(1, 2)
	fmt.Println(<-cc)
	cb := make(chan bool, 1)
	cb <- true
	fmt.Println(!<-cb)
	cs := make(chan string, 1)
	cs <- "héllo"
	fmt.Println(<-cs)
	cr := make(chan rune, 1)
	cr <- 'é'
	fmt.Println(<-cr)
	cn := make(chan Celsius, 1)
	cn <- 36.6
	fmt.Println(<-cn + 0.4)
	cid := make(chan ID, 2)
	cid <- 7
	cid <- ID(8)
	fmt.Println(<-cid * 2)

	ca := make(chan [3]int, 1)
	arr := [3]int{1, 2, 3}
	ca <- arr
	arr[0] = 100
	got := <-ca
	fmt.Println(got, arr)

	cp := make(chan Point, 1)
	p := Point{1, 2}
	cp <- p
	p.X = 50
	fmt.Println(<-cp, p)

	cr2 := make(chan Rec, 1)
	r := Rec{"r", []string{"x", "y"}, Point{3, 4}, [3]int{7, 8, 9}}
	cr2 <- r
	r.Name = "changed"
	r.Tags[0] = "shared"
	r.P.X = -1
	r.Arr[2] = -9
	g := <-cr2
	fmt.Println(g.Name, g.Tags, g.P, g.Arr) // the slice is shared, the rest was copied
}

// Output:
// -5
// 4
// 5
// (1+2i)
// false
// héllo
// 233
// 37
// 14
// [1 2 3] [100 2 3]
// {1 2} {50 2}
// r [shared y] {3 4} [7 8 9]
