package explain

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/lower"
)

func TestMain(m *testing.M) {
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Exit(m.Run())
}

// repoRoot returns the folder of the repository (this package is internal/explain).
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "..", "..")
}

// TestTopicsMatchKinds: the topics and the kinds of reasons are the same names, in the same order,
// so that every [kind] of the annotated view leads to a topic.
func TestTopicsMatchKinds(t *testing.T) {
	topics := Topics()
	if len(topics) < len(lower.AllKinds) {
		t.Fatalf("%d topics, %d kinds", len(topics), len(lower.AllKinds))
	}
	for _, tp := range topics[len(lower.AllKinds):] {
		if !tp.Reference || IsKind(tp.Name) {
			t.Errorf("topic %s comes after the kinds: it must be a reference topic with no kind", tp.Name)
		}
	}
	for _, tp := range topics[:len(lower.AllKinds)] {
		if tp.Reference {
			t.Errorf("topic %s is a kind: it can't be a reference topic", tp.Name)
		}
	}
	topics = topics[:len(lower.AllKinds)]
	for i, k := range lower.AllKinds {
		if topics[i].Name != string(k) {
			t.Errorf("topic %d is %s, the kind is %s", i, topics[i].Name, k)
		}
		if !IsKind(topics[i].Name) {
			t.Errorf("IsKind(%s) = false", topics[i].Name)
		}
	}
	names := KindNames()
	if !sort.StringsAreSorted(names) || len(names) != len(lower.AllKinds) {
		t.Errorf("KindNames = %v", names)
	}
}

// TestTopicTable checks what every topic needs to be shown: a title, a summary on one line, text,
// an example that exists and is used by one topic only, links that lead to topics, and names that
// no other topic has.
func TestTopicTable(t *testing.T) {
	seen := map[string]string{}
	examples := map[string]string{}
	for _, tp := range Topics() {
		if tp.Title == "" || tp.Summary == "" || strings.Contains(tp.Summary, "\n") || len(tp.Summary) > 100 {
			t.Errorf("%s: title %q, summary %q", tp.Name, tp.Title, tp.Summary)
		}
		if len(tp.Text) < 250 {
			t.Errorf("%s: text of %d bytes", tp.Name, len(tp.Text))
		}
		for _, n := range append([]string{tp.Name}, tp.Aliases...) {
			if n != strings.ToLower(n) || n == tp.Name && strings.ContainsAny(n, " \t") {
				t.Errorf("%s: name %q must be lower case (the name itself one word)", tp.Name, n)
			}
			if other, dup := seen[n]; dup {
				t.Errorf("name %q is used by %s and %s", n, other, tp.Name)
			}
			seen[n] = tp.Name
		}
		if other, dup := examples[tp.Example]; dup {
			t.Errorf("example %s is used by %s and %s", tp.Example, other, tp.Name)
		}
		examples[tp.Example] = tp.Name
		if src := tp.ExampleSource(); !strings.HasPrefix(src, "package main\n") || !strings.Contains(src, "func main()") {
			t.Errorf("%s: the example is not a program of package main", tp.Name)
		}
		for _, s := range tp.See {
			if FindTopic(s) == nil || s == tp.Name {
				t.Errorf("%s: see also %q", tp.Name, s)
			}
		}
	}
	entries, err := os.ReadDir("examples")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if _, ok := examples[e.Name()]; !ok {
			t.Errorf("examples/%s belongs to no topic", e.Name())
		}
	}
}

func TestFindTopic(t *testing.T) {
	for in, want := range map[string]string{
		"boxing": "boxing", "BOX": "boxing", " boxed ": "boxing", "Multiple Results": "results", "named types": "named", "lambda": "closures", "nochecks": "checks",
		"arc": "refcount", "aware": "recover",
	} {
		tp := FindTopic(in)
		if tp == nil || tp.Name != want {
			t.Errorf("FindTopic(%q) = %v, want %s", in, tp, want)
		}
	}
	if FindTopic("nothing") != nil || FindTopic("") != nil {
		t.Error("FindTopic found a topic for a name that is none")
	}
}

func TestSuggest(t *testing.T) {
	for in, want := range map[string]string{
		"defr": "defer", "boxng": "boxing", "closure": "closures", "chanel": "channels", "recovr": "recover",
		"initializaton": "initialization", "pani": "panics",
	} {
		got := Suggest(in)
		if len(got) == 0 || got[0] != want {
			t.Errorf("Suggest(%q) = %v, want %s first", in, got, want)
		}
		if len(got) > 3 {
			t.Errorf("Suggest(%q) = %v: more than 3", in, got)
		}
	}
	if got := Suggest("zzzzzzzzzz"); len(got) != 0 {
		t.Errorf("Suggest(zzzzzzzzzz) = %v, want none", got)
	}
}

// slug is how GitHub makes the anchor of a heading.
func slug(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	var b strings.Builder
	for _, r := range h {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

var headingRx = regexp.MustCompile("(?m)^#{2,3} (.+)$")

// TestTopicSpecs: each topic's link into the specs leads to a heading that exists, and the
// sections of docs/spec/cpp-mapping.md that no topic links to are the ones listed here, with the reason.
func TestTopicSpecs(t *testing.T) {
	root := repoRoot(t)
	anchors := map[string]map[string]bool{}
	load := func(file string) map[string]bool {
		if a, ok := anchors[file]; ok {
			return a
		}
		data, err := os.ReadFile(filepath.Join(root, "docs", "spec", file))
		if err != nil {
			t.Fatalf("docs/spec/%s: %v", file, err)
		}
		a := map[string]bool{}
		for _, m := range headingRx.FindAllStringSubmatch(string(data), -1) {
			a[slug(strings.ReplaceAll(m[1], "`", ""))] = true
		}
		anchors[file] = a
		return a
	}
	linked := map[string]bool{}
	check := func(what, spec string) {
		file, anchor, _ := strings.Cut(spec, "#")
		a := load(file)
		if anchor != "" {
			if !a[anchor] {
				t.Errorf("%s: docs/spec/%s has no heading %q", what, file, anchor)
			}
			if file == "cpp-mapping.md" {
				linked[anchor] = true
			}
		}
	}
	for _, tp := range Topics() {
		if tp.Spec == "" {
			t.Errorf("%s: no spec link", tp.Name)
			continue
		}
		check(tp.Name, tp.Spec)
	}
	for _, m := range Messages() {
		if m.Spec != "" {
			check(m.ID, m.Spec)
		}
	}
	// Sections that have no topic of their own: they are the same mechanism as a topic's (the
	// main sections cover their subsections), a reference part, or not a lowering decision.
	covered := map[string]string{
		"a-first-example":         "the tour of the whole mapping",
		"names":                   "the parent of names-inside-a-package",
		"packages":                "namespaces: gocpp explain names",
		"c-and-c-names":           "gocpp explain interop",
		"functions":               "plain functions: no decision to explain",
		"structs-and-named-types": "gocpp explain methods and embedding",
		"structs":                 "gocpp explain embedding",
		"what-the-emitter-writes-steps-54-and-55":                                 "gocpp explain interfaces",
		"promoted-methods-in-the-method-tables":                                   "gocpp explain embedding",
		"the-errors-package-and-native-functions-step-57":                         "gocpp explain interfaces",
		"later-phases-extend-this-document":                                       "not a mechanism",
		"operations-and-constraints-in-generic-code-steps-74-75-76":               "gocpp explain generics",
		"-gpconcepts-c20-concepts-for-gos-constraints-step-79":                    "gocpp explain generics (the flag adds concepts to the headers)",
		"the-internal-packages-and-their-natives-step-92":                         "library packages: no lowering decision",
		"generic-types-step-73":                                                   "gocpp explain generics",
		"sync-and-syncatomic":                                                     "a library package: no lowering decision",
		"range-over-functions":                                                    "gocpp explain range",
		"floating-point-numbers-and-package-math":                                 "a library package: no lowering decision (gocpp explain types for the number types)",
		"the-reflect-subset-step-93":                                              "a library package written in .gp: no lowering decision",
		"package-time-step-88":                                                    "a library package written in .gp: no lowering decision",
		"the-runtime-package-and-the-end-of-the-program-steps-89-and-810-adr-038": "the exit path: not a lowering decision",
	}
	data, err := os.ReadFile(filepath.Join(root, "docs", "spec", "cpp-mapping.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range headingRx.FindAllStringSubmatch(string(data), -1) {
		s := slug(strings.ReplaceAll(m[1], "`", ""))
		if !linked[s] && covered[s] == "" {
			// Not an error: a branch that adds or renames a heading of the spec must not break this
			// package's tests at merge. The log lists what to link or to cover.
			t.Logf("docs/spec/cpp-mapping.md section %q has no topic: link it from a topic's Spec or list it in covered", m[1])
		}
		if linked[s] && covered[s] != "" {
			t.Logf("section %q is linked by a topic: remove it from covered", s)
		}
	}
	for s := range covered {
		if !load("cpp-mapping.md")[s] {
			t.Logf("covered lists %q, which is no heading of cpp-mapping.md", s)
		}
	}
}

func TestWrap(t *testing.T) {
	got := Wrap("one two three four five six seven\n\nsecond paragraph", 12)
	want := "one two\nthree four\nfive six\nseven\n\nsecond\nparagraph\n"
	if got != want {
		t.Errorf("Wrap = %q, want %q", got, want)
	}
	if got := Wrap("  code line that is long and stays as it is", 5); got != "  code line that is long and stays as it is\n" {
		t.Errorf("Wrap of an indented line = %q", got)
	}
	for _, tp := range Topics() {
		for _, l := range strings.Split(Wrap(tp.Text, textWidth), "\n") {
			if len([]rune(l)) > textWidth && !strings.Contains(l[:len(l)/2], " ") {
				continue // one long word
			}
			if len([]rune(l)) > textWidth {
				t.Errorf("%s: line of %d runes: %q", tp.Name, len([]rune(l)), l)
			}
		}
	}
}

func TestWriteListAndTopic(t *testing.T) {
	var b bytes.Buffer
	if err := WriteList(&b, false); err != nil {
		t.Fatal(err)
	}
	for _, tp := range Topics() {
		if !strings.Contains(b.String(), tp.Name+" ") || !strings.Contains(b.String(), tp.Summary) {
			t.Errorf("-list has no line for %s", tp.Name)
		}
	}
	b.Reset()
	tp := FindTopic("defer")
	if err := WriteTopic(&b, tp, &Live{Source: "package main\n\nfunc main() {}\n", Cpp: "int x;\n\nint y;\n"}); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"defer: defer and the single exit\n", "Specification: docs/spec/cpp-mapping.md#defer\n", "See also: gocpp explain recover, gocpp explain panics, gocpp explain closures\n",
		"Example (defer.gp):\n\n    package main\n\n    func main() {}\n", "    int x;\n\n    int y;\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("WriteTopic lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, " \n") {
		t.Errorf("WriteTopic has trailing blanks:\n%s", out)
	}
	b.Reset()
	if err := WriteTopic(&b, tp, nil); err != nil || strings.Contains(b.String(), "Example") {
		t.Errorf("WriteTopic without a live example: %v\n%s", err, b.String())
	}
}

// --- messages ---

// TestMessagesCoverGates: every gate of the checker has an entry with its exact text, an
// explanation and a unique id.
func TestMessagesCoverGates(t *testing.T) {
	gates := check.Gates()
	if len(gates) < 100 {
		t.Fatalf("check.Gates() has %d rows", len(gates))
	}
	ids := map[string]bool{}
	for _, m := range Messages() {
		if ids[m.ID] {
			t.Errorf("duplicate id %s", m.ID)
		}
		ids[m.ID] = true
	}
	for _, g := range gates {
		ms := FindMessage("./main.gp:3:4: " + g.Message)
		if len(ms) == 0 || ms[0].Text != g.Message {
			t.Errorf("no exact entry for the gate %q", g.Message)
			continue
		}
		m := ms[0]
		if m.Why == "" || m.Fix == "" || m.Title == "" || m.Source != "checker" {
			t.Errorf("%s: incomplete entry %+v", g.Message, m)
		}
		if g.Phase != check.Never && !strings.Contains(m.Why, "Phase "+twoDigits(g.Phase)) {
			t.Errorf("%s: the explanation does not name Phase %02d: %s", g.Message, g.Phase, m.Why)
		}
		if g.Phase == check.Never && !strings.Contains(m.Why, "never") {
			t.Errorf("%s: the explanation does not say never: %s", g.Message, m.Why)
		}
		if m.Phase != g.Phase {
			t.Errorf("%s: phase %d, want %d", g.Message, m.Phase, g.Phase)
		}
	}
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// TestCuratedMessages: each curated entry is complete, its topic and spec exist, and no
// text or pattern is a duplicate.
func TestCuratedMessages(t *testing.T) {
	root := repoRoot(t)
	seen := map[string]bool{}
	for _, m := range curated {
		key := m.Text + m.Pattern
		if (m.Text == "") == (m.Pattern == "") {
			t.Errorf("%s: exactly one of Text and Pattern", m.ID)
		}
		if seen[key] {
			t.Errorf("%s: duplicate message %q", m.ID, key)
		}
		seen[key] = true
		if m.Title == "" || m.Why == "" || m.Fix == "" || m.Source == "" {
			t.Errorf("%s: incomplete", m.ID)
		}
		if m.Topic != "" && FindTopic(m.Topic) == nil {
			t.Errorf("%s: topic %q", m.ID, m.Topic)
		}
		if m.Spec != "" {
			if _, err := os.Stat(filepath.Join(root, "docs", "spec", strings.SplitN(m.Spec, "#", 2)[0])); err != nil {
				t.Errorf("%s: spec %q", m.ID, m.Spec)
			}
		}
		for _, g := range check.Gates() {
			if m.Text == g.Message {
				t.Errorf("%s: %q is a gate message: the table makes its entry itself", m.ID, m.Text)
			}
		}
		if strings.Contains(m.Text, "%s") {
			t.Errorf("%s: a %%s in Text (use Pattern)", m.ID)
		}
	}
}

func TestFindMessage(t *testing.T) {
	tests := []struct {
		in, id string
	}{
		{"interfaces are not supported yet (Phase 05)", "gate/feature/interfaces"},
		{"./main.gp:5:12: interfaces are not supported yet (Phase 05)", "gate/feature/interfaces"},
		{"main.gp:5:12: interfaces are not supported yet (Phase 05)\n\tother line", "gate/feature/interfaces"},
		{"/tmp/x/a.gp:5: generic types are not supported yet (Phase 07)", "gate/feature/generic types"},
		{"main.go:5:23: slices.Contains is not available yet (Phase 07)", "lower/std-generics"},
		{"gocpp vet: not available yet (Phase 13)", "driver/command"},
		{"gocpp build: building for plan9/arm on linux/amd64 is not supported yet (Phase 06)", "driver/cross-target"},
		{"./a.gp:3:6: cannot declare C - C is reserved for C's built-in types", "loader/reserved-c"},
		{"cannot find header stdio.hx", "check/header"},
		{"package net/http is not available yet (Phase 09)", "loader/package"},
		{"the format verb %x is not supported yet (Phase 05)", "gate/verb/%x"},
		{"x.gp:1:1: a thing is not supported yet (Phase 07)", "lower/phase-07"},
	}
	for _, tc := range tests {
		ms := FindMessage(tc.in)
		if len(ms) == 0 {
			t.Errorf("FindMessage(%q): nothing", tc.in)
			continue
		}
		var ids []string
		for _, m := range ms {
			ids = append(ids, m.ID)
		}
		if ms[0].ID != tc.id {
			t.Errorf("FindMessage(%q) = %v, want %s first", tc.in, ids, tc.id)
		}
	}
	// The most specific pattern only: "%s (Phase 07)" is not shown next to "%s.%s is not available yet (Phase 07)".
	if ms := FindMessage("slices.Contains is not available yet (Phase 07)"); len(ms) != 1 {
		t.Errorf("slices.Contains: %d entries, want the most specific one", len(ms))
	}
	for _, in := range []string{"", "nothing to see here", "interfaces are fine"} {
		if ms := FindMessage(in); len(ms) != 0 {
			t.Errorf("FindMessage(%q) = %d entries, want none", in, len(ms))
		}
	}
}

func TestLooksLikeMessage(t *testing.T) {
	for in, want := range map[string]bool{"defer": false, "boxing": false, "a b": true, "x(Phase": true, "main.gp:5": false} {
		if got := LooksLikeMessage(in); got != want {
			t.Errorf("LooksLikeMessage(%q) = %v", in, got)
		}
	}
	for in, want := range map[string]string{
		"./main.gp:5:12: a b":        "a b",
		"main.go:5: a b":             "a b",
		"a/b/c.gp:5:12:   a b\nmore": "a b",
		"plain message":              "plain message",
		"gocpp vet: x":               "gocpp vet: x",
		`C:\x\a.gp:5:2: a b`:         "a b",
		`C:/x/a.gp:5:2: a b`:         "a b",
		`D:\w\lib\b.go:7: a b`:       "a b",
	} {
		if got := StripPosition(in); got != want {
			t.Errorf("StripPosition(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- completeness: the messages of the source ---

// scanDirs are the packages whose user-facing messages must have an entry.
var scanDirs = []string{"internal/lower", "internal/cppgen", "internal/load", "cmd/gocpp"}

// phaseRx finds the messages of the family: those that name a phase or say that something is
// not available or not supported yet.
var phaseRx = regexp.MustCompile(`Phase \d\d|not available yet|not supported yet|reserved for C`)

// literalTemplates returns the message templates of the string literals of the source files of
// dir: a string made of literals, + and fmt.Sprintf is one template, in which what is not a
// literal is %s. Only the templates of the family count.
func literalTemplates(t *testing.T, dir string) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, de := range entries {
		name := de.Name()
		if de.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	out := map[string]string{}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			switch e := e.(type) {
			case *ast.BinaryExpr:
				if e.Op != token.ADD {
					return true
				}
			case *ast.BasicLit:
				if e.Kind != token.STRING {
					return true
				}
			case *ast.CallExpr:
				if !isSprintf(e) {
					return true
				}
			default:
				return true
			}
			tmpl, ok := template(e)
			if !ok || !phaseRx.MatchString(tmpl) || strings.Contains(tmpl, "\n") || len(tmpl) > 220 {
				return true
			}
			pos := fset.Position(e.Pos())
			out[tmpl] = pos.Filename + ":" + strconv.Itoa(pos.Line)
			return false // the whole message is one template: not its parts
		})
	}
	return out
}

func isSprintf(c *ast.CallExpr) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == "fmt" && (sel.Sel.Name == "Sprintf" || sel.Sel.Name == "Fprintf" || sel.Sel.Name == "Errorf") && len(c.Args) > 0
}

var verbRx = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z]`)

// template returns the template of e, if it is made of literals.
func template(e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		s, err := strconv.Unquote(e.Value)
		return s, err == nil && e.Kind == token.STRING
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "%s", true
		}
		a, ok1 := template(e.X)
		b, ok2 := template(e.Y)
		return a + b, ok1 || ok2
	case *ast.CallExpr:
		if isSprintf(e) {
			first := e.Args[0]
			if len(e.Args) > 1 && !isLiteral(first) {
				first = e.Args[1] // Fprintf(w, format, ...)
			}
			s, ok := template(first)
			return verbRx.ReplaceAllString(s, "%s"), ok
		}
		return "%s", true
	case *ast.ParenExpr:
		return template(e.X)
	}
	return "%s", true
}

func isLiteral(e ast.Expr) bool {
	_, ok := e.(*ast.BasicLit)
	return ok
}

// TestMessagesCoverSources: every message of the family "not available yet", "not supported yet" and
// "(Phase NN)" that the lowering, the emitter, the loader and the driver can print has an entry that
// explains it, so that a new message with no explanation fails here.
func TestMessagesCoverSources(t *testing.T) {
	root := repoRoot(t)
	n := 0
	for _, dir := range scanDirs {
		for tmpl, where := range literalTemplates(t, filepath.Join(root, filepath.FromSlash(dir))) {
			n++
			if fragment(tmpl) {
				// A fragment that the lowering completes with a name or a type: the entries
				// marked Fallback answer for it (TestFallbacks).
				continue
			}
			sample := strings.ReplaceAll(tmpl, "%s", "X")
			sample = strings.TrimSuffix(strings.TrimSuffix(sample, "\n"), "\\n")
			if !covered(tmpl, sample) {
				t.Errorf("%s: the message %q has no entry in internal/explain/messages.go (add a curated Message that matches it)", strings.TrimPrefix(where, root+"/"), tmpl)
			}
		}
	}
	if n < 25 {
		t.Errorf("only %d messages found in the sources: the scan is broken", n)
	}
}

// covered reports whether some entry explains a message of the template tmpl: the entry matches
// the text with each %s replaced by a word, with or without the "not supported yet: " that the
// lowering puts in front of what g.unsupported says.
func covered(tmpl, sample string) bool {
	for _, m := range Messages() {
		if m.Fallback {
			continue // a catch-all covers nothing: each message of the family has an entry of its own
		}
		for _, s := range []string{sample, "not supported yet: " + sample, "gocpp X: " + sample} {
			if m.Matches(s) {
				return true
			}
		}
	}
	return false
}

// TestGateTopicsExist: the topics that the gates link to exist.
func TestGateTopicsExist(t *testing.T) {
	for subject, topic := range gateTopics {
		if FindTopic(topic) == nil {
			t.Errorf("gateTopics[%q] = %q: no such topic", subject, topic)
		}
		found := false
		for _, g := range check.Gates() {
			if g.Subject == subject {
				found = true
			}
		}
		if !found {
			t.Errorf("gateTopics has %q, which is no gate", subject)
		}
	}
}

// TestFallbacks: the fragments that the completeness test skips are answered by the catch-all entries,
// and a catch-all never hides an entry of its own.
func TestFallbacks(t *testing.T) {
	for _, in := range []string{
		"x.gp:3:4: not supported yet: the operator &^ on float64",
		"x.gp:3:4: conversions of C/C++ values to int (Phase 12)",
		"x.gp:3:4: make of chan int (Phase 08)",
		"x.gp:3:4: index expressions on T (Phase 07)",
		"x.gp:3:4: something of C/C++ values (typed interop: Phase 12)",
		"x.gp:3:4: something (a later step of Phase 04)",
	} {
		ms := FindMessage(in)
		if len(ms) == 0 || !ms[0].Fallback {
			t.Errorf("FindMessage(%q) = %v, want a fallback entry", in, ms)
		}
	}
	for _, m := range Messages() {
		if !m.Fallback {
			continue
		}
		if m.Text != "" {
			t.Errorf("%s: a fallback is a pattern", m.ID)
		}
	}
	// An entry of its own comes before a catch-all that also matches.
	if ms := FindMessage("range over functions (Phase 07)"); len(ms) != 1 || ms[0].ID != "lower/range-func" {
		t.Errorf("range over functions: %v", ms)
	}
}

var phaseMarkRx = regexp.MustCompile(`\(?(?:typed interop: |a later step of )?Phase \d\d\)?`)

// fragment reports whether the template tmpl is a piece that the lowering completes with a name or a
// type, with fewer than three words of its own: "%s %s (Phase 07)", " (Phase 08)", "not supported yet: %s".
func fragment(tmpl string) bool {
	if tmpl == "not supported yet: %s" {
		return true
	}
	rest := phaseMarkRx.ReplaceAllString(strings.ReplaceAll(tmpl, "%s", " "), "")
	return len(strings.Fields(rest)) < 3
}

// --- the view: instances and the standard library ---

func TestBuildInstances(t *testing.T) {
	cpp := "#line 3 \"/p/a.gp\"\nvoid gp_inst_aa_F(int64_t x) {\n#line 4 \"/p/a.gp\"\n    int64_t y = x;\n#line 5 \"/p/a.gp\"\n}\n" +
		"#line 3 \"/p/a.gp\"\nvoid gp_inst_bb_F(Big x) {\n#line 4 \"/p/a.gp\"\n    gp::ptr<Big> y = gp::new_<Big>(x);\n#line 5 \"/p/a.gp\"\n}\n" +
		"#line 7 \"/p/a.gp\"\nvoid f() {\n#line 8 \"/p/a.gp\"\n    gp_inst_aa_F(1);\n}\n"
	reasons := []lower.Reason{
		{File: "/p/a.gp", Line: 4, Col: 6, Kind: lower.WhyBoxing, Msg: "y is boxed", Inst: "gp_inst_bb_F"},
		{File: "/p/a.gp", Line: 4, Col: 6, Kind: lower.WhyClosures, Msg: "captures y by copy", Inst: "gp_inst_aa_F"},
		{File: "/p/a.gp", Line: 8, Col: 2, Kind: lower.WhyGenerics, Msg: "F is gp_inst_aa_F"},
		{File: "/p/a.gp", Line: 9, Col: 2, Kind: lower.WhyChecks, Msg: "unplaced", Inst: "gp_inst_cc_F"},
	}
	v := Build([]File{{Path: "gpkg/x/a.gp.cpp", Data: []byte(cpp)}}, reasons, Options{})
	got := map[string][]string{}
	for _, c := range v.Files[0].Chunks {
		if c.GP == nil || c.GP.Line != 4 {
			continue
		}
		for _, r := range c.Reasons {
			got[c.Inst] = append(got[c.Inst], r.Msg)
		}
		if c.Cont {
			t.Errorf("the chunk of %s is continued", c.Inst)
		}
	}
	if !slices.Equal(got["gp_inst_bb_F"], []string{"y is boxed"}) || !slices.Equal(got["gp_inst_aa_F"], []string{"captures y by copy"}) {
		t.Errorf("reasons by instance: %v", got)
	}
	var plain []string
	for _, c := range v.Files[0].Chunks {
		if c.Inst == "" && c.GP != nil && c.GP.Line == 8 {
			for _, r := range c.Reasons {
				plain = append(plain, r.Msg)
			}
		}
	}
	if !slices.Equal(plain, []string{"F is gp_inst_aa_F"}) {
		t.Errorf("the reason outside an instance: %v", plain)
	}
	if len(v.Unplaced) != 1 || v.Unplaced[0].Inst != "gp_inst_cc_F" {
		t.Errorf("unplaced: %+v", v.Unplaced)
	}
	var b bytes.Buffer
	v.WriteText(&b, nil)
	if n := strings.Count(b.String(), "  [in gp_inst_"); n != 6 {
		t.Errorf("%d instance marks, want 6:\n%s", n, b.String())
	}
}

func TestBuildStd(t *testing.T) {
	std := "#line 3 \"$GOCPP/stdlib/errors/e.gp\"\nvoid New() {\n#line 1 \"$GOCPP/gen/gpkg/errors/e.gp.cpp\"\nint gen;\n}\n"
	own := "#line 3 \"/p/a.gp\"\nvoid main() {\n}\n"
	files := []File{{Path: "gpkg/errors/e.gp.cpp", Data: []byte(std), Std: true}, {Path: "gpkg/main/a.gp.cpp", Data: []byte(own)}}
	reasons := []lower.Reason{{File: "$GOCPP/stdlib/errors/e.gp", Line: 3, Kind: lower.WhyChecks, Msg: "std"}, {File: "/p/a.gp", Line: 3, Kind: lower.WhyChecks, Msg: "own"}}
	v := Build(files, reasons, Options{})
	if len(v.Files) != 1 || v.Files[0].Path != "gpkg/main/a.gp.cpp" || len(v.Unplaced) != 0 {
		t.Errorf("the default view: %+v", v)
	}
	w := Build(files, reasons, Options{Std: true})
	if len(w.Files) != 2 {
		t.Fatalf("-std view has %d files", len(w.Files))
	}
	for _, c := range w.Files[0].Chunks {
		if c.GP != nil && strings.Contains(c.GP.File, "gen/") {
			t.Errorf("a generated line of a standard package is a .gp place: %+v", c.GP)
		}
	}
	if n := len(w.Files[0].Chunks); n != 2 {
		t.Errorf("std file has %d chunks: %+v", n, w.Files[0].Chunks)
	}
}

func TestBuildContinuationLines(t *testing.T) {
	gp := "package main\n\nfunc main() {\n\tx := f(\n\t\ta,\n\t\tb(\")\"), // )\n\t)\n\t_ = x\n}\n"
	cpp := "#line 3 \"/p/a.gp\"\nvoid main() {\n#line 4 \"/p/a.gp\"\n    auto x = f(a, b(\")\"));\n#line 8 \"/p/a.gp\"\n    (void)x;\n}\n"
	reasons := []lower.Reason{
		{File: "/p/a.gp", Line: 5, Col: 3, Kind: lower.WhyChecks, Msg: "on line five"},
		{File: "/p/a.gp", Line: 6, Col: 3, Kind: lower.WhyChecks, Msg: "on line six"},
		{File: "/p/a.gp", Line: 7, Col: 2, Kind: lower.WhyChecks, Msg: "on the closing paren"},
		{File: "/p/a.gp", Line: 8, Col: 2, Kind: lower.WhyChecks, Msg: "own chunk"},
	}
	v := Build([]File{{Path: "gpkg/x/a.gp.cpp", Data: []byte(cpp)}}, reasons, Options{Source: func(string) []byte { return []byte(gp) }})
	if len(v.Unplaced) != 0 {
		t.Errorf("unplaced: %+v", v.Unplaced)
	}
	got := map[int][]string{}
	for _, c := range v.Files[0].Chunks {
		if c.GP != nil {
			for _, r := range c.Reasons {
				got[c.GP.Line] = append(got[c.GP.Line], r.Msg)
			}
		}
	}
	if !slices.Equal(got[4], []string{"on line five", "on line six", "on the closing paren"}) || !slices.Equal(got[8], []string{"own chunk"}) {
		t.Errorf("reasons by chunk line: %v", got)
	}
	// Lines asks for a continuation line and gets the statement's chunk.
	if cs := v.Lines("/p/a.gp", 6, 6); len(cs) != 1 || cs[0].GP.Line != 4 {
		t.Errorf("Lines(6,6) = %+v", cs)
	}
	// Without the source nothing can be moved: the reasons are unplaced.
	w := Build([]File{{Path: "gpkg/x/a.gp.cpp", Data: []byte(cpp)}}, reasons, Options{})
	if len(w.Unplaced) != 3 {
		t.Errorf("without the source: %d unplaced, want 3", len(w.Unplaced))
	}
}
