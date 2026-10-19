package main

import (
	"strings"
	"testing"
)

// TestTypedEmit pins what typed code generation writes (Phase 12 step 12.6, ADR-087), and that opaque
// mode, the default, writes what it always did, for the same program.
func TestTypedEmit(t *testing.T) {
	const hdr = `#pragma once
#include <memory>
#include <string>
struct Node { int v; Node* next; int get() const { return v; } };
struct Rect { int x, y, w, h; };
struct Counter { int n = 0; int inc() { return ++n; } };
void say(const char* s);
int take(const std::string& s);
`
	const src = `package main

import (
	"<cstring>"
	"<memory>"
	"<vector>"
	"./shapes.hpp"
)

func main() {
	var n shapes.Node
	p := &n
	p.v = 4
	_ = p.get()
	r := shapes.Rect{h: 480, x: 1, w: 640}
	_ = r
	up := std.make_unique[shapes.Counter]()
	up.inc()
	s := "hi" + "!"
	shapes.say(s)
	_ = shapes.take(s)
	var m int = cstring.strlen(s)
	_ = m
	v := std.vector[int]{1}
	v.at(0) = 5
	q := &v.at(0)
	_ = *q
}
`
	typedEnv(t)
	inDir(t, map[string]string{"shapes.hpp": hdr, "a.gp": src})
	needClang(t)
	t.Setenv("GOCPP_CXX", "g++")

	t.Setenv("GOCPP_TYPED_INTEROP", "1")
	typed := gocppRun("", false, "emit", "a.gp")
	if typed.code != 0 {
		t.Fatalf("typed emit: exit %d\n%s", typed.code, typed.stderr)
	}
	for _, want := range []string{
		"gp::nonnull(p)->v = 4;",
		"gp::nonnull(p)->get()",
		"::Rect{.x = 1, .w = 640, .h = 480}",
		"gp::smartnn(up)->inc()",
		"::say(gp::cstr(s).gp_ptr())",
		"::take(::std::string(s))",
		"gp::cgo::as<int64_t>(::strlen(gp::cstr(s).gp_ptr()))",
		"v.at(0) = 5;",
		"auto q = &v.at(0);",
		"*gp::nonnull(q)",
	} {
		if !strings.Contains(typed.stdout, want) {
			t.Errorf("typed emit has no %q", want)
		}
	}
	if strings.Contains(typed.stdout, "gp::deref") {
		t.Errorf("typed emit still has gp::deref:\n%s", typed.stdout)
	}

	// The opaque build rejects a.gp (the string arguments, the number, the assignment), so the default
	// is checked on a program that opaque mode accepts.
	t.Setenv("GOCPP_TYPED_INTEROP", "")
	inDir(t, map[string]string{"shapes.hpp": hdr, "b.gp": `package main

import "./shapes.hpp"

func main() {
	var n shapes.Node
	p := &n
	p.v = 4
	_ = p.get()
}
`})
	op := gocppRun("", false, "emit", "b.gp")
	if op.code != 0 || !strings.Contains(op.stdout, "gp::deref(p).v = 4;") || strings.Contains(op.stdout, "gp::nonnull") {
		t.Errorf("opaque emit changed (exit %d):\n%s%s", op.code, op.stdout, op.stderr)
	}
}
