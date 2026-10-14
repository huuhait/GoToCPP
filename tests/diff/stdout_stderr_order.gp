// phase: 02
// diff: combined

// fmt writes to stdout and println/print to stderr; both are unbuffered in
// Go, so in one pipe the lines come in the order they were written, also
// within a line and in a loop.
package main

import "fmt"

func both(i int) {
	fmt.Print("out ", i, " ")
	print("err ", i, " ")
	fmt.Println("out-end")
	println("err-end", i)
}

func main() {
	fmt.Println("to stdout")
	println("to stderr")
	fmt.Println("to stdout again")
	for i := range 5 {
		both(i)
	}
	fmt.Printf("%d%%\n", 100)
	print("no newline on stderr")
	fmt.Print(" then stdout\n")
	println()
	fmt.Println()
	for i := range 20 {
		if i%3 == 0 {
			println(i)
		} else {
			fmt.Println(i)
		}
	}
}
