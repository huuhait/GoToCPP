// phase: 09
//go:build linux
// ExitCode: 2
// Stderr: (?m)^fatal error: stack overflow$
// Stderr: (?m)^goroutine 1 \[running\]:$

// A mapping put into the gap below the main stack (mmap with a hint, 32 MiB below it) ends the stack
// above the end that the runtime computed at start; the overflow handler takes the fault just below
// the stack pointer as an overflow anyway (thread.cpp, main_overflow): Go's report, not a bare
// SIGSEGV. Not a Go program.
package main

import (
	"<sys/mman.h>"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unsafe"
)

var sink [4]int

func recurse(n int) int {
	sink[n%4] = n
	return recurse(n+1) + sink[(n+1)%4]
}

func main() {
	data, _ := os.ReadFile("/proc/self/maps")
	var lo uint64
	for _, l := range strings.Split(string(data), "\n") {
		if strings.HasSuffix(l, "[stack]") {
			lo, _ = strconv.ParseUint(l[:strings.Index(l, "-")], 16, 64)
		}
	}
	hint := uintptr(lo - 32<<20)
	p := mman.mmap(unsafe.Pointer(hint), 1<<20, mman.PROT_READ|mman.PROT_WRITE, mman.MAP_PRIVATE|mman.MAP_ANONYMOUS, -1, 0)
	fmt.Println("mapped at the hint:", uintptr(unsafe.Pointer(p)) == hint)
	fmt.Println(recurse(0))
}

// Output:
// mapped at the hint: true
