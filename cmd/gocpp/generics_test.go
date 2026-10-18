package main

import (
	"strings"
	"testing"
)

// TestGenericsEmit checks gocpp emit on programs with generic functions, with the gates open to
// Phase 07 (a generic function is built; generic types, type declarations inside generic functions
// and the stubs of the standard packages cmp, slices, maps and iter are user errors).
func TestGenericsEmit(t *testing.T) {
	t.Setenv(gatePhaseEnv, "07")
	tests := []struct {
		name, src string
		code      int
		want      []string // in stdout (exit 0) or stderr
	}{
		{"function", "package main\n\nfunc Id[T any](x T) T { return x }\n\nfunc main() { println(Id(1), Id(\"s\")) }\n", 0,
			[]string{"gp_inst_", "_Id(int64_t x)", "_Id(gp::string x)", "// Id[int]", "// Id[string]"}},
		{"stub", "package main\n\nimport \"cmp\"\n\nfunc main() { println(cmp.Compare(1, 2)) }\n", 1,
			[]string{"main.go:5:23: cmp.Compare is not available yet (Phase 07)"}},
		{"stub slices", "package main\n\nimport \"slices\"\n\nfunc main() { println(slices.Contains([]int{1}, 1)) }\n", 1,
			[]string{"main.go:5:23: slices.Contains is not available yet (Phase 07)"}},
		{"type", "package main\n\ntype Box[T any] struct{ v T }\n\nfunc main() { var b Box[int]; println(b.v) }\n", 1,
			[]string{"main.go:3:6: generic types are not supported yet (Phase 07)"}},
		{"local type", "package main\n\nfunc F[T any](v T) any {\n\ttype box struct{ v T }\n\treturn box{v}\n}\n\nfunc main() { println(F(1) != nil) }\n", 1,
			[]string{"main.go:4:2: type declarations inside generic functions are not supported yet (Phase 07)"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := checkIn(t, map[string]string{"main.go": tc.src}, "emit", "main.gp")
			out := r.stdout
			if tc.code != 0 {
				out = r.stderr
			}
			if r.code != tc.code {
				t.Errorf("exit %d, want %d:\n%s", r.code, tc.code, show(r))
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("no %q in\n%s", w, show(r))
				}
			}
		})
	}
}

// TestGenericsCxxOtherPackage checks that a generic function of another package that uses C/C++
// names is a user error where the importer instantiates it (the body is lowered with the importer's
// C/C++ state), while its other generic functions are fine.
func TestGenericsCxxOtherPackage(t *testing.T) {
	t.Setenv(gatePhaseEnv, "07")
	tree := map[string]string{
		"gocpp.mod":  "module example.com/gcx\n",
		"lib/lib.go": "package lib\n\nimport \"<string>\"\n\nfunc Len[T any](xs ...T) int { s := std.string(\"abc\"); return len(xs) + int(s.size()) }\n\nfunc Plain[T any](x T) T { return x }\n",
		"main.go":    "package main\n\nimport (\n\t\"example.com/gcx/lib\"\n)\n\nfunc main() { println(lib.Plain(1), lib.Len(\"a\")) }\n",
	}
	r := checkIn(t, tree, "emit", ".")
	want := "./main.go:7:37: lib.Len uses C/C++ names, so it can only be instantiated in its own package (Phase 12)"
	if r.code != 1 || !strings.Contains(r.stderr, want) {
		t.Errorf("exit %d, want 1 and %q:\n%s", r.code, want, show(r))
	}
}
