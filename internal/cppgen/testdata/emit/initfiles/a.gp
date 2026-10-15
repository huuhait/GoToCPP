// A package of three files, each with package variables and init functions: the variables
// are initialized in Go's order across the files (a depends on c in c.gp, which depends on
// b in b.gp), then the init functions run in file order (a.gp, b.gp, c.gp, as the go
// command sorts them) and, inside a file, in source order.
package main

import "fmt"

var a = note("a", c+1)

func init() {
	fmt.Println("a.gp init 1:", a, b, c)
}

func main() {
	fmt.Println("main:", a, b, c, steps)
}

func init() {
	fmt.Println("a.gp init 2")
}
