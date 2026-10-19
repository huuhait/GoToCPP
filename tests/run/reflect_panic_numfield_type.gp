// phase: 09
// ExitCode: 2
// Stderr: (?m)^panic: reflect: NumField of non-struct type int

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
	reflect.TypeOf(1).NumField()
	fmt.Println("not reached")
}

// Output:
// before
