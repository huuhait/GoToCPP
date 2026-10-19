// phase: 08

// Caller(1) is the function that called, at the line of its call; Caller(2) the one above.
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

func trace(skip int) string {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "no caller"
	}
	return fmt.Sprint(runtime.FuncForPC(pc).Name(), " ", base(file), ":", line)
}

func inner() string { return trace(1) }

func outer() string { return trace(2) }

func wrap() string { return outer() }

func main() {
	fmt.Println(inner())
	fmt.Println(wrap())
	fmt.Println(trace(0))
}

// Output:
// main.inner source:28
// main.wrap source:32
// main.trace source:21
