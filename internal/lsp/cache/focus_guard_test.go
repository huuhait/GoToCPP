package cache

import (
	"context"
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// Targeted tests of the guards of the focused check (focus.go). Some isolate invariants with
// synthetic state; the source-level tests separately exercise edits through the full pipeline.

// focusHandle returns the handle and the key of lib in the current snapshot of w.
func focusHandle(t *testing.T, w *world) (*handle, *keyed) {
	t.Helper()
	snap, release := w.s.Snapshot()
	defer release()
	st := snap.state.(*state)
	h, err := st.handle(context.Background(), pkgID{dir: filepath.Join(w.root, "lib")}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	k, err := h.keyed.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return h, k
}

// TestFocusEnvHashesWhatItNames: the environment of a focused check changes with each thing that
// can make the declarations of the package differ without the content of a file.
func TestFocusEnvHashesWhatItNames(t *testing.T) {
	w, _ := focusRun(t, focusModule())
	h, k := focusHandle(t, w)
	before := w.p.focusEnv(h, k)
	tests := []struct {
		name string
		do   func() (undo func())
	}{
		{"the configuration", func() func() {
			old := w.p.cfgKey
			w.p.cfgKey[0]++
			return func() { w.p.cfgKey = old }
		}},
		{"the folder", func() func() {
			old := h.id
			h.id.dir += "x"
			return func() { h.id = old }
		}},
		{"the import path", func() func() {
			old := h.importPath
			h.importPath += "x"
			return func() { h.importPath = old }
		}},
		{"the module file", func() func() {
			old := h.mod
			mod := *h.mod
			mod.hash[0]++
			h.mod = &mod
			return func() { h.mod = old }
		}},
		{"the list of files", func() func() {
			old := h.files
			h.files = h.files[1:]
			return func() { h.files = old }
		}},
		{"the name of a file", func() func() {
			old := h.files
			files := append([]*fileHandle(nil), h.files...)
			renamed := *files[0]
			renamed.path += "x"
			files[0] = &renamed
			h.files = files
			return func() { h.files = old }
		}},
		{"what the metadata read", func() func() {
			old := h.meta
			meta := *h.meta
			meta.digest[0]++
			h.meta = &meta
			return func() { h.meta = old }
		}},
		{"the API of an import", func() func() {
			old := k.deps
			deps := append([]*summary(nil), k.deps...)
			for i, dep := range deps {
				if dep != nil {
					changed := &summary{importPath: dep.importPath, apiHash: dep.apiHash}
					changed.apiHash[0]++
					deps[i] = changed
					break
				}
			}
			k.deps = deps
			return func() { k.deps = old }
		}},
		{"the import path of an import", func() func() {
			old := k.deps
			deps := append([]*summary(nil), k.deps...)
			for i, dep := range deps {
				if dep != nil {
					deps[i] = &summary{importPath: dep.importPath + "x", apiHash: dep.apiHash}
					break
				}
			}
			k.deps = deps
			return func() { k.deps = old }
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			undo := tt.do()
			defer undo()
			if after := w.p.focusEnv(h, k); after == before {
				t.Errorf("the environment is the same")
			}
		})
	}
	t.Run("nothing changed", func(t *testing.T) {
		if after := w.p.focusEnv(h, k); after != before {
			t.Errorf("the environment changed")
		}
	})
}

// TestFocusedLoadSkipsOtherBodies: the load of a focused check leaves the bodies of the other files
// out of the tree (load.Config.SkipBodies), and keeps those of the edited file and of init.
func TestFocusedLoadSkipsOtherBodies(t *testing.T) {
	files := focusModule()
	files["lib/b.gp"] += "\nfunc init() {\n\t_ = Helper(1)\n}\n"
	w, base := focusRun(t, files)
	bodies := make(map[string]bool)
	w.p.testLoaded = func(prog *load.Program) {
		for _, file := range prog.Roots[0].Files {
			for _, decl := range file.AST.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok {
					bodies[filepath.Base(file.Path)+"."+fn.Name.Name] = fn.Body != nil
				}
			}
		}
	}

	text := strings.Replace(files["lib/a.gp"], "c.n += k // EDIT", "c.n += k + 1 // EDIT", 1)
	focused, _ := focusedAndFull(t, w, base, "lib", "lib/a.gp", text)

	if focused == nil {
		t.Fatal("the focused check declined")
	}
	want := map[string]bool{
		"a.gp.Add": true, "a.gp.Describe": true,
		"b.gp.Join": false, "b.gp.Twice": false, "b.gp.init": true,
		"c.gp.Helper": false, "c.gp.Area": false,
	}
	for name, has := range want {
		if got, ok := bodies[name]; !ok || got != has {
			t.Errorf("%s has a body: %v, want %v (bodies: %v)", name, got, has, bodies)
		}
	}
}

// TestFocusedDeclinesBase: the forms of another file that make the base ineligible.
func TestFocusedDeclinesBase(t *testing.T) {
	tests := []struct {
		name string
		add  string // appended to lib/b.gp
	}{
		{"a generic type", "\ntype Box[T any] struct{ v T }\n"},
		{"a generic function", "\nfunc Pick[T any](x, y T) T { return x }\n"},
		{"a must function", "\nfunc Parse!() int { return 1 }\n\nfunc UseParse() int { return Parse!() }\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := focusModule()
			files["lib/b.gp"] += tt.add
			w, base := focusRun(t, files)
			if base.focusEligible() {
				t.Error("the base is eligible")
			}
			text := strings.Replace(files["lib/a.gp"], "c.n += k // EDIT", "c.n += k + 1 // EDIT", 1)
			focused, _ := focusedAndFull(t, w, base, "lib", "lib/a.gp", text)
			if focused != nil {
				t.Error("a focused check")
			}
		})
	}
}

// TestFocusEligibleCheckRewrites isolates the check-result guards from the syntax guards. Real
// must/constant rewrites may also make the loaded source ineligible; these synthetic results prove
// that the guards themselves decline even when the loaded source is otherwise plain.
func TestFocusEligibleCheckRewrites(t *testing.T) {
	_, base := focusRun(t, focusModule())
	tests := []struct {
		name   string
		change func(*check.Package)
	}{
		{"a must rewrite", func(cp *check.Package) {
			cp.Must = &check.MustInfo{}
		}},
		{"a constant rewrite", func(cp *check.Package) {
			cp.Consts = []check.ConstUse{{}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := &check.Package{
				Load: base.Check.Load, Types: base.Check.Types, Info: base.Check.Info,
			}
			candidate := &Package{Program: base.Program, Check: cp, summary: base.summary}
			if !candidate.computeFocusEligible() {
				t.Fatal("the unchanged base is not eligible")
			}

			tt.change(cp)
			if candidate.computeFocusEligible() {
				t.Error("a check with rewritten source is eligible")
			}
		})
	}
}

// editBody returns the text of a.gp with the line of the first marker replaced.
func editBody(files map[string]string, line string) string {
	return strings.Replace(files["lib/a.gp"], "c.n += k // EDIT", line, 1)
}

// TestFindBodyEdit: the change must lie between the braces of the body; the braces themselves are
// not inside it.
func TestFindBodyEdit(t *testing.T) {
	files := focusFiles()
	w, base := focusRun(t, files)
	path := filepath.Join(w.root, "lib", "a.gp")
	old := files["lib/a.gp"]
	const signature = "func (c *Counter) Add(k int) int {"
	swap := func(from, to string) string { return strings.Replace(old, from, to, 1) }
	tests := []struct {
		name string
		text string
		ok   bool
	}{
		{"a change in the body", editBody(files, "c.n += k + 1 // EDIT"), true},
		{"a statement right after the opening brace", swap(signature, signature+"\n\tz()"), true},
		{"a statement right before the closing brace", swap("c.n\n}", "c.n\n\tz()\n}"), true},
		{"the opening brace taken out", swap(signature, signature[:len(signature)-1]), false},
		{"the closing brace changed", swap("return c.n\n}", "return c.n\n)"), false},
		{"the closing brace taken out with its line", swap("return c.n\n}\n", "return c.n\n"), false},
		{"a change in the signature", swap("Add(k int)", "Add(k, j int)"), false},
		{"a change between two functions", swap("}\n\nfunc Describe", "}\n\n// z\nfunc Describe"), false},
		{"a change in a declaration", swap("struct{ n int }", "struct{ n, m int }"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.text == old {
				t.Fatal("the text was not changed")
			}
			if _, ok := findBodyEdit(base, path, []byte(tt.text)); ok != tt.ok {
				t.Errorf("findBodyEdit = %v, want %v", ok, tt.ok)
			}
		})
	}
}

// TestBodyEditConfirm: the new tree must have the same declarations, with the body of the function
// between the braces that the change puts there.
func TestBodyEditConfirm(t *testing.T) {
	files := focusFiles()
	w, base := focusRun(t, files)
	path := filepath.Join(w.root, "lib", "a.gp")
	old := files["lib/a.gp"]
	parse := func(text string) (*load.File, *token.FileSet) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, text, parser.ParseComments|parser.Phase10Syntax)
		if err != nil {
			t.Fatal(err)
		}
		return &load.File{Path: path, AST: f}, fset
	}
	edited := editBody(files, "c.n += k + 1 // EDIT")
	edit, ok := findBodyEdit(base, path, []byte(edited))
	if !ok {
		t.Fatal("no body edit")
	}

	t.Run("the edit itself", func(t *testing.T) {
		file, fset := parse(edited)
		if !edit.confirm(file, fset) {
			t.Error("declined")
		}
	})
	t.Run("a closing brace that is not where the change puts it", func(t *testing.T) {
		moved := edit
		moved.delta++
		file, fset := parse(edited)
		if moved.confirm(file, fset) {
			t.Error("confirmed")
		}
	})
	t.Run("an opening brace that moved", func(t *testing.T) {
		// One more space in the signature moves both braces by one: the closing one is where the
		// length says, the opening one is not where it was.
		spaced := strings.Replace(old, "Add(k int)", "Add(k  int)", 1)
		shifted := edit
		shifted.newLen = len(spaced)
		shifted.delta = 1
		file, fset := parse(spaced)
		if shifted.confirm(file, fset) {
			t.Error("confirmed")
		}
	})
	t.Run("another number of declarations", func(t *testing.T) {
		extra := edited + "\nfunc extra() {}\n"
		more := edit
		more.newLen = len(extra)
		file, fset := parse(extra)
		if more.confirm(file, fset) {
			t.Error("confirmed")
		}
	})
	t.Run("another length of the text", func(t *testing.T) {
		file, fset := parse(edited + "\n")
		if edit.confirm(file, fset) {
			t.Error("confirmed")
		}
	})
	t.Run("no tree", func(t *testing.T) {
		if edit.confirm(&load.File{}, token.NewFileSet()) {
			t.Error("confirmed")
		}
	})
}

func TestHasLineDirective(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{"line mapping", "package p\n//line virtual.gp:100\nfunc f() {}\n", true},
		{"block mapping", "package p\n/*line virtual.gp:100*/\nfunc f() {}\n", true},
		{"inline block mapping", "package p\nfunc f() { /*line virtual.gp:100:1*/ _ = 1 }\n", true},
		{"identity mapping", "package p\n//line /module/a.gp:3\nfunc f() {}\n", true},
		{"column mapping", "package p\n//line :100:1\nfunc f() {}\n", true},
		{"unparsed directive text", "package p\n//line example\nfunc f() {}\n", true},
		{"indented line comment", "package p\n //line virtual.gp:100\nfunc f() {}\n", false},
		{"quoted text", "package p\nconst Doc = \"//line virtual.gp:100\"\n", false},
		{"raw string", "package p\nconst Doc = `\n//line virtual.gp:100\n`\n", false},
		{"ordinary block comment", "package p\n/* usage: //line virtual.gp:100 */\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "/module/a.gp", tt.src, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			if got := hasLineDirective(fset, file); got != tt.want {
				t.Errorf("hasLineDirective = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBodyParses: a body with a syntax error declines before the package is loaded.
func TestBodyParses(t *testing.T) {
	tests := []struct {
		body string
		ok   bool
	}{
		{"{\n\tx := 1\n\t_ = x\n}", true},
		{"{\n\tx := (\n}", false},
		{"{\n\tx := 1 +\n}", false},
		{"{\n\t#Dbg(1)\n}", true},
		{"{ }\n func g() {}", true}, // the parse of the file decides, not this test
		{"", false},
	}
	for _, tt := range tests {
		body := []byte(tt.body)
		if tt.body == "" {
			body = nil
		}
		if got := bodyParses(body); got != tt.ok {
			t.Errorf("bodyParses(%q) = %v, want %v", tt.body, got, tt.ok)
		}
	}
	t.Run("the body out of the text", func(t *testing.T) {
		edit := bodyEdit{lbrace: 10, rbrace: 20, delta: 100}
		if got := edit.newBody(make([]byte, 50)); got != nil {
			t.Errorf("newBody = %q", got)
		}
	})
}
