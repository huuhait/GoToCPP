// Package testlib makes gptestlib, a small C library with a pkg-config
// file of its own, for the tests of //gocpp:pkg-config (Phase 04, step
// 4.15). Only tests import it.
//
// Inside the project's Nix dev shell, SDL2, raylib, sqlite and zlib
// compile and link even without the flags of pkg-config (the compiler
// wrapper adds their folders), so a test with them would pass even if
// gocpp dropped the flags. gptestlib needs every one of its flags:
//   - its header is in a folder that only the -I of gptestlib.pc names;
//   - the header stops with #error unless -DGPTESTLIB_ON (from the .pc) is
//     set;
//   - its archive is in a folder that only the -L of the .pc names.
//
// It also declares the C++ namespace gptl, for the namespace probe.
package testlib

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// Header is the library's header, gptestlib.h.
const Header = `#pragma once
// gptestlib: a test-only library of gocpp's tests (internal/build/testlib).
#ifndef GPTESTLIB_ON
#error "gptestlib.h needs -DGPTESTLIB_ON, which pkg-config --cflags gptestlib gives"
#endif
#ifdef __cplusplus
extern "C" {
#endif
int gptestlib_answer(void);
#ifdef __cplusplus
}
namespace gptl {
inline int twice(int x) { return 2 * x; }
}
#endif
`

// source is the library's code, compiled as C++.
const source = `#include "gptestlib.h"
extern "C" int gptestlib_answer(void) { return 42; }
`

// pcFile is gptestlib.pc, with @PREFIX@ for the folder of the library.
const pcFile = `prefix=@PREFIX@
includedir=${prefix}/include
libdir=${prefix}/lib

Name: gptestlib
Description: a test-only library of gocpp's tests
Version: 1.0
Cflags: -I${includedir} -DGPTESTLIB_ON
Libs: -L${libdir} -lgptestlib
`

// Install writes the library into the new folder dir: include/gptestlib.h,
// pkgconfig/gptestlib.pc and, if cxx is not nil, lib/libgptestlib.a,
// compiled with cxx and archived with ar (a test without ar is skipped).
// It returns the folder of the .pc file, for PKG_CONFIG_PATH.
func Install(t testing.TB, dir string, cxx *build.CXX) string {
	t.Helper()
	write := func(name, data string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("include/gptestlib.h", Header)
	write("pkgconfig/gptestlib.pc", strings.ReplaceAll(pcFile, "@PREFIX@", filepath.ToSlash(dir)))
	if cxx != nil {
		ar, err := exec.LookPath("ar")
		if err != nil {
			t.Skipf("making gptestlib needs ar: %v", err)
		}
		write("src/gptestlib.cpp", source)
		obj := filepath.Join(dir, "src", "gptestlib.o")
		lib := filepath.Join(dir, "lib", "libgptestlib.a")
		if err := os.MkdirAll(filepath.Dir(lib), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := cxx.Command(context.Background(), "-std=c++20", "-DGPTESTLIB_ON", "-I", filepath.Join(dir, "include"),
			"-c", filepath.Join(dir, "src", "gptestlib.cpp"), "-o", obj)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("compiling gptestlib: %v\n%s", err, out)
		}
		if out, err := exec.Command(ar, "rcs", lib, obj).CombinedOutput(); err != nil {
			t.Fatalf("archiving gptestlib: %v\n%s", err, out)
		}
	}
	return filepath.Join(dir, "pkgconfig")
}

// NeedPkgConfig skips the test when pkg-config is not installed.
func NeedPkgConfig(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("pkg-config"); err != nil {
		t.Skipf("pkg-config is not installed: %v", err)
	}
}
