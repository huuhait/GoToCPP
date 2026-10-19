// phase: 08

// The names of methods: main.T.Value for a value receiver, main.(*T).Pointer for a pointer
// receiver.
package main

import (
	"fmt"
	"runtime"
)

type T struct{}

func name() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}

func (T) Value() string    { return name() }
func (*T) Pointer() string { return name() }

type Named interface{ Pointer() string }

func main() {
	var t T
	fmt.Println(t.Value())
	fmt.Println(t.Pointer())
	var n Named = &t
	fmt.Println(n.Pointer())
	f := t.Value
	fmt.Println(f())
}

// Output:
// main.T.Value
// main.(*T).Pointer
// main.(*T).Pointer
// main.T.Value
