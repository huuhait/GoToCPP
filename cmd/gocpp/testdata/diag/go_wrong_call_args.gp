//test: cmd=check
package main

import "fmt"

func main() {
	fmt.Println(strlen("abc"))
	var f func(int) string
	f("x")
}
