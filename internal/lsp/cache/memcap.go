package cache

import "sync"

// The check cache holds the checked packages that the server has looked at
// lately (the least recently used go first when it is over its cap). A cap
// that is too small for the packages that one request looks at (a rename of a
// method that a module shares searches a hundred) makes every request check
// them again, which costs time and allocates garbage that costs memory, so the
// default follows the machine: 10% of its memory, at most 300 MB, at least
// 64 MB (an upper bound: the cache holds only what was looked at). The cap is
// also a setting of the server (gocpp.checkCacheMB, gocpp lsp -check-cache).
const (
	maxDefaultCheckBytes = 300 << 20
	minDefaultCheckBytes = 64 << 20
	// unknownMemoryCheckBytes is the default where the memory of the machine is
	// not known to the server.
	unknownMemoryCheckBytes = 200 << 20
)

var (
	checkBytesOnce sync.Once
	checkBytes     int64
)

// DefaultMaxCheckBytes returns the default cap of the check cache.
func DefaultMaxCheckBytes() int64 {
	checkBytesOnce.Do(func() { checkBytes = checkBytesFor(physicalMemory()) })
	return checkBytes
}

// checkBytesFor returns the default cap for a machine with mem bytes of memory
// (0: unknown).
func checkBytesFor(mem int64) int64 {
	if mem <= 0 {
		return unknownMemoryCheckBytes
	}
	return min(maxDefaultCheckBytes, max(minDefaultCheckBytes, mem/10))
}
