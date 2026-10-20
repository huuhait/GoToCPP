package server

import (
	"context"
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// Tests of the second round of review of step 11.13: the folders of the expansions of a server that lives long,
// callers of a function whose body was blanked, and the validity of what the repaired paths answer.

// TestExpansionsStaleKeepsLiveServers: a folder whose marker is old is removed only if the server that made it is gone
// and the folder is ours; a server that writes again touches the marker.
func TestExpansionsStaleKeepsLiveServers(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	old := time.Now().Add(-72 * time.Hour)
	mk := func(name, content string) string {
		dir := filepath.Join(tmp, name)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		m := filepath.Join(dir, expandMarker)
		if err := os.WriteFile(m, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(m, old, old)
		return dir
	}
	dead := exec.Command(os.Args[0], "-test.run=^$")
	if err := dead.Run(); err != nil {
		t.Fatal(err)
	}
	live := mk(expandDirPrefix+"live", fmt.Sprint(os.Getpid(), "\n")) // a session that has run more than a day
	gone := mk(expandDirPrefix+"gone", fmt.Sprint(dead.Process.Pid, "\n"))
	foreign := mk(expandDirPrefix+"foreign", fmt.Sprint(dead.Process.Pid, "\n"))
	removeStale(func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "foreign") })
	for dir, want := range map[string]bool{live: runtime.GOOS != "windows", gone: false, foreign: true} {
		if _, err := os.Stat(dir); (err == nil) != want && !(runtime.GOOS == "windows" && dir == live) {
			t.Errorf("%s: there = %v, want %v", filepath.Base(dir), err == nil, want)
		}
	}
	// a server that writes touches its marker
	var x expansions
	path, err := x.write("k", "n", "text")
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(filepath.Dir(filepath.Dir(path)), expandMarker)
	os.Chtimes(marker, old, old)
	if _, err := x.write("k", "n", "text"); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(marker); err != nil || time.Since(fi.ModTime()) > time.Minute {
		t.Errorf("the marker was not touched: %v %v", fi, err)
	}
	data, _ := os.ReadFile(marker)
	if strings.TrimSpace(string(data)) != fmt.Sprint(os.Getpid()) {
		t.Errorf("the marker holds %q, want the process id", data)
	}
	x.close()
}

// TestExpansionsTouchLoop: a server that lives on touches the marker by itself.
func TestExpansionsTouchLoop(t *testing.T) {
	save := touchEvery
	touchEvery = 10 * time.Millisecond
	defer func() { touchEvery = save }()
	var x expansions
	path, err := x.write("k", "n", "text")
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(filepath.Dir(filepath.Dir(path)), expandMarker)
	old := time.Now().Add(-72 * time.Hour)
	os.Chtimes(marker, old, old)
	end := time.Now().Add(10 * time.Second)
	for time.Now().Before(end) {
		if fi, err := os.Stat(marker); err == nil && time.Since(fi.ModTime()) < time.Hour {
			x.close()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the marker was not touched by the loop")
}

// TestCallersOfBlankedFunction: the body of a function with a syntax error is blanked and its signature stays, so
// that its callers check: the hover on a variable whose type comes from it, and on the call itself, answer as they do in a
// good file, and the hints and the tokens of the callers stay.
func TestCallersOfBlankedFunction(t *testing.T) {
	files, rel := macroProject()
	src := files[rel]
	c := startRefs(t, files, plainInit)
	c.open(rel, src)
	good := map[string]string{}
	marks := []string{"total := #mac00.Dbg(n + extra)", "n := Load|Policy2!(path)", "total| := #mac00.Dbg(n + extra)"}
	_ = marks
	hoverAll := func(text string) map[string]string {
		out := map[string]string{}
		for _, m := range []string{"tot|al := #mac00.Dbg(n + extra)", "n := Load|Policy2!(path)", "#mac00.D|bg(n + extra)"} {
			h, rng := c.hover(rel, text, m)
			first, _, _ := strings.Cut(h, "\n// the code")
			out[m] = first + "|" + rng
		}
		return out
	}
	for k, v := range hoverAll(src) {
		good[k] = v
	}
	// the `os` of os.Open(path) inside LoadPolicy2 is deleted: the body of that function has a syntax error
	bad := strings.Replace(src, "f := os.Open(path)", "f := .Open(path)", 1)
	if bad == src {
		t.Fatal("no place to break")
	}
	c.change(rel, 2, bad)
	for k, v := range hoverAll(bad) {
		if v != good[k] {
			t.Errorf("hover at %q in a file with a syntax error: %q, in the good file %q", k, v, good[k])
		}
		if strings.Contains(v, "invalid type") {
			t.Errorf("hover at %q: %q", k, v)
		}
	}
	locs := c.definitions("textDocument/definition", rel, bad, "n := Load|Policy2!(path)")
	if len(locs) != 1 || !strings.Contains(locs[0], "LoadPolicy2") {
		t.Errorf("definition of LoadPolicy2! in a file with a syntax error: %v", locs)
	}
	hints, toks := c.hints(rel, nil), c.semTokens(rel, nil)
	calls := 0
	for _, h := range hints {
		if strings.Contains(h, "→") {
			calls++
		}
	}
	if calls != 7 { // the 11 of the good file, but the 4 that LoadPolicy2 (the broken body) handles without a ! in the text
		t.Errorf("%d arrows in the hints of a file with a syntax error in one body, want 7", calls)
	}
	sortedTokens(t, toks)
	sortedHints(t, hints)
}

func sortedTokens(t *testing.T, toks []string) {
	t.Helper()
	pos := func(s string) (int, int) {
		var l, c int
		fmt.Sscanf(s, "%d:%d", &l, &c)
		return l, c
	}
	if !sort.SliceIsSorted(toks, func(i, j int) bool {
		li, ci := pos(toks[i])
		lj, cj := pos(toks[j])
		return li < lj || li == lj && ci < cj
	}) {
		t.Errorf("tokens out of order")
	}
}

func sortedHints(t *testing.T, hints []string) { t.Helper(); sortedTokens(t, hints) }

// TestRepairedAnswersAreValid: over many broken texts of the macro project's file, what the repaired paths answer is
// valid: the tokens are in order and inside the text (checkTokenOrder), the hints in order and inside the text, and
// nothing crashes.
func TestRepairedAnswersAreValid(t *testing.T) {
	files, rel := macroProject()
	c := startRefs(t, files, plainInit)
	src := files[rel]
	c.open(rel, src)
	step := max(len(src)/70, 1)
	version := 2
	for off := 40; off < len(src)-1; off += step {
		for _, bad := range []string{src[:off] + src[off+1:], src[:off] + "(" + src[off:], src[:off] + "{" + src[off:]} {
			c.change(rel, version, bad)
			version++
			checkTokenOrder(t, c, rel, bad)
			m := protocol.NewMapper(bad)
			var prev protocol.Position
			for _, h := range c.hintsRaw(rel, nil) {
				p := h.Position
				if int(p.Line) > strings.Count(bad, "\n") || m.Offset(p) > len(bad) || p.Line < prev.Line || p.Line == prev.Line && p.Character < prev.Character {
					t.Fatalf("a hint at %v after %v in the text broken at %d", p, prev, off)
				}
				prev = p
			}
		}
	}
}

// TestStaleBodyNeverReturnsOldPartial: changing a variable to a constant within the repaired body can change
// classifications and inferred types away from the edit. No old partial hint/token from that body may leak out.
func TestStaleBodyNeverReturnsOldPartial(t *testing.T) {
	src := "package a\n\nfunc broken() {\n\tx := 1\n\t_ = x\n\ty := 1 +\n}\n\nfunc Fine() int { return 1 }\nfunc use() { n := Fine(); _ = n }\n"
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "a/a.gp": src}
	c, s := startStale(t, files)
	c.open("a/a.gp", src)
	c.hover("a/a.gp", src, "_ = |n }")
	viewIs(t, s, c, "a/a.gp", src)
	hold := make(chan struct{})
	s.views.hold = hold
	defer close(hold)
	typed := strings.Replace(src, "x := 1", "const x = \"é😀\"", 1)
	c.change("a/a.gp", 2, typed)
	if text, rng := c.hover("a/a.gp", typed, "_ = |n }"); text != "var n int" || rng != "n" {
		t.Fatalf("unaffected hover %q (%q)", text, rng)
	}
	for _, h := range c.hintsRaw("a/a.gp", nil) {
		if h.Position.Line >= 3 && h.Position.Line <= 5 {
			t.Errorf("old-body hint at %v", h.Position)
		}
	}
	toks := c.semTokens("a/a.gp", nil)
	for _, tok := range toks {
		var line int
		fmt.Sscanf(tok, "%d:", &line)
		if line >= 4 && line <= 6 {
			t.Errorf("old-body token %s", tok)
		}
	}
	if !slicesContains(toks, "9:6 Fine ") {
		t.Errorf("unaffected tokens were lost: %v", toks)
	}
	checkTokenOrder(t, c, "a/a.gp", typed)
}

// TestBangActionParenthesizedCallee: an insertion after the name cannot turn (fail)() into a valid ! call.
// Suppress that unsupported edit, including qualified/repeated parentheses; still parse and check every offered edit.
func TestBangActionParenthesizedCallee(t *testing.T) {
	for _, tc := range []struct {
		call, mark string
		want       bool
	}{
		{"fail()", "_ = v; fa|il()", true},
		{"(fail)()", "(fa|il)()", false},
		{"((fail))()", "((fa|il))()", false},
		{"( /* keep */ fail )()", "fa|il )()", false},
		{"lib.Fail()", "lib.Fa|il()", true},
		{"(lib.Fail)()", "(lib.Fa|il)()", false},
		{"((lib.Fail))()", "((lib.Fa|il))()", false},
		{"((generic[int]))()", "((gen|eric[int]))()", false},
		{"(generic)[int]()", "(gen|eric)[int]()", false},
		{"(v).Fail()", "(v).Fa|il()", true},
		{"(v.Fail)()", "(v.Fa|il)()", false},
	} {
		t.Run(tc.call, func(t *testing.T) {
			head := "package main\n"
			if strings.Contains(tc.call, "lib.") {
				head += "import \"example.com/m/lib\"\n"
			}
			src := head + "func fail() error { return nil }\nfunc generic[T any]() error { return nil }\ntype receiver struct{}\nfunc (receiver) Fail() error { return nil }\nfunc main() { v := receiver{}; _ = v; " + tc.call + " }\n"
			c, s := startStale(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src, "lib/lib.gp": "package lib\nfunc Fail() error { return nil }\n"})
			c.open("main.gp", src)
			p := at(t, src, tc.mark)
			resp := c.call("textDocument/codeAction", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}},"context":{"diagnostics":[],"only":["refactor.rewrite"]}}`, c.uri("main.gp"), p.Line, p.Character, p.Line, p.Character))
			if resp.Error != nil {
				t.Fatal(resp.Error)
			}
			var actions []protocol.CodeAction
			if err := json.Unmarshal(resp.Result, &actions); err != nil {
				t.Fatal(err)
			}
			if !tc.want {
				if len(actions) != 0 {
					t.Errorf("unsupported callee offered an insertion: %s", resp.Result)
				}
				return
			}
			if len(actions) != 1 || actions[0].Edit == nil || len(actions[0].Edit.DocumentChanges) != 1 {
				t.Fatalf("supported callee has no action: %s", resp.Result)
			}
			var edits []protocol.TextEdit
			for _, e := range actions[0].Edit.DocumentChanges[0].TextDocumentEdit.Edits {
				switch e := e.Value.(type) {
				case protocol.TextEdit:
					edits = append(edits, e)
				case protocol.AnnotatedTextEdit:
					edits = append(edits, e.TextEdit)
				default:
					t.Fatalf("unexpected edit %T", e)
				}
			}
			changed := applyEdits(t, src, edits)
			if _, err := parser.ParseFile(token.NewFileSet(), "main.gp", changed, parser.Phase10Syntax); err != nil {
				t.Fatalf("action created invalid syntax: %s\n%v", changed, err)
			}
			c.change("main.gp", 2, changed)
			c.call(GocppStats, "{}")
			snap, free := s.session.Snapshot()
			defer free()
			pkg, err := snap.PackageOf(context.Background(), c.uri("main.gp"))
			if err != nil || pkg == nil || !pkg.Check.Checked() || pkg.Check.Errors.HasErrors() {
				t.Fatalf("edited call failed to check: %s\n%v, package %+v", changed, err, pkg)
			}
		})
	}
}

// TestReferencesAndActionsNeverUseRepairedText: the references and the rename of a name used in a body that is blanked
// for the repaired check still find that use, and the quick action to add a `!` is offered on a call in such a body.
func TestReferencesAndActionsNeverUseRepairedText(t *testing.T) {
	src := `package a

import "os"

func helper() int { return 1 }

func broken(path string) int {
	os.Remove(path)
	n := helper()
	return n + helper(
}

func fine() int { return helper() }
`
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "a/a.gp": src}
	c := startRefs(t, files, plainInit)
	c.open("a/a.gp", src)
	refs := c.references("a/a.gp", "func hel|per()", true)
	if len(refs) != 4 {
		t.Errorf("references of helper: %v, want the declaration and three uses (two in the broken function)", refs)
	}
	edited, err := c.rename("a/a.gp", "func hel|per()", "assist")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(edited["a/a.gp"], "assist"); got != 4 {
		t.Errorf("rename changed %d places, want 4:\n%s", got, edited["a/a.gp"])
	}
	// the action on os.Remove(path) in the broken function
	p := at(t, src, "os.Rem|ove(path)")
	resp := c.call("textDocument/codeAction", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}},"context":{"diagnostics":[]}}`,
		c.uri("a/a.gp"), p.Line, p.Character, p.Line, p.Character))
	if !strings.Contains(string(resp.Result), "Handle the error of Remove with !") {
		t.Errorf("no quick action on a call in a broken function: %s", resp.Result)
	}
}
