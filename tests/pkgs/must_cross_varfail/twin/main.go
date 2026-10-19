package main

import (
	"fmt"

	"example.com/varfail/lib"
)

func main() {
	fmt.Println("main must not run", lib.Port)
}
