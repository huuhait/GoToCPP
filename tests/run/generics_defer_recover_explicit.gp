// phase: 08

// recover inside a generic function that is deferred with explicit or inferred type arguments, or as a func value.
package main

import "fmt"

type MyErr struct{ code int }

func Rec[T any](x T) {
	fmt.Println("Rec called", x)
	if r := recover(); r != nil {
		fmt.Println("rec", x, r)
	}
}

func Catch[E any](out *string) {
	if r := recover(); r != nil {
		if e, ok := r.(E); ok {
			*out = fmt.Sprint("caught ", e)
			return
		}
		panic(r)
	}
}

func WithRec[T any](x T) {
	defer Rec(x)
	defer Rec[int](5)
	panic("p")
}

func f1() (out string) {
	defer Catch[MyErr](&out)
	panic(MyErr{7})
}

func f2() (out string) {
	defer func() { out += fmt.Sprint(" outer ", recover()) }()
	defer Catch[string](&out)
	panic(MyErr{8})
}

func f3() {
	f := Rec[string]
	defer f("value")
	panic("q")
}

func f4(done chan bool) {
	defer func() { done <- true }()
	defer Rec[float64](1.5)
	panic("goroutine")
}

func main() {
	WithRec("w")
	fmt.Println(f1())
	fmt.Println(f2())
	f3()
	done := make(chan bool)
	go f4(done)
	<-done
}

// Output:
// Rec called 5
// rec 5 p
// Rec called w
// caught {7}
//  outer {8}
// Rec called value
// rec value q
// Rec called 1.5
// rec 1.5 goroutine
