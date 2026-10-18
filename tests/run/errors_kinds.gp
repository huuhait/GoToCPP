// phase: 05

// Error types of every kind: ints, strings, arrays, slices, maps, funcs. errors.Is never panics for the ones Go cannot compare; errors.As stores each kind.

package main

import (
	"errors"
	"fmt"
)

type intErr int
type strErr string
type arrErr [2]int
type sliceErr []string
type mapErr map[string]int
type funcErr func() string
type ptrErr *int // cannot have methods

func (e intErr) Error() string   { return fmt.Sprint("int ", int(e)) }
func (e strErr) Error() string   { return "str " + string(e) }
func (e arrErr) Error() string   { return fmt.Sprint("arr ", e[0], e[1]) }
func (e sliceErr) Error() string { return fmt.Sprint("slice ", len(e)) }
func (e mapErr) Error() string   { return fmt.Sprint("map ", len(e)) }
func (e funcErr) Error() string  { return "func " + e() }

type wrap struct{ e error }

func (w wrap) Error() string { return "wrap" }
func (w wrap) Unwrap() error { return w.e }

var Sentinel = errors.New("sentinel")

func main() {
	all := []error{intErr(1), strErr("s"), arrErr{3, 4}, sliceErr{"a", "b"}, mapErr{"k": 1}, funcErr(func() string { return "f" })}
	for _, e := range all {
		fmt.Println(e.Error(), errors.Is(e, Sentinel), errors.Is(Sentinel, e), errors.Is(wrap{e}, Sentinel))
	}
	fmt.Println(errors.Is(intErr(1), intErr(1)), errors.Is(arrErr{3, 4}, arrErr{3, 4}), errors.Is(strErr("s"), strErr("t")))
	fmt.Println(errors.Is(sliceErr{"a"}, sliceErr{"a"}), errors.Is(mapErr{}, mapErr{}), errors.Is(all[5], all[5]))

	var i intErr
	var s strErr
	var a arrErr
	var sl sliceErr
	var m mapErr
	var f funcErr
	fmt.Println(errors.As(wrap{all[0]}, &i), i)
	fmt.Println(errors.As(wrap{all[1]}, &s), s)
	fmt.Println(errors.As(wrap{all[2]}, &a), a)
	fmt.Println(errors.As(wrap{all[3]}, &sl), sl)
	fmt.Println(errors.As(wrap{all[4]}, &m), m)
	fmt.Println(errors.As(wrap{all[5]}, &f), f())
	fmt.Println(errors.As(wrap{all[0]}, &s), errors.As(wrap{all[1]}, &i))
}

// Output:
// int 1 false false false
// str s false false false
// arr 3 4 false false false
// slice 2 false false false
// map 1 false false false
// func f false false false
// true true false
// false false false
// true int 1
// true str s
// true arr 3 4
// true slice 2
// true map 1
// true f
// false false
