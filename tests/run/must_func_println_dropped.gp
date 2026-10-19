// phase: 10

package main

import "fmt"

func hello!(name string) string {
	fmt.Println("hello", name)
	fmt.Printf("%d args\n", 1)
	return "done " + name
}

func main() {
	fmt.Println(hello("go"))
}

// Output:
// hello go
// 1 args
// done go <nil>
