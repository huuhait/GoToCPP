// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	var a chan int
	var b <-chan int
	var c chan<- int
	var d chan chan int
	var e chan<- chan int
	var f chan (<-chan int)
	var g <-chan chan<- int
	var h chan *string
	for _, v := range []any{a, b, c, d, e, f, g, h} {
		t := reflect.TypeOf(v)
		fmt.Println(t, t.ChanDir(), t.Elem())
	}
}

// Output:
// chan int chan int
// <-chan int <-chan int
// chan<- int chan<- int
// chan chan int chan chan int
// chan<- chan int chan<- chan int
// chan (<-chan int) chan <-chan int
// <-chan chan<- int <-chan chan<- int
// chan *string chan *string
