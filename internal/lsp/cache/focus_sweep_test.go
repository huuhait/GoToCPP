package cache

import (
	"context"
	"go/ast"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/bench"
)

// The sweep: in every function of every file of a body of code, three edits of its body, each
// checked against the full check: the focused diagnostics of the edited file must be those of the
// full check, and the full check must give the other files the diagnostics they had.

// focusEdits are the edits of a function body that the sweep makes, by the text between the braces.
var focusEdits = []struct {
	name string
	body func(old string) string
}{
	{"an undefined name first", func(old string) string { return "\n\t_ = focusUndefined" + old }},
	{"an unused variable first", func(old string) string { return "\n\tfocusUnused := 1" + old }},
	{"an empty body", func(string) string { return "\n" }},
}

// sweepFocused runs the sweep over the module files, and returns how many edits it made and how
// many of them the focused check answered.
func sweepFocused(t *testing.T, files map[string]string, phase int) (edits, answered int) {
	t.Helper()
	w := newWorld(t, phaseConfig(phase), files)
	var names []string
	for name := range files {
		if strings.HasSuffix(name, ".gp") && !strings.HasSuffix(name, "_test.gp") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		dir := path.Dir(name)
		w.open(name, files[name])
		base := w.pkg(dir)
		if base.Check == nil || len(base.Program.Roots) == 0 {
			w.close(name)
			continue
		}
		snap, release := w.s.Snapshot()
		pf, err := snap.ParseFile(context.Background(), w.uri(name), ParseFull)
		release()
		if err != nil || pf.Err != nil {
			w.close(name)
			continue
		}
		src := files[name]
		bodies := functionBodies(pf)
		// The last bodies first, so that the offsets of the others stay.
		slices.Reverse(bodies)
		for i, b := range bodies {
			if i >= 6 {
				break
			}
			for _, e := range focusEdits {
				text := src[:b.from] + e.body(src[b.from:b.to]) + src[b.to:]
				edits++
				if sweepOne(t, w, base, dir, name, text) {
					answered++
				}
			}
		}
		w.edit(name, src)
		w.close(name)
	}
	return edits, answered
}

// A body is the text between the braces of a function body: the byte offsets of its start and end.
type body struct{ from, to int }

// functionBodies returns the bodies of the functions of the parsed file, in source order.
func functionBodies(pf *ParsedFile) []body {
	var bodies []body
	for _, decl := range pf.File.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			bodies = append(bodies, body{pf.Tok.Offset(fn.Body.Lbrace) + 1, pf.Tok.Offset(fn.Body.Rbrace)})
		}
	}
	return bodies
}

// sweepOne makes one edit of the sweep, and reports whether the focused check answered it.
func sweepOne(t *testing.T, w *world, base *Package, dir, name, text string) bool {
	t.Helper()
	focused, full := focusedAndFull(t, w, base, dir, name, text)
	if focused == nil {
		return false
	}
	file := filepath.Join(w.root, filepath.FromSlash(name))
	if got, want := diagsIn(focused, file), diagsIn(full, file); !reflect.DeepEqual(got, want) {
		t.Errorf("%s, edited to\n%s\ndiagnostics of the edited file:\nfocused %+v\nfull    %+v",
			name, text, got, want)
	}
	for _, f := range base.Program.Roots[0].Files {
		if f.Path == file {
			continue
		}
		if got, want := diagsIn(full, f.Path), diagsIn(base, f.Path); !reflect.DeepEqual(got, want) {
			t.Errorf("%s edited: the diagnostics of %s changed:\nbase %+v\nfull %+v", name, f.Path, want, got)
		}
	}
	return true
}

func TestFocusedSweepGenerated(t *testing.T) {
	proj := bench.Generate(bench.Small)
	files := map[string]string{"gocpp.mod": "module " + proj.Module + "\n"}
	for _, rel := range proj.SourceFiles(false) {
		files[rel+".gp"] = proj.Source(rel)
	}
	for _, phase := range focusPhases {
		t.Run(phaseName(phase), func(t *testing.T) {
			edits, answered := sweepFocused(t, files, phase)
			if answered == 0 {
				t.Fatalf("the focused check answered none of the %d edits", edits)
			}
			t.Logf("%d of %d edits answered", answered, edits)
		})
	}
}

func TestFocusedSweepExamples(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "..", "..", "examples", "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range focusPhases {
		t.Run(phaseName(phase), func(t *testing.T) {
			total, totalAnswered := 0, 0
			for _, dir := range dirs {
				if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
					continue
				}
				files := readModule(dir)
				if _, ok := files["gocpp.mod"]; !ok {
					files["gocpp.mod"] = "module example.com/" + filepath.Base(dir) + "\n"
				}
				t.Run(filepath.Base(dir), func(t *testing.T) {
					edits, answered := sweepFocused(t, files, phase)
					total += edits
					totalAnswered += answered
				})
			}
			if totalAnswered == 0 {
				t.Fatalf("the focused check answered none of the %d edits", total)
			}
			t.Logf("%d of %d edits answered", totalAnswered, total)
		})
	}
}

// readModule reads the .gp files and the gocpp.mod of the folder dir into a map by slash-separated
// name.
func readModule(dir string) map[string]string {
	files := make(map[string]string)
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && (strings.HasSuffix(p, ".gp") || filepath.Base(p) == "gocpp.mod") {
			data, _ := os.ReadFile(p)
			rel, _ := filepath.Rel(dir, p)
			files[filepath.ToSlash(rel)] = string(data)
		}
		return nil
	})
	return files
}

// TestFocusedSweepCorpus runs the sweep over the programs with errors, every tenth of the programs
// that run, and the modules of several packages of the tests.
func TestFocusedSweepCorpus(t *testing.T) {
	tests := filepath.Join("..", "..", "..", "tests")
	var modules []map[string]string
	single := func(src string) map[string]string {
		return map[string]string{"gocpp.mod": "module example.com/t\n", "main.gp": src}
	}
	for _, sub := range []string{"errors", "run"} {
		names, err := filepath.Glob(filepath.Join(tests, sub, "*.gp"))
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(names)
		for i, name := range names {
			if sub == "run" && i%10 != 0 {
				continue
			}
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			modules = append(modules, single(string(data)))
		}
	}
	dirs, err := filepath.Glob(filepath.Join(tests, "pkgs", "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range dirs {
		if files := readModule(dir); files["gocpp.mod"] != "" {
			modules = append(modules, files)
		}
	}
	for _, phase := range focusPhases {
		t.Run(phaseName(phase), func(t *testing.T) {
			total, answered := 0, 0
			for _, files := range modules {
				edits, n := sweepFocused(t, files, phase)
				total += edits
				answered += n
			}
			if answered == 0 {
				t.Fatalf("the focused check answered none of the %d edits", total)
			}
			t.Logf("%d modules, %d of %d edits answered", len(modules), answered, total)
		})
	}
}
