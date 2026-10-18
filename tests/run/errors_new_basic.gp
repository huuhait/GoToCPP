// phase: 05

// errors.New returns a distinct error each time, even for the same text; the same value is equal to itself.

package main

import (
	"errors"
	"fmt"
)

func main() {
	a := errors.New("x")
	b := errors.New("x")
	c := a
	fmt.Println(a.Error(), b.Error())
	fmt.Println(a == b, a == c, a != b, c != a)
	var e error = a
	fmt.Println(e == a, e == b)
	fmt.Println(errors.New("") == errors.New(""), errors.New("").Error() == "")
	fmt.Println(len(errors.New("héllo, 世界").Error()))
}

// Output:
// x x
// false true true false
// true false
// false true
// 14
