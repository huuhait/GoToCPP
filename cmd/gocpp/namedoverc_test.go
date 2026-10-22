package main

import (
	"strings"
	"testing"
)

// A named type declared over a C/C++ type is not written yet (a later step of Phase 04): the
// build says so at the declaration, for the array, slice, map, channel, pointer and func types as
// for `type Count C.int`. `gocpp check` accepts them (they are Go types with C/C++ parts, see
// docs/spec/interop.md, "Bugs and gaps that are tracked"); a struct with C/C++ fields, and a
// container of such a struct, are written.
func TestNamedTypeOverCBuild(t *testing.T) {
	const unsupported = "./m.gp:3:6: internal error: not supported yet: named types declared over C/C++ types (T, a later step of Phase 04)"
	for _, c := range []struct{ name, decl, want string }{
		{"integer", "type T C.int", unsupported},
		{"array", "type T [2]C.int", unsupported},
		{"array of arrays", "type T [2][2]C.char", unsupported},
		{"slice", "type T []C.int", unsupported},
		{"map", "type T map[string]C.int", unsupported},
		{"map with a C key", "type T map[C.int]string", unsupported},
		{"pointer", "type T *C.int", unsupported},
		{"channel", "type T chan C.int", unsupported},
		{"func with a C parameter", "type T func(C.int)", unsupported},
		{"func with a C result", "type T func() C.int", unsupported},
		{"struct", "type T struct{ n C.int }", ""},
		{"slice of a struct with a C field", "type S struct{ n C.int }\n\ntype T []S", ""},
		{"slice of Go types", "type T []int", ""},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			forEachCXX(t, func(t *testing.T, cxx testCXX) {
				t.Setenv("GOCPP_GATE_PHASE", "15") // channels are a later phase
				typedEnv(t)
				dir := inDir(t, map[string]string{"m.gp": "package main\n\n" + c.decl + "\n\nfunc main() {\n\tvar t T\n\t_ = t\n}\n"})
				checked := gocppRun("", true, "check", "m.gp")
				if checked.code != 0 {
					t.Errorf("check:\n%s\nwant no error", checked.stdout)
				}
				built := gocppRun("", true, "build", "-o", dir+"/out", "m.gp")
				if c.want == "" {
					if built.code != 0 {
						t.Errorf("build:\n%s\nwant no error", built.stdout)
					}
					return
				}
				lines := strings.Split(strings.TrimSpace(built.stdout), "\n")
				if built.code != 2 || len(lines) < 2 || lines[1] != c.want {
					t.Errorf("build exit %d:\n%s\nwant the line %q", built.code, built.stdout, c.want)
				}
			})
		})
	}
}
