package util

import "fmt"

// Check panics with the place of the use.
macro Check(ok bool, msg lazy string) {
	if !ok {
		panic(fmt.Sprintf("%s:%d: %s", #file(), #line(), msg()))
	}
}

#Derive(Equal)
type Point struct{ X, Y int }
