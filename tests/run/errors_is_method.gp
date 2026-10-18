// phase: 05

// An error's Is(error) bool method decides a match, on a value type and on a pointer type.

package main

import (
	"errors"
	"fmt"
)

var ErrTimeout = errors.New("timeout")
var ErrTemp = errors.New("temporary")

type netErr struct{ code int }

func (e netErr) Error() string { return fmt.Sprintf("net error %d", e.code) }
func (e netErr) Is(target error) bool {
	if target == ErrTimeout {
		return e.code == 408
	}
	if target == ErrTemp {
		return e.code >= 500
	}
	t, ok := target.(netErr)
	return ok && t.code == e.code
}

type ptrErr struct{ name string }

func (e *ptrErr) Error() string { return e.name }
func (e *ptrErr) Is(target error) bool {
	t, ok := target.(*ptrErr)
	return ok && t.name == e.name
}

func main() {
	fmt.Println(errors.Is(netErr{408}, ErrTimeout), errors.Is(netErr{500}, ErrTimeout))
	fmt.Println(errors.Is(netErr{503}, ErrTemp), errors.Is(netErr{404}, ErrTemp))
	fmt.Println(errors.Is(netErr{7}, netErr{7}), errors.Is(netErr{7}, netErr{8}))
	a, b := &ptrErr{"x"}, &ptrErr{"x"}
	fmt.Println(a == b, errors.Is(a, b), errors.Is(b, a), errors.Is(a, &ptrErr{"y"}))
	fmt.Println(errors.Is(a, ErrTemp))
}

// Output:
// true false
// true false
// true false
// false true true false
// false
