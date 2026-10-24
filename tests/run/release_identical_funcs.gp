// phase: 09
// flags: -release -trace

// Two functions with the same body stay two functions in an optimized g++ build: g++'s -O2 folds
// identical functions into one (-fipa-icf), and runtime.Caller and FuncForPC (and a trace) would
// then name the one that was kept for both. gocpp builds with -fno-ipa-icf when g++ optimizes
// (internal/build lto.go noICF), in -release and -release -trace. The test is -release -trace:
// without the line tables a call that the compiler inlined is its caller's for runtime.Caller.
package main

import (
	"fmt"
	"runtime"
)

func first() string {
	pc, _, _, _ := runtime.Caller(0)
	return runtime.FuncForPC(pc).Name()
}

func second() string {
	pc, _, _, _ := runtime.Caller(0)
	return runtime.FuncForPC(pc).Name()
}

var funcs = []func() string{first, second}

func main() {
	for _, f := range funcs {
		fmt.Println(f())
	}
}

// Output:
// main.first
// main.second
