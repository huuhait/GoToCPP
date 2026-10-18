// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x[0-9a-f]+ addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 

// Uncaught panic: type W struct{ *St }; var w W; _ = w.A. The first lines of the report and the exit code are Go's.
package main

type St struct {
	A int
	B string
}

func main() {
	type W struct{ *St }
	var w W
	_ = w.A
}

// Output:
