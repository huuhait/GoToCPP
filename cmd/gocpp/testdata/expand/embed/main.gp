package main

import (
	_ "embed"
	"fmt"
)

//go:embed small.txt
var small string

func main() {
	// #embed is a constant: a long one is shortened.
	mine := "abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij" // the user's own long literal stays
	fmt.Println(mine, small, #embed("small.txt"), len(#embed("big.txt")), #file(), #line())
}
