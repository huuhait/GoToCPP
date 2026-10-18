// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: interface \{\} is main\.MyI, not int\n\ngoroutine 

// Uncaught panic: var a any = MyI(3); _ = a.(int). The first lines of the report and the exit code are Go's.
package main

type MyI int

func main() {
	var a any = MyI(3)
	_ = a.(int)
}

// Output:
