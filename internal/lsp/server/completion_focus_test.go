package server

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/version"
)

// The completions after a keystroke in a function body come from the focused check of the package
// (completion.go, focusedQuery); they must be those of the full check.

var focusCompFiles = map[string]string{
	"gocpp.mod": "module example.com/m\n",
	"a.gp": `package main

import (
	"fmt"
	"strings"
)

type T struct {
	Name string
	Size int
}

// Describe says what t is.
func (t T) Describe() string {
	return fmt.Sprint(t.Name, t.Size)
}

func work(t T) int {
	// BODY
	return t.Size + len(strings.TrimSpace(t.Name))
}
`,
	"b.gp": `package main

// Helper returns x.
func Helper(x int) int { return x }

func main() {
	_ = work(T{})
}
`,
}

// focusCompGenericFiles are focusCompFiles with generic declarations in g.gp, and a generic
// function in a.gp whose body has the marker // GBODY.
func focusCompGenericFiles() map[string]string {
	files := make(map[string]string)
	for name, text := range focusCompFiles {
		files[name] = text
	}
	files["a.gp"] += `
func firstOf[E Sized](xs []E) int {
	var e E
	// GBODY
	return e.Len() + len(xs)
}
`
	files["g.gp"] = `package main

// Sized has a length.
type Sized interface{ Len() int }

// Pick returns x.
func Pick[V any](x, y V) V { return x }

// Box holds a value.
type Box[V any] struct{ Value V }

// Get returns the value.
func (b Box[V]) Get() V { return b.Value }

// NewBox makes a box.
func NewBox[V any](v V) *Box[V] { return &Box[V]{Value: v} }

// Pair holds a key and a value.
type Pair[K comparable, V any] struct {
	Key K
	Val V
}

// Swap returns the pair.
func (p Pair[K, V]) Swap() Pair[K, V] { return p }
`
	return files
}

// startFocusComp starts a server over the files with the focused checks on or off, and opens the
// .gp files.
func startFocusComp(t *testing.T, focus bool, phase int, files map[string]string) (*compClient, *Server) {
	t.Helper()
	root := writeTree(t, files)
	cfg := testConfig()
	cfg.Phase = phase
	s := newServer(nil, cfg, debounce.RealClock{})
	c := &compClient{dclient: &dclient{client: startServer(t, s, func(s *Server) {
		s.comp.index.wait = hangLimit
		s.diags.noFocus = !focus
		s.diags.settle = time.Hour // the base stays the text that was opened
	}), root: root}, versions: make(map[string]int)}
	c.call("initialize", snippetInit)
	c.notify("initialized", "{}")
	var names []string
	for name := range files {
		if strings.HasSuffix(name, ".gp") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		c.open(name, files[name])
		c.versions[name] = 1
		c.next(name)
	}
	return c, s
}

func TestCompletionFocusedMatchesFull(t *testing.T) {
	for _, phase := range []int{check.LastPhase, version.Phase} {
		t.Run(fmt.Sprintf("phase %d", phase), func(t *testing.T) {
			completionFocusedMatchesFull(t, phase, focusCompFiles, "// BODY", focusCompPlaces, nil)
		})
	}
}

// TestCompletionFocusedMatchesFullGenerics is TestCompletionFocusedMatchesFull in a package with
// generic declarations, in an ordinary function and in a generic one.
func TestCompletionFocusedMatchesFullGenerics(t *testing.T) {
	generic := []compPlace{
		{"a generic function", "Pi|\n\t_ = 1"},
		{"a generic type", "var b Bo|\n\t_ = b"},
		{"the fields of an instance", "b := Box[T]{}\n\tb.|\n\t_ = 1"},
		{"the methods of an inferred instance", "NewBox(t).|\n\t_ = 1"},
		{"the result of an inferred call", "Pick(t, t).|\n\t_ = 1"},
		{"the fields of a struct of two type parameters", "p := Pair[string, T]{}\n\tp.|\n\t_ = 1"},
		{"the methods of a generic type", "Pair[int, int]{}.Sw|"},
	}
	params := []compPlace{
		{"a type parameter's methods", "e.|\n\t_ = 1"},
		{"a type parameter", "var x E|\n\t_ = x"},
		{"an instance with a type parameter", "Box[E]{}.|\n\t_ = 1"},
		{"an inferred instance with a type parameter", "Pick(e, e).|\n\t_ = 1"},
		{"a parameter", "_ = x|"},
	}
	// Labels that the answer at a place must have, so that the comparison is not of two empty lists.
	wants := map[string][]string{
		"a generic function":                            {"Pick"},
		"a generic type":                                {"Box"},
		"the fields of an instance":                     {"Value", "Get"},
		"the methods of an inferred instance":           {"Value", "Get"},
		"the result of an inferred call":                {"Name", "Describe"},
		"the fields of a struct of two type parameters": {"Key", "Val", "Swap"},
		"the methods of a generic type":                 {"Swap"},
		"a type parameter's methods":                    {"Len"},
		"an instance with a type parameter":             {"Value", "Get"},
		"an inferred instance with a type parameter":    {"Len"},
	}
	for _, phase := range []int{check.LastPhase, version.Phase} {
		t.Run(fmt.Sprintf("phase %d/an ordinary function", phase), func(t *testing.T) {
			completionFocusedMatchesFull(t, phase, focusCompGenericFiles(), "// BODY", append(generic, focusCompPlaces...), wants)
		})
		t.Run(fmt.Sprintf("phase %d/a generic function", phase), func(t *testing.T) {
			completionFocusedMatchesFull(t, phase, focusCompGenericFiles(), "// GBODY", params, wants)
		})
	}
}

// A compPlace is a text that a test of the focused completions puts at the marker of a.gp: | is the
// place of the completion.
type compPlace struct{ name, body string }

var focusCompPlaces = []compPlace{
	{"a field", "t.|\n\t_ = 1"},
	{"a field with a prefix", "t.Na|\n\t_ = 1"},
	{"a method", "t.De|\n\t_ = 1"},
	{"an imported package", "fmt.|\n\t_ = 1"},
	{"an imported function with a prefix", "strings.To|\n\t_ = 1"},
	{"a function of another file", "He|\n\t_ = 1"},
	{"a type of the package", "var x T|\n\t_ = x"},
	{"a local variable", "loc := 1\n\t_ = lo|"},
	{"a parameter", "_ = t|"},
	{"a keyword", "ret|"},
}

// completionFocusedMatchesFull compares the completions of a server with the focused checks with
// those of one without, at each place put at the marker of a.gp; wants are the labels that the
// answer at a place (by name) must have.
func completionFocusedMatchesFull(
	t *testing.T, phase int, files map[string]string, marker string, places []compPlace, wants map[string][]string,
) {
	checkGoroutines(t)
	with, withSrv := startFocusComp(t, true, phase, files)
	without, withoutSrv := startFocusComp(t, false, phase, files)
	for _, p := range places {
		text := strings.Replace(files["a.gp"], marker, p.body, 1)
		items := with.complete("a.gp", text)
		got, err := json.Marshal(items)
		if err != nil {
			t.Fatal(err)
		}
		want, err := json.Marshal(without.complete("a.gp", text))
		if err != nil {
			t.Fatal(err)
		}
		// The two servers have their files in folders of their own.
		gotText := strings.ReplaceAll(string(got), with.root, "ROOT")
		wantText := strings.ReplaceAll(string(want), without.root, "ROOT")
		t.Run(p.name, func(t *testing.T) {
			if gotText != wantText {
				t.Errorf("completions of the focused check:\n%.2000s\nof the full check:\n%.2000s",
					gotText, wantText)
			}
			for _, label := range wants[p.name] {
				if item(items, label) == nil {
					t.Errorf("no %s in %v", label, labels(items))
				}
			}
		})
	}
	t.Run("the places were answered by focused checks", func(t *testing.T) {
		if got := withSrv.pipeline.FocusedChecks(); got < int64(len(places)) {
			t.Errorf("%d focused checks for %d places", got, len(places))
		}
		if got := withoutSrv.pipeline.FocusedChecks(); got != 0 {
			t.Errorf("%d focused checks with the focused checks off", got)
		}
	})
	t.Run("the items are not empty", func(t *testing.T) {
		var items []protocol.CompletionItem
		if marker != "// BODY" {
			return
		}
		text := strings.Replace(files["a.gp"], marker, "t.|\n\t_ = 1", 1)
		items = with.complete("a.gp", text)
		if item(items, "Describe") == nil || item(items, "Name") == nil {
			t.Errorf("t. offers %v", labels(items))
		}
	})
}
