// phase: 05

// Errors with comparable value types: == compares the values; structs and arrays of errors compare field by field; map keys.

package main

import (
	"errors"
	"fmt"
)

type code int

func (c code) Error() string { return fmt.Sprint("code ", int(c)) }

type pair struct {
	a, b error
}

var E1 = errors.New("e1")

func main() {
	var x, y error = code(1), code(1)
	var z error = code(2)
	fmt.Println(x == y, x == z, x != z, x == error(nil))
	fmt.Println(errors.Is(x, y), errors.Is(x, z))
	p1, p2 := pair{code(1), E1}, pair{code(1), E1}
	fmt.Println(p1 == p2, p1 == pair{code(2), E1}, p1 == pair{code(1), errors.New("e1")})
	arr1 := [2]error{x, E1}
	arr2 := [2]error{y, E1}
	fmt.Println(arr1 == arr2, arr1 == [2]error{E1, x})
	m := map[error]string{code(1): "one", E1: "e1"}
	fmt.Println(m[x], m[E1], m[code(2)] == "", len(m))
	m[y] = "uno"
	fmt.Println(len(m), m[code(1)])
	var n error
	fmt.Println(m[n] == "", n == nil)
}

// Output:
// true false true false
// true false
// true false false
// true false
// one e1 true 2
// 2 uno
// true true
