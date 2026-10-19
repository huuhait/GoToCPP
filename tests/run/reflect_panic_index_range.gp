// phase: 09
// ExitCode: 2
// Stderr: (?m)^panic: reflect: slice index out of range

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
	_ = reflect.ValueOf([]int{1}).Index(5)
	fmt.Println("not reached")
}

// Output:
// before
