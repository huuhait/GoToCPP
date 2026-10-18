// phase: 05

package main

import "fmt"

type NotFound struct{ Name string }

func (e *NotFound) Error() string { return "not found: " + e.Name }

// try returns the error that a panic carried, as the idiom `err = r.(error)`.
func try(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = &textErr{fmt.Sprint("panic: ", r)}
			}
		}
	}()
	f()
	return nil
}

type textErr struct{ text string }

func (e *textErr) Error() string { return e.text }

func lookup(name string) {
	panic(&NotFound{name})
}

func main() {
	e1 := try(func() { lookup("a") })
	fmt.Println(e1)
	var nf *NotFound
	if p, ok := e1.(*NotFound); ok {
		nf = p
	}
	fmt.Println(nf != nil, nf.Name)
	fmt.Println(try(func() { var s []int; _ = s[3] }))
	fmt.Println(try(func() { panic("plain string") }))
	fmt.Println(try(func() { panic(7) }))
	fmt.Println(try(func() {}))
	fmt.Println(try(func() { panic(&textErr{"wrapped: " + e1.Error()}) }))
}

// Output:
// not found: a
// true a
// runtime error: index out of range [3] with length 0
// panic: plain string
// panic: 7
// <nil>
// wrapped: not found: a
