// phase: 08

// Generic functions are named with their type arguments shown as [...], as Go does (methods of
// generic types come with generic types, Phase 07).
package main

import (
	"fmt"
	"runtime"
)

func name() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}

func Id[T any](x T) string { return name() }

func main() {
	fmt.Println(Id(1))
	fmt.Println(Id("s"))
}

// Output:
// main.Id[...]
// main.Id[...]
