// phase: 07

// fmt on instances: String and Error methods, %v %+v %#v %T %s %d, nested values and pointers.
package main

import (
	"errors"
	"fmt"
)

type Wrapper[T any] struct{ V T }

func (w Wrapper[T]) String() string { return fmt.Sprintf("W(%v)", w.V) }

type Plain[T any] struct {
	A T
	B []T
}

type ErrBox[T any] struct{ Code T }

func (e *ErrBox[T]) Error() string { return fmt.Sprint("error code ", e.Code) }

type Temp[T ~float64] struct{ v T }

func (t Temp[T]) Format(f fmt.State, verb rune) { fmt.Fprintf(f, "<%.1f%c>", float64(t.v), verb) }

func main() {
	w := Wrapper[int]{4}
	fmt.Println(w, &w, []Wrapper[string]{{"a"}, {"b"}}, map[string]Wrapper[bool]{"k": {true}})
	fmt.Printf("%v %s %d|%+v|%T\n", w, w, w, w, w)
	p := Plain[string]{"x", []string{"y", "z"}}
	fmt.Printf("%v|%+v|%#v|%T\n", p, p, p, p)
	fmt.Printf("%v|%+v\n", &p, &Plain[int]{})
	var err error = &ErrBox[int]{404}
	fmt.Println(err)
	fmt.Printf("%v %s %T\n", err, err, err)
	var target *ErrBox[int]
	fmt.Println(errors.As(err, &target), target.Code)
	fmt.Printf("%v %s\n", Temp[float64]{36.6}, Temp[float64]{1})
}

// Output:
// W(4) W(4) [W(a) W(b)] map[k:W(true)]
// W(4) W(4) {4}|W(4)|main.Wrapper[int]
// {x [y z]}|{A:x B:[y z]}|main.Plain[string]{A:"x", B:[]string{"y", "z"}}|main.Plain[string]
// &{x [y z]}|&{A:0 B:[]}
// error code 404
// error code 404 error code 404 *main.ErrBox[int]
// true 404
// <36.6v> <1.0s>
