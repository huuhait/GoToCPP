// phase: 05

// Joins inside joins, joins inside wrappers: Is and the text of the tree.

package main

import (
	"errors"
	"fmt"
)

type wrap struct{ e error }

func (w wrap) Error() string { return "wrap[" + w.e.Error() + "]" }
func (w wrap) Unwrap() error { return w.e }

func count(s string, c byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			n++
		}
	}
	return n
}

func main() {
	a, b, c, d := errors.New("a"), errors.New("b"), errors.New("c"), errors.New("d")
	inner := errors.Join(a, b)
	outer := errors.Join(inner, wrap{errors.Join(c, nil)}, nil)
	fmt.Printf("%q\n", outer.Error())
	fmt.Println(count(outer.Error(), '\n'))
	var found []bool
	for _, e := range []error{a, b, c, d, inner, outer} {
		found = append(found, errors.Is(outer, e))
	}
	fmt.Println(found)
	fmt.Println(errors.Is(inner, c), errors.Is(wrap{inner}, b))
	big := make([]error, 100)
	for i := range big {
		if i%2 == 0 {
			big[i] = errors.New("e")
		}
	}
	j := errors.Join(big...)
	fmt.Println(count(j.Error(), '\n'), len(j.(interface{ Unwrap() []error }).Unwrap()))
}

// Output:
// "a\nb\nwrap[c]"
// 2
// [true true true false true true]
// false true
// 49 50
