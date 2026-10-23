package cache

import (
	"context"
	"fmt"
	"go/ast"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/version"
)

// focusPhases are the phases of the gates that the tests of the focused check run at: the last,
// where no gate fires, and the one of the product's default (gocpp lsp), where the gates of the
// later phases report their errors.
var focusPhases = []int{check.LastPhase, version.Phase}

func phaseName(phase int) string { return fmt.Sprintf("phase %d", phase) }

// phaseConfig is testConfig at the phase.
func phaseConfig(phase int) Config {
	cfg := testConfig()
	cfg.Phase = phase
	return cfg
}

// focusFiles is a module of a package lib of three files, with a package main that imports it. The
// markers of a.gp are the places of the edits of the tests.
func focusFiles() map[string]string {
	return map[string]string{
		"gocpp.mod": "module example.com/focus\n",
		"main.gp": `package main

import "example.com/focus/lib"

func main() {
	var c lib.Counter
	_ = c.Add(1)
}
`,
		"lib/a.gp": `package lib

import "fmt"

type Counter struct{ n int }

func (c *Counter) Add(k int) int {
	c.n += k // EDIT
	return c.n
}

func Describe(c *Counter) string {
	return fmt.Sprintf("counter %d", c.n) // EDIT2
}
`,
		"lib/b.gp": `package lib

import "strings"

var Names = []string{"a", "b"}

func Join() string {
	return strings.Join(Names, ",")
}

func Twice(k int) int {
	return Helper(k) * 2
}
`,
		"lib/c.gp": `package lib

import "strconv"

const Limit = 10

func Helper(k int) int {
	v, err := strconv.Atoi("5")
	if err != nil {
		return k
	}
	return v + k + Limit
}

type Shape interface{ Area() float64 }

type Square struct{ side float64 }

func (s Square) Area() float64 { return s.side * s.side }
`,
	}
}

// diagsIn returns the diagnostics of pkg that are in the file path, as the server reads them.
func diagsIn(pkg *Package, path string) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, l := range []*diag.List{pkg.Program.Errors, pkg.Check.Errors} {
		for _, d := range l.Diagnostics() {
			if d.Pos.Filename == path {
				out = append(out, d)
			}
		}
	}
	return out
}

// focusRun opens the files of lib, takes the full check of lib as the base, and returns the world
// with the base.
func focusRun(t *testing.T, files map[string]string) (*world, *Package) {
	t.Helper()
	return focusRunAt(t, files, check.LastPhase)
}

// focusRunAt is focusRun with the gates of the phase.
func focusRunAt(t *testing.T, files map[string]string, phase int) (*world, *Package) {
	t.Helper()
	w := newWorld(t, phaseConfig(phase), files)
	for name, src := range files {
		if strings.HasPrefix(name, "lib/") {
			w.open(name, src)
		}
	}
	return w, w.pkg("lib")
}

// focusedAndFull edits the file name to the text, and returns the focused package of the folder dir
// against base (nil when it declined) and the full check of lib.
func focusedAndFull(t *testing.T, w *world, base *Package, dir, name, text string) (focused, full *Package) {
	t.Helper()
	w.edit(name, text)
	snap, release := w.s.Snapshot()
	defer release()
	dir = filepath.Join(w.root, filepath.FromSlash(dir))
	focused, err := snap.Focused(context.Background(), dir, base)
	if err != nil {
		t.Fatal(err)
	}
	full, err = snap.Package(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return focused, full
}

func TestFocusedMatchesFullCheck(t *testing.T) {
	tests := []struct {
		name string
		edit string // the new text of the line with // EDIT
		edi2 string // the new text of the line with // EDIT2, or ""
		bad  bool   // the edited file has a diagnostic then
	}{
		{name: "a literal", edit: "c.n += k + 1"},
		{name: "a comment", edit: "c.n += k // typed"},
		{name: "an undefined name", bad: true, edit: "c.n += undefinedName"},
		{name: "an unused variable", bad: true, edit: "unused := 1"},
		{name: "a wrong type", bad: true, edit: "c.n += \"text\""},
		{name: "a function of another file", edit: "c.n += Helper(k)"},
		{name: "a function of another file with a wrong call", bad: true, edit: "c.n += Helper(k, k)"},
		{name: "a constant of another file", edit: "c.n += Limit"},
		{name: "a variable of another file", edit: "c.n += len(Names)"},
		{name: "a method of the interface of another file", edit: "var s Shape = Square{1}\n\t_ = s"},
		{name: "a missing method of that interface", bad: true, edit: "var s Shape = &Counter{}\n\t_ = s"},
		{name: "a function literal", edit: "f := func() int { return k }\n\tc.n += f()"},
		{name: "two statements", edit: "c.n += k\n\tc.n++"},
		{name: "a missing label", bad: true, edit: "goto done"},
		{name: "a package that is not imported", bad: true, edit: "c.n += len(strings.ToUpper(\"x\"))"},
		{name: "an import that is not used", bad: true, edi2: "return \"none\""},
		{name: "an import that is used again", edit: "c.n += len(fmt.Sprint(k))"},
	}
	for _, phase := range focusPhases {
		for _, tt := range tests {
			t.Run(phaseName(phase)+"/"+tt.name, func(t *testing.T) {
				files := focusFiles()
				w, base := focusRunAt(t, files, phase)
				text := files["lib/a.gp"]
				if tt.edit != "" {
					text = strings.Replace(text, "c.n += k // EDIT", tt.edit+" // EDIT", 1)
				}
				if tt.edi2 != "" {
					const line = `return fmt.Sprintf("counter %d", c.n) // EDIT2`
					text = strings.Replace(text, line, tt.edi2+" // EDIT2", 1)
				}
				focused, full := focusedAndFull(t, w, base, "lib", "lib/a.gp", text)
				path := filepath.Join(w.root, "lib", "a.gp")
				if focused == nil {
					t.Fatal("the focused check declined a change in a function body")
				}
				if got := focused.Focus(); got != path {
					t.Errorf("Focus() = %q, want %q", got, path)
				}
				if tt.bad && len(diagsIn(full, path)) == 0 {
					t.Errorf("the full check has no diagnostic in the edited file")
				}
				if got, want := diagsIn(focused, path), diagsIn(full, path); !reflect.DeepEqual(got, want) {
					t.Errorf("diagnostics of the edited file:\nfocused %+v\nfull    %+v", got, want)
				}
				for _, other := range []string{"b.gp", "c.gp"} {
					file := filepath.Join(w.root, "lib", other)
					if got, want := diagsIn(full, file), diagsIn(base, file); !reflect.DeepEqual(got, want) {
						t.Errorf("diagnostics of %s changed:\nbase %+v\nfull %+v", other, want, got)
					}
				}
			})
		}
	}
}

// focusModule is a module of a package lib and a package dep that lib imports, for the tests of
// what the focused check declines.
func focusModule() map[string]string {
	files := focusFiles()
	files["dep/dep.gp"] = "package dep\n\nfunc Value() int { return 1 }\n"
	imports := "import (\n\t\"strconv\"\n\n\t\"example.com/focus/dep\"\n)"
	files["lib/c.gp"] = strings.Replace(files["lib/c.gp"], `import "strconv"`, imports, 1) +
		"\nfunc FromDep() int { return dep.Value() }\n"
	return files
}

// editA returns the texts of an edit of a.gp that puts line where the line of the marker // EDIT
// was.
func editA(files map[string]string, line string) map[string]string {
	text := strings.Replace(files["lib/a.gp"], "c.n += k // EDIT", line+" // EDIT", 1)
	return map[string]string{"lib/a.gp": text}
}

// editWith is the edits of a table that edit a.gp with the line.
func editWith(line string) func(map[string]string) map[string]string {
	return func(files map[string]string) map[string]string { return editA(files, line) }
}

func TestFocusedDeclines(t *testing.T) {
	const a = "lib/a.gp"
	tests := []struct {
		name  string
		files func() map[string]string
		edits func(files map[string]string) map[string]string // the new texts by file
		// loaded is whether the check gets as far as loading the package: the proof that comes
		// before the load does not decline it.
		loaded bool
	}{
		{name: "a renamed function", edits: func(f map[string]string) map[string]string {
			return map[string]string{a: strings.Replace(f[a], "func Describe", "func Describe2", 1)}
		}},
		{name: "a new function", edits: func(f map[string]string) map[string]string {
			return map[string]string{a: f[a] + "\nfunc Extra() {}\n"}
		}},
		{name: "a changed signature", edits: func(f map[string]string) map[string]string {
			return map[string]string{a: strings.Replace(f[a], "Add(k int)", "Add(k, j int)", 1)}
		}},
		{name: "a comment before the declarations", edits: func(f map[string]string) map[string]string {
			return map[string]string{a: "// typed\n" + f[a]}
		}},
		{name: "a new import", edits: func(f map[string]string) map[string]string {
			imports := "import (\n\t\"fmt\"\n\t\"os\"\n)"
			return map[string]string{a: strings.Replace(f[a], `import "fmt"`, imports, 1)}
		}},
		{name: "a change of the text that is no change", edits: func(f map[string]string) map[string]string {
			return map[string]string{a: f[a]}
		}},
		{name: "two files", edits: func(f map[string]string) map[string]string {
			return map[string]string{
				a:          strings.Replace(f[a], "c.n += k // EDIT", "c.n += k + 1 // EDIT", 1),
				"lib/b.gp": strings.Replace(f["lib/b.gp"], "Helper(k) * 2", "Helper(k) * 3", 1),
			}
		}},
		{name: "a line directive in the body", edits: func(f map[string]string) map[string]string {
			return editA(f, "//line x.gp:100\n\tc.n += k")
		}},
		{name: "a syntax error in the body", edits: func(f map[string]string) map[string]string {
			return editA(f, "c.n += (")
		}},
		{name: "a brace that ends the function early", loaded: true, edits: editWith("c.n += k }\nfunc extra() {")},
		{name: "a directive in the body", loaded: true, edits: editWith("//gocpp:link m\n\tc.n += k")},
		{name: "a macro use in the body", loaded: true, edits: editWith("#Helper(k)")},
		{name: "a form with a bang", loaded: true, edits: editWith("x := Helper!(k)\n\t_ = x")},
		{name: "a package that imports changed its API", edits: func(f map[string]string) map[string]string {
			return map[string]string{
				a:            strings.Replace(f[a], "c.n += k // EDIT", "c.n += k + 1 // EDIT", 1),
				"dep/dep.gp": "package dep\n\nfunc Value() string { return \"\" }\n",
			}
		}},
		{
			name: "an initializer that calls a function",
			files: func() map[string]string {
				f := focusModule()
				f["lib/b.gp"] += "\nvar Seeded = Helper(2)\n"
				return f
			},
			loaded: true,
			edits: func(f map[string]string) map[string]string {
				return editA(f, "c.n += k + 1")
			},
		},
		{
			// A generic function of another file instantiates one with a type argument made of its
			// type parameter: an edge of weight 1 of go/types' graph of instantiation cycles.
			name: "a weighted instantiation in another file",
			files: func() map[string]string {
				f := focusModule()
				f["lib/b.gp"] += "\nfunc Pick[T any](x, y T) T { return x }\n\n" +
					"func Nest[T any](x T) []T { return Pick[[]T](nil, nil) }\n"
				return f
			},
			edits: func(f map[string]string) map[string]string {
				return editA(f, "c.n += k + 1")
			},
		},
		{
			name: "the constraint of a generic function",
			files: func() map[string]string {
				f := focusModule()
				f[a] += "\nfunc Pick[T any](x, y T) T {\n\treturn x // GEDIT\n}\n"
				return f
			},
			edits: func(f map[string]string) map[string]string {
				return map[string]string{a: strings.Replace(f[a], "Pick[T any]", "Pick[T comparable]", 1)}
			},
		},
		{
			name: "the type parameters of a generic type",
			files: func() map[string]string {
				f := focusModule()
				f[a] += "\ntype Box[T any] struct{ v T }\n\nfunc (b Box[T]) Get() T {\n\treturn b.v // GEDIT\n}\n"
				return f
			},
			edits: func(f map[string]string) map[string]string {
				text := strings.Replace(f[a], "Box[T any] struct{ v T }", "Box[T, U any] struct{ v T }", 1)
				return map[string]string{a: strings.Replace(text, "(b Box[T])", "(b Box[T, U])", 1)}
			},
		},
		{
			// The body of the generic function instantiates it with []T: a cycle, which go/types
			// reports when the package has no other error.
			name: "an instantiation cycle typed in a generic function",
			files: func() map[string]string {
				f := focusModule()
				f[a] += "\nfunc Pick[T any](x, y T) T {\n\treturn x // GEDIT\n}\n"
				return f
			},
			loaded: true,
			edits: func(f map[string]string) map[string]string {
				return map[string]string{a: strings.Replace(f[a], "return x // GEDIT",
					"_ = Pick[[]T](nil, nil)\n\treturn x // GEDIT", 1)}
			},
		},
		{
			name: "a file of the package with a syntax error",
			files: func() map[string]string {
				f := focusModule()
				f["lib/b.gp"] += "\nfunc broken( {\n"
				return f
			},
			edits: func(f map[string]string) map[string]string {
				return editA(f, "c.n += k + 1")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			build := tt.files
			if build == nil {
				build = focusModule
			}
			files := build()
			w, base := focusRun(t, files)
			for name, text := range tt.edits(files) {
				if _, ok := files[name]; strings.HasPrefix(name, "dep/") && ok {
					w.open(name, files[name])
				}
				w.edit(name, text)
			}
			before := w.p.FocusedChecks()
			snap, release := w.s.Snapshot()
			defer release()
			got, err := snap.Focused(context.Background(), filepath.Join(w.root, "lib"), base)
			if err != nil {
				t.Fatal(err)
			}
			if got != nil {
				t.Errorf("a focused check for %s", got.Focus())
			}
			if loaded := w.p.FocusedChecks() - before; (loaded == 1) != tt.loaded || loaded > 1 {
				t.Errorf("%d focused checks that loaded the package, loaded %v", loaded, tt.loaded)
			}
		})
	}
}

func TestFocusedNeedsABase(t *testing.T) {
	files := focusModule()
	w, _ := focusRun(t, files)
	w.edit("lib/a.gp", strings.Replace(files["lib/a.gp"], "c.n += k // EDIT", "c.n += k + 1 // EDIT", 1))
	snap, release := w.s.Snapshot()
	defer release()
	got, err := snap.Focused(context.Background(), filepath.Join(w.root, "lib"), nil)
	if err != nil || got != nil {
		t.Errorf("Focused without a base = %v, %v", got, err)
	}
}

// TestFocusedInitCycle: a function of one file calls a function of another file, which uses a
// variable that the first function initializes: go/types reports the initialization cycle at the
// variable, which the focused check cannot see without the other file's body.
func TestFocusedInitCycle(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/focus\n",
		"lib/a.gp":  "package lib\n\nvar V = g()\n\nfunc g() int {\n\treturn 1 // EDIT\n}\n",
		"lib/b.gp":  "package lib\n\nfunc h() int {\n\treturn V\n}\n",
	}
	w, base := focusRun(t, files)
	text := strings.Replace(files["lib/a.gp"], "return 1 // EDIT", "return h() // EDIT", 1)
	focused, full := focusedAndFull(t, w, base, "lib", "lib/a.gp", text)
	if got := len(diagsIn(full, filepath.Join(w.root, "lib", "a.gp"))); got == 0 {
		t.Fatal("the full check reports no initialization cycle")
	}
	if focused != nil {
		t.Errorf("a focused check for the edit that makes the cycle: %+v", diagsIn(focused, focused.Focus()))
	}
}

func TestPlainPackage(t *testing.T) {
	plain := func() *load.Package {
		return &load.Package{
			Errors: new(diag.List),
			Files:  []*load.File{{AST: &ast.File{}, Info: &load.FileInfo{}, Rewrite: &load.Rewrite{}}},
		}
	}
	if !plainPackage(plain()) {
		t.Fatal("a package of one plain file is not plain")
	}
	tests := []struct {
		name   string
		change func(*load.Package)
	}{
		{"an incomplete package", func(p *load.Package) { p.Incomplete = true }},
		{"a standard package", func(p *load.Package) { p.Std = true }},
		{"an error", func(p *load.Package) { p.Errors.Add(diag.Diagnostic{Msg: "x"}) }},
		{"a C file", func(p *load.Package) { p.CFiles = []string{"a.c"} }},
		{"a C++ file", func(p *load.Package) { p.CXXFiles = []string{"a.cc"} }},
		{"a macro", func(p *load.Package) { p.Macros = map[string]*load.MacroSig{"m": {}} }},
		{"a macro use", func(p *load.Package) { p.MacroUses = map[*ast.Ident]*load.MacroUse{{}: {}} }},
		{"a #Derive line", func(p *load.Package) { p.Derives = []*load.Derive{{}} }},
		{"a file that did not parse", func(p *load.Package) { p.Files[0].AST = nil }},
		{"a file without its imports", func(p *load.Package) { p.Files[0].Info = nil }},
		{"a directive", func(p *load.Package) { p.Files[0].Directives = []load.Directive{{}} }},
		{"a header import", func(p *load.Package) { p.Files[0].Info.Includes = []*load.Include{{}} }},
		{"a namespace rename", func(p *load.Package) {
			p.Files[0].Info.NamespaceRenames = map[string]*load.NamespaceRename{"fs": {}}
		}},
		{"a use of a header", func(p *load.Package) { p.Files[0].Rewrite.Uses = []*load.CppUse{{}} }},
		{"a use of a namespace", func(p *load.Package) {
			p.Files[0].Rewrite.Namespaces = []*load.NamespaceCandidate{{}}
		}},
		{"an import of C", func(p *load.Package) { p.Files[0].Rewrite.ImportC = &ast.ImportSpec{} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := plain()
			tt.change(p)
			if plainPackage(p) {
				t.Error("the package is plain")
			}
		})
	}
}

func TestFocusRequestAccept(t *testing.T) {
	root := &load.Package{
		Errors: new(diag.List),
		Files:  []*load.File{{Path: "a.gp", AST: &ast.File{}, Info: &load.FileInfo{}, Rewrite: &load.Rewrite{}}},
	}
	fr := &focusRequest{file: "a.gp"}

	t.Run("a package that is not plain", func(t *testing.T) {
		root.Files[0].Rewrite.Namespaces = []*load.NamespaceCandidate{{}}
		defer func() { root.Files[0].Rewrite.Namespaces = nil }()
		if fr.accept(&load.Program{Packages: []*load.Package{root}}, root) {
			t.Error("accepted")
		}
	})
	t.Run("a program of several packages", func(t *testing.T) {
		if fr.accept(&load.Program{Packages: []*load.Package{root, root}}, root) {
			t.Error("accepted")
		}
	})
	t.Run("a file that is not in the package", func(t *testing.T) {
		other := &focusRequest{file: "none.gp"}
		if other.accept(&load.Program{Packages: []*load.Package{root}}, root) {
			t.Error("accepted")
		}
	})
}
