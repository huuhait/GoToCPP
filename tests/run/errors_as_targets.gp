// phase: 05

// The target of errors.As can be any addressable variable: a field, an element, a package variable, a named pointer type.

package main

import (
	"errors"
	"fmt"
)

type E struct{ id int }

func (e *E) Error() string { return fmt.Sprint("E", e.id) }

type PE *E

type holder struct {
	last  *E
	other error
}

var global *E

func main() {
	e := &E{5}
	var h holder
	fmt.Println(errors.As(e, &h.last), h.last == e, h.last.id)
	arr := make([]*E, 3)
	fmt.Println(errors.As(e, &arr[1]), arr[0] == nil, arr[1] == e)
	fmt.Println(errors.As(e, &global), global == e)
	fmt.Println(errors.As(e, &h.other), h.other == error(e))
	hp := &holder{}
	fmt.Println(errors.As(e, &hp.last), hp.last == e)
	m := map[string]*holder{"k": {}}
	fmt.Println(errors.As(e, &m["k"].last), m["k"].last == e)
	var p *E
	pp := &p
	fmt.Println(errors.As(e, pp), p == e)
	var arr2 [2]error
	fmt.Println(errors.As(e, &arr2[1]), arr2[0] == nil, arr2[1] == error(e))
	f := func() {
		var local *E
		fmt.Println(errors.As(e, &local), local == e)
		g := func() { local = nil }
		g()
		fmt.Println(local == nil)
	}
	f()
}

// Output:
// true true 5
// true true true
// true true
// true true
// true true
// true true
// true true
// true true true
// true true
// true
