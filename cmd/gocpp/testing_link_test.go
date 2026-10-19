//go:build linux

package main

import (
	"debug/elf"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoTestingInPlainPrograms checks that a program that does not import package testing is linked
// without the natives of testing (runtime/src/testing*.cpp): they replace the global operator new
// (to count allocations) and hold a regular expression translator, so they would make every
// program bigger, and bypass the interceptors of operator new of AddressSanitizer.
func TestNoTestingInPlainPrograms(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		dir := inDir(t, map[string]string{"main.gp": mainSrc})
		exe := filepath.Join(dir, "prog")
		if r := gocppRun("", false, "build", "-o", exe, "."); r.code != 0 {
			t.Fatalf("gocpp build = %+v", r)
		}
		f, err := elf.Open(exe)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		syms, err := f.Symbols()
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range syms {
			if s.Section == elf.SHN_UNDEF {
				continue
			}
			if strings.Contains(s.Name, "7testing") || strings.Contains(s.Name, "alloc_users") || strings.Contains(s.Name, "plain_alloc") ||
				s.Name == "_Znwm" || s.Name == "_ZdlPv" {
				t.Errorf("a program that does not import testing defines %s", s.Name)
			}
		}
	})
}

// TestASanMismatchInPlainPrograms checks that -asan still reports a new[] released by delete in a
// program that does not import testing (ASan's own operator new is in charge there).
func TestASanMismatchInPlainPrograms(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"main.gp": mainSrc,
			"a.cpp": "extern \"C\" int mismatch() {\n\tint* p = new int[4];\n\tp[0] = 1;\n\tint v = p[0];\n\tdelete p;\n\treturn v;\n}\n"})
		callSources(t, `extern "C" int mismatch();`, "mismatch()")
		r := gocppRun("", false, "run", "-asan", ".")
		if r.code == 0 || !strings.Contains(r.stderr, "alloc-dealloc-mismatch") {
			t.Errorf("gocpp run -asan of new[] released by delete = %+v, want an alloc-dealloc-mismatch report", r)
		}
	})
}
