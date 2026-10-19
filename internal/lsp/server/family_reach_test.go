package server

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// The dynamic family of a rename (dynamic.go) reaches the types that a type
// assertion can take a value of out, wherever the assertion is, and only
// those: a type that no value of is converted to an interface anywhere cannot
// be the dynamic value of one.

// manyTypes is a module of n packages, each with a type T that has Close,
// Len and String (as the types of a real module do), and core, which asserts
// Closer on an any. mainBody is the body of main.
func manyTypes(n int, mainImports []int, mainBody string) map[string]string {
	files := map[string]string{
		"gocpp.mod": "module example.com/ff\n",
		"core/core.gp": `package core

type Closer interface{ Close() error }

func Shut(c Closer) error { return c.Close() }

func IsCloser(x any) bool { _, ok := x.(Closer); return ok }
`,
	}
	for i := range n {
		files[fmt.Sprintf("p%02d/p.gp", i)] = fmt.Sprintf(`package p%02d

type T struct{ n int }

func (t *T) Close() error { t.n = 0; return nil }
func (t *T) Len() int     { return t.n }
func (t *T) String() string { return "t" }

func Use(t *T) int { return t.Len() + len(t.String()) }
`, i)
	}
	imp := "\t\"example.com/ff/core\"\n"
	for _, i := range mainImports {
		imp += fmt.Sprintf("\t\"example.com/ff/p%02d\"\n", i)
	}
	files["cmd/main.gp"] = "package main\n\nimport (\n\t\"fmt\"\n\n" + imp + ")\n\nfunc main() {\n" + mainBody + "}\n"
	return files
}

func editedFiles(got map[string]string) []string {
	var out []string
	for f := range got {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// TestDynamicFamilyReaches: renaming p05.T.Close, which main hands to an any
// that core asserts on, renames the Closer's method and the types that are
// converted to an interface (p00.T goes to core.Shut), and none of the 28
// others, which have Close() error too.
func TestDynamicFamilyReaches(t *testing.T) {
	files := manyTypes(30, []int{0, 5}, `	a := &p00.T{}
	var x any = &p05.T{}
	_, ok := x.(core.Closer)
	fmt.Println(ok, core.Shut(a), core.IsCloser(a), core.IsCloser(&p05.T{}))
`)
	c := startRefs(t, files, plainInit)
	got, rerr := c.rename("p05/p.gp", "(t *T) Cl|ose()", "Shut2")
	if rerr != nil {
		t.Fatal(rerr)
	}
	if want := []string{"core/core.gp", "p00/p.gp", "p05/p.gp"}; strings.Join(editedFiles(got), " ") != strings.Join(want, " ") {
		t.Errorf("edited files %v, want %v", editedFiles(got), want)
	}
	if strings.Count(got["core/core.gp"], "Shut2") != 2 {
		t.Errorf("core.gp:\n%s", got["core/core.gp"])
	}
	// A type that nothing converts to an interface: itself only.
	c = startRefs(t, files, plainInit)
	got, rerr = c.rename("p17/p.gp", "(t *T) Cl|ose()", "Shut2")
	if rerr != nil || len(got) != 1 {
		t.Errorf("p17.T.Close: edited %v, error %v; want its own file only", editedFiles(got), rerr)
	}
	// References and highlights do not follow assertions: the declaration of the
	// method of the type, and of nothing else.
	c = startRefs(t, files, plainInit)
	if refs := c.references("p05/p.gp", "(t *T) Cl|ose()", true); len(refs) != 1 {
		t.Errorf("references of p05.T.Close: %q", refs)
	}
	if refs := c.references("core/core.gp", "interface{ Cl|ose", true); len(refs) != 3 {
		t.Errorf("references of Closer.Close: %q, want the declaration, the call in Shut and the types converted to Closer (p00.T)", refs)
	}
}

// TestDynamicFamilyAssertionElsewhere: the assertion is in core, which does
// not import the package of the type; main only passes a value to it. The
// rename still renames the Closer's method (the assertion in core takes the
// value out), and a type that main does not pass is left alone.
func TestDynamicFamilyAssertionElsewhere(t *testing.T) {
	files := manyTypes(30, []int{0, 5}, `	a := &p00.T{}
	fmt.Println(core.Shut(a), core.IsCloser(a), core.IsCloser(&p05.T{}))
`)
	c := startRefs(t, files, plainInit)
	got, rerr := c.rename("p05/p.gp", "(t *T) Cl|ose()", "Shut2")
	if rerr != nil {
		t.Fatal(rerr)
	}
	if want := []string{"core/core.gp", "p00/p.gp", "p05/p.gp"}; strings.Join(editedFiles(got), " ") != strings.Join(want, " ") {
		t.Errorf("edited files %v, want %v", editedFiles(got), want)
	}
	// The same from the interface: the types converted to an interface in main.
	c = startRefs(t, files, plainInit)
	got, rerr = c.rename("core/core.gp", "interface{ Cl|ose", "Shut2")
	if rerr != nil {
		t.Fatal(rerr)
	}
	if want := []string{"core/core.gp", "p00/p.gp", "p05/p.gp"}; strings.Join(editedFiles(got), " ") != strings.Join(want, " ") {
		t.Errorf("from the interface: edited files %v, want %v", editedFiles(got), want)
	}
}

// TestDynamicFamilyLimit: a family that the assertion would make larger than the
// limit is refused, with the count and the way to go on; with the limit lifted,
// it is renamed.
func TestDynamicFamilyLimit(t *testing.T) {
	all := make([]int, 30)
	var body strings.Builder
	for i := range all {
		all[i] = i
		fmt.Fprintf(&body, "\tvar v%[1]d any = &p%02[1]d.T{}\n\t_ = v%[1]d\n", i)
	}
	body.WriteString("\tfmt.Println(core.IsCloser(v0))\n")
	files := manyTypes(30, all, body.String())
	c := startRefs(t, files, plainInit)
	_, msg := c.renamed(t, "core/core.gp", "interface{ Cl|ose", "Shut2")
	for _, want := range []string{"cannot rename Close without asking", "30 methods", "core.gp:", "gocpp.dynamicLimit", "-dynamic-limit"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal %q lacks %q", msg, want)
		}
	}
	c.s.dynLimitOpt.Store(-1)
	got, msg := c.renamed(t, "core/core.gp", "interface{ Cl|ose", "Shut2")
	if msg != "" || len(got) != 31 {
		t.Errorf("with no limit: %d files, %q; want 31 files", len(got), msg)
	}
	c.s.dynLimitOpt.Store(30)
	if _, msg := c.renamed(t, "core/core.gp", "interface{ Cl|ose", "Shut2"); msg != "" {
		t.Errorf("limit 30 for 30 methods: %q", msg)
	}
	// The wide family is not limited (it is dev's, as it was).
	c.s.dynLimitOpt.Store(0)
}

// TestDynamicIndexFollowsEdits: the syntax index of the files (which have an
// assertion, which declare an interface with a method) is for the content of
// each file: an assertion that is added by an edit of an open document is seen
// by the next rename, as a fresh server would see it.
func TestDynamicIndexFollowsEdits(t *testing.T) {
	files := manyTypes(6, []int{0, 1}, `	a := &p00.T{}
	fmt.Println(core.Shut(a), &p01.T{})
`)
	// core has no assertion yet.
	files["core/core.gp"] = `package core

type Closer interface{ Close() error }

func Shut(c Closer) error { return c.Close() }
`
	c := startRefs(t, files, plainInit)
	got, rerr := c.rename("p01/p.gp", "(t *T) Cl|ose()", "Shut2")
	if rerr != nil || len(got) != 1 {
		t.Fatalf("no assertion: edited %v, %v; want p01 only", editedFiles(got), rerr)
	}
	// An assertion appears (the text of the open document, not saved).
	c.open("core/core.gp", `package core

type Closer interface{ Close() error }

func Shut(c Closer) error { return c.Close() }

func IsCloser(x any) bool { _, ok := x.(Closer); return ok }
`)
	got, rerr = c.rename("p01/p.gp", "(t *T) Cl|ose()", "Shut2")
	if want := []string{"core/core.gp", "p00/p.gp", "p01/p.gp"}; rerr != nil || strings.Join(editedFiles(got), " ") != strings.Join(want, " ") {
		t.Errorf("with the assertion: edited %v, %v; want %v", editedFiles(got), rerr, want)
	}
	// And gone again.
	c.change("core/core.gp", 2, files["core/core.gp"])
	got, rerr = c.rename("p01/p.gp", "(t *T) Cl|ose()", "Shut2")
	if rerr != nil || len(got) != 1 {
		t.Errorf("without it again: edited %v, %v", editedFiles(got), rerr)
	}
}

// TestDynamicTypeSwitchAndImporterOfInterface: the assertion is a type switch,
// in a package that imports the interface's package (not declares it) and not
// the type's; the type is converted in main.
func TestDynamicTypeSwitchAndImporterOfInterface(t *testing.T) {
	files := manyTypes(4, []int{0, 1}, `	fmt.Println(chk.IsCloser(&p01.T{}), core.Shut(&p00.T{}))
`)
	files["core/core.gp"] = `package core

type Closer interface{ Close() error }

func Shut(c Closer) error { return c.Close() }
`
	files["chk/chk.gp"] = `package chk

import "example.com/ff/core"

func IsCloser(x any) bool {
	switch x.(type) {
	case core.Closer:
		return true
	}
	return false
}
`
	files["cmd/main.gp"] = strings.Replace(files["cmd/main.gp"], "\t\"example.com/ff/core\"\n", "\t\"example.com/ff/chk\"\n\t\"example.com/ff/core\"\n", 1)
	c := startRefs(t, files, plainInit)
	got, rerr := c.rename("p01/p.gp", "(t *T) Cl|ose()", "Shut2")
	if want := []string{"core/core.gp", "p00/p.gp", "p01/p.gp"}; rerr != nil || strings.Join(editedFiles(got), " ") != strings.Join(want, " ") {
		t.Errorf("edited %v, %v; want %v", editedFiles(got), rerr, want)
	}
}

// TestDynamicSignatureMatters: a type with a Close of another signature does not
// implement the asserted interface, so it is not in the family, converted or not.
func TestDynamicSignatureMatters(t *testing.T) {
	files := manyTypes(3, []int{0, 1, 2}, `	fmt.Println(core.IsCloser(&p00.T{}), core.IsCloser(&p01.T{}), core.IsCloser(&p02.T{}))
`)
	files["p01/p.gp"] = `package p01

type T struct{ n int }

func (t *T) Close() int { return t.n }
`
	c := startRefs(t, files, plainInit)
	got, rerr := c.rename("p00/p.gp", "(t *T) Cl|ose()", "Shut2")
	if want := []string{"core/core.gp", "p00/p.gp", "p02/p.gp"}; rerr != nil || strings.Join(editedFiles(got), " ") != strings.Join(want, " ") {
		t.Errorf("edited %v, %v; want %v (not p01: its Close returns an int)", editedFiles(got), rerr, want)
	}
	c = startRefs(t, files, plainInit)
	got, rerr = c.rename("core/core.gp", "interface{ Cl|ose", "Shut2")
	if want := []string{"core/core.gp", "p00/p.gp", "p02/p.gp"}; rerr != nil || strings.Join(editedFiles(got), " ") != strings.Join(want, " ") {
		t.Errorf("from the interface: edited %v, %v; want %v", editedFiles(got), rerr, want)
	}
}

// TestDynamicLimitSettings: the limit is a setting of the client too: the
// initializationOptions, and workspace/didChangeConfiguration (flat or under
// "gocpp").
func TestDynamicLimitSettings(t *testing.T) {
	var body strings.Builder
	all := make([]int, 25)
	for i := range all {
		all[i] = i
		fmt.Fprintf(&body, "\tvar v%[1]d any = &p%02[1]d.T{}\n\t_ = v%[1]d\n", i)
	}
	body.WriteString("\tfmt.Println(core.IsCloser(v0))\n")
	files := manyTypes(25, all, body.String())
	c := startRefs(t, files, plainInit)
	rename := func() string { _, msg := c.renamed(t, "core/core.gp", "interface{ Cl|ose", "Shut2"); return msg }
	if msg := rename(); !strings.Contains(msg, "25 methods") || !strings.Contains(msg, "gocpp.dynamicLimit") {
		t.Fatalf("default limit 20, 25 methods: %q", msg)
	}
	c.notify("workspace/didChangeConfiguration", `{"settings":{"gocpp":{"dynamicLimit":25}}}`)
	c.call("test/sync", "")
	if msg := rename(); msg != "" {
		t.Errorf("limit 25 from the settings: %q", msg)
	}
	c.notify("workspace/didChangeConfiguration", `{"settings":{"dynamicLimit":3}}`)
	c.call("test/sync", "")
	if msg := rename(); !strings.Contains(msg, "25 methods") {
		t.Errorf("limit 3 (flat settings): %q", msg)
	}
	c.notify("workspace/didChangeConfiguration", `{"settings":{"dynamicLimit":-1}}`)
	c.call("test/sync", "")
	if msg := rename(); msg != "" {
		t.Errorf("no limit: %q", msg)
	}
	// initializationOptions
	c2 := startRefs(t, files, `{"processId":1,"rootUri":null,"capabilities":{},"initializationOptions":{"dynamicLimit":-1}}`)
	if _, msg := c2.renamed(t, "core/core.gp", "interface{ Cl|ose", "Shut2"); msg != "" {
		t.Errorf("no limit from the initializationOptions: %q", msg)
	}
}

// TestDynamicAssertionInTestFile: the assertion is in a test file of core (a
// helper of its tests) and the types are converted in main: the test file
// counts (the file does not mention the method's name, only the interface's).
func TestDynamicAssertionInTestFile(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/tf\n",
		"core/core.gp": `package core

type Stopper interface{ Halt() error }

func Stop(s Stopper) error { return s.Halt() }
`,
		"core/core_test.gp": `package core

func isStopper(x any) bool { _, ok := x.(Stopper); return ok }

func useIt() bool { return isStopper(nil) }
`,
		"p0/p.gp": "package p0\n\ntype T struct{}\n\nfunc (*T) Halt() error { return nil }\n",
		"p1/p.gp": "package p1\n\ntype T struct{}\n\nfunc (*T) Halt() error { return nil }\n",
		"cmd/main.gp": `package main

import (
	"fmt"

	"example.com/tf/core"
	"example.com/tf/p0"
	"example.com/tf/p1"
)

func main() { fmt.Println(core.Stop(&p0.T{}), &p1.T{}) }
`,
	}
	c := startRefs(t, files, plainInit)
	got, rerr := c.rename("p1/p.gp", "(*T) Ha|lt()", "Stop2")
	if want := []string{"core/core.gp", "p0/p.gp", "p1/p.gp"}; rerr != nil || strings.Join(editedFiles(got), " ") != strings.Join(want, " ") {
		t.Errorf("edited %v, %v; want %v", editedFiles(got), rerr, want)
	}
}

// TestRenameChecksPackagesThatUseTheNewName: the check of the result leaves out
// the packages that cannot be affected, but not one that uses the new name,
// which a rename can make ambiguous without touching the package: W embeds
// *p.T and O, and calls w.Shut(), which is O's until p.T gets a Shut too.
func TestRenameChecksPackagesThatUseTheNewName(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/nn\n",
		"p/p.gp":    "package p\n\ntype T struct{}\n\nfunc (*T) Close() error { return nil }\n",
		"q/q.gp": `package q

import "example.com/nn/p"

type O struct{}

func (O) Shut() {}

type W struct {
	*p.T
	O
}

func Use() { W{}.Shut() }
`,
	}
	c := startRefs(t, files, plainInit)
	_, msg := c.renamed(t, "p/p.gp", "(*T) Cl|ose()", "Shut")
	if !strings.Contains(msg, "q.W embeds") && (!strings.Contains(msg, "it would make an error") || !strings.Contains(msg, "q.gp")) {
		t.Errorf("rename to a name that q uses on a type that embeds the type: %q", msg)
	}
	// To another name, no package is affected.
	if got, msg := c.renamed(t, "p/p.gp", "(*T) Cl|ose()", "Stop"); msg != "" || len(got) != 1 {
		t.Errorf("rename to Stop: %v, %q", editedFiles(got), msg)
	}
}

// TestRenameLiteralInterfaceMethodChecksAll: the method of an interface literal
// belongs to every identical literal: a package that never names it can break
// (b.Show takes the interface{ M() int } that a.Ifc returns), so the check of
// the result does not leave it out, and the rename is refused.
func TestRenameLiteralInterfaceMethodChecksAll(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/lit2\n",
		"a/a.gp": `package a

type impl struct{}

func (impl) M() int { return 1 }

func Ifc() interface{ M() int } { return impl{} }
`,
		"b/b.gp": `package b

func Show(i interface{ M() int }) int { return i.M() }
`,
		"main.gp": `package main

import (
	"example.com/lit2/a"
	"example.com/lit2/b"
)

func main() { println(b.Show(a.Ifc())) }
`,
	}
	c := startRefs(t, files, plainInit)
	_, msg := c.renamed(t, "a/a.gp", "Ifc() interface{ |M() int }", "N")
	if !strings.Contains(msg, "it would make an error") {
		t.Errorf("rename of the method of an interface literal: %q, want a refusal (main.gp stops compiling)", msg)
	}
}

// The method set of a type that embeds the type of the renamed method is made by
// promotion: the rename can give it an ambiguous method or hide one, in a
// package that never names the method, and the embedder stops implementing an
// interface (a compile error) or calls another method (none).
func promotionFiles(extra map[string]string) map[string]string {
	files := map[string]string{
		"gocpp.mod":  "module example.com/mb\n",
		"a/a.gp":     "package a\n\ntype A struct{}\n\nfunc (A) M() int { return 1 }\n",
		"b/b.gp":     "package b\n\ntype B struct{}\n\nfunc (B) N() int { return 2 }\n",
		"lib/lib.gp": "package lib\n\ntype I interface{ N() int }\n\nfunc Use(i I) int { return i.N() }\n",
	}
	for k, v := range extra {
		files[k] = v
	}
	return files
}

// TestRenamePromotionAmbiguity (the reviewer's mb1): W embeds a.A and b.B and
// is passed to lib.Use, which wants N(); c does not name N. Renaming A.M to N
// makes W.N ambiguous: refused.
func TestRenamePromotionAmbiguity(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"embedder and use in one package": promotionFiles(map[string]string{
			"c/c.gp": "package c\n\nimport (\n\t\"example.com/mb/a\"\n\t\"example.com/mb/b\"\n\t\"example.com/mb/lib\"\n)\n\ntype W struct {\n\ta.A\n\tb.B\n}\n\nfunc Run() int { return lib.Use(W{}) }\n",
		}),
		"embedder in one package, the use in another": promotionFiles(map[string]string{
			"c/c.gp": "package c\n\nimport (\n\t\"example.com/mb/a\"\n\t\"example.com/mb/b\"\n)\n\ntype W struct {\n\ta.A\n\tb.B\n}\n",
			"d/d.gp": "package d\n\nimport (\n\t\"example.com/mb/c\"\n\t\"example.com/mb/lib\"\n)\n\nfunc Run() int { return lib.Use(c.W{}) }\n",
		}),
		"embedder of the embedder": promotionFiles(map[string]string{
			"c/c.gp": "package c\n\nimport (\n\t\"example.com/mb/a\"\n\t\"example.com/mb/b\"\n)\n\ntype W struct {\n\ta.A\n\tb.B\n}\n",
			"d/d.gp": "package d\n\nimport \"example.com/mb/c\"\n\ntype V struct{ c.W }\n\nfunc (v V) Show() int { return 1 }\n",
			"e/e.gp": "package e\n\nimport (\n\t\"example.com/mb/d\"\n\t\"example.com/mb/lib\"\n)\n\nfunc Run() int { return lib.Use(d.V{}) }\n",
		}),
	} {
		for _, skip := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s (embedder check skipped: %v)", name, skip), func(t *testing.T) {
				skipEmbedderCheck = skip
				defer func() { skipEmbedderCheck = false }()
				c := startRefs(t, files, plainInit)
				got, msg := c.renamed(t, "a/a.gp", "(A) |M()", "N")
				if msg == "" {
					t.Errorf("renamed: %v, want a refusal (W.N would be ambiguous)", editedFiles(got))
				}
				if skip && !strings.Contains(msg, "it would make an error") {
					t.Errorf("the check of the result alone: %q, want the error of the program that does not compile", msg)
				}
			})
		}
	}
}

// TestRenamePromotionHides: the new name is a field or a method that the embedder
// already has, at the depth of the method (ambiguous), above it (the embedder
// loses the method) or below it (the embedder calls this method now): refused
// whatever the package does with it; a name that nothing in an embedder has is
// renamed, and so is a type that nothing embeds.
func TestRenamePromotionHides(t *testing.T) {
	for name, extra := range map[string]string{
		"same depth":     "type W struct {\n\ta.A\n\tb.B\n}\n",
		"field above":    "type W struct {\n\ta.A\n\tN int\n}\n",
		"method above":   "type W struct{ a.A }\n\nfunc (W) N() int { return 3 }\n",
		"deeper":         "type X struct{ b.B }\n\ntype W struct {\n\ta.A\n\tX\n}\n",
		"interface":      "type J interface {\n\tI\n\tM() int\n}\n",
		"through a type": "type X struct{ a.A }\n\ntype W struct {\n\tX\n\tb.B\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			src := "package c\n\nimport (\n\t\"example.com/mb/a\"\n\t\"example.com/mb/b\"\n\t\"example.com/mb/lib\"\n)\n\nvar _ = a.A{}\nvar _ = b.B{}\nvar _ lib.I\n\n" + extra
			if name == "interface" {
				src = "package c\n\nimport \"example.com/mb/lib\"\n\ntype I = lib.I\n\n" + extra
				// The interface embeds lib.I, whose N is renamed.
				files := promotionFiles(map[string]string{"c/c.gp": src})
				c := startRefs(t, files, plainInit)
				if _, msg := c.renamed(t, "lib/lib.gp", "I interface{ |N() int }", "M"); msg == "" {
					t.Errorf("renamed lib.I.N to M where J has M already")
				}
				return
			}
			c := startRefs(t, promotionFiles(map[string]string{"c/c.gp": src}), plainInit)
			if got, msg := c.renamed(t, "a/a.gp", "(A) |M()", "N"); msg == "" {
				t.Errorf("renamed A.M to N: %v, want a refusal (%s)", editedFiles(got), name)
			}
		})
	}
	// Nothing in an embedder has Q.
	src := "package c\n\nimport (\n\t\"example.com/mb/a\"\n\t\"example.com/mb/b\"\n)\n\ntype W struct {\n\ta.A\n\tb.B\n}\n"
	c := startRefs(t, promotionFiles(map[string]string{"c/c.gp": src}), plainInit)
	if got, msg := c.renamed(t, "a/a.gp", "(A) |M()", "Q"); msg != "" || len(got) != 1 {
		t.Errorf("rename to Q: %v, %q", editedFiles(got), msg)
	}
}

// TestCheckCacheSetting: the cap of the cache of checked packages is a setting
// (checkCacheMB, flat or under "gocpp"), and 0 is the default.
func TestCheckCacheSetting(t *testing.T) {
	c := startRefs(t, manyTypes(2, nil, ""), plainInit)
	maxBytes := func() int64 { return c.s.pipeline.Metrics().Caches["check"].MaxBytes }
	def := maxBytes()
	if def < 64<<20 || def > 300<<20 {
		t.Fatalf("default cap %d", def)
	}
	c.notify("workspace/didChangeConfiguration", `{"settings":{"gocpp":{"checkCacheMB":128}}}`)
	c.call("test/sync", "")
	if got := maxBytes(); got != 128<<20 {
		t.Errorf("cap %d after the setting, want 128 MB", got)
	}
	c.notify("workspace/didChangeConfiguration", `{"settings":{"checkCacheMB":0}}`)
	c.call("test/sync", "")
	if got := maxBytes(); got != def {
		t.Errorf("cap %d after 0, want the default %d", got, def)
	}
}

// TestRenameWarnsOfTheNameAsString: reflect finds a method by a string, which a
// rename leaves: the user is told where such a string is.
func TestRenameWarnsOfTheNameAsString(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/str\n",
		"a/a.gp": `package a

import "reflect"

type T struct{}

func (*T) Close() error { return nil }

func Call(v any) { reflect.ValueOf(v).MethodByName("Close").Call(nil) }
`,
	}
	c := startRefs(t, files, plainInit)
	if _, msg := c.renamed(t, "a/a.gp", "(*T) Cl|ose()", "Shut"); msg != "" {
		t.Fatal(msg)
	}
	found := false
	for _, m := range c.takeShown() {
		found = found || strings.Contains(m.Message, `the string "Close" is at a.gp:9`)
	}
	if !found {
		t.Error("no warning about the string")
	}
	// A name that no string has: no warning.
	c = startRefs(t, files, plainInit)
	if _, msg := c.renamed(t, "a/a.gp", "(*T) Cl|ose()", "Shut"); msg != "" {
		t.Fatal(msg)
	}
}
