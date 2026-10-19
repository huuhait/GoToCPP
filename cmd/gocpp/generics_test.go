package main

import (
	"os/exec"
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

// TestConceptsFlag checks -gp:concepts of gocpp emit, build and run (Phase 07 step 7.9): the
// package header gets the concepts of the constraints with the flag and none without it, the flag
// is in the usage, and a program built with it prints what it prints without it.
func TestConceptsFlag(t *testing.T) {
	t.Setenv(gatePhaseEnv, "07")
	const src = "package main\n\nimport \"fmt\"\n\ntype Num interface{ ~int | ~float64 }\n\n" +
		"func Sum[T Num](xs ...T) (s T) {\n\tfor _, x := range xs {\n\t\ts += x\n\t}\n\treturn\n}\n\n" +
		"func main() { fmt.Println(Sum(1, 2, 3), Sum(1.5, 2)) }\n"
	tree := map[string]string{"main.go": src}
	off := checkIn(t, tree, "emit", "main.gp")
	on := checkIn(t, tree, "emit", "-gp:concepts", "main.gp")
	if off.code != 0 || on.code != 0 {
		t.Fatalf("emit: %s\n%s", show(off), show(on))
	}
	for _, w := range []string{"concept gp_concept_Num", "concept gp_sat_Sum", "#include <gocpp/concepts.hpp>", "::gp::concepts::tilde<gp_T, int64_t>"} {
		if !strings.Contains(on.stdout, w) {
			t.Errorf("emit -gp:concepts has no %q", w)
		}
		if strings.Contains(off.stdout, w) {
			t.Errorf("emit without -gp:concepts has %q", w)
		}
	}
	if r := gocppRun("", false, "build", "-h"); !strings.Contains(r.stdout+r.stderr, "-gp:concepts") {
		t.Errorf("gocpp build -h does not describe -gp:concepts:\n%s", r.stdout+r.stderr)
	}
	anyCXX(t)
	inDir(t, map[string]string{"main.gp": src})
	for _, cmd := range []string{"run", "build"} {
		args := []string{cmd, "-gp:concepts"}
		if cmd == "build" {
			args = append(args, "-o", "prog"+exeSuffix)
		}
		r := gocppRun("", false, append(args, "main.gp")...)
		if r.code != 0 {
			t.Fatalf("gocpp %s -gp:concepts main.gp = %s", cmd, show(r))
		}
		if cmd == "run" && r.stdout != "6 3.5\n" {
			t.Errorf("gocpp run -gp:concepts printed %q, want %q", r.stdout, "6 3.5\n")
		}
	}
	out, err := exec.Command("./prog" + exeSuffix).CombinedOutput()
	if err != nil || string(out) != "6 3.5\n" {
		t.Errorf("the program built with -gp:concepts: %v, printed %q", err, out)
	}
}
