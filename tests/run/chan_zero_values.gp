// phase: 08

// Receiving from a closed channel gives the zero value of the element type.
package main

import "fmt"

type S struct {
	A int
	B string
	C []int
	D *S
}

type Shape interface{ Area() float64 }

func main() {
	ci, cf, cb := make(chan int), make(chan float64), make(chan bool)
	close(ci)
	close(cf)
	close(cb)
	fmt.Println(<-ci, <-cf, <-cb)
	cs, cr := make(chan string), make(chan rune)
	close(cs)
	close(cr)
	fmt.Printf("%q %v\n", <-cs, <-cr)
	css := make(chan S)
	close(css)
	s := <-css
	fmt.Println(s.A, s.B == "", s.C == nil, s.D == nil)
	ca, cas := make(chan [3]int), make(chan [2]string)
	close(ca)
	close(cas)
	fmt.Println(<-ca, <-cas == [2]string{})
	cp, csh, ce := make(chan *S), make(chan Shape), make(chan error)
	close(cp)
	close(csh)
	close(ce)
	fmt.Println(<-cp == nil, <-csh == nil, <-ce == nil)
	cm, cfn, ccc := make(chan map[string]int), make(chan func()), make(chan chan int)
	close(cm)
	close(cfn)
	close(ccc)
	fmt.Println(<-cm == nil, <-cfn == nil, <-ccc == nil)
	cany := make(chan any)
	close(cany)
	fmt.Println(<-cany == nil)
}

// Output:
// 0 0 false
// "" 0
// 0 true true true
// [0 0 0] true
// true true true
// true true true
// true
