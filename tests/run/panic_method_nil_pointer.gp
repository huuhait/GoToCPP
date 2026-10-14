// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\nmain\.\(\*List\)\.Push\(.*\)\n\t.*panic_method_nil_pointer\.g[op]:24( \+0x[0-9a-f]+)?\nmain\.main\(\)\n

// A pointer method gets a nil pointer (step 3.10): it can test it, and dereferencing it panics
// inside the method, which the trace shows as main.(*List).Push, as Go does.
package main

import "fmt"

type List struct {
	head *int
	n    int
}

func (l *List) Len() int {
	if l == nil {
		return 0
	}
	return l.n
}

func (l *List) Push(v int) {
	l.n++
	l.head = &v
}

func main() {
	var l *List
	fmt.Println(l.Len())
	l.Push(1)
}

// Output:
// 0
