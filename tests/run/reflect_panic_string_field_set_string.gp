// phase: 09
// ExitCode: 2
// Stderr: (?m)^panic: reflect: call of reflect\.Value\.SetString on int Value

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
	reflect.ValueOf(&x).Elem().Field(0).SetString("a")
	fmt.Println("not reached")
}

// Output:
// before
