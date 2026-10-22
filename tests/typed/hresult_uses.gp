// phase: 12
// opaque: reject
// windows: yes

// The variable made from the call of a function that returns an HRESULT has the type error where it
// is declared, so every use of it is Go's: the text of the error is a string (concatenation, the
// functions of package strings, a variable of type string, a slice expression, a range), the error
// goes into append and into the elements of a slice or a map literal, and it can be set to nil.
package main

import (
	"fmt"

	"./hresult.hpp"
)

type box struct{ items []error }

// upper and contains stand for the functions of package strings, which a program passes the text to.
func upper(s string) string {
	out := ""
	for _, r := range s {
		if 'a' <= r && r <= 'z' {
			r -= 'a' - 'A'
		}
		out += string(r)
	}
	return out
}

func contains(s, part string) bool {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return true
		}
	}
	return false
}

// describe takes errors the way a program that collects them does.
func describe(errs ...error) int {
	n := 0
	for _, e := range errs {
		if e != nil {
			n++
		}
	}
	return n
}

// present returns the call itself as the error of the function.
func present() error {
	return hresult.status_of(3)
}

func main() {
	e := hresult.status_of(2)

	// The text is a string.
	fmt.Println("failed: " + e.Error())
	fmt.Println(e.Error() + "!")
	msg := e.Error()
	fmt.Println(upper(msg), contains(msg, "E_FAIL"), len(msg))
	var t string = msg
	fmt.Println(t == msg, t[:4], e.Error()[:4], string(e.Error()[5]))
	runes := 0
	for _, r := range e.Error() {
		if r == 'E' {
			runes++
		}
	}
	fmt.Println(runes)

	// The error goes where an error goes.
	var xs []error
	xs = append(xs, e)
	xs = append(xs, hresult.status_of(0))
	b := box{}
	b.items = append(b.items, hresult.status_of(4))
	b.items = append(b.items, e, nil)
	fmt.Println(len(xs), xs[1] == nil, len(b.items), b.items[2] == nil)
	lit := []error{e, hresult.status_of(0), hresult.status_of(4)}
	fmt.Println(describe(lit...), describe(e, hresult.status_of(0)))

	// An error variable can be set to nil, and to another error.
	f := hresult.status_of(4)
	f = nil
	fmt.Println(f == nil, f != e)
	f = hresult.status_of(4)
	fmt.Println(f == nil, f.Error())

	// The error is a map key, in a literal as well.
	m := map[error]string{e: "dev"}
	n := map[error]int{hresult.status_of(4): 1, hresult.status_of(0): 2}
	fmt.Println(len(m), m[e], len(n), n[f], n[nil])

	// The call is the error of the function that returns it, and of a statement with an initializer.
	fmt.Println(present())
	if err := hresult.status_of(4); err == nil {
		fmt.Println("not reached")
	} else {
		fmt.Println("removed", err.Error()[:7] == "HRESULT")
	}
}

// Output:
// failed: HRESULT 0x80004005 (E_FAIL)
// HRESULT 0x80004005 (E_FAIL)!
// HRESULT 0X80004005 (E_FAIL) true 27
// true HRES HRES L
// 2
// 2 true 3 true
// 2 1
// true true
// false HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)
// 1 dev 2 1 2
// <nil>
// removed true
