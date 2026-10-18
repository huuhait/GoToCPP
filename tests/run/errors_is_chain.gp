// phase: 05

// errors.Is follows Unwrap() error through a chain of custom wrappers of two kinds.

package main

import (
	"errors"
	"fmt"
)

var Root = errors.New("root")

type ptrWrap struct{ inner error }

func (w *ptrWrap) Error() string { return "ptr(" + w.inner.Error() + ")" }
func (w *ptrWrap) Unwrap() error { return w.inner }

type valWrap struct{ inner error }

func (w valWrap) Error() string { return "val(" + w.inner.Error() + ")" }
func (w valWrap) Unwrap() error { return w.inner }

type endWrap struct{}

func (endWrap) Error() string { return "end" }
func (endWrap) Unwrap() error { return nil }

func main() {
	var err error = Root
	for i := 0; i < 6; i++ {
		if i%2 == 0 {
			err = &ptrWrap{err}
		} else {
			err = valWrap{err}
		}
	}
	fmt.Println(err.Error())
	fmt.Println(errors.Is(err, Root), errors.Is(err, errors.New("root")))
	n := 0
	for e := err; e != nil; e = errors.Unwrap(e) {
		n++
	}
	fmt.Println(n)
	fmt.Println(errors.Is(endWrap{}, Root), errors.Is(valWrap{endWrap{}}, endWrap{}))
}

// Output:
// val(ptr(val(ptr(val(ptr(root))))))
// true false
// 7
// false true
