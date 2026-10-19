package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The promoted members of embedded C++ classes (Phase 12 step 12.9, ADR-098).

const promotedHdr = `#pragma once
#include <string>
struct Counter { int n = 0; int inc() { return ++n; } int get() const { return n; } };
struct Named { std::string name; void rename(const char* s) { name = s; } };
`

// TestTypedEmitPromoted pins what typed code generation writes for a member that a Go struct takes from
// an embedded class: the C++ type of the embedded field decides `.` or `->` (no gp::deref), and opaque
// mode, the default, writes what it always did, for a program that it accepts.
func TestTypedEmitPromoted(t *testing.T) {
	const src = `package main

import (
	"<vector>"
	"./h.hpp"
)

type Inner struct {
	std.vector[int]
	k int
}

type Outer struct {
	*Inner
	h.Named
}

type Ptr struct{ *h.Counter }

func main() {
	var i Inner
	i.push_back(1)
	i.at(0) = 5
	n := i.size()
	_ = n
	o := Outer{Inner: &Inner{}}
	o.push_back(2)
	o.rename("x")
	q := &o.at(0)
	_ = q
	var c Ptr
	c.Counter = &h.Counter{}
	c.inc()
	var s Inner
	m := s.at(0) + 1
	_ = m
}
`
	typedEnv(t)
	inDir(t, map[string]string{"h.hpp": promotedHdr, "a.gp": src})
	needClang(t)
	t.Setenv("GOCPP_CXX", "g++")

	t.Setenv("GOCPP_TYPED_INTEROP", "1")
	typed := gocppRun("", false, "emit", "a.gp")
	if typed.code != 0 {
		t.Fatalf("typed emit: exit %d\n%s", typed.code, typed.stderr)
	}
	for _, want := range []string{
		"i.vector.push_back(1);",
		"i.vector.at(0) = 5;",
		"o.Inner->vector.push_back(2);",
		"o.Named.rename(\"x\");",
		"auto q = &o.Inner->vector.at(0);",
		"gp::nonnull(c.Counter)->inc();",
		"s.vector.at(0)",
	} {
		if !strings.Contains(typed.stdout, want) {
			t.Errorf("typed emit has no %q", want)
		}
	}
	if strings.Contains(typed.stdout, "gp::deref") {
		t.Errorf("typed emit still has gp::deref:\n%s", typed.stdout)
	}

	// Opaque mode accepts the program with one class per struct, and writes gp::deref.
	t.Setenv("GOCPP_TYPED_INTEROP", "")
	op := gocppRun("", false, "emit", "a.gp")
	if op.code == 0 {
		t.Fatalf("opaque emit of a struct with two classes succeeded:\n%s", op.stdout)
	}
	if !strings.Contains(op.stderr, "promoted fields and methods of several embedded C/C++ types are not supported yet (Phase 12)") {
		t.Errorf("opaque mode:\n%s", op.stderr)
	}
	inDir(t, map[string]string{"h.hpp": promotedHdr, "b.gp": `package main

import "<vector>"

type Inner struct {
	std.vector[int]
	k int
}

func main() {
	var i Inner
	i.push_back(1)
}
`})
	one := gocppRun("", false, "emit", "b.gp")
	if one.code != 0 || !strings.Contains(one.stdout, "gp::deref(i.vector).push_back(1);") || strings.Contains(one.stdout, "gp::nonnull") {
		t.Errorf("opaque emit changed (exit %d):\n%s%s", one.code, one.stdout, one.stderr)
	}
}

// TestTypedCheckPromoted runs the hidden command that prints the checker's errors on the programs of
// the second pass: the errors are at the selector, in the words of Go, and the program that is fine
// has none. -v lists no silent expression for a promoted call.
func TestTypedCheckPromoted(t *testing.T) {
	typedEnv(t)
	inDir(t, map[string]string{"h.hpp": promotedHdr, "a.gp": `package main

import (
	"<vector>"
	"./h.hpp"
)

type S struct {
	std.vector[int]
	h.Counter
	h.Named
}

func main() {
	var s S
	s.push_back(1)
	s.inc()
	s.rename("a")
	_ = s.get()
	_ = s.size()
	_ = s.nosuch
}
`, "bad.gp": `package main

import (
	"<vector>"
	"./h.hpp"
)

type S struct {
	std.vector[int]
	h.Counter
}

func main() {
	var s S
	s.push_back("a")
	s.inc(1)
}
`, "ok.gp": `package main

import (
	"<vector>"
	"./h.hpp"
)

type S struct {
	std.vector[int]
	h.Counter
}

func main() {
	var s S
	s.push_back(1)
	s.inc()
	_ = s.get()
	_ = s.size()
}
`})
	needClang(t)
	r := gocppRun("", false, "__typed-check", "a.gp")
	if r.code != 1 {
		t.Fatalf("exit %d\n%s", r.code, r.stderr)
	}
	for _, want := range []string{
		"./a.gp:21:8: s.nosuch undefined (type S has no field or method nosuch; none of the embedded C++ types std.vector[int], Counter, Named has a member nosuch)",
	} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("no %q in\n%s", want, r.stderr)
		}
	}
	// With the names decided, the typed checker's own errors follow for the uses: the arguments.
	if bad := gocppRun("", false, "__typed-check", "-build", "clang++", "bad.gp"); bad.code != 1 ||
		!strings.Contains(bad.stderr, "./bad.gp:15:2: no overload of s.push_back matches the arguments (untyped string)") ||
		!strings.Contains(bad.stderr, "./bad.gp:16:2: too many arguments in call to s.inc (have 1, want 0)") {
		t.Errorf("bad.gp: exit %d\n%s", bad.code, bad.stderr)
	}
	if strings.Contains(r.stderr, "not supported yet") || strings.Contains(r.stderr, "ambiguous") {
		t.Errorf("a gate or an ambiguity where the classes have the member:\n%s", r.stderr)
	}
	if ok := gocppRun("", false, "__typed-check", "-v", "ok.gp"); ok.code != 0 || strings.Contains(ok.stderr, "# silent") {
		t.Errorf("ok.gp: exit %d\n%s", ok.code, ok.stderr)
	}
}

// TestTypedEmbedInterop runs the programs of tests/interop that embed C/C++ types in typed mode and in
// opaque mode: they print the same (the compatibility rule: typed mode only refines what opaque mode does).
func TestTypedEmbedInterop(t *testing.T) {
	root := filepath.Join(typedDir, "..", "interop")
	forEachCXX(t, func(t *testing.T, c testCXX) {
		typedEnv(t)
		files := map[string]string{}
		for _, name := range []string{"embed_cxx.h", "embed_cxx.gp", "embed_cxx_depth.gp", "embed_cxx_store.gp"} {
			b, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			files[name] = string(b)
		}
		inDir(t, files)
		needClang(t)
		for _, name := range []string{"embed_cxx.gp", "embed_cxx_depth.gp", "embed_cxx_store.gp"} {
			t.Run(strings.TrimSuffix(name, ".gp"), func(t *testing.T) {
				t.Setenv("GOCPP_TYPED_INTEROP", "")
				op := gocppRun("", false, "run", name)
				t.Setenv("GOCPP_TYPED_INTEROP", "1")
				ty := gocppRun("", false, "run", name)
				if ty.code != op.code || ty.stdout != op.stdout {
					t.Errorf("typed: exit %d\n%s\nopaque: exit %d\n%s\ntyped stderr:\n%s", ty.code, ty.stdout, op.code, op.stdout, ty.stderr)
				}
			})
		}
	})
}
