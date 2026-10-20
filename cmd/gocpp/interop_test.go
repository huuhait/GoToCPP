package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
	t.Run("default", func(t *testing.T) {
		got, r := note(t, "a.gp")
		if r.code != 0 || !strings.HasPrefix(got, "interop: ") || !strings.Contains(got, "(the default mode)") {
			t.Fatalf("exit %d, note %q\n%s", r.code, got, r.stderr)
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
		if r.code != 0 || !strings.HasPrefix(got, "interop: clang++ not found, using opaque mode") || strings.Count(r.stderr, "\n") != 1 {
			t.Fatalf("exit %d, note %q, stderr (want one line):\n%s", r.code, got, r.stderr)
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
	cross := "windows"
	if runtime.GOOS == "windows" {
		cross = "linux"
	}
	if h := gocppRun("", false, "check", "-interop=typed", "s.gp"); h.code != 1 || !strings.Contains(h.stderr, "too many arguments in call to cmath.sqrt") {
		t.Fatalf("typed check of the library alone: exit %d\n%s", h.code, h.stderr)
	}
	c := gocppRun("", false, "check", "-interop=typed", "-os", cross, "s.gp")
	if c.code != 0 || strings.Contains(c.stderr, "too many arguments") {
		t.Fatalf("typed check for %s: exit %d\n%s", cross, c.code, c.stderr)
	}
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
	cross := "windows"
	if runtime.GOOS == "windows" {
		cross = "linux"
	}
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
	if x, o := emit("typed", "own.gp", "-os", cross), emit("opaque", "own.gp", "-os", cross); x.stdout != o.stdout || strings.Contains(x.stdout, "gp::nonnull") {
		t.Fatalf("typed emit for %s with a header of its own is not opaque's:\n%s", cross, x.stdout)
	}
	// opaque mode rejects v.at(0) = 5; the library's headers alone are typed for the other system too.
	if r := gocppRun("", false, "emit", "-interop=typed", "-os", cross, "std.gp"); r.code != 0 {
		t.Fatalf("typed emit of the library alone for %s: exit %d\n%s", cross, r.code, r.stderr)
	}
	if r := gocppRun("", false, "emit", "-interop=opaque", "-os", cross, "std.gp"); r.code == 0 {
		t.Fatalf("opaque mode accepts v.at(0) = 5: the test shows nothing\n%s", r.stdout)
	}
}
