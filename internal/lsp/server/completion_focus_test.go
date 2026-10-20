package server

import (
	"encoding/json"
	"fmt"
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

// startFocusComp starts a server over focusCompFiles with the focused checks on or off, and opens
// the files.
func startFocusComp(t *testing.T, focus bool, phase int) (*compClient, *Server) {
	t.Helper()
	root := writeTree(t, focusCompFiles)
	cfg := testConfig()
	cfg.Phase = phase
	s := newServer(nil, cfg, debounce.RealClock{})
	c := &compClient{dclient: &dclient{client: startServer(t, s, func(s *Server) {
		s.comp.index.wait = time.Minute
		s.diags.noFocus = !focus
		s.diags.settle = time.Hour // the base stays the text that was opened
	}), root: root}, versions: make(map[string]int)}
	c.call("initialize", snippetInit)
	c.notify("initialized", "{}")
	for _, name := range []string{"a.gp", "b.gp"} {
		c.open(name, focusCompFiles[name])
		c.versions[name] = 1
		c.next(name)
	}
	return c, s
}

func TestCompletionFocusedMatchesFull(t *testing.T) {
	for _, phase := range []int{check.LastPhase, version.Phase} {
		t.Run(fmt.Sprintf("phase %d", phase), func(t *testing.T) {
			completionFocusedMatchesFull(t, phase)
		})
	}
}

func completionFocusedMatchesFull(t *testing.T, phase int) {
	checkGoroutines(t)
	places := []struct{ name, body string }{
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
	with, withSrv := startFocusComp(t, true, phase)
	without, withoutSrv := startFocusComp(t, false, phase)
	for _, p := range places {
		text := strings.Replace(focusCompFiles["a.gp"], "// BODY", p.body, 1)
		got, err := json.Marshal(with.complete("a.gp", text))
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
		text := strings.Replace(focusCompFiles["a.gp"], "// BODY", "t.|\n\t_ = 1", 1)
		items = with.complete("a.gp", text)
		if item(items, "Describe") == nil || item(items, "Name") == nil {
			t.Errorf("t. offers %v", labels(items))
		}
	})
}
