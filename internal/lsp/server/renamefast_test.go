package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// fastFiles is a module for the shortcut of the check of a rename (renamefast.go): a
// declares names; b uses them by their qualified names, c uses b and a, d
// dot-imports a, e imports a under another name, and f has a name of its own
// that the new name would clash with only if the use were not qualified.
var fastFiles = map[string]string{
	"gocpp.mod": "module example.com/fast\n",
	"a/a.gp": `package a

type T struct{ N int }

func (t T) M() int { return t.N }

func F() int { return 1 }

var V = 2

const K = 3

func inA() int { return F() + V + K }
`,
	"b/b.gp": `package b

import "example.com/fast/a"

func Call() int { return a.F() + a.V + a.K + a.T{N: 1}.M() }
`,
	"c/c.gp": `package c

import (
	"example.com/fast/a"
	"example.com/fast/b"
)

func Use() int { return a.F() + b.Call() + a.V }
`,
	"d/d.gp": `package d

import . "example.com/fast/a"

func Dot() int { return F() + V + K }
`,
	"e/e.gp": `package e

import x "example.com/fast/a"

func Alias() int { return x.F() + x.V + x.K }
`,
	"mixed/qualified.gp": `package mixed

import "example.com/fast/a"

func Qualified() int { return a.F() + a.V + a.K }
`,
	"mixed/dotted.gp": `package mixed

import . "example.com/fast/a"

var Keep T
func MixedFunc() int { return 1 }
var MixedVar = 2
const MixedConst = 3
`,
	"g/g.gp": `package g

import . "example.com/fast/a"

func Zed() int { return K }
`,
	"f/f.gp": `package f

import "example.com/fast/a"

func G() int { return 7 }

var W = 9

func Fn() int { return a.F() + G() + W }
`,
}

// fastRename renames at the mark of a/a.gp in a new server, and returns the
// edited files (or the error) and the number of package checks the rename made.
func fastRename(t *testing.T, all bool, mark, newName string) (string, int) {
	t.Helper()
	return fastRenameFiles(t, fastFiles, "a/a.gp", all, mark, newName)
}

func fastRenameFiles(
	t *testing.T, sourceFiles map[string]string, file string, all bool, mark, newName string,
) (string, int) {
	t.Helper()
	recheckAllImporters = all
	defer func() { recheckAllImporters = false }()
	root := writeTree(t, sourceFiles)
	srv := newServer(nil, testConfig(), debounce.RealClock{})
	c := &refClient{dclient: &dclient{client: startServer(t, srv, nil), root: root}, src: sourceFiles}
	c.call("initialize", plainInit)
	c.notify("initialized", "{}")
	c.skipShown = true
	c.open(file, sourceFiles[file])
	c.next(file)
	// The searches check the importers once; the rename's own checks are counted.
	if _, err := c.rename(file, mark, newName+"Warm"); err != nil {
		_ = err // (a refusal also leaves the importers checked)
	}
	_, _, before := srv.pipeline.Stats()
	edits, rerr := c.rename(file, mark, newName)
	_, _, after := srv.pipeline.Stats()
	if rerr != nil {
		return "error: " + rerr.Message, int(after - before)
	}
	var files []string
	for name, text := range edits {
		files = append(files, name+":\n"+text)
	}
	slices.Sort(files)
	return strings.Join(files, "\n"), int(after - before)
}

func bindingProofFiles() map[string]string {
	return map[string]string{
		"gocpp.mod": "module example.com/binding\n",
		"a/a.gp": `package a
// Emoji before the declaration exercises UTF-16 positions: 🙂
func F(n int) int {
	if n == 0 { return 0 }
	return F(n - 1)
}
func Use() int { return F(2) }
var V = 1
const K = 2
type T struct{}
func (T) M() int { return 1 }
`,
		"a/other.gp": `package a
func Shadowed() int {
	F := func(int) int { return 9 }
	return F(0)
}
`,
		"b/b.gp": `package b
import alias "example.com/binding/a"
func Use() int { return alias.F(1) }
`,
	}
}

func TestRenameDeclaringBindingProof(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setup   func(map[string]string)
		file    string
		mark    string
		newName string
		outcome string
	}{
		{name: "recursive and shadowed old-name uses", outcome: "proved"},
		{name: "request at own-package use", mark: "return |F(n", outcome: "proved"},
		{name: "request at qualified importer use", file: "b/b.gp", mark: "return alias.|F(",
			outcome: "proved"},
		{name: "unrelated local new-name declaration", outcome: "proved",
			setup: func(files map[string]string) {
				files["a/other.gp"] = strings.ReplaceAll(files["a/other.gp"], "F", "Renamed")
			}},
		{name: "Unicode identifier and UTF-16 positions", mark: "func |𝑭(",
			newName: "Édit", outcome: "proved", setup: func(files map[string]string) {
				for name, text := range files {
					files[name] = strings.ReplaceAll(text, "F", "𝑭")
				}
			}},
		{name: "declaring-scope collision", outcome: "refuse", setup: func(files map[string]string) {
			files["a/other.gp"] += "\nvar Renamed = 1\n"
		}},
		{name: "same-typed local capture", outcome: "refuse", setup: func(files map[string]string) {
			files["a/other.gp"] = "package a\nfunc Other() int {\n" +
				"Renamed := func(int) int { return 9 }; return F(2)\n}\n"
		}},
		{name: "unedited declaring-file import collision", outcome: "refuse",
			setup: func(files map[string]string) {
				files["c/c.gp"] = "package c\nconst Value = 1\n"
				files["a/other.gp"] = "package a\nimport Renamed \"example.com/binding/c\"\n" +
					"func Other() int { return Renamed.Value }\n"
			}},
		{name: "declaring-file dot-import binding", outcome: "refuse",
			setup: func(files map[string]string) {
				files["c/c.gp"] = "package c\nfunc Renamed() int { return 1 }\n"
				files["a/other.gp"] = "package a\nimport . \"example.com/binding/c\"\n" +
					"func Other() int { return Renamed() }\n"
			}},
		{name: "mixed qualified and dot-import counterexample", outcome: "refuse",
			setup: func(files map[string]string) {
				files["b/dot.gp"] = "package b\nimport . \"example.com/binding/a\"\n" +
					"var Keep = K\nfunc Renamed() int { return 1 }\n"
			}},
		{name: "generic caller type-parameter capture", outcome: "refuse",
			setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\n" +
					"func Generic[Renamed any](n Renamed) int { return F(1) }\n"
			}},
		{name: "unrelated generic caller", outcome: "proved", setup: func(files map[string]string) {
			files["a/other.gp"] = "package a\nfunc Generic[U any](n U) int { return F(1) }\n"
		}},
		{name: "broken importer retains scratch validation", outcome: "fallback",
			setup: func(files map[string]string) {
				files["b/b.gp"] += "\nvar Broken = undefinedName\n"
			}},
		{name: "visibility loss", newName: "renamed", outcome: "refuse"},
		{name: "reserved C name", newName: "C", outcome: "refuse"},
		{name: "stdlib-shaped module path", outcome: "fallback", setup: func(files map[string]string) {
			for name, text := range files {
				files[name] = strings.ReplaceAll(text, "example.com/binding", "fmt")
			}
		}},
		{name: "generic function target", mark: "func |F[", outcome: "fallback",
			setup: func(files map[string]string) {
				files["a/a.gp"] = strings.ReplaceAll(files["a/a.gp"],
					"func F(n int) int", "func F[T ~int](n T) T")
			}},
		{name: "variable target", mark: "var |V", outcome: "fallback"},
		{name: "constant target", mark: "const |K", outcome: "fallback"},
		{name: "type target", mark: "type |T", outcome: "fallback"},
		{name: "method target", mark: "func (T) |M", outcome: "fallback"},
		{name: "untouched test file", outcome: "fallback", setup: func(files map[string]string) {
			files["a/unrelated_test.gp"] = "package a\nfunc TestHelper() int { return 3 }\n"
		}},
		{name: "bang-origin package", outcome: "fallback", setup: func(files map[string]string) {
			files["a/a.gp"] += "\nfunc Might() (int, error) { return 1, nil }\n" +
				"func Handler() (int, error) { return Might!(), nil }\n"
		}},
		{name: "builtin macro origin", outcome: "fallback", setup: func(files map[string]string) {
			files["a/a.gp"] += "\nfunc Place() string { return #file() }\n"
		}},
		{name: "macro declaration", outcome: "fallback", setup: func(files map[string]string) {
			files["a/a.gp"] += "\nmacro Identity(v int) int { return v }\n"
		}},
		{name: "Go-only validator with untracked companion source", outcome: "proved",
			setup: func(files map[string]string) {
				// The snapshot/full validator models GP files, not C++ references.
				files["a/native.c"] = "int native_value = 1;\n"
			}},
		{name: "directive-bearing package", outcome: "fallback", setup: func(files map[string]string) {
			files["a/a.gp"] = "//gocpp:link m\n" + files["a/a.gp"]
		}},
		{name: "broken declaring package", outcome: "fallback", setup: func(files map[string]string) {
			files["a/other.gp"] = "package a\nvar Broken = undefinedName\n"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := bindingProofFiles()
			if tc.setup != nil {
				tc.setup(files)
			}
			mark := tc.mark
			if mark == "" {
				mark = "func |F("
			}
			newName := tc.newName
			if newName == "" {
				newName = "Renamed"
			}
			file := tc.file
			if file == "" {
				file = "a/a.gp"
			}
			full, fullChecks := fastRenameFiles(t, files, file, true, mark, newName)
			fast, fastChecks := fastRenameFiles(t, files, file, false, mark, newName)
			if full != fast {
				t.Fatalf("proof changes the full-validator answer\nfull:\n%s\nproof:\n%s", full, fast)
			}
			refused := strings.HasPrefix(fast, "error:")
			if refused != (tc.outcome == "refuse") {
				t.Fatalf("unexpected refusal/success: %s", fast)
			}
			if tc.outcome == "proved" && (fastChecks != 0 || fullChecks == 0) {
				t.Fatalf("proof=%d checks, full=%d; want 0 vs positive", fastChecks, fullChecks)
			}
			if tc.outcome == "fallback" && fastChecks == 0 {
				t.Fatal("unproven form did not fall back to scratch validation")
			}
		})
	}
}

func TestRenameBindingProofRequiresConflictGuards(t *testing.T) {
	skipConflictChecks = true
	defer func() { skipConflictChecks = false }()
	files := bindingProofFiles()
	full, _ := fastRenameFiles(t, files, "a/a.gp", true, "func |F(", "Renamed")
	fast, checks := fastRenameFiles(t, files, "a/a.gp", false, "func |F(", "Renamed")
	if full != fast || strings.HasPrefix(fast, "error:") || checks == 0 {
		t.Fatal("binding proof was used without the existing conflict guards")
	}
}

func bindingProofRenamer(t *testing.T) *renamer {
	t.Helper()
	files := bindingProofFiles()
	root := writeTree(t, files)
	srv := newServer(nil, testConfig(), debounce.RealClock{})
	client := &refClient{
		dclient: &dclient{client: startServer(t, srv, nil), root: root},
		src:     files,
	}
	client.call("initialize", plainInit)
	client.notify("initialized", "{}")
	client.skipShown = true
	client.open("a/a.gp", files["a/a.gp"])
	client.next("a/a.gp")
	snapshot, release := srv.session.Snapshot()
	t.Cleanup(release)
	var params protocol.TextDocumentPositionParams
	if err := json.Unmarshal([]byte(client.posParams(
		"a/a.gp", files["a/a.gp"], "func |F(")), &params); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	query, err := newRefQuery(ctx, snapshot, params.TextDocument.URI, params.Position)
	if err != nil || query == nil {
		t.Fatalf("query: %v", err)
	}
	symbol, err := query.symbolAt(ctx, true)
	if err != nil || symbol == nil {
		t.Fatalf("symbol: %v", err)
	}
	result, err := srv.search(ctx, symbol)
	if err != nil {
		t.Fatal(err)
	}
	return &renamer{
		s: srv, e: env{snapshot: snapshot}, sym: symbol, res: result,
		newName: "Renamed", docs: newDocs(snapshot),
	}
}

func TestRenameBindingProofRequiresEveryOccurrence(t *testing.T) {
	rn := bindingProofRenamer(t)
	result := *rn.res
	result.hits = slices.Clone(result.hits)
	rn.res = &result
	removed := false
	for i, h := range result.hits {
		if !h.def && h.path == rn.sym.q.file.Path {
			result.hits = append(result.hits[:i:i], result.hits[i+1:]...)
			removed = true
			break
		}
	}
	if !removed {
		t.Fatal("fixture has no own-package occurrence to omit")
	}
	recheckAllImporters = true
	defer func() { recheckAllImporters = false }()
	_, fullErr := rn.edits(context.Background())
	recheckAllImporters = false
	_, proofErr := rn.edits(context.Background())
	if fullErr == nil || proofErr == nil || fullErr.Error() != proofErr.Error() {
		t.Fatalf("missing occurrence: full=%v, proof=%v", fullErr, proofErr)
	}
}

func TestRenameBindingFactsRequireExactOrdinaryOrigin(t *testing.T) {
	rn := bindingProofRenamer(t)
	normal := rn.sym.q.view()
	if normal.pkg.IsVariant() || !normal.normalBindingFacts() {
		t.Fatal("eligible exact ordinary facts were disabled")
	}
	// Warm the immutable package memo, then reject each view-specific mismatch before consulting it.
	t.Run("inexact info", func(t *testing.T) {
		inexact := *normal
		info := *normal.info
		inexact.info = &info
		if inexact.normalBindingFacts() {
			t.Fatal("inexact type information reused normal facts")
		}
	})
	t.Run("inexact file set", func(t *testing.T) {
		inexact := *normal
		inexact.fset = token.NewFileSet()
		if inexact.normalBindingFacts() {
			t.Fatal("inexact file positions reused normal facts")
		}
	})
	t.Run("different syntax view", func(t *testing.T) {
		inexact := *normal
		inexact.files = make(map[string]*ast.File, len(normal.files))
		for path, file := range normal.files {
			inexact.files[path] = file
		}
		inexact.files[rn.sym.q.file.Path] = new(ast.File)
		if inexact.normalBindingFacts() {
			t.Fatal("a different syntax view reused normal facts")
		}
	})
	path := rn.sym.q.file.Path
	src, err := rn.e.snapshot.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		text []byte
	}{
		{"identical replacement bytes", append([]byte(nil), src...)},
		{"complete plain variant", append(append([]byte(nil), src...), []byte("\n// patched input\n")...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for range 2 { // fresh computation and cached hit must retain the same immutable origin
				pkg, err := rn.e.snapshot.PackageWithFile(context.Background(), path, tc.text)
				if err != nil {
					t.Fatal(err)
				}
				if !pkg.IsVariant() || viewOf(pkg).normalBindingFacts() {
					t.Fatal("a complete plain patched package entered the ordinary binding proof")
				}
			}
		})
	}
	if !normal.normalBindingFacts() {
		t.Fatal("negative provenance checks poisoned the eligible ordinary package memo")
	}
}

func TestRenameBindingProofCancellation(t *testing.T) {
	rn := bindingProofRenamer(t)
	defer func() { recheckAllImporters = false }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, all := range []bool{true, false} {
		recheckAllImporters = all
		_, err := rn.edits(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("full=%v: got %v, want context.Canceled", all, err)
		}
	}
	recheckAllImporters = false
}

// TestRenameQualifiedOnlyShortcut: the answer of a rename is what it is without
// the shortcut, also with the conflict checks off (then the check of the result
// is the only thing that refuses an error), and it checks fewer packages.
func TestRenameQualifiedOnlyShortcut(t *testing.T) {
	for _, noConflicts := range []bool{false, true} {
		t.Run(fmt.Sprintf("conflicts-off=%v", noConflicts), func(t *testing.T) {
			skipConflictChecks = noConflicts
			defer func() { skipConflictChecks = false }()
			testQualifiedOnly(t)
		})
	}
}

func testQualifiedOnly(t *testing.T) {
	for _, tc := range []struct {
		mark, name string
		fewer      bool // the shortcut checks fewer packages
	}{
		{"func |F()", "Zed", false}, // g dot-imports a and has Zed: it has no use of F, and still breaks
		{"var |V = 2", "Zed", false},
		{"func |F()", "G", true},          // b, c, e use it by a qualified name; d (dot) and a are checked
		{"var |V = 2", "W", true},         // a variable
		{"const |K = 3", "L", true},       // a constant
		{"func |F()", "g", false},         // not exported: the importers can't use it
		{"func |F()", "V", false},         // the name is taken in a (an error there)
		{"func |F()", "G", true},          // (again, in a fresh server)
		{"type |T struct", "U", false},    // a type: not shortcut
		{"func (t T) |M()", "N", false},   // a method: not shortcut
		{"func inA|() int", "inB", false}, // unexported: no importer
		{"var |V = 2", "F", false},        // clashes with F in a
		{"const |K = 3", "K2", true},      // a constant, another name
		{"func |F()", "Fn", true},         // f has Fn, but it uses a.F qualified: fine in f, and a is fine
		{"func |F()", "TypeName", true},   // any other name
		{"func |F()", "Call", true},       // b has Call: qualified use in b is fine
		{"var |V = 2", "Use", true},       // c has Use
		{"const |K = 3", "Dot", false},    // d dot-imports a and has Dot: the dot import clashes only if it is checked
		{"func |F()", "Alias", false},     // e has Alias, d dot-imports a: the dot-importer sees both
		{"var |V = 2", "Dot", false},      // the same for a variable
		{"func |F()", "x", false},         // lowercase
		{"const |K = 3", "NoSuchName", true},
		{"func |F()", "MixedFunc", false}, // qualified uses and a dot import are in different files
		{"var |V = 2", "MixedVar", false},
		{"const |K = 3", "MixedConst", false},
		{"func |F()", "MixedSafe", true}, // mixed imports without a clash still give the same edits
	} {
		t.Run(fmt.Sprintf("%s->%s", strings.Trim(tc.mark, "|"), tc.name), func(t *testing.T) {
			full, nFull := fastRename(t, true, tc.mark, tc.name)
			fast, nFast := fastRename(t, false, tc.mark, tc.name)
			mixedClash := tc.name == "MixedFunc" || tc.name == "MixedVar" || tc.name == "MixedConst"
			if mixedClash && !strings.HasPrefix(full, "error:") {
				t.Fatal("mixed-file dot-import conflict was not refused")
			}
			t.Logf("without: %d checks; with: %d checks; %.60q", nFull, nFast, fast)
			if full != fast {
				t.Errorf("the shortcut changes the answer\nwithout:\n%s\nwith:\n%s", full, fast)
			}
			if tc.fewer && nFast >= nFull {
				t.Errorf("%d package checks with the shortcut, %d without: want fewer", nFast, nFull)
			}
			if !tc.fewer && (nFast > nFull || strings.Contains(tc.mark, "type |") && nFast != nFull) {
				t.Errorf("%d package checks with the shortcut, %d without", nFast, nFull)
			}
		})
	}
}
