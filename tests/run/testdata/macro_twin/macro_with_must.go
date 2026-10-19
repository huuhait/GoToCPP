// The twin of macro_with_must.gp: the must function and the must call are if err != nil, the built-in
// macros are the constants that the compiler puts in their place.

package main

import (
	"errors"
	"fmt"
)

func parse(s string) (int, error) {
	if s == "" {
		return 0, errors.New(fmt.Sprint("empty at line ", 15, " in ", "main.parse"))
	}
	return len(s), nil
}

func use(s string) (int, error) {
	n, err := parse(s)
	if err != nil {
		return 0, err
	}
	fmt.Println("parse!(s)", n, "main.use")
	return n + 23, nil
}

func main() {
	n, err := use("abc")
	fmt.Println(n, err)
	_, err = use("")
	fmt.Println(err)
}
