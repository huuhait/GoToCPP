package rand

import "sync/atomic"

// The runtime's generator is a native of gocpp (a program cannot declare one): runtime_rand in the copy of the package is this splitmix64 on an atomic counter.
var runtimeRandState atomic.Uint64

func runtime_rand() uint64 {
	z := runtimeRandState.Add(0x9E3779B97F4A7C15)
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
