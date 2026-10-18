// phase: 05

package main

import "fmt"

func run() {
	for i := 0; i < 4; i++ {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("iteration", i, "recovered", r)
			} else {
				fmt.Println("iteration", i, "nothing")
			}
		}()
	}
	panic("in run")
}

func main() {
	run()
	fmt.Println("after run")
}
