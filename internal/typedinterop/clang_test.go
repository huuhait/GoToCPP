package typedinterop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

func TestMain(m *testing.M) { tsupport.Main(m) }

// runner is shared by the tests: it memoizes tables, as the real pipeline will.
var runner = new(probe.Runner)

// analyze writes the program (main.gp) and its files in a new folder, checks
// it with gocpp's own check, and runs the typed pipeline on it with clang.
func analyze(t testing.TB, src string, files map[string]string, opts Options) *Report {
	t.Helper()
	clang := tsupport.Clang(t)
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "main.gp"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p, fset := tsupport.CheckDir(t, dir, "main.gp")
	if errs := p.Errors.Diagnostics(); len(errs) > 0 {
		t.Fatalf("the program doesn't check: %v\n%s", errs, src)
	}
	opts.Clang = clang
	if opts.Runner == nil {
		opts.Runner = runner
	}
	rep, err := Analyze(context.Background(), p, fset, opts)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	return rep
}

// describe is the one-line form of a result that the tests compare:
//
//	"prvalue unsigned long"   an expression: its category and type without the reference
//	"type std::vector<long>"  a type
//	"error: message"          a failed probe
//	"skipped"                 not probed
//
// followed by " = value" for a constant.
func describe(r *probe.Result) string {
	var s string
	switch {
	case r.Skipped != "":
		return "skipped"
	case !r.OK:
		return "error: " + r.Error
	case r.Form == probe.FormType:
		s = "type " + r.Type
	default:
		s = string(r.Category) + " " + r.Base
	}
	if r.Value != nil {
		s += " = " + r.Value.String()
	}
	return s
}

// find returns the result of the reference with a site whose text is text
// (a name of a local variable, "local:x", for the reference of its type).
func find(t testing.TB, rep *Report, text string) *probe.Result {
	t.Helper()
	var hits []*collect.Ref
	for _, r := range rep.Set.Refs {
		if !r.Standalone && r.Incomplete == "" && !strings.HasPrefix(text, "name:") {
			continue
		}
		want := strings.TrimPrefix(strings.TrimPrefix(text, "local:"), "name:")
		for _, s := range r.Sites {
			isLocal := r.Kind == collect.KindLocal
			if s.Text == want && isLocal == strings.HasPrefix(text, "local:") {
				hits = append(hits, r)
				break
			}
		}
	}
	if len(hits) != 1 {
		var keys []string
		for _, h := range hits {
			keys = append(keys, h.Key)
		}
		var all []string
		for _, r := range rep.Set.Refs {
			if r.Standalone {
				for _, s := range r.Sites {
					all = append(all, fmt.Sprintf("%q(%s)", s.Text, r.Kind))
				}
			}
		}
		t.Fatalf("%d references have the site %q: %v\nall: %s", len(hits), text, keys, strings.Join(all, " "))
	}
	return rep.Table.Result(hits[0].ID)
}

// expectResult compares the result for the site text with want: exact, or a prefix
// when want ends with "..." (a message with the compiler's own text).
func expectResult(t testing.TB, rep *Report, text, want string) {
	t.Helper()
	got := describe(find(t, rep, text))
	if strings.HasSuffix(want, "...") {
		if !strings.HasPrefix(got, strings.TrimSuffix(want, "...")) {
			t.Errorf("%s: got %q, want prefix %q", text, got, want)
		}
		return
	}
	if strings.HasPrefix(want, "~") {
		if !strings.Contains(got, want[1:]) {
			t.Errorf("%s: got %q, want it to contain %q", text, got, want[1:])
		}
		return
	}
	if got != want {
		t.Errorf("%s: got %q, want %q", text, got, want)
	}
}

type expect struct{ site, want string }

type clangCase struct {
	name    string
	imports string // import lines
	header  string // lib.hpp, imported as lib when imports mention it
	body    string
	want    []expect
}

func (c clangCase) program() string {
	imports := c.imports
	return "package main\n\nimport (\n\t\"fmt\"\n" + imports + ")\n\nfunc main() {\n" + c.body + "\n\tfmt.Println()\n}\n"
}

const libImport = "\tlib \"./lib.hpp\"\n"

func runCases(t *testing.T, cases []clangCase) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			files := map[string]string{}
			if c.header != "" {
				files["lib.hpp"] = c.header
			}
			rep := analyze(t, c.program(), files, Options{})
			if t.Failed() {
				return
			}
			for _, e := range c.want {
				expectResult(t, rep, e.site, e.want)
			}
			if t.Failed() || os.Getenv("TYPED_DUMP") != "" {
				var b strings.Builder
				for _, r := range rep.Set.Refs {
					res := rep.Table.Result(r.ID)
					fmt.Fprintf(&b, "%s %-6s %q => %s\n", map[bool]string{true: " ", false: "-"}[r.Standalone], r.Kind, firstSite(r), describe(res))
				}
				t.Logf("\n%s", b.String())
			}
		})
	}
}

func firstSite(r *collect.Ref) string {
	if len(r.Sites) == 0 {
		return r.Key
	}
	return r.Sites[0].Text
}
