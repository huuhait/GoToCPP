// phase: 09
// ExitCode: 2
// Stderr: (?m)^panic: reflect: call of reflect\.Value\.Len on int Value

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
	_ = reflect.ValueOf(1).Len()
	fmt.Println("not reached")
}

// Output:
// before
