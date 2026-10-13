package main

import (
	"fmt"

	"./mathx.h" // a header next to this file: package mathx
)

func main() {
	fmt.Println(mathx.hypot3(1, 2, 2))
}
