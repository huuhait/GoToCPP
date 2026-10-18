// phase: 08

// A select in a loop that terminates by closing channels; a goroutine blocked in select{} for ever
// does not keep the program from ending.
package main

import "fmt"

func main() {
	go func() {
		select {}
	}()
	a := make(chan int)
	b := make(chan int)
	go func() {
		close(a)
	}()
	go func() {
		close(b)
	}()
	closed := 0
	for closed < 2 {
		select {
		case _, ok := <-a:
			if !ok {
				a = nil
				closed++
			}
		case _, ok := <-b:
			if !ok {
				b = nil
				closed++
			}
		}
	}
	fmt.Println("both closed", closed)
}

// Output:
// both closed 2
