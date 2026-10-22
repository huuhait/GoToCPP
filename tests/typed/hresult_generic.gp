// phase: 12
// opaque: reject
// windows: yes

// A package with a generic function is lowered without the typed view, and the calls that return an
// HRESULT are errors in it all the same: check wrapped them, and the lowering writes the error.
package main

import (
	"fmt"

	"./hresult.hpp"
)

func fail[T any](v T) error {
	e := hresult.status_of(4)
	fmt.Println(v, e.Error()[:7])
	return e
}

func main() {
	fmt.Println(fail(3) != nil, fail("x") != nil)
	e := hresult.status_of(2)
	fmt.Println(e != nil, hresult.status_of(0) == nil)
}

// Output:
// 3 HRESULT
// x HRESULT
// true true
// true true
