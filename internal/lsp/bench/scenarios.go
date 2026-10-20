package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// A scenario is one thing an editor does, repeated: iters measured
// repetitions after warm that are not counted (the very first repetition,
// the warm-up one, is kept as the "first" figure: what the first request of
// its kind costs).
type scenario struct {
	name      string
	iters     int
	warm      int
	gocppOnly bool

	// prep runs once before the repetitions; an error skips the scenario.
	prep func(ctx context.Context, s *Session) error
	// run does repetition i and returns its latency, and the name of the
	// series it belongs to ("" is the scenario's own name).
	run func(ctx context.Context, s *Session, i int) (series string, d time.Duration, err error)
	// done puts the documents back as they were.
	done func(ctx context.Context, s *Session) error
}

// A mark is a place in a document: the byte after the first occurrence of
// needle's start plus delta.
type mark struct {
	needle string
	delta  int
}

// Names of the scenarios (the rows of the report and of the budgets).
const (
	ScInit            = "initialize"
	ScOpenCold        = "open-cold"
	ScOpenWarm        = "open-warm"
	ScKeystroke       = "keystroke"
	ScKeystrokeDep    = "keystroke-imported"
	ScKeystrokeNew    = "keystroke-new"
	ScKeystrokeDepNew = "keystroke-imported-new"
	ScCompletionDot   = "completion-dot"
	ScCompletionEdit  = "completion-edit"
	ScHover           = "hover"
	ScDefinition      = "definition"
	ScSignature       = "signature-help"
	ScRefsPopular     = "references-popular"
	ScRefsLocal       = "references-local"
	ScRefsFamily      = "references-family"
	ScRename          = "rename"
	ScWorkspaceSymbol = "workspace-symbol"
	ScDocumentSymbol  = "document-symbol"
	ScSemanticTokens  = "semantic-tokens"
	ScFormat          = "format-1k"
	ScSwitchFile      = "switch-file"
	ScAPIChange       = "api-change"
	ScAPIFix          = "api-fix"
	ScCppOpenCold     = "open-cpp-cold"
	ScCppOpenWarm     = "open-cpp-warm"
	ScTypoFirstCold   = "typo-first-cold"
	ScTypoFirstWarm   = "typo-first-warm"
	ScTypoErrorCold   = "typo-error-cold"
	ScTypoErrorWarm   = "typo-error-warm"
)

// order is the order of the rows in a report.
var order = []string{ScInit, ScOpenCold, ScOpenWarm, ScCppOpenCold, ScCppOpenWarm, ScTypoFirstCold, ScTypoErrorCold, ScTypoFirstWarm, ScTypoErrorWarm,
	ScKeystroke, ScKeystrokeDep, ScKeystrokeNew, ScKeystrokeDepNew, ScCompletionDot, ScCompletionEdit,
	ScHover, ScDefinition, ScSignature, ScRefsPopular, ScRefsLocal, ScRefsFamily, ScRename, ScWorkspaceSymbol, ScDocumentSymbol, ScSemanticTokens,
	ScFormat, ScSwitchFile, ScAPIChange, ScAPIFix}

// settle is the quiet time after an edit that is put back, so that its
// diagnostics (and a cancelled run) don't reach into the next repetition.
const settle = 80 * time.Millisecond

// afterOpen is the time between the open of the project and the first
// scenario: the user reads the code before asking for anything, and a server
// may use the time for what it can prepare (gopls loads the workspace, gocpp
// checks the importers of the open packages).
const afterOpen = time.Second

// think is the time between two keystrokes of the keystroke scenarios: a very fast typist
// (40 keys a second); editors do not send the next character the moment the
// diagnostics of the last one arrive, and a server may use the gap (a
// garbage collection, say) as long as it is quick to answer when the next key comes.
const think = 25 * time.Millisecond

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}

// scenarios returns the session scenarios of the project, in the order
// they run (the edits of each are put back).
func scenarios(p *Project) []scenario {
	hot := p.Hot
	hotMarks := []mark{{"core.NewBase(", 7}, {"fmt.Sprintf(", 6}, {"strings.Join(", 10}, {"func (r *", 9}, {"\tr.mu.Lock()", 1}}
	// gocpp answers definitions in the project, not (yet) in the embedded
	// standard library: the marks are the project's names.
	defMarks := []mark{{"core.NewBase(", 7}, {"core.ErrEmpty", 8}, {"r.index[tag]", 2}, {"it.Add(", 3}, {"\tcore.Base\n", 7}}
	sigMarks := []mark{{"core.NewBase(", len("core.NewBase(")}, {"it.Add(", len("it.Add(")}, {"Label(tag", len("Label(tag")}}
	scs := []scenario{
		keystroke(ScKeystroke, hot, 40, 3),
		keystroke(ScKeystrokeDep, p.Dep, 20, 2),
		keystrokeNew(ScKeystrokeNew, hot, 40, 3),
		keystrokeNew(ScKeystrokeDepNew, p.Dep, 20, 2),
		completionDot(hot, 60),
		completionEdit(hot, 30),
		position(ScHover, "textDocument/hover", hot, hotMarks, 60),
		position(ScDefinition, "textDocument/definition", hot, defMarks, 60),
		position(ScSignature, "textDocument/signatureHelp", hot, sigMarks, 60),
		position(ScRefsPopular, "textDocument/references", p.Core, []mark{{"type Base struct", 5}}, 6),
		position(ScRefsLocal, "textDocument/references", hot, []mark{{"\ntype ", 6}}, 20),
		position(ScRefsFamily, "textDocument/references", hot, []mark{{") Add(tag string)", 2}}, 8),
		renameLabel(p),
		workspaceSymbol(p.Queries, 30),
		whole(ScDocumentSymbol, "textDocument/documentSymbol", hot, 40),
		whole(ScSemanticTokens, "textDocument/semanticTokens/full", hot, 20),
		formatBig(p),
		switchFile(p),
		apiChange(p),
	}
	return scs
}

// keystroke types one character into the middle of a function (the name
// of a variable gets an x, so that the file has errors) and takes it out
// again, and measures the time to the new diagnostics: the keystroke to
// the editor's squiggle.
func keystroke(name, rel string, iters, warm int) scenario {
	var off, base int
	return scenario{
		name: name, iters: iters, warm: warm,
		prep: func(ctx context.Context, s *Session) (err error) {
			base = s.baseline(rel)
			off, err = s.Find(rel, "\tsum := 0\n", 4)
			return err
		},
		run: func(ctx context.Context, s *Session, i int) (string, time.Duration, error) {
			seq := s.C.Seq()
			var t0 time.Time
			var err error
			ok := moreErrors(base)
			if i%2 == 0 {
				t0, err = s.Edit(rel, off, off, "x")
			} else {
				t0, err = s.Edit(rel, off, off+1, "")
				ok = fewerErrors(base)
			}
			if err != nil {
				return "", 0, err
			}
			d, _, err := s.Wait(ctx, rel, seq, t0, ok)
			sleep(ctx, think)
			return "", d, err
		},
		done: func(ctx context.Context, s *Session) error {
			if strings.Contains(s.Doc(rel).Text, "sumx := 0") {
				_, err := s.Edit(rel, off, off+1, "")
				return err
			}
			return nil
		},
	}
}

// keystrokeNew is keystroke with text that no server has seen: the two
// texts of keystroke come back every other edit, so a server that
// remembers its results by the content of the file answers from memory;
// an editor's text is new at each keystroke. Each repetition replaces the
// initial value of the variable by a number that was never in the file
// (the diagnostics stay what they were) or by the name of a variable that
// does not exist, with a number in it (one more error), in turn.
func keystrokeNew(name, rel string, iters, warm int) scenario {
	var off, cur int
	const head = "\tsum := "
	return scenario{
		name: name, iters: iters, warm: warm,
		prep: func(ctx context.Context, s *Session) (err error) {
			off, err = s.Find(rel, head+"0\n", len(head))
			cur = 1
			return err
		},
		run: func(ctx context.Context, s *Session, i int) (string, time.Duration, error) {
			seq := s.C.Seq()
			old := s.Doc(rel).Text[off:]
			old = old[:strings.IndexByte(old, '\n')]
			cur++
			next := fmt.Sprintf("%d", 1000+cur)
			// The wait is for the diagnostics of this text: the error about the name
			// that this edit wrote, or none about any such name.
			undefined := fmt.Sprintf("undefinedName%d", cur)
			wantError := i%2 == 0
			if wantError {
				next = undefined
			}
			t0, err := s.Edit(rel, off, off+len(old), next)
			if err != nil {
				return "", 0, err
			}
			ver := s.Doc(rel).Version
			ok := func(p *Publish) bool { return freshDiagnostics(p, ver, undefined, wantError) }
			d, _, err := s.Wait(ctx, rel, seq, t0, ok)
			sleep(ctx, think)
			return "", d, err
		},
		done: func(ctx context.Context, s *Session) error {
			old := s.Doc(rel).Text[off:]
			old = old[:strings.IndexByte(old, '\n')]
			if old != "0" {
				_, err := s.Edit(rel, off, off+len(old), "0")
				return err
			}
			return nil
		},
	}
}

// freshDiagnostics refuses unversioned or stale publishes, and on a fixed edit
// refuses errors from ANY earlier synthetic name, not merely the new one. An
// ambiguous publish is a failed measurement, never an apparent cache speedup.
func freshDiagnostics(p *Publish, version int32, name string, wantError bool) bool {
	if p.Version < version {
		return false
	}
	found := false
	for _, d := range p.Diags {
		if strings.Contains(d.Message, "undefinedName") {
			words := strings.FieldsFunc(d.Message, func(r rune) bool {
				return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_')
			})
			if !wantError || !slices.Contains(words, name) {
				return false
			}
			found = true
		}
	}
	return found == wantError
}

// completionDot asks for the completions after the dot of r.mu.Lock(),
// with nothing changed since the last check.
func completionDot(rel string, iters int) scenario {
	return position(ScCompletionDot, "textDocument/completion", rel, []mark{{"\tr.mu.Lock()", 3}}, iters)
}

// completionEdit types "r." at the start of a line and asks for the
// completions at once, as an editor does: the time from the keystroke to
// the menu, with the check that the new text needs.
func completionEdit(rel string, iters int) scenario {
	const typedLine = "\tr.\n"
	var off int
	return scenario{
		name: ScCompletionEdit, iters: iters, warm: 2,
		prep: func(ctx context.Context, s *Session) (err error) {
			off, err = s.Find(rel, "\tr.mu.Lock()\n", 0)
			return err
		},
		run: func(ctx context.Context, s *Session, i int) (string, time.Duration, error) {
			// A comment line with a number in front, so that the text is new each
			// time (see keystrokeNew).
			typed := fmt.Sprintf("\t// n%d\n", i) + typedLine
			t0, err := s.Edit(rel, off, off, typed)
			if err != nil {
				return "", 0, err
			}
			p := s.at(rel, off+len(typed)-len(typedLine)+3)
			_, raw, err := s.call(ctx, "textDocument/completion", p)
			d := time.Since(t0)
			if err == nil && !nonEmpty(raw) {
				err = fmt.Errorf("completion after %q: no items", typed)
			}
			if _, e := s.Edit(rel, off, off+len(typed), ""); err == nil {
				err = e
			}
			sleep(ctx, settle)
			return "", d, err
		},
	}
}

// position asks method at each of the marks of the file in turn.
func position(name, method, rel string, marks []mark, iters int) scenario {
	offs := make([]int, len(marks))
	return scenario{
		name: name, iters: iters, warm: 2,
		prep: func(ctx context.Context, s *Session) error {
			for i, m := range marks {
				off, err := s.Find(rel, m.needle, m.delta)
				if err != nil {
					return err
				}
				offs[i] = off
			}
			return nil
		},
		run: func(ctx context.Context, s *Session, i int) (string, time.Duration, error) {
			p := s.at(rel, offs[i%len(offs)])
			if method == "textDocument/references" {
				p.Context = &struct {
					IncludeDeclaration bool `json:"includeDeclaration"`
				}{true}
			}
			d, raw, err := s.call(ctx, method, p)
			if err == nil && !nonEmpty(raw) {
				err = fmt.Errorf("%s at %q: empty result", method, marks[i%len(marks)].needle)
			}
			if err == nil && method == "textDocument/references" {
				var locs []json.RawMessage
				if json.Unmarshal(raw, &locs) == nil {
					s.Facts[name] = fmt.Sprintf("%s: %d locations", name, len(locs))
				}
			}
			return "", d, err
		},
	}
}

// whole asks method about the whole document.
func whole(name, method, rel string, iters int) scenario {
	return scenario{
		name: name, iters: iters, warm: 2,
		run: func(ctx context.Context, s *Session, i int) (string, time.Duration, error) {
			params := map[string]any{"textDocument": map[string]any{"uri": s.Doc(rel).URI}}
			d, raw, err := s.call(ctx, method, params)
			if err == nil && !nonEmpty(raw) {
				err = fmt.Errorf("%s: empty result", method)
			}
			return "", d, err
		},
	}
}

// renameLabel prepares and renames the function Label of the hot package, which
// the packages that import it call: the time of both requests together.
func renameLabel(p *Project) scenario {
	rel := p.HotPkg + "/f00"
	var off int
	return scenario{
		name: ScRename, iters: 5, warm: 1,
		prep: func(ctx context.Context, s *Session) (err error) {
			if _, err := s.Open(rel); err != nil {
				return err
			}
			off, err = s.Find(rel, "func Label(", 5)
			return err
		},
		run: func(ctx context.Context, s *Session, i int) (string, time.Duration, error) {
			d1, _, err := s.call(ctx, "textDocument/prepareRename", s.at(rel, off))
			if err != nil {
				return "", 0, err
			}
			par := s.at(rel, off)
			// A name each time: a server that remembers the result of a rename by
			// its edits would answer the same one at once. A user renames once.
			par.NewName = fmt.Sprintf("TagOf%d", i)
			d2, raw, err := s.call(ctx, "textDocument/rename", par)
			if err == nil && !nonEmpty(raw) {
				err = fmt.Errorf("rename: empty edit")
			}
			return "", d1 + d2, err
		},
	}
}

// workspaceSymbol queries the symbols of the workspace.
func workspaceSymbol(queries []string, iters int) scenario {
	return scenario{
		name: ScWorkspaceSymbol, iters: iters, warm: 2,
		run: func(ctx context.Context, s *Session, i int) (string, time.Duration, error) {
			d, raw, err := s.call(ctx, "workspace/symbol", map[string]any{"query": queries[i%len(queries)]})
			if err == nil && !nonEmpty(raw) {
				err = fmt.Errorf("workspace/symbol %q: empty result", queries[i%len(queries)])
			}
			return "", d, err
		},
	}
}

// formatBig formats the file of 1,000 lines on save: with its indentation
// taken out, so that every line needs an edit.
func formatBig(p *Project) scenario {
	rel := p.Big
	var good, messy string
	return scenario{
		name: ScFormat, iters: 15, warm: 2,
		prep: func(ctx context.Context, s *Session) error {
			if _, err := s.Open(rel); err != nil {
				return err
			}
			good = s.Doc(rel).Text
			messy = strings.ReplaceAll(good, "\n\t", "\n")
			return nil
		},
		run: func(ctx context.Context, s *Session, i int) (string, time.Duration, error) {
			if _, err := s.SetText(rel, messy); err != nil {
				return "", 0, err
			}
			sleep(ctx, 150*time.Millisecond)
			params := map[string]any{"textDocument": map[string]any{"uri": s.Doc(rel).URI},
				"options": map[string]any{"tabSize": 8, "insertSpaces": false}}
			d, raw, err := s.call(ctx, "textDocument/formatting", params)
			if err == nil && !nonEmpty(raw) {
				err = fmt.Errorf("formatting: no edits")
			}
			if _, e := s.SetText(rel, good); err == nil {
				err = e
			}
			sleep(ctx, 150*time.Millisecond)
			return "", d, err
		},
	}
}

// switchFile closes the file open and opens another, in another package:
// the time from the open to its diagnostics.
func switchFile(p *Project) scenario {
	files := []string{p.Dep, p.Dependent}
	return scenario{
		name: ScSwitchFile, iters: 20, warm: 2,
		prep: func(ctx context.Context, s *Session) error {
			for _, f := range files {
				if err := s.Close(f); err != nil {
					return err
				}
			}
			sleep(ctx, 5*settle)
			return nil
		},
		run: func(ctx context.Context, s *Session, i int) (string, time.Duration, error) {
			f := files[i%2]
			seq := s.C.Seq()
			t0, err := s.Open(f)
			if err != nil {
				return "", 0, err
			}
			d, _, err := s.Wait(ctx, f, seq, t0, nil)
			if e := s.Close(f); err == nil {
				err = e
			}
			sleep(ctx, settle)
			return "", d, err
		},
		done: func(ctx context.Context, s *Session) error {
			for _, f := range files {
				seq := s.C.Seq()
				t0, err := s.Open(f)
				if err != nil {
					return err
				}
				s.Wait(ctx, f, seq, t0, nil)
			}
			return nil
		},
	}
}

// apiChange changes the signature of an exported function that another package
// calls (a parameter added), saves the file, and measures how long the
// error in the caller takes to show (api-change); then takes the
// parameter out again and measures how long the error takes to go
// (api-fix): the packages that depend on the file re-check.
func apiChange(p *Project) scenario {
	file, user := p.APIFile, p.APIUser
	const sig = "func Label(s string"
	var off, base int
	return scenario{
		name: ScAPIChange, iters: 8, warm: 2,
		prep: func(ctx context.Context, s *Session) (err error) {
			for _, f := range []string{file, user} {
				if _, err := s.Open(f); err != nil {
					return err
				}
			}
			off, err = s.Find(file, sig, len(sig))
			if err != nil {
				return err
			}
			// Both files are checked before the first change.
			sleep(ctx, 200*time.Millisecond)
			base = s.baseline(user)
			return nil
		},
		run: func(ctx context.Context, s *Session, i int) (string, time.Duration, error) {
			seq := s.C.Seq()
			var t0 time.Time
			var err error
			series, ok := ScAPIChange, moreErrors(base)
			if i%2 == 0 {
				t0, err = s.Edit(file, off, off, ", n int")
			} else {
				t0, err = s.Edit(file, off, off+len(", n int"), "")
				series, ok = ScAPIFix, fewerErrors(base)
			}
			if err == nil {
				err = s.Save(file)
			}
			if err != nil {
				return series, 0, err
			}
			d, _, err := s.Wait(ctx, user, seq, t0, ok)
			return series, d, err
		},
		done: func(ctx context.Context, s *Session) error {
			if strings.Contains(s.Doc(file).Text, sig+", n int") {
				if _, err := s.Edit(file, off, off+len(", n int"), ""); err != nil {
					return err
				}
			}
			return s.Save(file)
		},
	}
}
