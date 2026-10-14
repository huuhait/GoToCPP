package build

import (
	"slices"
	"testing"
)

func TestFlags(t *testing.T) {
	debug := []string{"-std=c++20", "-fwrapv", "-fno-strict-aliasing", "-ffp-contract=off", "-g", "-gno-column-info", "-O0"}
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
	// No columns in the debug info: they would be C++ columns on .gp lines.
	for _, f := range []Family{GCC, Clang} {
		if flags := Flags(CXX{Family: f}, false); !slices.Contains(flags, "-gno-column-info") {
			t.Errorf("Flags(%v) = %q, without -gno-column-info", f, flags)
		}
	}
	// CompileFlags: Flags, and -DGOCPP_NOCHECKS with nochecks (last, after the
	// family's flags).
	for _, f := range []Family{GCC, Clang} {
		cxx := CXX{Family: f}
		if got := CompileFlags(cxx, true, false); !slices.Equal(got, Flags(cxx, true)) {
			t.Errorf("CompileFlags(%v, asan, no nochecks) = %q, want Flags: %q", f, got, Flags(cxx, true))
		}
		if got, want := CompileFlags(cxx, false, true), append(Flags(cxx, false), "-DGOCPP_NOCHECKS"); !slices.Equal(got, want) {
			t.Errorf("CompileFlags(%v, nochecks) = %q, want %q", f, got, want)
		}
	}
	// Flags returns a new slice each time: a caller may append to it.
	a := Flags(CXX{Family: GCC}, false)
	_ = append(a[:len(a):len(a)], "-x")
	if b := Flags(CXX{Family: GCC}, false); !slices.Equal(b, debug) {
		t.Errorf("Flags changed: %q", b)
	}
}
