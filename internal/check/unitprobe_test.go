package check

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// TestHeaderEndsInADeclaration checks a header that ends in the middle of a declaration: the
// build includes it in front of other code, and fails (C++: expected ';'), so the check is
// an error at the import too. The C check probe takes `int f(void)` for a valid C header, and
// the probe of the namespaces used to end it with a ; of its own.
func TestHeaderEndsInADeclaration(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		for _, c := range []struct{ name, header, want string }{
			{"function declarator", "#pragma once\nint cut(void)", `header "./cut.h" ends inside a declaration or a block that it doesn't close`},
			{"struct", "#pragma once\nstruct Cut { int x; }", `header "./cut.h" doesn't compile: cut.h:2:`},
			{"open block", "#pragma once\nnamespace n {\nint x;\n", `header "./cut.h" ends inside a declaration or a block that it doesn't close`},
			{"complete", "#pragma once\nint cut(void);\n", ""},
		} {
			got := probeErrors(t, probeConfig(cxx, LastPhase), map[string]string{
				"main.gp": "package main\n\nimport \"./cut.h\"\n\nfunc main() {}\n",
				"cut.h":   c.header,
			})
			want := ""
			if c.want != "" {
				want = "# example.com/m\n./main.gp:3:8: " + c.want
			}
			if (want == "" && got != "") || (want != "" && !strings.HasPrefix(got, want)) || strings.Count(got, "\n") > 2 {
				t.Errorf("%s: errors:\n%s\nwant %q", c.name, got, want)
			}
		}
	})
}

// TestHeaderUnitProbe checks the probe of all the headers of a package together: two files import
// headers that are fine alone and conflict with each other, as in every translation unit of the
// build (a C header and a C++ header declare f). The error is at the import of the second header,
// and it is the only one: a header that fails alone is reported by its own probe.
func TestHeaderUnitProbe(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		files := func(extra map[string]string) map[string]string {
			m := map[string]string{
				"a.gp":    "package main\n\nimport \"./a.h\"\n\nfunc A() { println(a.f(1)) }\n",
				"b.gp":    "package main\n\nimport \"./b.h\"\n\nfunc B() { println(b.f(2)) }\n",
				"main.gp": "package main\n\nfunc main() { A(); B() }\n",
				"a.h":     "#pragma once\n#include <string>\nint f(int x);\n",
				"b.h":     "int f(int x);\n",
			}
			for name, src := range extra {
				m[name] = src
			}
			return m
		}
		got := probeErrors(t, probeConfig(cxx, LastPhase), files(nil))
		// The text of the compiler differs: g++ says "conflicting declaration of 'int f(int)' with
		// 'C' linkage", clang++ "declaration of 'f' has a different language linkage".
		const prefix = "# example.com/m\n./b.gp:3:8: header \"./b.h\" doesn't compile with the headers of the other files of the package: b.h:1:5: "
		if !strings.HasPrefix(got, prefix) || !strings.Contains(got, "linkage") || strings.Count(got, "\n") != 2 {
			t.Errorf("conflicting headers: errors:\n%s\nwant one error starting %q, about the linkage", got, prefix)
		}

		// Headers that agree: no error, and the same for one file with both.
		agree := map[string]string{"b.h": "#pragma once\n#include <string>\nint g(int x);\n"}
		if got := probeErrors(t, probeConfig(cxx, LastPhase), files(agree)); got != "" {
			t.Errorf("headers that agree: errors:\n%s", got)
		}

		// A header that does not compile at all is the error of its own probe, and not also of the unit.
		bad := map[string]string{"b.h": "#pragma once\nint g(int x) { return nosuchname; }\n"}
		got = probeErrors(t, probeConfig(cxx, LastPhase), files(bad))
		if !strings.HasPrefix(got, "# example.com/m\n./b.gp:3:8: header \"./b.h\" doesn't compile: b.h:2:") ||
			strings.Count(got, "\n") != 2 {
			t.Errorf("header with an error: errors:\n%s\nwant one error of the header alone", got)
		}

		// A header that fails alone is that header's error, once.
		broken := map[string]string{"b.h": "#pragma once\nint g(int x)\n"}
		got = probeErrors(t, probeConfig(cxx, LastPhase), files(broken))
		want := `# example.com/m
./b.gp:3:8: header "./b.h" ends inside a declaration or a block that it doesn't close
`
		if got != want {
			t.Errorf("broken header: errors:\n%s\nwant:\n%s", got, want)
		}
	})
}

// TestHeaderUnitProbeRuns checks how often the unit is probed: not for a package whose files
// import the same headers (their set is the unit), once for files with different sets.
func TestHeaderUnitProbeRuns(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		count := func(files map[string]string) []probeCall {
			rec := new(probeRecorder)
			cfg := probeConfig(cxx, LastPhase)
			cfg.Probe = rec.probe
			probeErrors(t, cfg, files)
			return rec.calls
		}
		same := count(map[string]string{
			"a.gp": "package main\n\nimport \"<vector>\"\n\nvar a = std.vector[int]{}\n",
			"b.gp": "package main\n\nimport \"<vector>\"\n\nvar b = std.vector[int]{}\n\nfunc main() {}\n",
		})
		if len(same) != 1 {
			t.Errorf("the same header in both files: %d probes %q, want 1", len(same), same)
		}
		one := count(map[string]string{
			"a.gp": "package main\n\nimport \"<vector>\"\n\nvar a = std.vector[int]{}\n\nfunc main() {}\n",
		})
		if len(one) != 1 {
			t.Errorf("one file: %d probes %q, want 1", len(one), one)
		}
		different := count(map[string]string{
			"a.gp": "package main\n\nimport \"<vector>\"\n\nvar a = std.vector[int]{}\n",
			"b.gp": "package main\n\nimport \"<map>\"\n\nvar b = std.map[int, int]{}\n\nfunc main() {}\n",
		})
		if len(different) != 3 {
			t.Errorf("different headers: %d probes %q, want 3 (two sets and the unit)", len(different), different)
		}
		var unit int
		for _, c := range different {
			if len(c.includes) == 2 && len(c.candidates) == 0 {
				unit++
			}
		}
		if unit != 1 {
			t.Errorf("different headers: %d probes of both headers without candidates in %q, want 1", unit, different)
		}
	})
}
