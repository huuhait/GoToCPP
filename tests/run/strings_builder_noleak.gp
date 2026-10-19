// phase: 09
//go:build linux

// A strings.Builder frees its buffer: 1200000 builders of about 1 KB each are made and dropped, and the resident memory of
// the process stays below 640 MB (it would grow by 1.2 GB if every Builder were kept; AddressSanitizer's quarantine adds
// some). Go's code stores the Builder's own address in a *Builder field, which is a counted reference to itself in gocpp
// (ADR-004): the port keeps the address as a uintptr (tools/portstd/patches/strings.rules), or every Builder and its
// buffer would stay in memory for good.

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func rssPages() int {
	b, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		panic(err)
	}
	f := strings.Fields(string(b))
	n, _ := strconv.Atoi(f[1])
	return n
}

var kb = strings.Repeat("0123456789", 100)

func build(i int) string {
	var b strings.Builder
	b.WriteString(kb)
	return b.String()[:5]
}

func main() {
	build(0)
	before := rssPages()
	n := 0
	for i := 0; i < 1200000; i++ {
		n += len(build(i))
	}
	grown := (rssPages() - before) * 4096 / (1 << 20)
	fmt.Println(n, grown < 640)
}

// Output:
// 6000000 true
