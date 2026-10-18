// phase: 08

// Printing a nil channel, and comparing channels printed as values.
package main

import "fmt"

func main() {
	var c chan int
	fmt.Println(c)
	fmt.Printf("%v %t\n", c, c == nil)
	d := make(chan string)
	fmt.Println(d != nil, fmt.Sprint(c) == "<nil>", fmt.Sprint(d) == fmt.Sprint(d), fmt.Sprint(d) == "<nil>")
	var arr [2]chan int
	fmt.Println(arr)
	type T struct {
		C chan int
		N int
	}
	fmt.Println(T{nil, 3})
	fmt.Printf("%+v\n", T{})
}

// Output:
// <nil>
// <nil> true
// true true true false
// [<nil> <nil>]
// {<nil> 3}
// {C:<nil> N:0}
