package main

import "fmt"

func main() {
	defer fmt.Println("deferred calls run while a panic unwinds")
	var m map[string]int
	fmt.Println(m["a"])
	panic("boom")
}
