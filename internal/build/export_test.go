package build

import (
	"slices"
	"testing"
)

// The object cache key of a library source depends on the flags that the recipe
// gives it (cxxlib_build.go); the tests of package build_test ask the cache through these.

// ForEachCompiler runs f for g++ and clang++, skipping a compiler that isn't installed.
func ForEachCompiler(t *testing.T, f func(t *testing.T, cxx CXX)) {
	t.Helper()
	forEachCompiler(t, f)
}

// CompileLibSource compiles the file src as a source of the library lib with
// the library's own flags (public, then private) through the object cache in
// the folder cache, as the build does, and reports whether the compiler ran
// (a miss) or the object came from the cache (a hit).
func CompileLibSource(t *testing.T, cxx CXX, cache, src string, lib *CxxLib) (ran bool) {
	t.Helper()
	flags := slices.Concat(lib.Public, lib.Private)
	return libCompile(t, cxx, cache, src, lib.Module, lib.Version, flags...).ran
}
