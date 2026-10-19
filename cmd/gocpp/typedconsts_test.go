package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// The constants of typed interop (Phase 12 step 12.8, gap E1, ADR-097) through the driver: with
// GOCPP_TYPED_INTEROP=1 the C/C++ names that sit where Go needs a constant are probed before go/types
// and become constants; without it, or without clang, the program is checked as in opaque mode.
func TestTypedConsts(t *testing.T) {
	const hdr = `#pragma once
#include <cstdio>
#define ROWS 3
#define NEG (-7)
#define NAME "gocpp"
#define TWICE(x) ((x) * 2)
extern int counter;
#define NEXT (counter++)
`
	const src = `package main

import (
	"<cstdio>"
	"./k.hpp"
)

const half = cstdio.BUFSIZ / 2
const name = k.NAME

var buf [cstdio.BUFSIZ]byte
var grid [k.ROWS][k.ROWS + 1]int

func f(c int) int {
	switch c {
	case cstdio.EOF:
		return 1
	case k.NEG:
		return 2
	}
	return 3 << k.ROWS
}

func main() { _ = f(0) + len(buf) + len(grid) + half + len(name) }
`
	typedEnv(t)
	inDir(t, map[string]string{"k.hpp": hdr, "a.gp": src, "gocpp.mod": "module m\n"})
	needClang(t)
	t.Setenv("GOCPP_CXX", "clang++")

	// Typed mode: the program checks, and the constants are in the C++.
	t.Setenv("GOCPP_TYPED_INTEROP", "1")
	if r := gocppRun("", false, "check", "a.gp"); r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Fatalf("typed check: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	em := gocppRun("", false, "emit", "a.gp")
	if em.code != 0 {
		t.Fatalf("typed emit: exit %d\n%s", em.code, em.stderr)
	}
	for _, want := range []string{
		"extern gp::array<uint8_t, 8192> buf;",
		"extern gp::array<gp::array<int64_t, 4>, 3> grid;",
		"case -1: {",
		"case -7: {",
		"f(0) + 8192 + 3 + 4096 + 5", // len(buf), len(grid), half, len(name)
		"return 24;",                 // 3 << 3, folded by Go's constants
	} {
		if !strings.Contains(em.stdout, want) {
			t.Errorf("typed emit has no %q:\n%s", want, em.stdout)
		}
	}

	// Opaque mode (the default) is unchanged: the names are not constants for go/types, the phase gate says so.
	opaqueWant := "./a.gp:8:14: C macros in constant expressions are not supported yet (Phase 12)"
	t.Setenv("GOCPP_TYPED_INTEROP", "")
	if r := gocppRun("", false, "check", "a.gp"); r.code == 0 || !strings.Contains(r.stderr, opaqueWant) {
		t.Errorf("opaque check: exit %d\n%s", r.code, r.stderr)
	}

	// Typed mode without a clang to probe with: the same as opaque mode, no error of its own.
	t.Setenv("GOCPP_TYPED_INTEROP", "1")
	t.Setenv("GOCPP_CLANG", "/nonexistent/clang++")
	if r := gocppRun("", false, "check", "a.gp"); r.code == 0 || !strings.Contains(r.stderr, opaqueWant) || strings.Contains(r.stderr, "cannot evaluate") {
		t.Errorf("typed check without clang: exit %d\n%s", r.code, r.stderr)
	}
	t.Setenv("GOCPP_CLANG", "")

	// A name that is no constant is an error at its place, in Go's words, from gocpp check as well as build.
	inDir(t, map[string]string{"k.hpp": hdr, "gocpp.mod": "module m\n", "b.gp": `package main

import "./k.hpp"

var a [k.NEXT]int
const b = k.TWICE
const c = k.NEG
var d [c]int

func main() {}
`})
	t.Setenv("GOCPP_TYPED_INTEROP", "1")
	r := gocppRun("", false, "check", "-e", "b.gp")
	want := `# command-line-arguments
./b.gp:5:8: array length k.NEXT must be constant: the C++ compiler cannot evaluate it at compile time (non-type template argument is not a constant expression)
./b.gp:6:11: k.TWICE is not constant: it is a macro that does not expand to an expression (it is empty, function-like or a statement): use of undeclared identifier 'TWICE'
./b.gp:8:8: invalid array length c (untyped int constant -7)
`
	if r.code != 1 || r.stderr != want {
		t.Errorf("errors: exit %d\n%s\nwant:\n%s", r.code, r.stderr, want)
	}
}

// The guard of ADR-097 §7: the translation unit asserts that its compiler sees the values that clang gave.
func TestTypedConstsGuard(t *testing.T) {
	const hdr = `#pragma once
#include <cstdio>
#ifdef __clang__
#define VV 1
#else
#define VV 2
#endif
#define SAME 7
#define NAME "a" "b"
constexpr double kHalf = 0.5;
`
	const src = `package main

import (
	"<cstdio>"
	"./g.hpp"
)

const v = g.VV
const s = g.SAME + g.SAME
const n = g.NAME
const h = g.kHalf * 2

var arr [g.SAME]int
var buf [cstdio.BUFSIZ]byte

func main() { _ = v + s + len(n) + len(arr) + len(buf) + int(h) }
`
	typedEnv(t)
	inDir(t, map[string]string{"g.hpp": hdr, "a.gp": src, "gocpp.mod": "module m\n"})
	needClang(t)
	t.Setenv("GOCPP_TYPED_INTEROP", "1")
	t.Setenv("GOCPP_CXX", "clang++")

	em := gocppRun("", false, "emit", "a.gp")
	if em.code != 0 {
		t.Fatalf("emit: exit %d\n%s", em.code, em.stderr)
	}
	for _, want := range []string{
		`static_assert(static_cast<long long>(VV) == 1LL, "gocpp: g.VV differs between the probe compiler (clang++) and this compiler (a.gp:8:11)");`,
		`static_assert(static_cast<long long>(SAME) == 7LL, "gocpp: g.SAME differs between the probe compiler (clang++) and this compiler (a.gp:9:11)");`,
		`static_assert(std::string_view(NAME, sizeof(NAME) - 1) == std::string_view("ab", 2), "gocpp: g.NAME differs`,
		`static_assert(static_cast<double>(::kHalf) == 0.5, "gocpp: g.kHalf differs`,
		`static_assert(static_cast<long long>(BUFSIZ) == 8192LL, "gocpp: cstdio.BUFSIZ differs`,
	} {
		if !strings.Contains(em.stdout, want) {
			t.Errorf("emit lacks %q:\n%s", want, em.stdout)
		}
	}
	if n := strings.Count(em.stdout, "static_assert(static_cast<long long>(SAME)"); n != 1 {
		t.Errorf("SAME is asserted %d times, want once however often it is used", n)
	}
	// Opaque mode writes none.
	t.Setenv("GOCPP_TYPED_INTEROP", "")
	inDir(t, map[string]string{"g.hpp": hdr, "b.gp": "package main\n\nimport \"./g.hpp\"\n\nfunc main() { _ = g.VV }\n", "gocpp.mod": "module m\n"})
	if op := gocppRun("", false, "emit", "b.gp"); op.code != 0 || strings.Contains(op.stdout, "static_assert") {
		t.Errorf("opaque emit (exit %d):\n%s%s", op.code, op.stdout, op.stderr)
	}

	// clang builds it; g++ sees VV as 2 and the build fails at the line of the use, in the words of the guard.
	t.Setenv("GOCPP_TYPED_INTEROP", "1")
	inDir(t, map[string]string{"g.hpp": hdr, "a.gp": src, "gocpp.mod": "module m\n"})
	t.Setenv("GOCPP_CXX", "clang++")
	if r := gocppRun("", false, "build", "-o", filepath.Join(t.TempDir(), "x"), "a.gp"); r.code != 0 {
		t.Fatalf("clang++ build: exit %d\n%s", r.code, r.stderr)
	}
	t.Setenv("GOCPP_CXX", "g++")
	r := gocppRun("", false, "build", "-o", filepath.Join(t.TempDir(), "x"), "a.gp")
	want := "./a.gp:8: C++: static assertion failed: gocpp: g.VV differs between the probe compiler (clang++) and this compiler (a.gp:8:11)"
	if r.code == 0 || !strings.Contains(r.stderr, want) || strings.Contains(r.stderr, "internal error") {
		t.Errorf("g++ build: exit %d\n%s\nwant a line %q", r.code, r.stderr, want)
	}
}

// A name that is the place or the time of its expansion is never a constant of the program; a shift whose
// result the type of its context rejects is the C++ compiler's, as in opaque mode.
func TestTypedConstsLocationAndShift(t *testing.T) {
	const hdr = `#pragma once
#define WHERE __LINE__
#define WHICH __FILE__
#define VIA WHERE
#define S31 31
#define S70 70
`
	typedEnv(t)
	inDir(t, map[string]string{"loc.hpp": hdr, "gocpp.mod": "module m\n", "a.gp": `package main

import "./loc.hpp"

const a = loc.WHERE
var b [loc.VIA]int
const c = loc.WHICH

func main() {}
`, "s.gp": `package main

import "./loc.hpp"

var m int32 = 1 << loc.S31
var n = 1 << loc.S70
var k = 1000 >> loc.S31

func main() { _, _, _ = m, n, k }
`})
	needClang(t)
	t.Setenv("GOCPP_TYPED_INTEROP", "1")
	r := gocppRun("", false, "check", "-e", "a.gp")
	why := "its value depends on where or when it is used (the macro expands to __%s__), so the probe's value would not be the program's"
	want := "# command-line-arguments\n" +
		"./a.gp:5:11: loc.WHERE is not constant: " + strings.Replace(why, "__%s__", "__LINE__", 1) + "\n" +
		"./a.gp:6:8: array length loc.VIA must be constant: " + strings.Replace(why, "__%s__", "__LINE__", 1) + "\n" +
		"./a.gp:7:11: loc.WHICH is not constant: " + strings.Replace(why, "__%s__", "__FILE__", 1) + "\n"
	if r.code != 1 || r.stderr != want {
		t.Errorf("location macros: exit %d\n%s\nwant:\n%s", r.code, r.stderr, want)
	}
	// Shifts: both modes accept the file with the C++ compiler deciding.
	if r := gocppRun("", false, "check", "-e", "s.gp"); r.code != 0 || r.stderr != "" {
		t.Errorf("typed shifts: exit %d\n%s", r.code, r.stderr)
	}
	t.Setenv("GOCPP_TYPED_INTEROP", "")
	if r := gocppRun("", false, "check", "-e", "s.gp"); r.code != 0 || r.stderr != "" {
		t.Errorf("opaque shifts: exit %d\n%s", r.code, r.stderr)
	}
}
