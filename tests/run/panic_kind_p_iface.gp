// phase: 05
// ExitCode: 2
// Stderr: ^panic: \(main\.T1\) 0x[0-9a-f]+\n\ngoroutine 

// Uncaught panic: var i I = T1{}; panic(i). The first lines of the report and the exit code are Go's.
package main

type I interface{ M() }

type T1 struct{}

func (T1) M() {}

func main() {
	var i I = T1{}
	panic(i)
}

// Output:
