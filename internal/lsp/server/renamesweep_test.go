package server

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/bench"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// identPlaces returns the byte offsets of the first few occurrences of each name that the file
// writes (a name of the package clause, the imports and the blank name left out), in order.
func identPlaces(t *testing.T, name, src string, perName int) []int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	skip := map[*ast.Ident]bool{file.Name: true}
	for _, spec := range file.Imports {
		if spec.Name != nil {
			skip[spec.Name] = true
		}
	}
	seen := make(map[string]int)
	var places []int
	ast.Inspect(file, func(node ast.Node) bool {
		id, ok := node.(*ast.Ident)
		if !ok || skip[id] || id.Name == "_" || seen[id.Name] >= perName {
			return true
		}
		seen[id.Name]++
		places = append(places, fset.Position(id.Pos()).Offset)
		return true
	})
	return places
}

// sweepAnswers renames the name at each place of the file in a new server to the name at the same
// index of newNames, and returns the answers as the server sent them.
func sweepAnswers(
	t *testing.T, files map[string]string, file string, places []int, newNames []string, all bool,
) []string {
	t.Helper()
	recheckAllImporters = all
	defer func() { recheckAllImporters = false }()
	root := writeTree(t, files)
	srv := newServer(nil, testConfig(), debounce.RealClock{})
	c := &refClient{dclient: &dclient{client: startServer(t, srv, nil), root: root}, src: files}
	c.call("initialize", plainInit)
	c.notify("initialized", "{}")
	c.skipShown = true
	c.open(file, files[file])
	c.next(file)
	src := files[file]
	mapper := protocol.NewMapper(src)
	answers := make([]string, len(places))
	for i, offset := range places {
		newName := newNames[i]
		position := mapper.Position(offset)
		resp := c.call("textDocument/rename", fmt.Sprintf(
			`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d},"newName":%q}`,
			c.uri(file), position.Line, position.Character, newName))
		if resp.Error != nil {
			answers[i] = "error: " + strings.ReplaceAll(resp.Error.Message, root, "ROOT")
		} else {
			answers[i] = strings.ReplaceAll(string(resp.Result), filepath.ToSlash(root), "ROOT")
		}
	}
	return answers
}

// usedAtLine matches the line that an error names for the use of a hidden declaration: the
// checker takes it from a map of uses, so it is any one of the uses, run to run, with or
// without a proof.
var usedAtLine = regexp.MustCompile(`(used at \S+\.gp):\d+`)

// embedderName matches the embedding type that the guard names first: the guard visits the
// types of a map, so it is any one of the embedders, run to run, with or without a proof.
var embedderName = regexp.MustCompile(`: \S+ embeds `)

// importedIn matches the file that the guard names for an import that the new name would clash
// with: the guard visits the files of a map, so it is any one of the files that import it, run
// to run, with or without a proof. The refusal and the rest of the text stay exact.
var importedIn = regexp.MustCompile(`is imported in \S+ \(\S+:\d+\)`)

// sameAnswer reports whether two answers are the same, up to the use, the embedder and the
// importing file that a message names.
func sameAnswer(one, other string) bool {
	return unnamed(one) == unnamed(other)
}

func unnamed(answer string) string {
	answer = usedAtLine.ReplaceAllString(answer, "$1")
	answer = importedIn.ReplaceAllString(answer, "is imported in FILE (FILE:LINE)")
	return embedderName.ReplaceAllString(answer, ": T embeds ")
}

// everyPair returns each place once for each other name of the file that has the exportedness
// of the name at the place.
func everyPair(src string, places []int) ([]int, []string) {
	var names []string
	seen := make(map[string]bool)
	for _, offset := range places {
		if name := wordAt(src, offset); !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	var pairPlaces []int
	var pairNames []string
	for _, offset := range places {
		name := wordAt(src, offset)
		for _, other := range names {
			if other != name && token.IsExported(other) == token.IsExported(name) {
				pairPlaces = append(pairPlaces, offset)
				pairNames = append(pairNames, other)
			}
		}
	}
	return pairPlaces, pairNames
}

// freshNames returns for each place a name that nothing uses, of the exportedness of the name there.
func freshNames(src string, places []int) []string {
	out := make([]string, len(places))
	for i, offset := range places {
		name := wordAt(src, offset)
		out[i] = "zz" + name
		if token.IsExported(name) {
			out[i] = "Zz" + name
		}
	}
	return out
}

// takenNames returns for each place another name that the file writes, of the exportedness of
// the name there when there is one: the renames that can clash, hide or be hidden.
func takenNames(src string, places []int) []string {
	var names []string
	seen := make(map[string]bool)
	for _, offset := range places {
		if name := wordAt(src, offset); !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	fresh := freshNames(src, places)
	out := make([]string, len(places))
	for i, offset := range places {
		name := wordAt(src, offset)
		out[i] = fresh[i]
		for step := range len(names) {
			other := names[(i*7+3+step)%len(names)]
			sameKind := token.IsExported(other) == token.IsExported(name)
			if other != name && sameKind && token.IsIdentifier(other) {
				out[i] = other
				break
			}
		}
	}
	return out
}

func wordAt(src string, offset int) string {
	end := offset
	for end < len(src) && (src[end] == '_' || src[end] >= '0' && src[end] <= '9' ||
		src[end] >= 'a' && src[end] <= 'z' || src[end] >= 'A' && src[end] <= 'Z') {
		end++
	}
	return src[offset:end]
}

// TestRenameProofSweepGenerated: every name of four files of the generated project (a file of the
// hot package, of a package it imports, of one that imports it, and the core one) renamed with
// the proofs gets the answer of the check of the result, byte for byte.
func TestRenameProofSweepGenerated(t *testing.T) {
	if testing.Short() {
		t.Skip("renames every name of four files twice")
	}
	project := bench.Generate(bench.Small)
	dir := t.TempDir()
	if err := project.Write(dir, ".gp"); err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		files[filepath.ToSlash(rel)] = string(text)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	proved := 0
	for _, rel := range []string{project.Hot, project.Dep, project.Dependent, project.Core} {
		file := strings.TrimSuffix(rel, filepath.Ext(rel)) + ".gp"
		proved += sweepFile(t, files, file, false)
	}
	if proved == 0 {
		t.Fatal("no rename succeeded: the sweep checks nothing")
	}
}

// sweepFile renames the names of the file to fresh names and to names that the file writes,
// with the proofs and without, and returns the number of renames that succeeded. With every,
// each place is also renamed to each other name of the file, one pair at a time.
func sweepFile(t *testing.T, files map[string]string, file string, every bool) int {
	t.Helper()
	succeeded := 0
	src := files[file]
	places := identPlaces(t, file, src, 2)
	type batch struct {
		places []int
		names  []string
	}
	batches := map[string]batch{
		"fresh": {places, freshNames(src, places)},
		"taken": {places, takenNames(src, places)},
	}
	if every {
		pairPlaces, pairNames := everyPair(src, places)
		batches["every"] = batch{pairPlaces, pairNames}
	}
	for kind, one := range batches {
		places, newNames := one.places, one.names
		full := sweepAnswers(t, files, file, places, newNames, true)
		fast := sweepAnswers(t, files, file, places, newNames, false)
		for i, offset := range places {
			if !sameAnswer(full[i], fast[i]) {
				t.Errorf("%s: %q to %q (%s names) at byte %d\nfull:  %.300s\nproof: %.300s",
					file, wordAt(src, offset), newNames[i], kind, offset, full[i], fast[i])
			}
			if !strings.HasPrefix(full[i], "error:") {
				succeeded++
			}
		}
		t.Logf("%s: %d renames to %s names", file, len(places), kind)
	}
	return succeeded
}

// TestRenameProofSweepAdversarial: the same sweep over a module that has what the review found
// and what the generated project has not: results with names and bare returns in blocks, loops
// and closures, structs written in a package that the declaring package is not imported by, a
// struct of exported fields only and one with an unexported field, embedded types, methods tied
// to interfaces and conversions.
func TestRenameProofSweepAdversarial(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/adv\n",
		"a/a.gp": `package a

type Item struct {
	Title string
}

func Make() Item { return Item{Title: "t"} }

type Hidden struct {
	Name  string
	inner int
}

type Pair struct{ Left, Right int }

type Twin struct{ Left, Right int }

func Swap(p Pair) Twin { return Twin(p) }

type Base struct{ ID int }

func (b Base) Describe() string { return "base" }

type Derived struct {
	Base
	Label string
}

type Speaker interface{ Speak() string }

type Dog struct{}

func (Dog) Speak() string { return "woof" }

func (Dog) Wag() int { return 1 }

func Talk(s Speaker) string { return s.Speak() }

func Results(n int) (r int, err error) {
	for i := 0; i < n; i++ {
		x := i
		if x > 3 {
			return
		}
		r += x
	}
	{
		y := n
		_ = y
		return
	}
}

func Closure() int {
	f := func() (out int) {
		if true {
			z := 1
			_ = z
			return
		}
		return
	}
	v := f()
	return v
}

func Shadow(items []int) int {
	sum := 0
	for _, it := range items {
		sum += it
	}
	total := 0
	return sum + total
}
`,
		"c/c.gp": `package c

func Make() struct{ Title string } { return struct{ Title string }{} }

func Generic[T ~struct{ Title string }](t T) string { return t.Title }

var Rows []struct{ Title string }

type Row = struct{ Title string }

type Cell = struct {
	Name string
	Mark int
}
`,
		"b/b.gp": `package b

import (
	"example.com/adv/a"
	"example.com/adv/c"
)

func Use() string { return c.Generic(a.Make()) }

func Take() a.Item { var o a.Item = c.Make(); return o }

func Add() { c.Rows = append(c.Rows, a.Make()) }

func Read(i a.Item, h a.Hidden) string { return i.Title + h.Name + a.Make().Title }

func Embed() int {
	var d a.Derived
	return d.ID + len(d.Describe())
}

func Hear() string { return a.Talk(a.Dog{}) }
`,
		"d/d.gp": `package d

import (
	"example.com/adv/a"
	"example.com/adv/c"
)

func Show() string {
	var s c.Row = a.Make()
	return s.Title
}
`,
	}
	succeeded := 0
	for _, file := range []string{"a/a.gp", "b/b.gp", "c/c.gp", "d/d.gp"} {
		succeeded += sweepFile(t, files, file, true)
	}
	if succeeded == 0 {
		t.Fatal("no rename succeeded: the sweep checks nothing")
	}
}

// TestSweepSameAnswerIsNotTooLenient: the comparison forgives the file, use and embedder that a
// message names, and nothing else.
func TestSweepSameAnswerIsNotTooLenient(t *testing.T) {
	for _, tc := range []struct {
		name       string
		one, other string
		same       bool
	}{
		{"another importing file",
			"error: cannot rename a to fmt: fmt is imported in core.gp (core.gp:6)",
			"error: cannot rename a to fmt: fmt is imported in log.gp (log.gp:3)", true},
		{"another use of a hidden name",
			"error: cannot rename f to int: it would hide the declaration of int that is used at f.gp:72",
			"error: cannot rename f to int: it would hide the declaration of int that is used at f.gp:77", true},
		{"another embedder",
			"error: cannot rename Name to ID: pkg0.A embeds core.Base, whose Name",
			"error: cannot rename Name to ID: pkg1.B embeds core.Base, whose Name", true},
		{"a refusal and an edit",
			"error: cannot rename a to fmt: fmt is imported in core.gp (core.gp:6)",
			`{"changes":{}}`, false},
		{"another new name",
			"error: cannot rename a to fmt: fmt is imported in core.gp (core.gp:6)",
			"error: cannot rename a to os: os is imported in core.gp (core.gp:6)", false},
		{"another kind of refusal",
			"error: cannot rename a to fmt: fmt is imported in core.gp (core.gp:6)",
			"error: cannot rename a to fmt: fmt is already declared in the package", false},
	} {
		if got := sameAnswer(tc.one, tc.other); got != tc.same {
			t.Errorf("%s: sameAnswer = %v, want %v", tc.name, got, tc.same)
		}
	}
}
