// phase: 05

// errors.As leaves the target alone when nothing matches; it finds a type deep in a chain.

package main

import (
	"errors"
	"fmt"
)

type A struct{ n int }

func (a *A) Error() string { return "A" }

type B struct{ n int }

func (b *B) Error() string { return "B" }

type link struct {
	err error
	tag string
}

func (l *link) Error() string { return l.tag }
func (l *link) Unwrap() error { return l.err }

func main() {
	deep := &B{9}
	var err error = deep
	for i := 0; i < 5; i++ {
		err = &link{err, fmt.Sprint("link", i)}
	}
	a := &A{1}
	keep := a
	fmt.Println(errors.As(err, &a), a == keep)
	var b *B
	fmt.Println(errors.As(err, &b), b == deep, b.n)
	var l *link
	fmt.Println(errors.As(err, &l), l.tag, l == err)
	var l2 *link
	fmt.Println(errors.As(errors.Unwrap(err), &l2), l2.tag)
}

// Output:
// false true
// true true 9
// true link4 true
// true link3
