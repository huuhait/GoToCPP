// phase: 08

// A value written before a send is read after the receive.
package main

import "fmt"

var data []int

func main() {
	ch := make(chan bool)
	go func() {
		data = append(data, 1, 2, 3)
		ch <- true
	}()
	<-ch
	fmt.Println(data)
}

// Output:
// [1 2 3]
