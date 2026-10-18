// phase: 05
// ExitCode: 2
// Stderr: ^panic: first\n\tpanic: assignment to entry in nil map\n\ngoroutine 

// Uncaught panic: defer func() { var m map[int]int; m[1] = 1 }(); panic("first"). The first lines of the report and the exit code are Go's.
package main

func main() {
	defer func() { var m map[int]int
	m[1] = 1 }()
	panic("first")
}

// Output:
