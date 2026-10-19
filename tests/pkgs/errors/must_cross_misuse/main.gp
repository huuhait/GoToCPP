// phase: 08

// Misuse of must functions of another package (step 10.13): the extra error result is part of the
// signature that the importing package sees, and the rules of F! hold across packages.
package main

import "example.com/mustbad/store"

func f() {
	db := store.Open("x") // ERROR "assignment mismatch: 1 variable but store.Open returns 2 values"
	_ = db
	n := store.Run!() // ERROR "store.Run!\(...\) returns only an error: use it as a statement"
	_ = n
	go store.Run!() // ERROR "go store.Run!\(...\): the error of Run can't be returned to anyone from another goroutine"
	_ = store.Typed!() // ERROR "store.Typed! requires the last result of store.Typed to be exactly error, got \*store.MyErr"
	var starter interface{ Start() } = store.Srv{} // ERROR "cannot use store.Srv\{\} \(value of struct type store.Srv\) as interface\{Start\(\)\} value in variable declaration: store.Srv does not implement interface\{Start\(\)\} \(wrong type for method Start\)"
	_ = starter
}

func main() { f() }
