package typedinterop

import "testing"

// A header package whose name is also the namespace it declares (oop.hpp with
// namespace oop): the names are looked up in the namespace first, as the opaque
// build does, then at the global level.
func TestClangHeaderNamedLikeItsNamespace(t *testing.T) {
	const hdr = `#pragma once
#define LEVEL 3
namespace oop {
struct Vec { int x; double y; };
inline int scale(int n) { return n * 2; }
inline const int LEVEL_NS = 4;
}
inline double global_half(double d) { return d / 2; }
inline long scale_global(long n) { return n; }
`
	rep := analyze(t, "package main\n\nimport (\n\t\"fmt\"\n\t\"./oop.hpp\"\n)\n\nfunc main() {\n"+
		"\tr := oop.scale(4)\n\tvar v oop.Vec\n\tx := v.x\n\th := oop.global_half(3)\n\tl := oop.LEVEL\n\tfmt.Println(r, x, h, l)\n}\n",
		map[string]string{"oop.hpp": hdr}, Options{})
	if t.Failed() {
		return
	}
	expectResult(t, rep, "C.scale(4)", "prvalue int")
	expectResult(t, rep, "v.x", "lvalue int")
	expectResult(t, rep, "C.global_half(3)", "prvalue double")
	expectResult(t, rep, "C.LEVEL", "prvalue int = 3")
	expectResult(t, rep, "C.Vec", "type oop::Vec")
}
