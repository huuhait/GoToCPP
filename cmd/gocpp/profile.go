package main

import (
	"fmt"
	"os"
	"runtime/pprof"
)

// startProfile starts a CPU profile of this run of gocpp when GOCPP_CPUPROFILE names a file, and
// returns the function that ends it. It is for finding out where a build spends its time
// (go tool pprof gocpp file); the build's own time in the C++ compiler is not in it, only gocpp's.
func startProfile() (stop func()) {
	name := os.Getenv("GOCPP_CPUPROFILE")
	if name == "" {
		return func() {}
	}
	f, err := os.Create(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gocpp: GOCPP_CPUPROFILE: %v\n", err)
		return func() {}
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		fmt.Fprintf(os.Stderr, "gocpp: GOCPP_CPUPROFILE: %v\n", err)
		f.Close()
		return func() {}
	}
	return func() {
		pprof.StopCPUProfile()
		f.Close()
	}
}
