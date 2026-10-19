package main

import (
	"fmt"
	"strconv"
)

//go:noinline
func f!(s string) (int, error) { // after brace
	/* block */ x := strconv.Atoi(s) // trailing x
	text := `multi
  line`
	// before the loop
	for i := 0; i < strconv.Atoi!("3"); i++ { // loop trailing
		fmt.Println(i, text) // inside
	}

	// before return

	return x, nil /* end */
}

type T struct{}

// Method is a must method.
func (t *T) Method!(s string) int {
	return strconv.Atoi(s)
}

var table = map[string]int{
	"a": strconv.Atoi!("1"),
	"b": 2,
}

func main() {}
