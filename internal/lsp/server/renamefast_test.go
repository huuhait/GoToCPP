package server

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/debounce"
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
	recheckAllImporters = all
	defer func() { recheckAllImporters = false }()
	root := writeTree(t, fastFiles)
	srv := newServer(nil, testConfig(), debounce.RealClock{})
	c := &refClient{dclient: &dclient{client: startServer(t, srv, nil), root: root}, src: fastFiles}
	c.call("initialize", plainInit)
	c.notify("initialized", "{}")
	c.skipShown = true
	c.open("a/a.gp", fastFiles["a/a.gp"])
	c.next("a/a.gp")
	// The searches check the importers once; the rename's own checks are counted.
	if _, err := c.rename("a/a.gp", mark, newName+"Warm"); err != nil {
		_ = err // (a refusal also leaves the importers checked)
	}
	_, _, before := srv.pipeline.Stats()
	edits, rerr := c.rename("a/a.gp", mark, newName)
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
