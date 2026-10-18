// phase: 05

// An error type that cannot be compared (it holds a slice) is fine for errors.Is: it never panics
// unless the program compares two values of that type with ==.

package main

import (
	"errors"
	"fmt"
)

type listErr struct{ parts []string }

func (e listErr) Error() string { return fmt.Sprint(len(e.parts), " parts") }

type listIs struct{ parts []string }

func (e listIs) Error() string { return "listIs" }
func (e listIs) Is(target error) bool {
	t, ok := target.(listIs)
	return ok && len(t.parts) == len(e.parts)
}

type wrapErr struct{ err error }

func (w wrapErr) Error() string { return "wrap: " + w.err.Error() }
func (w wrapErr) Unwrap() error { return w.err }

var Sentinel = errors.New("sentinel")

func main() {
	a := listErr{[]string{"a"}}
	b := listErr{[]string{"a"}}
	fmt.Println(errors.Is(a, Sentinel))
	fmt.Println(errors.Is(Sentinel, a))
	fmt.Println(errors.Is(a, b))
	fmt.Println(errors.Is(a, a))
	x, y := listIs{[]string{"p", "q"}}, listIs{[]string{"r", "s"}}
	fmt.Println(errors.Is(x, y), errors.Is(x, listIs{nil}))
	w := wrapErr{a}
	fmt.Println(errors.Is(w, Sentinel), errors.Is(w, a), errors.Unwrap(w) != nil)
}

// Output:
// false
// false
// false
// false
// true false
// false false true
