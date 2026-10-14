// phase: 03
// ExitCode: 2
// Stderr: ^before\nf ran\npanic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// Calling a nil func panics like a nil pointer dereference, after its arguments are evaluated,
// with Go's signal line.
package main

func f() int {
	println("f ran")
	return 1
}

func main() {
	var fn func(int)
	println("before")
	fn(f())
}

// Output:
