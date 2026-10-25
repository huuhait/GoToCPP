// phase: 05
// ExitCode: 2
// Stderr: ^panic: boom 100 \[recovered, repanicked\]\n\ngoroutine 1 \[running\]:\n
// Stderr: \nmain\.thrower\([^\n]*\)\n\t[^\n]+\nmain\.inner\([^\n]*\)\n\t[^\n]+\nmain\.middle\([^\n]*\)\n\t[^\n]+\nmain\.outer\([^\n]*\)\n\t[^\n]+\nmain\.main\(\)\n\t[^\n]+\n

// An unrecovered panic that two functions with deferred calls let go on, after the same way was
// taken by 100 panics that were recovered: the trace shows the frames of the panic, from where it
// was thrown to main. That is the default build's trace. In an optimized build the walks of the
// panic's stack use the rules that they kept on the way of the recovered ones (the fast walk of
// runtime/src/os/linux/trace.cpp, whose walks runtime/test/stackwalk_test.cpp checks against the
// unwinder's). A -release trace has no lines and leaves frames out, as it did before the fast
// walk, so this test's Stderr lines hold for the default build only.
package main

import "fmt"

var deferred int

func thrower(i int) {
	panic(fmt.Sprint("boom ", i))
}

func inner(i int) {
	defer func() { deferred++ }()
	thrower(i)
}

func middle(i int) {
	defer func() {
		if i == 100 {
			fmt.Println("middle's deferred call, deferred", deferred)
		}
	}()
	inner(i)
}

func outer(i int) {
	defer func() {
		if i < 100 {
			recover()
			return
		}
		r := recover()
		panic(r) // the same value: Go prints it once, "[recovered, repanicked]"
	}()
	middle(i)
}

func main() {
	for i := 0; i < 100; i++ {
		outer(i)
	}
	fmt.Println("recovered", deferred)
	outer(100)
}

// Output:
// recovered 100
// middle's deferred call, deferred 101
