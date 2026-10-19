// phase: 09
// ExitCode: 2
// Stderr: (?m)^panic: reflect: reflect\.Value\.SetInt using value obtained using unexported field

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
	x := S{}
	reflect.ValueOf(&x).Elem().Field(1).SetInt(2)
	fmt.Println("not reached")
}

// Output:
// before
