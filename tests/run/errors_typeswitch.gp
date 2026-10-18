// phase: 05

// Type switches on errors: the types of the errors package are distinguishable from user types.

package main

import (
	"errors"
	"fmt"
)

type NotFound struct{ name string }

func (e *NotFound) Error() string { return e.name + " not found" }

type Timeout struct{}

func (Timeout) Error() string { return "timeout" }

type wrap struct{ e error }

func (w wrap) Error() string { return "wrap" }
func (w wrap) Unwrap() error { return w.e }

func classify(err error) string {
	switch e := err.(type) {
	case nil:
		return "nil"
	case *NotFound:
		return "notfound:" + e.name
	case Timeout:
		return "timeout"
	case interface{ Unwrap() []error }:
		return fmt.Sprint("multi:", len(e.Unwrap()))
	case interface{ Unwrap() error }:
		return "wrapper of " + classify(e.Unwrap())
	default:
		return "other:" + e.Error()
	}
}

func main() {
	for _, err := range []error{
		nil,
		&NotFound{"a"},
		Timeout{},
		errors.New("plain"),
		errors.Join(Timeout{}, nil, &NotFound{"b"}),
		wrap{wrap{Timeout{}}},
		wrap{errors.Join(errors.New("x"), errors.New("y"), errors.New("z"))},
		errors.ErrUnsupported,
	} {
		fmt.Println(classify(err))
	}
}

// Output:
// nil
// notfound:a
// timeout
// other:plain
// multi:2
// wrapper of wrapper of timeout
// wrapper of multi:3
// other:unsupported operation
