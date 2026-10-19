// phase: 06

package main

import "fmt"

func down(n int) int {
	return down(n+1) + 1
}

func main() {
	fmt.Println("start")
	fmt.Println(down(0))
}
