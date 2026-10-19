package main

import "fmt"

func main() {
	var small uint8 = 250
	small += 10
	var n int = 7
	var m int64 = 7
	var f float32 = 1.5
	s := "text"
	fmt.Println(small, any(n) == any(m), f, len(s))
}
