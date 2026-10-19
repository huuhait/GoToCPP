// phase: 05

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

// chk prints its name and fails for "bad".
func chk(name string, v bool) (bool, error) {
	fmt.Println("chk", name, v)
	if name == "bad" {
		return false, fmt.Errorf("chk %s: %w", name, ErrBad)
	}
	return v, nil
}

func and(a, b string, x, y bool) (bool, error) {
	if chk!(a, x) && chk!(b, y) {
		return true, nil
	}
	return false, nil
}

func or(a, b string, x, y bool) (bool, error) {
	return chk!(a, x) || chk!(b, y), nil
}

func main() {
	fmt.Println(and("a", "b", true, true))
	fmt.Println(and("a", "bad", false, true))
	fmt.Println(and("a", "bad", true, true))
	fmt.Println(or("a", "bad", true, false))
	fmt.Println(or("a", "b", false, true))
	fmt.Println(or("bad", "b", false, true))
	fmt.Println(or("a", "bad", false, true))
}

// Output:
// chk a true
// chk b true
// true <nil>
// chk a false
// false <nil>
// chk a true
// chk bad true
// false chk bad: bad
// chk a true
// true <nil>
// chk a false
// chk b true
// true <nil>
// chk bad false
// false chk bad: bad
// chk a false
// chk bad true
// false chk bad: bad
