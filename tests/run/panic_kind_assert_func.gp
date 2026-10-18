// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: interface \{\} is func\(int\), not func\(string\)\n\ngoroutine 

// Uncaught panic: var a any = func(int) {}; _ = a.(func(string)). The first lines of the report and the exit code are Go's.
package main

func main() {
	var a any = func(int) {}
	_ = a.(func(string))
}

// Output:
