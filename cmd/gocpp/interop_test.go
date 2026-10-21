package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// The interop mode (Phase 12 step 12.11, ADR-101): -interop and GOCPP_INTEROP, the order of their
// sources, the clang-missing fallback and the early errors of typed mode in gocpp check.

const interopHdr = `#pragma once
struct Rect { int x, y, w, h; };
inline int area(const Rect& r) { return r.w * r.h; }
inline int add(int a, int b) { return a + b; }
`

const interopOK = `package main

import "./lib.hpp"

func main() {
	r := lib.Rect{x: 1, y: 2}
	_ = lib.area(r)
}
`

// interopBad calls a function with too many arguments: typed mode says so before the C++ compile.
const interopBad = `package main

import "./lib.hpp"

func main() {
	n := 4
	_ = lib.add(n, 2, 3)
}
`

// interopBadStd is the same mistake with the C++ library's header alone.
const interopBadStd = `package main

import "<cmath>"

func main() {
	x := 4.0
	_ = cmath.sqrt(x, 2.0, 3.0)
}
`

const interopGoOnly = `package main

func main() {}
`

func interopSetup(t *testing.T, files map[string]string) {
	t.Helper()
	typedEnv(t)
	t.Setenv("GOCPP_INTEROP", "")
	t.Setenv("GOCPP_TYPED_INTEROP", "")
	t.Setenv("GOCPP_CXX", "clang++")
	inDir(t, files)
	// A file written a moment ago is "racy" for the object cache (internal/build/objcache.go): a build
	// that starts within 2 s of the write is not stamped, and a test of the up-to-date check would
	// see "no record of an earlier build". Make the files old.
	old := time.Now().Add(-time.Hour)
	for name := range files {
		if err := os.Chtimes(name, old, old); err != nil {
			t.Fatal(err)
		}
	}
}

// note returns the line of "check -v" about the mode ("interop: typed mode (-interop=typed)").
func note(t *testing.T, args ...string) (string, result) {
	t.Helper()
	r := gocppRun("", false, append([]string{"check", "-v"}, args...)...)
	for _, l := range strings.Split(r.stderr, "\n") {
		if strings.HasPrefix(l, "interop: ") {
			return l, r
		}
	}
	return "", r
}

// TestInteropPrecedence pins the order of the sources of the mode: the flag, GOCPP_INTEROP,
// GOCPP_TYPED_INTEROP, the default; and the errors for values that are no mode.
func TestInteropPrecedence(t *testing.T) {
	interopSetup(t, map[string]string{"lib.hpp": interopHdr, "a.gp": interopOK})
	needClang(t)
	tests := []struct {
		name string
		env  map[string]string
		args []string
		want string // the note of -v
	}{
		{"flag typed", nil, []string{"-interop=typed"}, "interop: typed mode (-interop=typed)"},
		{"flag opaque", nil, []string{"-interop=opaque"}, "interop: opaque mode (-interop=opaque)"},
		{"flag auto", nil, []string{"-interop=auto"}, "interop: typed mode (-interop=auto)"},
		{"flag two words", nil, []string{"-interop", "typed"}, "interop: typed mode (-interop=typed)"},
		{"env typed", map[string]string{"GOCPP_INTEROP": "typed"}, nil, "interop: typed mode (GOCPP_INTEROP=typed)"},
		{"env opaque", map[string]string{"GOCPP_INTEROP": "opaque"}, nil, "interop: opaque mode (GOCPP_INTEROP=opaque)"},
		{"flag wins over env", map[string]string{"GOCPP_INTEROP": "opaque"}, []string{"-interop=typed"}, "interop: typed mode (-interop=typed)"},
		{"legacy 1 is typed", map[string]string{"GOCPP_TYPED_INTEROP": "1"}, nil, "interop: typed mode (GOCPP_TYPED_INTEROP=1)"},
		{"legacy 0 is opaque", map[string]string{"GOCPP_TYPED_INTEROP": "0"}, nil, "interop: opaque mode (GOCPP_TYPED_INTEROP=0)"},
		{"env wins over legacy", map[string]string{"GOCPP_INTEROP": "opaque", "GOCPP_TYPED_INTEROP": "1"}, nil, "interop: opaque mode (GOCPP_INTEROP=opaque)"},
		{"flag wins over legacy", map[string]string{"GOCPP_TYPED_INTEROP": "1"}, []string{"-interop=opaque"}, "interop: opaque mode (-interop=opaque)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got, r := note(t, append(tt.args, "a.gp")...)
			if r.code != 0 || got != tt.want {
				t.Fatalf("exit %d, note %q, want %q\n%s", r.code, got, tt.want, r.stderr)
			}
		})
	}
	t.Run("default is typed with clang++", func(t *testing.T) {
		got, r := note(t, "a.gp")
		const want = "interop: typed mode (the default mode)"
		if r.code != 0 || got != want {
			t.Fatalf("exit %d, note %q, want %q\n%s", r.code, got, want, r.stderr)
		}
	})
	t.Run("bad flag", func(t *testing.T) {
		r := gocppRun("", false, "check", "-interop=fast", "a.gp")
		if r.code != 2 || !strings.Contains(r.stderr, `invalid value "fast" for flag -interop: want typed, opaque or auto`) {
			t.Fatalf("exit %d\n%s", r.code, r.stderr)
		}
	})
	t.Run("bad env", func(t *testing.T) {
		t.Setenv("GOCPP_INTEROP", "fast")
		r := gocppRun("", false, "check", "a.gp")
		if r.code != 2 || r.stderr != "gocpp: GOCPP_INTEROP=fast: want typed, opaque or auto\n" {
			t.Fatalf("exit %d\n%s", r.code, r.stderr)
		}
		// The flag hides a bad variable, like every flag of gocpp.
		if r := gocppRun("", false, "check", "-interop=opaque", "a.gp"); r.code != 0 {
			t.Fatalf("exit %d\n%s", r.code, r.stderr)
		}
	})
}

// TestInteropFlagEverywhere: every command that loads packages takes -interop, and a bad value is a
// usage error (status 2) for each.
func TestInteropFlagEverywhere(t *testing.T) {
	interopSetup(t, map[string]string{"lib.hpp": interopHdr, "a.gp": interopOK})
	for _, cmd := range [][]string{{"check"}, {"build"}, {"run"}, {"emit"}, {"vet"}, {"expand"}, {"test"}, {"debug"}, {"export", "cmake"}} {
		t.Run(strings.Join(cmd, "_"), func(t *testing.T) {
			r := gocppRun("", false, append(append([]string{}, cmd...), "-interop=fast", "a.gp")...)
			if r.code != 2 || !strings.Contains(r.stderr, `invalid value "fast" for flag -interop`) {
				t.Fatalf("exit %d\n%s", r.code, r.stderr)
			}
			r = gocppRun("", false, append(append([]string{}, cmd...), "-h")...)
			if !strings.Contains(r.stderr, "-interop") {
				t.Fatalf("the help of %v does not list -interop:\n%s", cmd, r.stderr)
			}
		})
	}
}

// TestInteropNoClang: without clang++, typed is an error (status 3) in every command that checks
// packages, auto and opaque go on in opaque mode, and a program without C/C++ imports never asks.
func TestInteropNoClang(t *testing.T) {
	interopSetup(t, map[string]string{"lib.hpp": interopHdr, "a.gp": interopOK, "go.gp": interopGoOnly})
	t.Setenv("GOCPP_CLANG", "/nonexistent/clang++")
	out := filepath.Join(t.TempDir(), "x")
	commands := map[string][]string{
		"check":  {"check"},
		"build":  {"build", "-o", out},
		"run":    {"run"},
		"emit":   {"emit"},
		"vet":    {"vet"},
		"expand": {"expand"},
	}
	for name, argv := range commands {
		t.Run(name+"/typed", func(t *testing.T) {
			r := gocppRun("", false, append(append([]string{}, argv...), "-interop=typed", "a.gp")...)
			if r.code != exitNoClang {
				t.Fatalf("exit %d, want %d\n%s", r.code, exitNoClang, r.stderr)
			}
			for _, want := range []string{"gocpp: -interop=typed: typed interop needs clang++", "GOCPP_CLANG", "-interop=opaque"} {
				if !strings.Contains(r.stderr, want) {
					t.Errorf("stderr lacks %q:\n%s", want, r.stderr)
				}
			}
		})
	}
	t.Run("legacy variable is typed too", func(t *testing.T) {
		t.Setenv("GOCPP_TYPED_INTEROP", "1")
		if r := gocppRun("", false, "check", "a.gp"); r.code != exitNoClang || !strings.Contains(r.stderr, "GOCPP_TYPED_INTEROP=1: typed interop needs clang++") {
			t.Fatalf("exit %d\n%s", r.code, r.stderr)
		}
	})
	for _, mode := range []string{"auto", "opaque"} {
		t.Run("check/"+mode, func(t *testing.T) {
			r := gocppRun("", false, "check", "-interop="+mode, "a.gp")
			if r.code != 0 || r.stderr != "" || r.stdout != "" {
				t.Fatalf("exit %d: a program must check without clang++ and without a word\n%s%s", r.code, r.stdout, r.stderr)
			}
		})
	}
	t.Run("auto note at -v", func(t *testing.T) {
		got, r := note(t, "-interop=auto", "a.gp")
		const note = "interop: no usable clang++ (not found, or it does not work), using opaque mode"
		if r.code != 0 || !strings.HasPrefix(got, note) || strings.Count(r.stderr, "\n") != 1 {
			t.Fatalf("exit %d, note %q, stderr (want one line):\n%s", r.code, got, r.stderr)
		}
	})
	t.Run("default is opaque without a word", func(t *testing.T) {
		// Nothing is set (interopSetup clears the variables): the default mode is auto.
		r := gocppRun("", false, "check", "a.gp")
		if r.code != 0 || r.stderr != "" || r.stdout != "" {
			t.Fatalf("exit %d: the default mode must check without clang++ and without a word\n%s%s", r.code, r.stdout, r.stderr)
		}
		got, r := note(t, "a.gp")
		const note = "interop: no usable clang++ (not found, or it does not work), using opaque mode"
		if r.code != 0 || !strings.HasPrefix(got, note) || !strings.Contains(got, "(the default mode;") {
			t.Fatalf("exit %d, note %q\n%s", r.code, got, r.stderr)
		}
	})
	t.Run("run and build go on", func(t *testing.T) {
		t.Setenv("GOCPP_INTEROP", "auto")
		if r := gocppRun("", false, "build", "-o", out, "a.gp"); r.code != 0 || r.stderr != "" {
			t.Fatalf("build: exit %d\n%s", r.code, r.stderr)
		}
		if r := gocppRun("", false, "emit", "a.gp"); r.code != 0 || r.stderr != "" {
			t.Fatalf("emit: exit %d\n%s", r.code, r.stderr)
		}
	})
	t.Run("nothing to probe", func(t *testing.T) {
		if r := gocppRun("", false, "check", "-interop=typed", "go.gp"); r.code != 0 || r.stderr != "" {
			t.Fatalf("a program of Go alone needs no clang++: exit %d\n%s", r.code, r.stderr)
		}
	})
}

// otherTarget returns the -os and -arch flags of a target that the probes cannot see the headers of
// (typedinterop.ProbesTarget is false): not this machine's, and not windows/amd64, which is probed
// from every system (Phase 14 step 14.6).
func otherTarget() []string {
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		return []string{"-os", "linux", "-arch", "arm64"}
	}
	return []string{"-os", "linux", "-arch", "amd64"}
}

// needMingwClang skips a test of the Windows target when this machine has no way to probe it:
// clang++ with the MinGW-w64 headers (the g++ of the cross build names them).
func needMingwClang(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	if _, err := exec.LookPath("x86_64-w64-mingw32-g++"); err != nil {
		t.Skip("no MinGW-w64 g++: clang++ has no headers for the Windows target")
	}
}

// TestInteropEarlyErrors: typed mode reports a mistake in a call of C++ code in gocpp check, in
// Go's words; opaque mode leaves it to the C++ compiler (gocpp check is clean, the build fails).
func TestInteropEarlyErrors(t *testing.T) {
	interopSetup(t, map[string]string{"lib.hpp": interopHdr, "a.gp": interopBad, "s.gp": interopBadStd})
	needClang(t)
	opaque := gocppRun("", false, "check", "-interop=opaque", "a.gp")
	if opaque.code != 0 || opaque.stderr != "" {
		t.Fatalf("opaque check: exit %d\n%s", opaque.code, opaque.stderr)
	}
	typed := gocppRun("", false, "check", "-interop=typed", "a.gp")
	if typed.code != 1 || !strings.Contains(typed.stderr, "too many arguments in call to lib.add") {
		t.Fatalf("typed check: exit %d\n%s", typed.code, typed.stderr)
	}
	// The build says it too, before the C++ compile (it never runs the C++ compiler).
	b := gocppRun("", false, "build", "-interop=typed", "-o", filepath.Join(t.TempDir(), "x"), "a.gp")
	if b.code != 1 || !strings.Contains(b.stderr, "too many arguments in call to lib.add") || strings.Contains(b.stderr, "C++") {
		t.Fatalf("typed build: exit %d\n%s", b.code, b.stderr)
	}
	// A build for another system than this one gets no early errors: the probes read this machine's headers.
	// (The Windows target is the exception: its probes read the MinGW-w64 headers, see below.)
	cross := otherTarget()
	if h := gocppRun("", false, "check", "-interop=typed", "s.gp"); h.code != 1 || !strings.Contains(h.stderr, "too many arguments in call to cmath.sqrt") {
		t.Fatalf("typed check of the library alone: exit %d\n%s", h.code, h.stderr)
	}
	t.Run("another target", func(t *testing.T) {
		t.Setenv("GOCPP_CXX", "g++") // names the compiler of the target (there is no cross compiler here)
		c := gocppRun("", false, append(append([]string{"check", "-interop=typed"}, cross...), "s.gp")...)
		if c.code != 0 || strings.Contains(c.stderr, "too many arguments") {
			t.Fatalf("typed check for %v: exit %d\n%s", cross, c.code, c.stderr)
		}
	})
	t.Run("the windows target is probed", func(t *testing.T) {
		needMingwClang(t)
		w := gocppRun("", false, "check", "-interop=typed", "-os", "windows", "s.gp")
		if w.code != 1 || !strings.Contains(w.stderr, "too many arguments in call to cmath.sqrt") {
			t.Fatalf("typed check for windows: exit %d\n%s", w.code, w.stderr)
		}
	})
	// With g++ as the build compiler the early errors are warnings: the C++ compiler decides.
	t.Setenv("GOCPP_CXX", "g++")
	w := gocppRun("", false, "check", "-interop=typed", "a.gp")
	if w.code != 0 || !strings.Contains(w.stderr, "warning: too many arguments") {
		t.Fatalf("typed check with g++: exit %d\n%s", w.code, w.stderr)
	}
}

// TestInteropUpToDate: the mode is in the key of the up-to-date check of gocpp build, so a build in
// another mode is not "up to date" (the flag is in the key; the variables are in the environment part).
func TestInteropUpToDate(t *testing.T) {
	interopSetup(t, map[string]string{"lib.hpp": interopHdr, "a.gp": interopOK})
	needClang(t)
	t.Setenv("GOCPP_NOOBJCACHE", "0")
	t.Setenv("GOCPP_NOUPTODATE", "")
	build := func(args ...string) result {
		t.Helper()
		r := gocppRun("", false, append(append([]string{"build", "-x"}, args...), "-o", "o", "a.gp")...)
		if strings.Contains(r.stderr, "WORK=") {
			workDir(t, r.stderr)
		}
		if r.code != 0 {
			t.Fatalf("build %v: exit %d\n%s", args, r.code, r.stderr)
		}
		return r
	}
	build("-interop=opaque")
	if r := build("-interop=opaque"); !isUpToDate(r.stderr) {
		t.Fatalf("the second build of the same mode is not up to date:\n%s", r.stderr)
	}
	if r := build("-interop=typed"); isUpToDate(r.stderr) {
		t.Fatalf("the build in another mode is up to date:\n%s", r.stderr)
	}
	if r := build("-interop=typed"); !isUpToDate(r.stderr) {
		t.Fatalf("the second typed build is not up to date:\n%s", r.stderr)
	}
	t.Setenv("GOCPP_INTEROP", "opaque")
	if r := build(); isUpToDate(r.stderr) {
		t.Fatalf("the variable changes the mode, the build can't be up to date:\n%s", r.stderr)
	}
}

// TestInteropUpToDateSeesClangAppear: the build of a program without headers does not run clang++,
// but its up-to-date key records the clang++ that a path lookup finds, so that a clang++ installed
// into the same directory (auto becomes typed) makes the next build run once and not be hidden by
// the old stamp.
func TestInteropUpToDateSeesClangAppear(t *testing.T) {
	interopSetup(t, map[string]string{"a.gp": "package main\n\nfunc main() {}\n"})
	t.Setenv("GOCPP_NOOBJCACHE", "0")
	t.Setenv("GOCPP_NOUPTODATE", "")
	t.Setenv("GOCPP_INTEROP", "")
	t.Setenv("GOCPP_TYPED_INTEROP", "")
	bin := t.TempDir()
	for _, tool := range []string{"g++", "gcc", "c++", "cc", "ar", "ld", "as"} {
		if path, err := exec.LookPath(tool); err == nil {
			if err := os.Symlink(path, filepath.Join(bin, tool)); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("GOCPP_CXX", "g++")
	if _, err := exec.LookPath("clang++"); err == nil {
		t.Fatal("the test directory must not have clang++ yet")
	}
	build := func() result {
		t.Helper()
		r := gocppRun("", false, "build", "-x", "-o", "o", "a.gp")
		if strings.Contains(r.stderr, "WORK=") {
			workDir(t, r.stderr)
		}
		if r.code != 0 {
			t.Fatalf("build: exit %d\n%s", r.code, r.stderr)
		}
		return r
	}
	build()
	if r := build(); !isUpToDate(r.stderr) {
		t.Fatalf("the second build without clang++ is not up to date:\n%s", r.stderr)
	}
	stub := filepath.Join(bin, "clang++")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if r := build(); isUpToDate(r.stderr) {
		t.Fatalf("a clang++ appeared and the build is still up to date:\n%s", r.stderr)
	}
	if r := build(); !isUpToDate(r.stderr) {
		t.Fatalf("the build after the clang++ appeared is not up to date again:\n%s", r.stderr)
	}
}

// TestInteropStampParts: the up-to-date key follows the mode that applies and, unless it is opaque,
// the clang++ that a path lookup finds, so that installing clang++ later (auto becomes typed) is
// not hidden by an old stamp.
func TestInteropStampParts(t *testing.T) {
	t.Setenv("GOCPP_INTEROP", "")
	t.Setenv("GOCPP_TYPED_INTEROP", "")
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	without := interopStampParts("", "linux", "amd64")
	if want := []string{"interop=auto", "clang="}; !slices.Equal(without, want) {
		t.Fatalf("without clang++ the parts are %q, want %q", without, want)
	}
	clang := filepath.Join(bin, "clang++")
	if err := os.WriteFile(clang, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	with := interopStampParts("", "linux", "amd64")
	if want := []string{"interop=auto", "clang=" + clang}; !slices.Equal(with, want) {
		t.Fatalf("with clang++ the parts are %q, want %q", with, want)
	}
	if got := interopStampParts("opaque", "linux", "amd64"); !slices.Equal(got, []string{"interop=opaque"}) {
		t.Fatalf("for opaque the parts are %q, want only the mode", got)
	}
	t.Setenv("GOCPP_INTEROP", "opaque")
	if got := interopStampParts("", "linux", "amd64"); !slices.Equal(got, []string{"interop=opaque"}) {
		t.Fatalf("with GOCPP_INTEROP=opaque the parts are %q, want only the mode", got)
	}
}

// TestInteropCrossEmit: a build for another system than this one is typed only for the headers of the C++
// library (the probes read this machine's headers, not the target's): a header of the program's own gives the
// opaque C++ for the target, and the library's headers alone the typed C++.
func TestInteropCrossEmit(t *testing.T) {
	const hdr = "#pragma once\nstruct Node { int v; };\n"
	const own = `package main

import "./lib.hpp"

func main() {
	var n lib.Node
	p := &n
	p.v = 4
}
`
	const std = `package main

import "<vector>"

func main() {
	v := std.vector[int]{1}
	v.at(0) = 5
}
`
	interopSetup(t, map[string]string{"lib.hpp": hdr, "own.gp": own, "std.gp": std})
	needClang(t)
	cross := otherTarget()
	t.Setenv("GOCPP_CXX", "g++") // names the compiler of the other target (there is no cross compiler here)
	emit := func(mode, file string, args ...string) result {
		t.Helper()
		r := gocppRun("", false, append(append([]string{"emit", "-interop=" + mode}, args...), file)...)
		if r.code != 0 {
			t.Fatalf("emit %s %s %v: exit %d\n%s", mode, file, args, r.code, r.stderr)
		}
		return r
	}
	if typed := emit("typed", "own.gp"); !strings.Contains(typed.stdout, "gp::nonnull(p)->v = 4;") {
		t.Fatalf("typed emit of a program with its own header, for this system:\n%s", typed.stdout)
	}
	x, o := emit("typed", "own.gp", cross...), emit("opaque", "own.gp", cross...)
	if x.stdout != o.stdout || strings.Contains(x.stdout, "gp::nonnull") {
		t.Fatalf("typed emit for %v with a header of its own is not opaque's:\n%s", cross, x.stdout)
	}
	// The windows target is probed (MinGW-w64's headers, LLP64): its typed emit is this system's.
	t.Run("windows", func(t *testing.T) {
		needMingwClang(t)
		t.Setenv("GOCPP_CXX", "") // the MinGW-w64 g++ of the cross build
		if w := emit("typed", "own.gp", "-os", "windows"); !strings.Contains(w.stdout, "gp::nonnull(p)->v = 4;") {
			t.Fatalf("typed emit for windows with a header of its own:\n%s", w.stdout)
		}
	})
	// opaque mode rejects v.at(0) = 5; the library's headers alone are typed for the other system too.
	if r := gocppRun("", false, append([]string{"emit", "-interop=typed"}, append(cross, "std.gp")...)...); r.code != 0 {
		t.Fatalf("typed emit of the library alone for %v: exit %d\n%s", cross, r.code, r.stderr)
	}
	if r := gocppRun("", false, append([]string{"emit", "-interop=opaque"}, append(cross, "std.gp")...)...); r.code == 0 {
		t.Fatalf("opaque mode accepts v.at(0) = 5: the test shows nothing\n%s", r.stdout)
	}
}
