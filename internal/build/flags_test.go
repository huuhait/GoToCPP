package build

import (
	"slices"
	"testing"
)

func TestFlags(t *testing.T) {
	debug := []string{"-std=c++20", "-fwrapv", "-fno-strict-aliasing", "-ffp-contract=off", "-g", "-O0"}
	asan := []string{"-fsanitize=address,undefined", "-fno-omit-frame-pointer", "-fno-sanitize-recover=all"}
	for _, c := range []struct {
		family    Family
		asan      bool
		want, gen []string
	}{
		{GCC, false, debug, []string{"-Wno-overflow"}},
		{GCC, true, slices.Concat(debug, asan), []string{"-Wno-overflow"}},
		{Clang, false, slices.Concat(debug, []string{"-Qunused-arguments"}), []string{"-Wno-tautological-compare"}},
		{Clang, true, slices.Concat(debug, asan, []string{"-Qunused-arguments"}), []string{"-Wno-tautological-compare"}},
	} {
		cxx := CXX{Family: c.family}
		if got := Flags(cxx, c.asan); !slices.Equal(got, c.want) {
			t.Errorf("Flags(%v, asan=%v) = %q, want %q", c.family, c.asan, got, c.want)
		}
		if got := GeneratedFlags(cxx); !slices.Equal(got, c.gen) {
			t.Errorf("GeneratedFlags(%v) = %q, want %q", c.family, got, c.gen)
		}
	}
	// Flags returns a new slice each time: a caller may append to it.
	a := Flags(CXX{Family: GCC}, false)
	_ = append(a[:len(a):len(a)], "-x")
	if b := Flags(CXX{Family: GCC}, false); !slices.Equal(b, debug) {
		t.Errorf("Flags changed: %q", b)
	}
}
