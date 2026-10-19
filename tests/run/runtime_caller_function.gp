// phase: 08

// runtime.Caller(0) in a plain function: the source file, the line of the call, and the function's
// name in Go's form.
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

func where() string {
	pc, file, line, ok := runtime.Caller(0)
	return fmt.Sprint(runtime.FuncForPC(pc).Name(), " ", base(file), ":", line, " ", ok)
}

func main() {
	fmt.Println(where())
}

// Output:
// main.where source:22 true
