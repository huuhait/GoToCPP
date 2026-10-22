// phase: 12
// opaque: reject
// windows: yes

// The program may declare its own error in the scope of a call that returns an HRESULT: the call is
// still Go's predeclared error (check wraps it with the universe type, not with the name error).
package main

import (
	"fmt"

	"./hresult.hpp"
)

func asVariable() {
	error := "text"
	e := hresult.status_of(4)
	fmt.Println(error, e != nil, e.Error()[:7])
}

func asType() {
	type error int
	var n error = 7
	e := hresult.status_of(2)
	fmt.Println(n, e != nil, hresult.status_of(0) == nil)
}

func main() {
	asVariable()
	asType()
}

// Output:
// text true HRESULT
// 7 true true
