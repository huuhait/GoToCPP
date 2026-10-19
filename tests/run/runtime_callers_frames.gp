// phase: 08

// Callers fills a slice with return addresses and CallersFrames turns them into function names,
// files and lines. Only the frames of package main are printed (Go also lists runtime.main and
// runtime.goexit, gocpp stops at main).
package main

import (
	"fmt"
	"runtime"
)

// base names the source file: the test's own name is main.go for Go and the .gp file for gocpp,
// so it only says that the file is a Go source (or gocpp's), whatever the name is.
func base(path string) string {
	if len(path) > 3 && (path[len(path)-3:] == ".go" || path[len(path)-3:] == ".gp") {
		return "source"
	}
	return path
}

func c() {
	pcs := make([]uintptr, 20)
	n := runtime.Callers(1, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		f, more := frames.Next()
		if len(f.Function) > 5 && f.Function[:5] == "main." {
			fmt.Println(f.Function, base(f.File), f.Line)
		}
		if !more {
			break
		}
	}
}

func b() { c() }
func a() { b() }

func main() {
	a()
}

// Output:
// main.c source 24
// main.b source 37
// main.a source 38
// main.main source 41
