// phase: 09
// ExitCode: 2
// Stderr: (?m)^panic: assignment to entry in nil map

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
	var m map[string]int
	reflect.ValueOf(m).SetMapIndex(reflect.ValueOf("a"), reflect.ValueOf(1))
	fmt.Println("not reached")
}

// Output:
// before
