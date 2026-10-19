// phase: 08

// FuncForPC gives the function of an address: its name, its entry and the file and line of an
// address inside it. FuncForPC(0) is nil.
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

func target() (uintptr, int) {
	pc, _, line, _ := runtime.Caller(0)
	return pc, line
}

func main() {
	pc, line := target()
	f := runtime.FuncForPC(pc)
	fmt.Println(f.Name())
	file, l := f.FileLine(pc)
	fmt.Println(base(file), l == line)
	fmt.Println(f.Entry() <= pc, f.Entry() != 0)
	fmt.Println(runtime.FuncForPC(0) == nil)
}

// Output:
// main.target
// source true
// true true
// true
