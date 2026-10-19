// phase: 05

// The code of a function under a wrap macro keeps Go's own rules for its results (step 10.9): a
// variable that hides a result, a bare return, return f() that forwards several values, a defer that
// changes the results. The twin (testdata/macro_twin/umacro_results.go) is plain Go: the macro's code
// written into each function by hand.

package main

import (
	"errors"
	"fmt"
)

// VM says when the function starts and when it ends.
macro VM() {
	fmt.Println("vm", #func())
	defer fmt.Println("vm end", #func())
	#body()
}

func g(fail bool) error {
	if fail {
		return errors.New("g failed")
	}
	return nil
}

func pair(a int) (int, string) { return a * 2, "pair" }

#VM()
func shadowErr(fail bool) (n int, err error) {
	if err := g(fail); err != nil {
		return 7, err
	}
	return 1, nil
}

#VM()
func shadowN() (n int) {
	{
		n := 10
		_ = n
		return 5
	}
}

#VM()
func shadowLoop() (n int) {
	n = 1
	for n := 0; n < 3; n++ {
		if n == 2 {
			return n + 100
		}
	}
	return
}

#VM()
func forward(a int) (int, string) {
	return pair(a)
}

#VM()
func bare(a int) (sum int, label string) {
	label = "bare"
	for i := range a {
		sum += i
	}
	return
}

#VM()
func deferred(a int) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
			n = -1
		}
	}()
	defer func() { n *= 2 }()
	n = a
	if a > 3 {
		panic("big")
	}
	return n + 1, nil
}

#VM()
func unnamed(a int) (int, error) {
	if a < 0 {
		return 0, errors.New("negative")
	}
	return a, nil
}

#VM()
func blank(a int) (_ int, ok bool) {
	return a, a > 0
}

func main() {
	fmt.Println(shadowErr(true))
	fmt.Println(shadowErr(false))
	fmt.Println(shadowN())
	fmt.Println(shadowLoop())
	fmt.Println(forward(4))
	fmt.Println(bare(5))
	fmt.Println(deferred(1))
	fmt.Println(deferred(9))
	fmt.Println(unnamed(-1))
	fmt.Println(unnamed(3))
	fmt.Println(blank(2))
}

// Output:
// vm main.shadowErr
// vm end main.shadowErr
// 7 g failed
// vm main.shadowErr
// vm end main.shadowErr
// 1 <nil>
// vm main.shadowN
// vm end main.shadowN
// 5
// vm main.shadowLoop
// vm end main.shadowLoop
// 102
// vm main.forward
// vm end main.forward
// 8 pair
// vm main.bare
// vm end main.bare
// 10 bare
// vm main.deferred
// vm end main.deferred
// 4 <nil>
// vm main.deferred
// vm end main.deferred
// -1 recovered: big
// vm main.unnamed
// vm end main.unnamed
// 0 negative
// vm main.unnamed
// vm end main.unnamed
// 3 <nil>
// vm main.blank
// vm end main.blank
// 2 true
