// phase: 09
// ExitCode: 2
// Stderr: (?m)^panic: reflect\.Value\.Interface: cannot return value obtained from unexported field or method

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
	_ = reflect.ValueOf(x).Field(1).Interface()
	fmt.Println("not reached")
}

// Output:
// before
