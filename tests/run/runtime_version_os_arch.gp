// phase: 08

// Version, GOOS and GOARCH are the constants that a Go 1.25 program sees.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	v := runtime.Version()
	fmt.Println(len(v) > 4 && v[:4] == "go1.")
	known := map[string]bool{"linux": true, "darwin": true, "windows": true, "freebsd": true}
	fmt.Println(known[runtime.GOOS])
	arch := map[string]bool{"amd64": true, "arm64": true, "386": true, "arm": true, "riscv64": true}
	fmt.Println(arch[runtime.GOARCH])
	const c = runtime.GOOS + "/" + runtime.GOARCH
	fmt.Println(len(c) > 3)
	fmt.Println(runtime.Compiler == "gc" || runtime.Compiler == "gocpp")
}

// Output:
// true
// true
// true
// true
// true
