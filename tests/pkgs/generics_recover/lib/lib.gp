package lib

import "fmt"

// Catch recovers a panic whose value has type E, and passes any other on.
func Catch[E any](out *string) {
	if r := recover(); r != nil {
		if e, ok := r.(E); ok {
			*out = fmt.Sprint("caught ", e)
			return
		}
		panic(r)
	}
}

func Swallow[T any](tag T) {
	if r := recover(); r != nil {
		fmt.Println("swallowed", tag, r)
	}
}
