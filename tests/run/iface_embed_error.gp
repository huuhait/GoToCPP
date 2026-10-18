// phase: 05

// The idiom of wrapping an error: a struct that embeds the predeclared interface error gets
// Error() promoted, so it is an error itself, and can override the message; embedding any adds
// no methods.
package main

import "fmt"

type base struct{ msg string }

func (b *base) Error() string { return b.msg }

type wrapped struct {
	error
	code int
}

type loud struct {
	wrapped
}

func (l loud) Error() string { return "LOUD " + l.wrapped.Error() }

type holder struct {
	any
	n int
}

func fail(code int) error {
	if code == 0 {
		return nil
	}
	return wrapped{error: &base{"failed"}, code: code}
}

func main() {
	e := fail(3)
	fmt.Println(e.Error(), e != nil, fail(0) == nil)
	var err error = loud{wrapped{error: &base{"inner"}, code: 1}}
	fmt.Println(err.Error())
	w := e.(wrapped)
	fmt.Println(w.code, w.error.Error())
	h := holder{any: 42, n: 1}
	fmt.Println(h.any.(int), h.n, h.any.(int)+1)
	var a any = h
	_, isErr := a.(error)
	fmt.Println(isErr)
	var a2 any = wrapped{error: &base{"x"}}
	_, isErr = a2.(error)
	fmt.Println(isErr)
}

// Output:
// failed true true
// LOUD inner
// 3 failed
// 42 1 43
// false
// true
