// phase: 05

// A long chain of wrappers: errors.Is and errors.As loop (no recursion), and freeing the chain does not overflow the stack.

package main

import (
	"errors"
	"fmt"
)

type link struct {
	next error
	n    int
}

func (l *link) Error() string { return "link" }
func (l *link) Unwrap() error { return l.next }

type tail struct{}

func (tail) Error() string { return "tail" }

var Root = errors.New("root")

func build(n int) error {
	var err error = Root
	for i := 0; i < n; i++ {
		err = &link{err, i}
	}
	return err
}

func main() {
	err := build(200000)
	fmt.Println(errors.Is(err, Root), errors.Is(err, errors.New("root")))
	var l *link
	fmt.Println(errors.As(err, &l), l.n)
	count := 0
	for e := err; e != nil; e = errors.Unwrap(e) {
		count++
	}
	fmt.Println(count)
	var t tail
	fmt.Println(errors.As(err, &t))
	err = nil
	l = nil
	fmt.Println(build(10).Error())
}

// Output:
// true false
// true 199999
// 200001
// false
// link
