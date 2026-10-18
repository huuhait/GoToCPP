// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference \[recovered\]\n\tpanic: other\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

package main

import "fmt"

type T struct{ n int }

func main() {
	defer func() {
		r := recover()
		fmt.Println("recovered:", r)
		panic("other")
	}()
	var p *T
	fmt.Println(p.n)
}

// Output:
// recovered: runtime error: invalid memory address or nil pointer dereference
