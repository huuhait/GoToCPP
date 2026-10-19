// phase: 09
// ExitCode: 2
// Stderr: (?m)^panic: reflect: Elem of invalid type int

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
	_ = reflect.TypeOf(1).Elem()
	fmt.Println("not reached")
}

// Output:
// before
