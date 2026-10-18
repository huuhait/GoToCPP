// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: main\.I is main\.T1, not \*main\.PT\n\ngoroutine 

// Uncaught panic: var a I = T1{}; _ = a.(*PT). The first lines of the report and the exit code are Go's.
package main

type I interface{ M() }

type T1 struct{}

func (T1) M() {}

type PT struct{}

func (*PT) M() {}

func main() {
	var a I = T1{}
	_ = a.(*PT)
}

// Output:
