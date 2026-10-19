// phase: 09
// ExitCode: 2
// Stderr: (?m)^panic: reflect: call of reflect\.Value\.Int on zero Value

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
	var v reflect.Value
	_ = v.Int()
	fmt.Println("not reached")
}

// Output:
// before
