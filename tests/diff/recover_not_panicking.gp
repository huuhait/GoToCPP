// phase: 05

package main

import "fmt"

func helper() any { return recover() }

func f() {
	defer func() {
		fmt.Println("deferred, no panic:", recover())
	}()
	fmt.Println("body")
}

func main() {
	fmt.Println("direct:", recover())
	fmt.Println("helper:", helper())
	f()
	func() {
		fmt.Println("in a literal:", recover())
	}()
	fmt.Println("end")
}
