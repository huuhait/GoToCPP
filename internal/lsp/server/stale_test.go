package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

func TestShiftMaps(t *testing.T) {
	// "abcdef" -> "abXYZcdef": two characters before... insertion of XYZ at 2
	sh := diffShift([]byte("abcdef"), []byte("abXYZcdef"))
	if sh != (shift{2, 2, 5}) {
		t.Fatalf("insertion: %+v", sh)
	}
	for _, tc := range []struct {
		old     int
		new     int
		okNew   bool
		okOldOf bool
	}{{0, 0, true, true}, {1, 1, true, true}, {2, 0, false, true}, {3, 6, true, true}, {6, 9, true, true}} {
		n, ok := sh.toNew(tc.old)
		if ok != tc.okNew || ok && n != tc.new {
			t.Errorf("toNew(%d) = %d, %v", tc.old, n, ok)
		}
	}
	// a point inside the new text has no old place; the ones next to it do
	if _, ok := sh.toOld(3); ok {
		t.Errorf("toOld(3) inside the insertion")
	}
	if o, ok := sh.toOld(5); !ok || o != 2 {
		t.Errorf("toOld(5) = %d, %v; want 2", o, ok)
	}
	if o, ok := sh.toOld(1); !ok || o != 1 {
		t.Errorf("toOld(1) = %d, %v", o, ok)
	}
	// ranges: before the change, after it, never touching it
	for _, tc := range []struct {
		a, b   int
		na, nb int
		ok     bool
	}{{0, 1, 0, 1, true}, {0, 2, 0, 0, false}, {1, 3, 0, 0, false}, {2, 4, 0, 0, false}, {3, 5, 6, 8, true}, {2, 2, 0, 0, false}} {
		a, b, ok := sh.rangeToNew(tc.a, tc.b)
		if ok != tc.ok || ok && (a != tc.na || b != tc.nb) {
			t.Errorf("rangeToNew(%d, %d) = %d, %d, %v", tc.a, tc.b, a, b, ok)
		}
	}
	// a replacement and a deletion, equal texts
	if sh := diffShift([]byte("abcdef"), []byte("abQef")); sh != (shift{2, 4, 3}) {
		t.Errorf("replacement: %+v", sh)
	}
	if sh := diffShift([]byte("abcdef"), []byte("abcdef")); sh.p != 6 || sh.oldEnd != 6 || sh.newEnd != 6 {
		t.Errorf("equal: %+v", sh)
	}
	if sh := diffShift([]byte("aaaa"), []byte("aaa")); sh.oldEnd-sh.p != 1 || sh.newEnd != sh.p {
		t.Errorf("deletion in a run: %+v", sh)
	}
}

// startStale starts a server with the views of the files with a syntax error (stale.go), whose builds start at once.
func startStale(t *testing.T, files map[string]string) (*refClient, *Server) {
	t.Helper()
	root := writeTree(t, files)
	s := newServerOpts(nil, testConfig(), debounce.RealClock{}, Options{stale: true})
	s.views.gap = 0
	c := &dclient{client: startServer(t, s, nil), root: root}
	c.skipShown = true
	c.call("initialize", plainInit)
	c.notify("initialized", "{}")
	return &refClient{dclient: c, src: files}, s
}

// viewIs waits for the build of the text of the file.
func viewIs(t *testing.T, s *Server, c *refClient, name, text string) {
	t.Helper()
	fv := s.views.file(c.uri(name))
	end := time.Now().Add(time.Minute)
	for time.Now().Before(end) {
		fv.mu.Lock()
		ok := fv.last != nil && string(fv.last.text) == text
		busy := fv.running
		fv.mu.Unlock()
		if ok && !busy {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("no view of the text of %s", name)
}

// TestStaleViews: a request in a file with a syntax error that comes while the check of its text is not there is
// answered from the view of an earlier text, moved: hover at a place that did not change, the tokens and the hints
// of the rest of the file. A hover in a blanked body uses its current partial check, never an old body's answers.
func TestStaleViews(t *testing.T) {
	files, rel := macroProject()
	c, s := startStale(t, files)
	src := files[rel]
	c.open(rel, src)
	// 1. one syntax error: the first request waits for the first view, which is of this very text
	bad := strings.Replace(src, "n := LoadPolicy2!(path)", "n := LoadPolicy2!(path", 1)
	c.change(rel, 2, bad)
	mark := "f := os.Op|en!(path)"
	text, rng := c.hover(rel, bad, mark)
	if !strings.Contains(text, "returns it") || rng != "Open" {
		t.Fatalf("hover on the first view: %q (%q)", text, rng)
	}
	viewIs(t, s, c, rel, bad)
	tokens0 := c.semTokens(rel, nil)
	hints0 := c.hints(rel, nil)

	// 2. a line is typed above (the place moves) and a character at the broken place: the view is an older text
	// (the build of the new one waits until the test lets it)
	hold := make(chan struct{})
	s.views.hold = hold
	typed := strings.Replace(bad, "\treturn total, nil", "\t/* typed here */ return total, nil", 1)
	c.change(rel, 3, typed)
	text, rng = c.hover(rel, typed, mark)
	if !strings.Contains(text, "returns it") || rng != "Open" {
		t.Errorf("hover after typing: %q (%q)", text, rng)
	}
	toks := c.semTokens(rel, nil)
	hints := c.hints(rel, nil)
	if len(toks) < len(tokens0)/2 || len(hints) < len(hints0)/2 {
		t.Errorf("after typing: %d tokens, %d hints (before: %d, %d)", len(toks), len(hints), len(tokens0), len(hints0))
	}
	// the ranges are the ones of the new text: every token names what the new text has at its place
	checkTokens(t, c, rel, typed)
	// a range of the request asks for part of the hints of the older text, moved
	line := strings.Count(typed[:strings.Index(typed, "func SumPolicy2")], "\n")
	part := c.hintsRaw(rel, &protocol.Range{Start: protocol.Position{Line: 0}, End: protocol.Position{Line: uint32(line)}})
	for _, h := range part {
		if int(h.Position.Line) > line {
			t.Errorf("a hint at line %d for the range up to %d", h.Position.Line+1, line+1)
		}
	}
	if len(part) == 0 || len(part) >= len(hints) {
		t.Errorf("%d hints in the first %d lines, %d in all", len(part), line, len(hints))
	}
	// the stale answers have the ranges of the new text: the token at "Open" is where Open is in typed
	if want := fmt.Sprintf("%d:%d Open ", strings.Count(typed[:strings.Index(typed, "os.Open!")], "\n")+1, len("\tf := os.")+1); !slicesContains(toks, want) {
		t.Errorf("no token %q among the stale ones", want)
	}
	close(hold)
	viewIs(t, s, c, rel, typed)
	// 3. the view is of the text now: the same answers, from it
	text2, rng2 := c.hover(rel, typed, mark)
	if text2 != text || rng2 != rng {
		t.Errorf("hover on the exact view: %q (%q), the stale one was %q (%q)", text2, rng2, text, rng)
	}
	toks2 := c.semTokens(rel, nil)
	if len(toks2) < len(toks) {
		t.Errorf("fewer tokens on the exact view: %d < %d", len(toks2), len(toks))
	}
}

// checkTokens checks the tokens of the file as the client gets them: in order, in the text, one line each.
func checkTokens(t *testing.T, c *refClient, name, src string) {
	t.Helper()
	checkTokenOrder(t, c, name, src)
}

// TestStaleRefresh: after an answer from an older text the server asks a client that can for new tokens and hints when
// the check of the text as it is is done.
func TestStaleRefresh(t *testing.T) {
	files, rel := macroProject()
	root := writeTree(t, files)
	s := newServerOpts(nil, testConfig(), debounce.RealClock{}, Options{stale: true})
	s.views.gap = 0
	c := &dclient{client: startServer(t, s, nil), root: root}
	c.skipShown = true
	c.call("initialize", `{"processId":1,"rootUri":null,"capabilities":{"workspace":{"semanticTokens":{"refreshSupport":true},"inlayHint":{"refreshSupport":true}}}}`)
	c.notify("initialized", "{}")
	rc := &refClient{dclient: c, src: files}
	src := files[rel]
	rc.open(rel, src)
	bad := strings.Replace(src, "n := LoadPolicy2!(path)", "n := LoadPolicy2!(path", 1)
	rc.change(rel, 2, bad)
	rc.hover(rel, bad, "f := os.Op|en!(path)")
	viewIs(t, s, rc, rel, bad)
	typed := strings.Replace(bad, "\treturn total, nil", "\t/* typed */ return total, nil", 1)
	hold := make(chan struct{})
	s.views.hold = hold
	rc.change(rel, 3, typed)
	rc.semTokens(rel, nil) // from the older view: the build of the new text is held
	close(hold)
	// the requests of the server: they come with the end of the build
	seen := map[string]bool{}
	deadline := time.After(time.Minute)
	for !(seen["workspace/semanticTokens/refresh"] && seen["workspace/inlayHint/refresh"]) {
		select {
		case m := <-c.msgs:
			if req, ok := m.(*protocol.Request); ok && !req.IsNotification() {
				seen[req.Method] = true
			}
		case <-deadline:
			t.Fatalf("no refresh requests: %v", seen)
		}
	}
}

// TestHoverBlankedBodyUsesCurrentPartial: this place has no answer in the repaired body. A hover must use the
// current partial check without waiting for the deliberately blocked, unused repaired build, including after API
// changes/reverts and UTF-16 shifts. It must not borrow a previous snapshot's partial types to get there faster.
func TestHoverBlankedBodyUsesCurrentPartial(t *testing.T) {
	for _, mode := range []string{"same-file", "sibling", "import"} {
		t.Run(mode, func(t *testing.T) {
			src := "package a\n\nfunc broken() { value := Value(); _ = value; _ = 1 +\n}\nfunc fine() { n := Value(); _ = n }\n"
			provider := "package a\n\nfunc Value() int { return 1 }\n"
			rel, dep := "a/a.gp", "a/value.gp"
			if mode == "import" {
				dep = "dep/value.gp"
				provider = strings.Replace(provider, "package a", "package dep", 1)
				src = strings.Replace(src, "package a\n", "package a\n\nimport \"example.com/m/dep\"\n", 1)
				src = strings.ReplaceAll(src, "Value()", "dep.Value()")
			}
			files := map[string]string{"gocpp.mod": "module example.com/m\n", rel: src, dep: provider}
			if mode == "same-file" {
				delete(files, dep)
				dep = rel
				src = strings.Replace(src, "func broken", "func Value() int { return 1 }\n\nfunc broken", 1)
				files[rel] = src
				provider = src
			}
			c, s := startStale(t, files)
			hold := make(chan struct{})
			s.views.hold = hold // no feature request has started a view builder yet
			defer close(hold)
			if dep != rel {
				c.open(dep, provider)
			}
			c.open(rel, src)
			c.call(GocppStats, "{}")
			// Prime only the original partial check, not a repaired view.
			snap, free := s.session.Snapshot()
			q, err := newQuery(context.Background(), snap, c.uri(rel), at(t, src, "_ = va|lue;"))
			if err != nil || q == nil {
				free()
				t.Fatalf("initial partial query: %v %v", q, err)
			}
			q.close()
			free()
			for i, want := range []string{"string", "int"} {
				current := src
				text := provider
				if i == 0 {
					text = strings.Replace(text, "func Value() int { return 1 }", "func Value() string { return \"one\" }", 1)
					current = strings.Replace(current, "_ = value", "_ = \"é😀\"; _ = value", 1)
				}
				if dep == rel {
					if i == 0 {
						current = strings.Replace(current, "func Value() int { return 1 }", "func Value() string { return \"one\" }", 1)
					}
				} else {
					c.change(dep, i+2, text)
				}
				c.change(rel, i+2, current)
				c.call(GocppStats, "{}")
				snap, free = s.session.Snapshot()
				// The repaired view is held until the end of the test, so a hover that waits for it
				// never ends by itself: the limit only turns that hang into a failure, and no
				// answer depends on how fast the machine is.
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				p := protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: c.uri(rel)},
					Position:     at(t, current, "_ = va|lue;"),
				}
				h, rng, err := s.hoverAt(ctx, env{snapshot: snap, views: s.views}, p, true)
				cancel()
				free()
				if err != nil || h == nil || h.signature != "var value "+want {
					t.Fatalf("current %s hover waited for repair or used old types: %+v (%v)", want, h, err)
				}
				m := protocol.NewMapper(current)
				a, b, err := m.RangeOffsets(rng)
				if err != nil || current[a:b] != "value" {
					t.Errorf("current UTF-16 range %+v: %v", rng, err)
				}
			}
		})
	}
}

// TestPartialBodyHoverMatchesRepairedOrigin: ! and macro hovers inside a blanked body are exactly the current
// original query that the full repaired-view path would select, including their explanations and ranges.
func TestPartialBodyHoverMatchesRepairedOrigin(t *testing.T) {
	files, rel := macroProject()
	src := strings.Replace(files[rel], "\treturn total, nil", "\ty := #mac00.Dbg(\n\treturn total, nil", 1)
	files[rel] = src
	c, s := startStale(t, files)
	c.open(rel, src)
	c.call(GocppStats, "{}")
	snap, free := s.session.Snapshot()
	defer free()
	for _, mark := range []string{"n := Load|Policy2!(path)", "total := #mac00.D|bg(n + extra)"} {
		position := at(t, src, mark)
		current, err := newHoverQuery(context.Background(), env{snapshot: snap}, c.uri(rel), position, true)
		if err != nil || current == nil {
			t.Fatalf("current partial hover at %q: %v", mark, err)
		}
		got, gr, err := current.hover(context.Background())
		current.close()
		if err != nil || got == nil {
			t.Fatalf("empty current partial hover at %q: %v", mark, err)
		}
		baseline, err := newViewQuery(context.Background(), env{snapshot: snap}, c.uri(rel), position, false, false)
		if err != nil || baseline == nil {
			t.Fatalf("repaired-origin query at %q: %v", mark, err)
		}
		want, wr, err := baseline.hover(context.Background())
		baseline.close()
		if err != nil || want == nil || got.format(false) != want.format(false) || gr != wr {
			t.Errorf("partial and repaired-origin hovers differ at %q: got %+v %v, want %+v %v (%v)", mark, got, gr, want, wr, err)
		}
	}
}

// TestStaleHoverAtTheChange: a hover where the text just changed is not answered from an older text.
func TestStaleHoverAtTheChange(t *testing.T) {
	files, rel := macroProject()
	c, s := startStale(t, files)
	src := files[rel]
	c.open(rel, src)
	bad := strings.Replace(src, "n := LoadPolicy2!(path)", "n := LoadPolicy2!(path", 1)
	c.change(rel, 2, bad)
	c.hover(rel, bad, "f := os.Op|en!(path)")
	viewIs(t, s, c, rel, bad)
	// the name under the cursor changes: Open becomes Opened (the build of the new text is held: an older view is all there is)
	hold := make(chan struct{})
	s.views.hold = hold
	go func() { time.Sleep(300 * time.Millisecond); close(hold) }()
	typed := strings.Replace(bad, "f := os.Open!(path)", "f := os.Opened!(path)", 1)
	c.change(rel, 3, typed)
	text, _ := c.hover(rel, typed, "f := os.Op|ened!(path)")
	if strings.Contains(text, "func os.Open(") {
		t.Errorf("hover on Opened gave the answer for Open: %q", text)
	}
	// inside the characters just typed there is no old place at all
	hold2 := make(chan struct{})
	s.views.hold = hold2
	typed2 := strings.Replace(typed, "f := os.Opened!(path)", "f := os.Openedd!(path)", 1)
	go func() { time.Sleep(300 * time.Millisecond); close(hold2) }()
	c.change(rel, 4, typed2)
	if text, _ := c.hover(rel, typed2, "f := os.Opened|d!(path)"); strings.Contains(text, "func os.Open(") {
		t.Errorf("hover inside the typed characters gave the answer for Open: %q", text)
	}
}

func slicesContains(list []string, want string) bool {
	for _, x := range list {
		if strings.HasPrefix(x, want) {
			return true
		}
	}
	return false
}

// TestStaleViewsForgetOnClose: the view of a file, with the snapshot it holds, goes with the file.
func TestStaleViewsForgetOnClose(t *testing.T) {
	files, rel := macroProject()
	c, s := startStale(t, files)
	src := files[rel]
	c.open(rel, src)
	bad := strings.Replace(src, "n := LoadPolicy2!(path)", "n := LoadPolicy2!(path", 1)
	c.change(rel, 2, bad)
	c.hover(rel, bad, "f := os.Op|en!(path)")
	viewIs(t, s, c, rel, bad)
	if !s.views.known(c.uri(rel)) {
		t.Fatal("no view")
	}
	c.notify("textDocument/didClose", fmt.Sprintf(`{"textDocument":{"uri":%q}}`, c.uri(rel)))
	c.hover(rel, bad, "f := os.Op|en!(path)") // (the notification is read before the request)
	if s.views.known(c.uri(rel)) {
		t.Errorf("the view is still there after the file is closed")
	}
}

// TestHoverInvalidFallsBack: a name that a blanked declaration declares has no type in the repaired check: the hover is the one of the
// text as it is, not "invalid type".
func TestHoverInvalidFallsBack(t *testing.T) {
	src := "package a\n\ntype T struct {\n\tX int\n\nfunc use() int {\n\tvar t T\n\t_ = t\n\treturn 1\n}\n"
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "a/a.gp": src}
	c := startRefs(t, files, plainInit)
	c.open("a/a.gp", src)
	text, _ := c.hover("a/a.gp", src, "var |t T")
	if strings.Contains(text, "invalid type") || text == "" {
		t.Errorf("hover on t: %q", text)
	}
}

// TestCompletionHidesGpMust: the hidden import of the `!` pass is not a candidate.
func TestCompletionHidesGpMust(t *testing.T) {
	files, rel := macroProject()
	c := startComp(t, files, plainCompInit)
	src := files[rel]
	i := strings.Index(src, "\treturn total, nil")
	items := c.complete(rel, src[:i]+"\tgp|\n"+src[i:])
	for _, it := range items {
		if strings.HasPrefix(it.Label, "gp_") {
			t.Errorf("candidate %q", it.Label)
		}
	}
}

// TestStaleSemanticInputs compares repaired answers with a fresh server after semantic inputs change, including a revert.
func TestStaleSemanticInputs(t *testing.T) {
	for _, mode := range []string{"sibling", "import", "same-file"} {
		t.Run(mode, func(t *testing.T) {
			provider := "package a\n\nfunc Value() int { return 1 }\n"
			src := "package a\n\nfunc broken() {\n\tx := 1 +\n}\n\nfunc use() { n := Value(); _ = n }\n"
			rel, dep := "a/a.gp", "a/value.gp"
			if mode == "import" {
				dep = "dep/value.gp"
				provider = strings.Replace(provider, "package a", "package dep", 1)
				src = strings.Replace(src, "package a\n", "package a\n\nimport \"example.com/m/dep\"\n", 1)
				src = strings.Replace(src, "Value()", "dep.Value()", 1)
			}
			files := map[string]string{"gocpp.mod": "module example.com/m\n", rel: src, dep: provider}
			if mode == "same-file" {
				delete(files, dep)
				dep = rel
				src = strings.Replace(src, "func broken", "func Value() int { return 1 }\n\nfunc broken", 1)
				files[rel] = src
				provider = src
			}
			c, s := startStale(t, files)
			c.open(rel, src)
			c.hover(rel, src, "_ = |n }")
			viewIs(t, s, c, rel, src)
			if dep != rel {
				c.open(dep, provider)
			}
			changed := strings.Replace(provider, "func Value() int { return 1 }", "var Value = func() string { return \"one\" }", 1)
			for version, text := range []string{changed, provider} {
				c.change(dep, version+2, text)
				files[dep] = text
				current := files[rel]
				fresh := startRefs(t, files, plainInit)
				fresh.open(rel, current)
				got, gr := c.hover(rel, current, "_ = |n }")
				want, wr := fresh.hover(rel, current, "_ = |n }")
				if got != want || gr != wr {
					t.Errorf("version %d hover %q (%s), fresh %q (%s)", version+2, got, gr, want, wr)
				}
				mark := "n := Val|ue()"
				if mode == "import" {
					mark = "n := dep.Val|ue()"
				}
				gotDefs := c.definitions("textDocument/definition", rel, current, mark)
				wantDefs := fresh.definitions("textDocument/definition", rel, current, mark)
				sameList(t, "definitions", gotDefs, wantDefs)
				sameList(t, "hints", c.hints(rel, nil), fresh.hints(rel, nil))
				sameList(t, "tokens", c.semTokens(rel, nil), fresh.semTokens(rel, nil))
			}
		})
	}
}

// TestStaleLexicalBoundaries: delimiters/comments/strings cannot turn old body positions into answers in unrelated
// current text. Compare every answer with a fresh server; the UTF-16 edit also checks shifted, in-bounds tokens.
func TestStaleLexicalBoundaries(t *testing.T) {
	for _, insert := range []string{"}", "/*", "`", "/* é😀 */", "\n// typed\n"} {
		t.Run(fmt.Sprintf("%q", insert), func(t *testing.T) {
			src := "package a\n\nfunc broken() {\n\tx := 1 +\n}\n\nfunc Fine() int { return 1 }\nfunc use() { n := Fine(); _ = n }\n"
			files := map[string]string{"gocpp.mod": "module example.com/m\n", "a/a.gp": src}
			c, s := startStale(t, files)
			c.open("a/a.gp", src)
			c.hover("a/a.gp", src, "_ = |n }")
			viewIs(t, s, c, "a/a.gp", src)
			typed := strings.Replace(src, "x := 1 +", insert+"x := 1 +", 1)
			c.change("a/a.gp", 2, typed)
			files["a/a.gp"] = typed
			fresh := startRefs(t, files, plainInit)
			fresh.open("a/a.gp", typed)
			got, gr := c.hover("a/a.gp", typed, "_ = |n }")
			want, wr := fresh.hover("a/a.gp", typed, "_ = |n }")
			if got != want || gr != wr {
				t.Errorf("hover %q (%s), fresh %q (%s)", got, gr, want, wr)
			}
			sameList(t, "definitions", c.definitions("textDocument/definition", "a/a.gp", typed, "n := Fi|ne()"), fresh.definitions("textDocument/definition", "a/a.gp", typed, "n := Fi|ne()"))
			// Definition cannot use a stale view, so the completed view now has the exact current body too.
			sameList(t, "hints", c.hints("a/a.gp", nil), fresh.hints("a/a.gp", nil))
			sameList(t, "tokens", c.semTokens("a/a.gp", nil), fresh.semTokens("a/a.gp", nil))
			checkTokenOrder(t, c, "a/a.gp", typed)
		})
	}
}

func TestStaleEventChainAndCancellation(t *testing.T) {
	files, rel := macroProject()
	c, s := startStale(t, files)
	src := files[rel]
	c.open(rel, src)
	bad := strings.Replace(src, "n := LoadPolicy2!(path)", "n := LoadPolicy2!(path", 1)
	c.change(rel, 2, bad)
	c.hover(rel, bad, "f := os.Op|en!(path)")
	viewIs(t, s, c, rel, bad)
	fv := s.views.file(c.uri(rel))
	// A target edit, then a sibling event, then another target edit must not bridge the unknown input.
	snap, free := s.session.Snapshot()
	id := snap.ID()
	free()
	s.views.changed(c.uri(rel), id, id+1)
	s.views.changed(c.uri(rel), id+2, id+3)
	fv.mu.Lock()
	bridged := fv.sameInputs(fv.last, id+3)
	fv.mu.Unlock()
	if bridged {
		t.Fatal("the target-only event chain bridged a sibling event")
	}

	hold := make(chan struct{})
	s.views.hold = hold
	defer close(hold)
	typed := strings.Replace(bad, "\treturn total, nil", "\t/* typed */ return total, nil", 1)
	c.change(rel, 3, typed)
	c.call(GocppStats, "{}") // notification barrier
	snap, free = s.session.Snapshot()
	defer free()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	q, err := newViewQuery(ctx, env{snapshot: snap, views: s.views}, c.uri(rel), at(t, typed, "/* ty|ped */"), false, true)
	q.close()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a request waiting for the current body: %v", err)
	}
}

// TestStaleCandidateAfterCurrentBuild: a request may hold a reference to the old view while another request
// completes the current one. Resuming the old candidate must take the current view, not omit body tokens forever
// after the builder has already made its refresh decision.
func TestStaleCandidateAfterCurrentBuild(t *testing.T) {
	src := "package main\nfunc broken() {\n var a int; _ = a; _ = 1 +\n}\nfunc fine() { println(1) }\nfunc main() { fine() }\n"
	c, s := startStale(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	c.open("main.gp", src)
	c.semTokens("main.gp", nil)
	viewIs(t, s, c, "main.gp", src)
	fv := s.views.file(c.uri("main.gp"))
	fv.mu.Lock()
	old := fv.ref(fv.last) // the first request is descheduled with this reference
	fv.mu.Unlock()
	consumed := false
	defer func() {
		if !consumed {
			fv.unref(old)
		}
	}()
	typed := strings.Replace(src, "_ = a", "_ = 0", 1)
	c.change("main.gp", 2, typed)
	c.call(GocppStats, "{}") // didChange barrier before taking the request's snapshot
	snap, free := s.session.Snapshot()
	defer free()
	pr, _, err := prepareRepair(context.Background(), snap, c.uri("main.gp"))
	if err != nil || pr == nil {
		t.Fatalf("repair: %v %v", pr, err)
	}
	// A request inside the edited body waits for a current view and finishes the build first.
	q, err := newViewQuery(context.Background(), env{snapshot: snap, views: s.views}, c.uri("main.gp"), at(t, typed, "var a i|nt"), false, false)
	if err != nil || q == nil {
		t.Fatalf("current query: %v %v", q, err)
	}
	q.close()
	viewIs(t, s, c, "main.gp", typed)
	q, err = fv.candidate(context.Background(), old, pr, protocol.Position{}, true, true, viewKey{pr.pf.Hash, snap.ID()})
	consumed = true
	if err != nil || q == nil {
		t.Fatalf("resumed candidate: %v %v", q, err)
	}
	defer q.close()
	if q.shift != nil || q.staleBlank != [2]int{} || string(q.src) != typed {
		t.Fatalf("the resumed candidate did not take the exact current view")
	}
	found := false
	for _, tok := range q.semanticTokens() {
		if string(q.src[tok.off:tok.end]) == "a" {
			found = true
		}
	}
	if !found {
		t.Error("current tokens omitted the edited body's declaration of a")
	}
	fv.mu.Lock()
	pending := fv.staleServed
	fv.mu.Unlock()
	if pending {
		t.Error("staleServed was stranded after the current build completed")
	}
}

// TestStaleHoverInsideNewText: a hover inside the characters just typed (a comment) has no old place, and is not answered
// with what the old text has at the same offset.
func TestStaleHoverInsideNewText(t *testing.T) {
	files, rel := macroProject()
	c, s := startStale(t, files)
	src := files[rel]
	c.open(rel, src)
	bad := strings.Replace(src, "n := LoadPolicy2!(path)", "n := LoadPolicy2!(path", 1)
	c.change(rel, 2, bad)
	c.hover(rel, bad, "f := os.Op|en!(path)")
	viewIs(t, s, c, rel, bad)
	hold := make(chan struct{})
	s.views.hold = hold
	go func() { time.Sleep(300 * time.Millisecond); close(hold) }()
	typed := strings.Replace(bad, "func ClosePolicy2", "// a comment of sixty characters, typed just now, in front of it\nfunc ClosePolicy2", 1)
	c.change(rel, 3, typed)
	if text, rng := c.hover(rel, typed, "// a co|mment of sixty"); text != "" {
		t.Errorf("hover in a comment: %q (%q)", text, rng)
	}
}
