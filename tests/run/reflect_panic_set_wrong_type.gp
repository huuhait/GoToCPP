// phase: 09
// ExitCode: 2
// Stderr: (?m)^panic: reflect\.Set: value of type string is not assignable to type int

package main

import (
	"fmt"
	"reflect"
)

type S struct {
	Pub  int
	priv int
}

var _ = fmt.Sprint
var _ = reflect.TypeOf

func main() {
	fmt.Println("before")
	x := 1
	reflect.ValueOf(&x).Elem().Set(reflect.ValueOf("s"))
	fmt.Println("not reached")
}

// Output:
// before
