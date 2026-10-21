package server

import (
	"context"
	"go/types"
	"maps"
	"path/filepath"
	"strings"
	"testing"
)

func memberProofFiles() map[string]string {
	return map[string]string{
		"gocpp.mod": "module example.com/members\n",
		"a/a.gp": `package a

type Item struct {
	Title string
	Count int
	inner int
}

func (i Item) Name() string { return i.Title }

func (i *Item) Bump() { i.Count++ }

type Other struct {
	Title string
}

func Make() Item { return Item{Title: "x", Count: 1} }

type Shape struct {
	Width, Height int
	hid           int
}

type Twin struct {
	Width, Height int
	hid           int
}

func Convert(s Shape) Twin { return Twin(s) }

type Pair struct{ Left, Right int }

type Mirror struct {
	Near2, Far2 int
	hid         int
}

type Image struct {
	Near2, Far2 int
	hid         int
}

type Near struct {
	Alpha, Gamma int
	hid          int
}

type Lone struct {
	Alpha, Beta int
	hid         int
}

type Base struct{ ID int }

func (b Base) Describe() string { return "base" }

type Derived struct {
	Base
	Label string
}

type Box[P any] struct{ Value P }

func Anonymous() int {
	v := struct{ Field int }{1}
	return v.Field
}

type Speaker interface{ Speak() string }

type Dog struct{}

func (Dog) Speak() string { return "woof" }

func (Dog) Wag() int { return 1 }

func Talk(s Speaker) string { return s.Speak() }

func UseDog() string { return Talk(Dog{}) }
`,
		"b/b.gp": `package b

import "example.com/members/a"

func Read(i a.Item) string { return i.Title + i.Name() }

var anon struct{ Left, Right int }

func Left() int { return anon.Left }
`,
	}
}

// TestRenameMemberProof: the rename of a field or a method gives the answer of
// the check of the result, and the forms the proofs cover make no scratch check.
func TestRenameMemberProof(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setup   func(map[string]string)
		mark    string
		newName string
		outcome string
	}{
		{name: "field used by an importer", mark: "|Title string\n\tCount", newName: "Heading",
			outcome: "proved"},
		{name: "field of a struct of exported fields only", mark: "|Title string\n}\n\nfunc Make",
			newName: "Heading", outcome: "fallback"},
		{name: "second field", mark: "|Count int", newName: "Total", outcome: "proved"},
		{name: "unexported field", mark: "|inner int", newName: "core", outcome: "proved"},
		{name: "field of the type of a method", mark: "|Title string\n\tCount", newName: "Name",
			outcome: "refuse"},
		{name: "field renamed to another field", mark: "|Title string\n\tCount", newName: "Count",
			outcome: "refuse"},
		{name: "field of two identical structs", mark: "|Width, Height", newName: "Across",
			outcome: "refuse"},
		{name: "field of a struct that an importer assigns to its own literal",
			mark: "|Title string\n}\n\nfunc Make", newName: "Heading", outcome: "refuse",
			setup: func(files map[string]string) {
				files["b/b.gp"] += "\nfunc Copy(o a.Other) string {\n" +
					"\tvar x struct{ Title string } = o\n\treturn x.Title\n}\n"
			}},
		{name: "field of a struct that an importer writes by its names", mark: "|Left, Right",
			newName: "Near", outcome: "fallback"},
		{name: "field of a struct that another named struct has the names of",
			mark: "|Near2, Far2", newName: "Close2", outcome: "proved"},
		{name: "field renamed to the names of another named struct", mark: "|Gamma int",
			newName: "Beta", outcome: "proved"},
		{name: "field of a struct converted to a pointer of another", mark: "|Near2, Far2",
			newName: "Close2", outcome: "refuse", setup: func(files map[string]string) {
				files["a/a.gp"] += "\nfunc Reflect(m *Mirror) *Image { return (*Image)(m) }\n"
			}},
		{name: "field of a struct converted by a type parameter", mark: "|Near2, Far2",
			newName: "Close2", outcome: "refuse", setup: func(files map[string]string) {
				files["a/a.gp"] += "\nfunc Generic[T interface{ Mirror | Image }](m Mirror) T {\n" +
					"\treturn T(m)\n}\n"
			}},
		{name: "field of a struct whose pointer type is made of nil", mark: "|Count int",
			newName: "Total", outcome: "proved", setup: func(files map[string]string) {
				files["a/a.gp"] += "\nvar _ = (*Item)(nil)\n\nvar _ = Item(Item{})\n"
			}},
		{name: "field of a struct that a named type is declared as", mark: "|Count int",
			newName: "Total", outcome: "fallback", setup: func(files map[string]string) {
				files["a/a.gp"] += "\ntype Holder Item\n\ntype Boxed struct{ Holder }\n"
			}},
		{name: "field of an embedded type", mark: "|ID int", newName: "Ident",
			outcome: "fallback"},
		{name: "field of a type embedded by an alias", mark: "|Count int", newName: "Total",
			outcome: "fallback", setup: func(files map[string]string) {
				files["a/a.gp"] += "\ntype Alias = Item\n\ntype Wrapper struct{ Alias }\n"
			}},
		{name: "field of a generic struct", mark: "|Value P", newName: "Content",
			outcome: "fallback"},
		{name: "field of an anonymous struct", mark: "|Field int", newName: "Part",
			outcome: "fallback"},
		{name: "field to an unexported name", mark: "|Title string\n\tCount", newName: "title",
			outcome: "refuse"},
		{name: "field used by a test file", mark: "|Count int", newName: "Total",
			outcome: "fallback", setup: func(files map[string]string) {
				files["a/a_test.gp"] = "package a\n\nfunc helper() int { return Item{}.Count }\n"
			}},
		{name: "method of no interface", mark: "func (Dog) |Wag()", newName: "Wave",
			outcome: "proved"},
		{name: "method with a pointer receiver", mark: "func (i *Item) |Bump()", newName: "Raise",
			outcome: "proved"},
		{name: "method used by an importer", mark: "func (i Item) |Name()", newName: "Called",
			outcome: "proved"},
		{name: "method of an interface", mark: "|Speak() string }", newName: "Say",
			outcome: "proved"},
		{name: "method that an interface needs", mark: "func (Dog) |Speak()", newName: "Say",
			outcome: "proved"},
		{name: "method of an interface beside an unrelated interface of the same method",
			mark: "|Speak() string }", newName: "Say", outcome: "proved",
			setup: func(files map[string]string) {
				files["b/b.gp"] += "\ntype Barker interface{ Speak() string }\n\ntype Cat struct{}\n\n" +
					"func (Cat) Speak() string { return \"meow\" }\n\n" +
					"func Hear(b Barker) string { return b.Speak() }\n\n" +
					"func UseCat() string { return Hear(Cat{}) }\n"
			}},
		{name: "method of an interface that an importer calls", mark: "|Speak() string }",
			newName: "Say", outcome: "proved", setup: func(files map[string]string) {
				files["b/b.gp"] += "\nfunc Run() string { return Hear(a.Dog{}) }\n\n" +
					"func Hear(s a.Speaker) string { return s.Speak() }\n"
			}},
		{name: "method of an interface that an importer's second interface needs",
			mark: "func (Dog) |Speak()", newName: "Say", outcome: "proved",
			setup: func(files map[string]string) {
				files["b/b.gp"] += "\ntype Barker interface{ Speak() string }\n\n" +
					"func Hear(b Barker) string { return b.Speak() }\n\n" +
					"func Run() string { return Hear(a.Dog{}) }\n"
			}},
		{name: "method of an interface that an embedder has", mark: "|Speak() string }",
			newName: "Say", outcome: "proved", setup: func(files map[string]string) {
				files["a/a.gp"] += "\ntype Loud struct{ Dog }\n\n" +
					"func Shout(s Speaker) string { return s.Speak() }\n\n" +
					"func UseLoud() string { return Shout(Loud{}) }\n"
			}},
		{name: "method of an interface to the name of a method of a tied interface",
			mark: "|Speak() string }", newName: "Say", outcome: "fallback",
			setup: func(files map[string]string) {
				files["a/a.gp"] += "\ntype Talker interface{ Say() string }\n\ntype Cat struct{}\n\n" +
					"func (Cat) Say() string { return \"meow\" }\n\n" +
					"func Chat(t Talker) string { return t.Say() }\n\n" +
					"func UseCat() string { return Chat(Cat{}) }\n"
			}},
		{name: "method of an interface to a name that an embedder gets from another type",
			mark: "|Speak() string }", newName: "Say", outcome: "refuse",
			setup: func(files map[string]string) {
				files["c/c.gp"] = "package c\n\ntype Talker interface{ Say() string }\n\n" +
					"type Cat struct{}\n\nfunc (Cat) Say() string { return \"meow\" }\n"
				files["b/b.gp"] += "\ntype Both struct {\n\ta.Dog\n\tc.Cat\n}\n\n" +
					"func Use(t c.Talker) string { return t.Say() }\n\n" +
					"func Run() string { return Use(Both{}) }\n"
				files["b/b.gp"] = strings.Replace(files["b/b.gp"],
					"import \"example.com/members/a\"",
					"import (\n\t\"example.com/members/a\"\n\t\"example.com/members/c\"\n)", 1)
			}},
		{name: "method of an interface to the name of a method that the type has",
			mark: "|Speak() string }", newName: "Wag", outcome: "refuse"},
		{name: "method of an interface that a type assertion takes out",
			mark: "|Speak() string }", newName: "Say", outcome: "fallback",
			setup: func(files map[string]string) {
				files["a/a.gp"] += "\nfunc Ask(v any) string {\n\tif s, ok := v.(Speaker); ok {\n" +
					"\t\treturn s.Speak()\n\t}\n\treturn \"\"\n}\n"
			}},
		{name: "method to a name that the package writes", mark: "func (i *Item) |Bump()",
			newName: "Make", outcome: "fallback"},
		{name: "method to a field name of its type", mark: "func (i *Item) |Bump()",
			newName: "Count", outcome: "refuse"},
		{name: "method of an embedded type", mark: "func (b Base) |Describe()",
			newName: "Show", outcome: "proved"},
		{name: "method of an embedded type to a name of the embedder",
			mark: "func (b Base) |Describe()", newName: "Label", outcome: "refuse"},
		{name: "method to an unexported name", mark: "func (i *Item) |Bump()", newName: "bump",
			outcome: "fallback"},
		{name: "method used by a test file", mark: "func (i *Item) |Bump()", newName: "Raise",
			outcome: "fallback", setup: func(files map[string]string) {
				files["a/a_test.gp"] = "package a\n\nfunc helper() { var i Item; i.Bump() }\n"
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := memberProofFiles()
			if tc.setup != nil {
				tc.setup(files)
			}
			full, fullChecks := fastRenameFiles(t, files, "a/a.gp", true, tc.mark, tc.newName)
			fast, fastChecks := fastRenameFiles(t, files, "a/a.gp", false, tc.mark, tc.newName)
			if full != fast {
				t.Fatalf("proof changes the full-validator answer\nfull:\n%s\nproof:\n%s", full, fast)
			}
			refused := strings.HasPrefix(fast, "error:")
			if refused != (tc.outcome == "refuse") {
				t.Fatalf("unexpected refusal or success: %s", fast)
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

// TestFieldProofChecksTheStructs: with the guards' outcome assumed, the proof of
// a field holds exactly where nothing but the lookup of the name sees it.
func TestFieldProofChecksTheStructs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mark    string
		newName string
		proved  bool
		setup   func(map[string]string)
	}{
		{"free field", "|Title string\n\tCount", "Heading", true, nil},
		{"struct of exported fields only", "|Title string\n}\n\nfunc Make", "Heading", false, nil},
		{"field of a struct alone of its names", "|Count int", "Total", true, nil},
		{"identical struct of the package", "|Width, Height", "Across", false, nil},
		{"identical struct of an importer", "|Left, Right", "Near", false, nil},
		{"names of another named struct", "|Near2, Far2", "Close2", true, nil},
		{"new names of another named struct", "|Gamma int", "Beta", true, nil},
		{"new names of an unnamed struct", "|Gamma int", "Beta", false,
			func(files map[string]string) {
				files["a/a.gp"] += "\nvar Literal struct {\n\tAlpha, Beta int\n\thid int\n}\n"
			}},
		{"names of an alias of an unnamed struct", "|Count int", "Total", false,
			func(files map[string]string) {
				files["a/a.gp"] += "\ntype Literal = struct {\n\tTitle string\n\tCount int\n\tinner int\n}\n"
			}},
		{"names of an unnamed struct", "|Count int", "Total", false,
			func(files map[string]string) {
				files["a/a.gp"] += "\nvar Literal struct {\n\tTitle string\n\tCount int\n\tinner int\n}\n"
			}},
		{"embedded type", "|ID int", "Ident", false, nil},
		{"generic struct", "|Value P", "Content", false, nil},
		{"anonymous struct", "|Field int", "Part", false, nil},
		{"visibility", "|Title string\n\tCount", "title", false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := memberProofFiles()
			if tc.setup != nil {
				tc.setup(files)
			}
			rn := renamerFor(t, files, "a/a.gp", tc.mark, tc.newName)
			rn.bindingsChecked = true
			if got := rn.fieldProvesResult(context.Background()); got != tc.proved {
				t.Fatalf("proved = %v, want %v", got, tc.proved)
			}
		})
	}
}

// TestFieldProofNeedsEveryOccurrence: a use of the field that the search did not
// find keeps the check of the result.
func TestFieldProofNeedsEveryOccurrence(t *testing.T) {
	rn := renamerFor(t, memberProofFiles(), "a/a.gp", "|Title string\n\tCount", "Heading")
	rn.bindingsChecked = true
	if !rn.fieldProvesResult(context.Background()) {
		t.Fatal("the complete search is not proved")
	}
	partial := *rn.res
	partial.hits = nil
	skipped := false
	for _, h := range rn.res.hits {
		if !h.def && !skipped && h.v.dir == rn.sym.declDir {
			skipped = true
			continue
		}
		partial.hits = append(partial.hits, h)
	}
	if !skipped {
		t.Fatal("fixture has no use of the field in its package")
	}
	rn.res = &partial
	if rn.fieldProvesResult(context.Background()) {
		t.Fatal("a search without one use is proved")
	}
}

// TestFieldProofNeedsTheGuards: without the guards' outcome, or on cancel, no proof.
func TestFieldProofNeedsTheGuards(t *testing.T) {
	rn := renamerFor(t, memberProofFiles(), "a/a.gp", "|Title string\n\tCount", "Heading")
	if rn.fieldProvesResult(context.Background()) {
		t.Fatal("a rename without the guards is proved")
	}
	rn.bindingsChecked = true
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if rn.fieldProvesResult(ctx) {
		t.Fatal("a canceled request is proved")
	}
}

// methodProofInputs are the folders and views that checkResult gives to the
// proof of a method: the folders of the edits, and the views by folder.
func methodProofInputs(rn *renamer) (dirs, edited map[string]bool, viewOf map[string]*view) {
	edited = make(map[string]bool)
	for _, h := range rn.res.hits {
		edited[filepath.Dir(h.path)] = true
	}
	dirs = maps.Clone(edited)
	viewOf = make(map[string]*view)
	for _, v := range rn.res.views {
		viewOf[v.dir] = v
	}
	return dirs, edited, viewOf
}

// TestMethodProofNeedsEveryOccurrence: a use of the method, in an importer, that
// the search did not find keeps the check of the result.
func TestMethodProofNeedsEveryOccurrence(t *testing.T) {
	rn := renamerFor(t, memberProofFiles(), "a/a.gp", "func (i Item) |Name()", "Called")
	rn.bindingsChecked = true
	rn.declFilesKnown = true
	dirs, edited, viewOf := methodProofInputs(rn)
	if !rn.methodProvesResult(context.Background(), dirs, edited, viewOf) {
		t.Fatal("the complete search is not proved")
	}
	partial := *rn.res
	partial.hits = nil
	skipped := false
	for _, h := range rn.res.hits {
		if !h.def && !skipped && h.v.dir != rn.sym.declDir {
			skipped = true
			continue
		}
		partial.hits = append(partial.hits, h)
	}
	if !skipped {
		t.Fatal("fixture has no use of the method in an importer")
	}
	rn.res = &partial
	dirs, edited, viewOf = methodProofInputs(rn)
	if rn.methodProvesResult(context.Background(), dirs, edited, viewOf) {
		t.Fatal("a search without one use is proved")
	}
}

// TestMethodProofNeedsEveryPackage: a package to check that is not an edited one
// keeps the check of the result.
func TestMethodProofNeedsEveryPackage(t *testing.T) {
	rn := renamerFor(t, memberProofFiles(), "a/a.gp", "func (Dog) |Wag()", "Wave")
	rn.bindingsChecked = true
	rn.declFilesKnown = true
	dirs, edited, viewOf := methodProofInputs(rn)
	if !rn.methodProvesResult(context.Background(), dirs, edited, viewOf) {
		t.Fatal("a method of no interface is not proved")
	}
	dirs["elsewhere"] = true
	if rn.methodProvesResult(context.Background(), dirs, edited, viewOf) {
		t.Fatal("a package that may break is proved")
	}
	delete(dirs, "elsewhere")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if rn.methodProvesResult(ctx, dirs, edited, viewOf) {
		t.Fatal("a canceled request is proved")
	}
	rn.bindingsChecked = false
	if rn.methodProvesResult(context.Background(), dirs, edited, viewOf) {
		t.Fatal("a rename without the guards is proved")
	}
}

// speakTie returns the tie of Dog to Speaker in the view of the package a of the renamer.
func speakTie(t *testing.T, rn *renamer) tie {
	t.Helper()
	for _, v := range rn.res.views {
		for _, candidate := range v.ties() {
			if candidate.names["Speak"] && candidate.src != nil && !candidate.asserted {
				return candidate
			}
		}
	}
	t.Fatal("the fixture has no tie of Dog to Speaker")
	return tie{}
}

// TestTieIsRenamedAlike: a tie that has the old name is harmless when its two ends are renamed
// together or neither is, and not when only one end is (it would no longer be satisfied).
func TestTieIsRenamedAlike(t *testing.T) {
	rn := renamerFor(t, memberProofFiles(), "a/a.gp", "|Speak() string }", "Say")
	tie := speakTie(t, rn)
	var speaker, dog *types.Func
	for m := range tie.dst.Underlying().(*types.Interface).Methods() {
		speaker = m
	}
	obj, _, _ := types.LookupFieldOrMethod(tie.src, true, speaker.Pkg(), "Speak")
	dog = obj.(*types.Func)
	family := rn.familyKeys()
	if !family[keyOf(speaker)] || !family[keyOf(dog)] {
		t.Fatal("the family of the method does not have both ends of the tie")
	}
	if !rn.tieIsRenamedAlike(tie) {
		t.Fatal("a tie renamed on both ends is not alike")
	}
	delete(family, keyOf(dog))
	if rn.tieIsRenamedAlike(tie) {
		t.Fatal("a tie whose value's method is not renamed is alike")
	}
	family[keyOf(dog)] = true
	delete(family, keyOf(speaker))
	if rn.tieIsRenamedAlike(tie) {
		t.Fatal("a tie whose interface method is not renamed is alike")
	}
	delete(family, keyOf(dog))
	if !rn.tieIsRenamedAlike(tie) {
		t.Fatal("a tie that the rename does not touch is not alike")
	}
	tie.asserted = true
	if rn.tieIsRenamedAlike(tie) {
		t.Fatal("an assertion is alike")
	}
}
