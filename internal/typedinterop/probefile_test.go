package typedinterop

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// probePrograms are programs of tests/interop whose probe files are goldens:
// the text of the file that clang gets, with the kinds that the kind probe
// found (so the golden says the :: and . decisions too).
var probePrograms = []string{
	"cxx_containers", "cookbook_std_containers", "c_operators", "cmath_funcs", "stdio_calls", "string_ops",
	"map_ops", "template_or_index", "range_containers", "header_namespace", "pointer_chains", "macro_collisions",
}

func TestProbeFileGoldens(t *testing.T) {
	clang := tsupport.Clang(t)
	root, err := filepath.Abs(filepath.Join("..", "..", "tests", "interop"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range probePrograms {
		t.Run(name, func(t *testing.T) {
			p, fset := tsupport.CheckDir(t, root, name+".gp")
			if errs := p.Errors.Diagnostics(); len(errs) > 0 {
				t.Skipf("the program doesn't check here: %v", errs[0])
			}
			rep, err := Analyze(context.Background(), p, fset, Options{Clang: clang, Runner: new(probe.Runner)})
			if err != nil {
				t.Fatal(err)
			}
			got := rep.Table.Source.Text
			path := filepath.Join("testdata", "probefile", name+".cpp")
			if *update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run with -update)", err)
			}
			if string(want) != got {
				t.Errorf("the probe file changed (run with -update and review the diff)\n%s", firstDiff(string(want), got))
			}
		})
	}
}

func firstDiff(want, got string) string {
	w, g := splitLines(want), splitLines(got)
	for i := 0; i < len(w) || i < len(g); i++ {
		var a, b string
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			b = g[i]
		}
		if a != b {
			return "line " + itoa(i+1) + ":\n- " + a + "\n+ " + b
		}
	}
	return ""
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
