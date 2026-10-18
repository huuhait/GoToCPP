// phase: 05

package main

import "fmt"

func handler(tag string, next string) {
	r := recover()
	fmt.Println(tag, "recovered", r)
	if next != "" {
		panic(next)
	}
}

func main() {
	defer func() { fmt.Println("final:", recover()) }()
	defer func() { handler("h3", "") }()
	defer func() { handler("h2", "p3") }()
	defer func() { handler("h1", "p2") }()
	panic("p1")
}
