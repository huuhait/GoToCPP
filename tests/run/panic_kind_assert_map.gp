// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: interface \{\} is map\[string\]int, not map\[string\]string\n\ngoroutine 

// Uncaught panic: var a any = map[string]int{}; _ = a.(map[string]string). The first lines of the report and the exit code are Go's.
package main

func main() {
	var a any = map[string]int{}
	_ = a.(map[string]string)
}

// Output:
