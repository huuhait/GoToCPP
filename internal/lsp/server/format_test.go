package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	gpformat "github.com/huuhait/gocpp/internal/format"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// repoFiles returns the .gp files of the repository (tests, examples,
// testdata, the standard library), with their contents.
func repoFiles(t *testing.T) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	root := filepath.Join("..", "..", "..")
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".gp") {
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			files[p] = src
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 200 {
		t.Fatalf("only %d .gp files found", len(files))
	}
	return files
}

// scrambles returns versions of src that gocpp fmt formats again: without
// indentation, indented with spaces, with spaces around "=" removed, and
// with "\r\n" line ends.
func scrambles(src []byte) map[string]string {
	lines := strings.Split(string(src), "\n")
	noIndent := make([]string, len(lines))
	spaces := make([]string, len(lines))
	for i, l := range lines {
		noIndent[i] = strings.TrimLeft(l, " \t")
		spaces[i] = strings.ReplaceAll(l, "\t", "   ")
	}
	return map[string]string{
		"same":      string(src),
		"no indent": strings.Join(noIndent, "\n"),
		"spaces":    strings.Join(spaces, "\n"),
		"no blanks": strings.ReplaceAll(string(src), " = ", "="),
		"crlf":      strings.ReplaceAll(strings.Join(noIndent, "\n"), "\n", "\r\n"),
	}
}

// TestFormattingRepoFiles checks, for every .gp file of the repository and
// scrambled versions of it, that the formatting edits turn the text into
// exactly what gocpp fmt prints (with the "\r\n" line ends kept), and
// that a formatted file gets no edits.
func TestFormattingRepoFiles(t *testing.T) {
	n, broken := 0, 0
	for name, src := range repoFiles(t) {
		for kind, text := range scrambles(src) {
			want, err := gpformat.Source(name, []byte(text))
			if err != nil {
				continue // doesn't parse (an error test), or not with this change
			}
			doc := &document{path: name, src: []byte(text), mapper: protocol.NewMapper(text)}
			edits, err := formatEdits(context.Background(), doc)
			if _, perr := parser.ParseFile(token.NewFileSet(), name, want, gpformat.ParserMode); perr != nil {
				// The go/printer bug with range (std.vector[int]{7, 8})
				// (the plan's TODO): the safety net leaves the file alone.
				if err == nil || !strings.Contains(err.Error(), "gocpp fmt would break the file") {
					t.Errorf("%s (%s): gocpp fmt breaks it (%v), but formatting gives %v", name, kind, perr, err)
				}
				broken++
				continue
			}
			if err != nil {
				t.Errorf("%s (%s): %v", name, kind, err)
				continue
			}
			n++
			got := applyEdits(t, text, edits)
			if kind == "crlf" {
				if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
					t.Errorf("%s (%s): a line end lost its \\r", name, kind)
				}
				got = strings.ReplaceAll(got, "\r\n", "\n")
			}
			if got != string(want) {
				t.Errorf("%s (%s): the edits give\n%s\nwant (gocpp fmt)\n%s", name, kind, got, want)
			}
			if text == string(want) && len(edits) > 0 {
				t.Errorf("%s (%s): a formatted file got %d edits", name, kind, len(edits))
			}
		}
	}
	t.Logf("%d texts formatted, %d left alone (gocpp fmt would break them)", n, broken)
}

// importsOf returns the imports of the parsed file f: "name path" each.
func importsOf(f *ast.File) []string {
	var all []string
	for _, s := range f.Imports {
		name := ""
		if s.Name != nil {
			name = s.Name.Name
		}
		p, _ := strconv.Unquote(s.Path.Value)
		all = append(all, name+" "+p)
	}
	return all
}

// TestOrganizeImportsRepoFiles organizes the imports of every .gp file of
// the repository (with no unused imports) and checks that the result
// parses, has the same imports, the header and namespace imports after the
// gocpp ones and in the same order, is formatted when the file was, and
// stays the same when it is organized again.
func TestOrganizeImportsRepoFiles(t *testing.T) {
	n, changed := 0, 0
	for name, src := range repoFiles(t) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, src, cache.ParseFull)
		if err != nil {
			continue
		}
		n++
		res, err := organizeImports(name, src, f, fset.File(f.FileStart), nil)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !bytes.Equal(res, src) {
			changed++
		}
		fset2 := token.NewFileSet()
		f2, err := parser.ParseFile(fset2, name, res, cache.ParseFull)
		if err != nil {
			t.Errorf("%s: the result doesn't parse: %v\n%s", name, err, res)
			continue
		}
		before, after := importsOf(f), importsOf(f2)
		if a, b := slices.Compact(slices.Sorted(slices.Values(before))), slices.Compact(slices.Sorted(slices.Values(after))); !slices.Equal(a, b) {
			t.Errorf("%s: the imports were %v, now %v", name, before, after)
		}
		headers := func(all []string) []string {
			var h []string
			for _, s := range all {
				_, p, _ := strings.Cut(s, " ")
				if load.IsHeaderOrNamespace(p) {
					h = append(h, s)
				}
			}
			return h
		}
		if !slices.Equal(headers(before), headers(after)) {
			t.Errorf("%s: the headers were %v, now %v", name, headers(before), headers(after))
		}
		seenHeader := false
		for _, s := range after {
			_, p, _ := strings.Cut(s, " ")
			if load.IsHeaderOrNamespace(p) {
				seenHeader = true
			} else if seenHeader {
				t.Errorf("%s: the gocpp import %s comes after a header:\n%s", name, s, res)
			}
		}
		if fsrc, err := gpformat.Source(name, src); err == nil && bytes.Equal(fsrc, src) {
			if fres, err := gpformat.Source(name, res); err != nil || !bytes.Equal(fres, res) {
				t.Errorf("%s: the result is not formatted (%v):\n%s", name, err, res)
			}
		}
		if again, err := organizeImports(name, res, f2, fset2.File(f2.FileStart), nil); err != nil || !bytes.Equal(again, res) {
			t.Errorf("%s: a second time (%v):\n%s\nwant:\n%s", name, err, again, res)
		}
	}
	t.Logf("%d files organized, %d of them changed", n, changed)
}

// startFormat starts a server over a module tree with the files.
func startFormat(t *testing.T, files map[string]string) *dclient {
	t.Helper()
	return startDiag(t, testConfig(), files, nil)
}

// formatRequest asks for the formatting of the file name and returns the
// response.
func (c *dclient) formatRequest(name string) *protocol.Response {
	c.t.Helper()
	return c.call("textDocument/formatting", fmt.Sprintf(`{"textDocument":{"uri":%q},"options":{"tabSize":4,"insertSpaces":true}}`, c.uri(name)))
}

func editsOf(t *testing.T, resp *protocol.Response) []protocol.TextEdit {
	t.Helper()
	var edits []protocol.TextEdit
	if resp.Error != nil || json.Unmarshal(resp.Result, &edits) != nil {
		t.Fatalf("got %s (error %+v), want edits", resp.Result, resp.Error)
	}
	return edits
}

// TestFormatting formats documents through the server: unsaved text with
// non-ASCII characters and emoji (the UTF-16 positions), "\r\n" line ends,
// a file that is only on disk, and a syntax error.
func TestFormatting(t *testing.T) {
	checkGoroutines(t)
	const disk = "package main\n\nfunc main()   {\n}\n"
	c := startFormat(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": disk})
	text := "package main\n\nimport \"fmt\"\n\nfunc main() {\n// héllo 😀 wörld\nx :=   \"😀😀\"   +   \"é\"\nfmt.Println(x)\n}\n"
	want := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\t// héllo 😀 wörld\n\tx := \"😀😀\" + \"é\"\n\tfmt.Println(x)\n}\n"
	c.open("main.gp", text)
	edits := editsOf(t, c.formatRequest("main.gp"))
	if got := applyEdits(t, text, edits); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	// The line with emoji: its edit ends after "😀😀"   +  , which is 19
	// UTF-16 units (23 bytes) long.
	found := false
	for _, e := range edits {
		if e.Range.Start.Line == 6 {
			found = true
			if e.Range != (protocol.Range{Start: protocol.Position{Line: 6}, End: protocol.Position{Line: 6, Character: 19}}) || e.NewText != "\tx := \"😀😀\" +" {
				t.Errorf("the edit of line 6: %+v", e)
			}
		}
	}
	if !found {
		t.Errorf("no edit of line 6: %+v", edits)
	}

	// The same with "\r\n": the line ends stay.
	crlf := strings.ReplaceAll(text, "\n", "\r\n")
	c.change("main.gp", 2, crlf)
	if got := applyEdits(t, crlf, editsOf(t, c.formatRequest("main.gp"))); got != strings.ReplaceAll(want, "\n", "\r\n") {
		t.Errorf("CRLF: got %q", got)
	}

	// Formatted: no edits.
	c.change("main.gp", 3, want)
	if edits := editsOf(t, c.formatRequest("main.gp")); len(edits) != 0 {
		t.Errorf("a formatted file got edits %+v", edits)
	}

	// A syntax error: an error, like gopls.
	c.change("main.gp", 4, "package main\n\nfunc main() {\n")
	wantError(t, c.formatRequest("main.gp"), protocol.RequestFailed, "expected '}'")

	// A file that isn't open: the file on disk.
	c.notify("textDocument/didClose", docParams(c.uri("main.gp")))
	if got := applyEdits(t, disk, editsOf(t, c.formatRequest("main.gp"))); got != "package main\n\nfunc main() {\n}\n" {
		t.Errorf("on disk: got %q", got)
	}
}

// TestFormattingPhase10Syntax formats a document with the syntax of Phase
// 10 (a must call, a must function, a macro, a lazy parameter, a line of
// macros on top): gocpp fmt accepts it, and so does the safety net that
// parses the result (it must use the mode of gocpp fmt).
func TestFormattingPhase10Syntax(t *testing.T) {
	checkGoroutines(t)
	c := startFormat(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": "package main\n"})
	text := "package main\n\nmacro Assert(c bool, msg   lazy string) {\nif !c { panic(msg()) }\n}\n\n#Trace()\nfunc f!() int {\nx :=   F!(1)\n#Assert(x>0, \"x\")\nreturn x\n}\n"
	want := "package main\n\nmacro Assert(c bool, msg lazy string) {\n\tif !c {\n\t\tpanic(msg())\n\t}\n}\n\n#Trace()\nfunc f!() int {\n\tx := F!(1)\n\t#Assert(x > 0, \"x\")\n\treturn x\n}\n"
	c.open("main.gp", text)
	if got := applyEdits(t, text, editsOf(t, c.formatRequest("main.gp"))); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	c.change("main.gp", 2, want)
	if edits := editsOf(t, c.formatRequest("main.gp")); len(edits) != 0 {
		t.Errorf("a formatted file got edits %+v", edits)
	}
}

// TestFormattingCancelled checks that a cancelled request stops.
func TestFormattingCancelled(t *testing.T) {
	c := startFormat(t, map[string]string{"gocpp.mod": "module example.com/m\n"})
	c.open("main.gp", "package main\n\nimport \"fmt\"\n\nfunc main(){}\n")
	editsOf(t, c.formatRequest("main.gp")) // after the didOpen
	snap, release := c.s.Session().Snapshot()
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := env{snapshot: snap, log: c.s.log}
	if _, err := formatting(ctx, e, json.RawMessage(docParams(c.uri("main.gp")))); !errors.Is(err, context.Canceled) {
		t.Errorf("formatting: got %v, want context.Canceled", err)
	}
	params := fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"context":{"diagnostics":[]}}`, c.uri("main.gp"))
	if _, err := codeAction(ctx, e, json.RawMessage(params)); !errors.Is(err, context.Canceled) {
		t.Errorf("codeAction: got %v, want context.Canceled", err)
	}
}

// codeActions asks for the code actions of the file name with the context
// (JSON) and returns them.
func (c *dclient) codeActions(name, context string) []protocol.CodeAction {
	c.t.Helper()
	resp := c.call("textDocument/codeAction", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"context":`+context+`}`, c.uri(name)))
	var actions []protocol.CodeAction
	if resp.Error != nil || json.Unmarshal(resp.Result, &actions) != nil {
		c.t.Fatalf("got %s (error %+v), want code actions", resp.Result, resp.Error)
	}
	return actions
}

// organized applies the edits of the one "Organize Imports" action to text.
func organized(t *testing.T, actions []protocol.CodeAction, uri protocol.DocumentURI, version int32, text string) string {
	t.Helper()
	if len(actions) != 1 || actions[0].Kind != protocol.SourceOrganizeImports || actions[0].Edit == nil || len(actions[0].Edit.DocumentChanges) != 1 {
		t.Fatalf("got the actions %+v, want one Organize Imports", actions)
	}
	te := actions[0].Edit.DocumentChanges[0].TextDocumentEdit
	if te == nil || te.TextDocument.URI != uri || te.TextDocument.Version != version {
		t.Fatalf("got the document edit %+v, want one of %s version %d", te, uri, version)
	}
	var edits []protocol.TextEdit
	for _, e := range te.Edits {
		// The JSON of a TextEdit decodes as an AnnotatedTextEdit too.
		switch v := e.Value.(type) {
		case protocol.TextEdit:
			edits = append(edits, v)
		case protocol.AnnotatedTextEdit:
			if v.AnnotationID != nil {
				t.Fatalf("an annotated edit: %+v", v)
			}
			edits = append(edits, v.TextEdit)
		default:
			t.Fatalf("an edit of type %T", v)
		}
	}
	return applyEdits(t, text, edits)
}

// TestOrganizeImportsAction checks the code action: the unused gocpp
// imports go (by the type check), the header imports stay and go last, and
// the kinds of the request decide whether the action comes.
func TestOrganizeImportsAction(t *testing.T) {
	checkGoroutines(t)
	c := startFormat(t, map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"lib/lib.gp": "package lib\n\nfunc F() int { return 1 }\n",
	})
	text := "package main\n\nimport (\n\t\"<vector>\"\n\t\"strings\"\n\t\"example.com/m/lib\"\n\t\"os\" // not used\n\t\"fmt\"\n)\n\n" +
		"func main() { fmt.Println(strings.ToUpper(\"a\"), lib.F()) }\n"
	want := "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n\n\t\"example.com/m/lib\"\n\n\t\"<vector>\"\n)\n\n" +
		"func main() { fmt.Println(strings.ToUpper(\"a\"), lib.F()) }\n"
	c.open("main.gp", text)
	c.next("main.gp") // the diagnostics: the check is done
	uri := c.uri("main.gp")
	const only = `{"diagnostics":[],"only":["source.organizeImports"],"triggerKind":2}`
	if got := organized(t, c.codeActions("main.gp", only), uri, 1, text); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	for _, ctx := range []string{
		`{"diagnostics":[],"only":["source"]}`,
		`{"diagnostics":[],"only":[""]}`,
		`{"diagnostics":[]}`,                 // asked for: every kind
		`{"diagnostics":[],"triggerKind":1}`, // invoked
	} {
		if got := organized(t, c.codeActions("main.gp", ctx), uri, 1, text); got != want {
			t.Errorf("%s: got:\n%s", ctx, got)
		}
	}
	for _, ctx := range []string{
		`{"diagnostics":[],"only":["quickfix"]}`,
		`{"diagnostics":[],"only":["source.fixAll"]}`,
		`{"diagnostics":[],"triggerKind":2}`, // automatic: quick fixes only
	} {
		if actions := c.codeActions("main.gp", ctx); len(actions) != 0 {
			t.Errorf("%s: got %+v, want no actions", ctx, actions)
		}
	}
	// Organized: no action.
	c.change("main.gp", 2, want)
	c.next("main.gp")
	if actions := c.codeActions("main.gp", only); len(actions) != 0 {
		t.Errorf("organized: got %+v, want no actions", actions)
	}
	// A syntax error: no action (like gopls), and no error.
	c.change("main.gp", 3, "package main\n\nimport (\n\t\"os\"\n\t\"fmt\"\n)\n\nfunc main() {\n")
	if actions := c.codeActions("main.gp", only); len(actions) != 0 {
		t.Errorf("syntax error: got %+v, want no actions", actions)
	}
}

// TestFormatCapabilities checks what initialize offers: formatting, and
// the code action kinds when the client takes code action literals.
func TestFormatCapabilities(t *testing.T) {
	c := start(t)
	resp := c.call("initialize", `{"processId":1,"rootUri":null,"capabilities":{"textDocument":{"codeAction":{"codeActionLiteralSupport":{"codeActionKind":{"valueSet":["quickfix","source.organizeImports"]}}}}}}`)
	var res struct {
		Capabilities struct {
			DocumentFormattingProvider bool            `json:"documentFormattingProvider"`
			CodeActionProvider         json.RawMessage `json:"codeActionProvider"`
		} `json:"capabilities"`
	}
	if resp.Error != nil || json.Unmarshal(resp.Result, &res) != nil {
		t.Fatalf("got %s (%+v)", resp.Result, resp.Error)
	}
	if !res.Capabilities.DocumentFormattingProvider || string(res.Capabilities.CodeActionProvider) != `{"codeActionKinds":["source.organizeImports"]}` {
		t.Errorf("got %s", resp.Result)
	}
}

// TestOrganizeImportsProvisional: while the namespace probe hasn't answered
// the check of the package, no import is removed (its answers decide which
// names are C++ names); organize imports still sorts. After the answers
// come, the unused import goes.
func TestOrganizeImportsProvisional(t *testing.T) {
	checkGoroutines(t)
	release := make(chan struct{})
	cfg := testConfig()
	cfg.CXX = func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "/fake/c++", Family: build.Clang}, nil
	}
	cfg.Probe = func(ctx context.Context, _ build.CXX, _ string, _, _, cands []string) (interop.Result, error) {
		select {
		case <-release:
		case <-ctx.Done():
			return interop.Result{}, ctx.Err()
		}
		r := interop.Result{Kinds: make(map[string]interop.Kind)}
		for _, c := range cands {
			r.Kinds[c] = interop.NotNamespace
			if c == "std" {
				r.Kinds[c] = interop.Namespace
			}
		}
		return r, nil
	}
	c := startDiag(t, cfg, map[string]string{"gocpp.mod": "module example.com/m\n"}, nil)
	text := "package main\n\nimport (\n\t\"os\"\n\t\"<vector>\"\n\t\"fmt\"\n)\n\nfunc main() {\n\tvar v std.vector[int]\n\tfmt.Println(v.size())\n}\n"
	c.open("main.gp", text)
	want(t, c.next("main.gp"), 1, "3:1-3:5 error: \"os\" imported and not used\n")
	const only = `{"diagnostics":[],"only":["source.organizeImports"]}`
	uri := c.uri("main.gp")
	sorted := "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\n\t\"<vector>\"\n)\n\nfunc main() {\n\tvar v std.vector[int]\n\tfmt.Println(v.size())\n}\n"
	if got := organized(t, c.codeActions("main.gp", only), uri, 1, text); got != sorted {
		t.Errorf("before the probe's answers: got:\n%s\nwant:\n%s", got, sorted)
	}
	close(release)
	// The package is checked again in the background (the diagnostics
	// stay the same, so nothing is published).
	clean := strings.Replace(sorted, "\t\"os\"\n", "", 1)
	deadline := time.Now().Add(10 * time.Second)
	for {
		got := organized(t, c.codeActions("main.gp", only), uri, 1, text)
		if got == clean {
			break
		}
		if got != sorted || time.Now().After(deadline) {
			t.Fatalf("after the probe's answers: got:\n%s\nwant:\n%s", got, clean)
		}
		time.Sleep(time.Millisecond)
	}
}
