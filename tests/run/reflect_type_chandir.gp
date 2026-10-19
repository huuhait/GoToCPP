// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	for _, v := range []any{make(chan int), make(<-chan int), make(chan<- int)} {
		t := reflect.TypeOf(v)
		d := t.ChanDir()
		fmt.Println(t, d, int(d), d&reflect.RecvDir != 0, d&reflect.SendDir != 0)
	}
}

// Output:
// chan int chan 3 true true
// <-chan int <-chan 1 true false
// chan<- int chan<- 2 false true
