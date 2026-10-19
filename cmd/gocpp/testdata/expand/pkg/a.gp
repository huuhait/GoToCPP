package main

import (
	"fmt"

	"example.com/app/util"
)

func hello(n int) {
	#util.Check(n > 0, "n must be positive")
	fmt.Println("hello", n)
}
