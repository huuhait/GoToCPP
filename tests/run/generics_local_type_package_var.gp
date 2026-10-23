// phase: 07

// A generic type declared inside a function literal that initializes a package variable.
package main

import "fmt"

var global = func() string {
	type Opt[T any] struct {
		v  T
		ok bool
	}
	o := Opt[string]{"g", true}
	return fmt.Sprintf("%v %+v %#v %T", o, o, o, o)
}()

func main() {
	fmt.Println(global)
}

// Output:
// {g true} {v:g ok:true} main.Opt[string]{v:"g", ok:true} main.Opt[string]
