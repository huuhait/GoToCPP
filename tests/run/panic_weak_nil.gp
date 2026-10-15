// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// Calling Value through a nil *weak.Pointer panics with Go's nil dereference, after the value
// of a nil weak pointer itself was fine (the zero weak pointer's Value is nil).
package main

import (
	"fmt"
	"weak"
)

type T struct{ n int }

type Holder struct{ w *weak.Pointer[T] }

func main() {
	var w weak.Pointer[T]
	fmt.Println(w.Value() == nil)
	var h Holder
	fmt.Println(h.w.Value() == nil)
}

// Output:
// true
