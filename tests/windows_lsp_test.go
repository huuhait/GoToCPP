package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/huuhait/gocpp/internal/lsp/bench"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/winssh"
)

// The language server on a real Windows PC (Phase 14 step 14.5, ADR-096): gocpp.exe lsp runs there, started over ssh
// with its standard input and output connected to this test, which plays the editor (the client of the benchmark
// harness, internal/lsp/bench). The workspace is a folder with spaces and non-ASCII characters in its name
// (Wörk Dir 日本語), with CRLF sources, a file with a byte order mark and a file name with non-ASCII letters, and the
// editor's URIs are VS Code's (`file:///c%3A/Users/...`: a lower-case drive letter, the colon escaped), or in a
// spelling of the folder that is not the disk's (upper case, lower case). The files are made and changed on the PC by
// tests/winlspctl, a small program that works with Unicode names (cmd.exe could not carry them).
//
// What it checks: the URIs of the answers, the case of the paths, the diagnostics with UTF-16 columns on CRLF lines,
// hover, definition, references and rename across packages, a document opened under one spelling and changed under
// another, files changed, created and renamed on disk (the editor's watcher events, in several spellings), the BOM, and
// formatting that keeps CRLF. GOCPP_WINDOWS_SSH=user@host turns it on (the repository never names a machine).

// winTarget is a PC test: the run's folder, the helper and the server.
type winLSP struct {
	t    *testing.T
	n    *native
	root string // the workspace, an absolute Windows path
	ctls int
	c    *bench.Client
	p    *winssh.Process
}

// ctlOp is one operation of tests/winlspctl.
type ctlOp struct {
	Op   string `json:"op"`
	Path string `json:"path"`
	To   string `json:"to,omitempty"`
	Text string `json:"text,omitempty"`
	Ms   int    `json:"ms,omitempty"`
}

// ctlScript writes the operations to the PC as a script of winlspctl and returns its name there.
func (w *winLSP) ctlScript(ops ...ctlOp) string {
	w.t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(ops); err != nil {
		w.t.Fatal(err)
	}
	w.ctls++
	name := fmt.Sprintf("ctl/s%d.json", w.ctls)
	w.n.putFile(w.t, name, buf.String())
	return strings.ReplaceAll(name, "/", `\`)
}

// ctl runs the operations on the PC and returns what the helper printed.
func (w *winLSP) ctl(ops ...ctlOp) string {
	w.t.Helper()
	res := w.n.cmd(w.t, "", nil, `winlspctl.exe `+w.ctlScript(ops...))
	if res.Code != 0 {
		w.t.Fatalf("winlspctl %v: exit %d\n%s\n%s", ops, res.Code, res.Stdout, res.Stderr)
	}
	return normNL(res.Stdout)
}

func (w *winLSP) write(rel, text string) ctlOp {
	return ctlOp{Op: "write", Path: w.root + `\` + strings.ReplaceAll(rel, "/", `\`), Text: strings.ReplaceAll(text, "\n", "\r\n")}
}

// fileURI is the URI of the Windows path p: the form the server answers with (file:///C:/...) or VS Code's (a
// lower-case drive letter and an escaped colon).
func fileURI(p string, vscode bool) string {
	p = strings.ReplaceAll(p, `\`, "/")
	u := (&url.URL{Scheme: "file", Path: "/" + p}).String()
	if vscode {
		u = "file:///" + strings.ToLower(p[:1]) + "%3A" + u[len("file:///C:"):]
	}
	return u
}

// canonical is the DocumentURI the server answers with for a URI: what this side's protocol package makes of it.
func canonical(t *testing.T, uri string) protocol.DocumentURI {
	t.Helper()
	d, err := protocol.ParseDocumentURI(uri)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// column returns the UTF-16 column of the first byte of marker in its line, counted by hand.
func column(text, marker string) (line, col int) {
	i := strings.Index(text, marker)
	if i < 0 {
		panic("no " + marker)
	}
	line = strings.Count(text[:i], "\n")
	start := strings.LastIndexByte(text[:i], '\n') + 1
	return line, len(utf16.Encode([]rune(text[start:i])))
}

const (
	wlMain = "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/ws/util\"\n)\n\nfunc main() {\n\tfmt.Println(\"😀\", util.Greet(\"world\"))\n}\n"
	wlUtil = "package util\n\n// Greet says hello.\nfunc Greet(name string) string {\n\treturn \"hello \" + name\n}\n"
	wlBOM  = "package util; func Bom() int { return 1 }\n"
)

// lspOpts are the choices of startWindowsLSP.
type lspOpts struct {
	rootURI func(root string) string // the root in the editor's form
	// env is more environment for the server; its names and values may hold any character (they go through
	// winlspctl run, as cmd.exe could not carry them). The cache folder of the server is then its default one, below LOCALAPPDATA.
	env map[string]string
}

// startWindowsLSP makes the workspace on the PC, starts gocpp.exe lsp and initializes it.
func startWindowsLSP(t *testing.T, n *native, opts lspOpts) *winLSP {
	t.Helper()
	w := &winLSP{t: t, n: n, root: n.abs("Wörk Dir 日本語")}
	ctl := filepath.Join(t.TempDir(), "winlspctl.exe")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", ctl, "./tests/winlspctl")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build winlspctl: %v\n%s", err, out)
	}
	if err := n.r.Put(t.Context(), ctl, "winlspctl.exe"); err != nil {
		t.Fatal(err)
	}
	w.ctl(
		ctlOp{Op: "mkdir", Path: w.root + `\util`},
		w.write("gocpp.mod", "module example.com/ws\n\ngo 1.25\n"),
		w.write("main.gp", wlMain),
		w.write("util/grüß.gp", wlUtil),
		ctlOp{Op: "writebom", Path: w.root + `\util\bom.gp`, Text: wlBOM},
	)

	env := n.env[:len(n.env):len(n.env)]
	line := n.exe + " lsp -logfile " + quote(n.abs("lsp.log"))
	if len(opts.env) == 0 {
		env = append(env, "GOCPP_LSP_CACHE="+n.abs("lspcache"))
	} else {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(opts.env); err != nil {
			t.Fatal(err)
		}
		n.putFile(t, "ctl/env.json", buf.String())
		line = `winlspctl.exe run ctl\env.json ` + quote(n.abs(n.exe)) + " lsp -logfile " + quote(n.abs("lsp.log")) // Go's exec runs no file of the current folder
	}
	p, err := n.r.Start(t.Context(), env, line)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Kill)
	w.p = p
	w.c = bench.NewClient(p.Stdout, p.Stdin, nil)
	var init map[string]any
	root := opts.rootURI(w.root)
	_, err = w.c.Call(w.ctx(), "initialize", map[string]any{
		"processId":  os.Getpid(),
		"clientInfo": map[string]any{"name": "winlsp-test"},
		"rootUri":    root,
		"capabilities": map[string]any{
			"workspace": map[string]any{"workspaceFolders": true, "didChangeWatchedFiles": map[string]any{"dynamicRegistration": true}},
			"textDocument": map[string]any{
				"publishDiagnostics": map[string]any{"relatedInformation": true},
				"hover":              map[string]any{"contentFormat": []string{"plaintext", "markdown"}},
				"rename":             map[string]any{"prepareSupport": true},
			},
		},
		"workspaceFolders": []map[string]any{{"uri": root, "name": "ws"}},
	}, &init)
	if err != nil {
		t.Fatalf("initialize: %v\nserver stderr:\n%s", err, p.Stderr())
	}
	if err := w.c.Notify("initialized", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	return w
}

func (w *winLSP) ctx() context.Context {
	ctx, cancel := context.WithTimeout(w.t.Context(), 40*time.Second)
	w.t.Cleanup(cancel)
	return ctx
}

func (w *winLSP) call(method string, params, result any) {
	w.t.Helper()
	if _, err := w.c.Call(w.ctx(), method, params, result); err != nil {
		w.t.Fatalf("%v\nserver stderr:\n%s", err, w.p.Stderr())
	}
}

func (w *winLSP) open(uri, text string, version int) {
	w.t.Helper()
	err := w.c.Notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "gocpp", "version": version, "text": text}})
	if err != nil {
		w.t.Fatal(err)
	}
}

func (w *winLSP) change(uri, text string, version int) {
	w.t.Helper()
	err := w.c.Notify("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": uri, "version": version}, "contentChanges": []map[string]any{{"text": text}}})
	if err != nil {
		w.t.Fatal(err)
	}
}

func (w *winLSP) watched(typ int, uris ...string) {
	w.t.Helper()
	var changes []map[string]any
	for _, u := range uris {
		changes = append(changes, map[string]any{"uri": u, "type": typ})
	}
	if err := w.c.Notify("workspace/didChangeWatchedFiles", map[string]any{"changes": changes}); err != nil {
		w.t.Fatal(err)
	}
}

// publish waits for a publishDiagnostics of uri after mark for which ok is true.
func (w *winLSP) publish(uri string, mark int64, ok func(*bench.Publish) bool) *bench.Publish {
	w.t.Helper()
	p, err := w.c.WaitPublish(w.ctx(), canonical(w.t, uri), mark, ok)
	if err != nil {
		last := "none"
		if lp := w.c.LastPublish(canonical(w.t, uri)); lp != nil {
			last = fmt.Sprintf("version %d: %+v", lp.Version, lp.Diags)
		}
		w.t.Fatalf("no wanted diagnostics of %s: %v\nthe last publish: %s\nserver stderr:\n%s\nlog:\n%s", uri, err, last, w.p.Stderr(), w.serverLog())
	}
	return p
}

// serverLog returns the log file of the server on the PC (for a failure message).
func (w *winLSP) serverLog() string {
	res := w.n.cmd(w.t, "", nil, `type lsp.log`)
	return string(res.Stdout)
}

func errorsOf(p *bench.Publish) []bench.Diag {
	var out []bench.Diag
	for _, d := range p.Diags {
		if d.Severity <= 1 {
			out = append(out, d)
		}
	}
	return out
}

func noErrors(p *bench.Publish) bool { return len(errorsOf(p)) == 0 }

func hasError(substr string) func(*bench.Publish) bool {
	return func(p *bench.Publish) bool {
		for _, d := range errorsOf(p) {
			if strings.Contains(d.Message, substr) {
				return true
			}
		}
		return false
	}
}

type position = protocol.Position

func (w *winLSP) at(uri string, line, col int) map[string]any {
	return map[string]any{"textDocument": map[string]any{"uri": uri}, "position": position{Line: uint32(line), Character: uint32(col)}}
}

func (w *winLSP) locations(method string, params any) []protocol.Location {
	w.t.Helper()
	var raw json.RawMessage
	w.call(method, params, &raw)
	var locs []protocol.Location
	if err := json.Unmarshal(raw, &locs); err != nil {
		w.t.Fatalf("%s: %s: %v", method, raw, err)
	}
	return locs
}

func TestWindowsLSP(t *testing.T) {
	n := nativeRemote(t)
	w := startWindowsLSP(t, n, lspOpts{rootURI: func(root string) string { return fileURI(root, true) }})
	grüß := w.root + `\util\grüß.gp`
	mainP := w.root + `\main.gp`

	// The disk's case of a path that is spelled in lower case: the helper uses the wide API, so this is the case the
	// file system stores, which the server must give back.
	if got := strings.TrimSpace(w.ctl(ctlOp{Op: "realcase", Path: strings.ToLower(grüß)})); got != `"`+strings.ReplaceAll(grüß, `\`, `\\`)+`"` {
		t.Logf("the disk's own case of %s is %s", strings.ToLower(grüß), got)
	}

	mainURI := fileURI(mainP, true) // VS Code: a lower-case drive letter, the colon escaped
	utilURI := canonical(t, fileURI(grüß, false))
	text := strings.ReplaceAll(wlMain, "\n", "\r\n")

	t.Run("diagnostics", func(t *testing.T) {
		mark := w.c.Seq()
		w.open(mainURI, text, 1)
		p := w.publish(mainURI, mark, nil)
		if !noErrors(p) || p.Version != 1 {
			t.Fatalf("a good file: %+v", p)
		}
		// An error after an emoji on a CRLF line: the column counts UTF-16 units.
		bad := strings.Replace(text, `util.Greet("world")`, `util.Greet(1)`, 1)
		mark = w.c.Seq()
		w.change(mainURI, bad, 2)
		p = w.publish(mainURI, mark, func(p *bench.Publish) bool { return p.Version == 2 && !noErrors(p) })
		line, _ := column(bad, "1)")
		if errs := errorsOf(p); len(errs) != 1 || errs[0].Line != line || !strings.Contains(errs[0].Message, "cannot use 1") {
			t.Errorf("diagnostics of the changed file: %+v, want one error on line %d", errs, line)
		}
		mark = w.c.Seq()
		w.change(mainURI, text, 3)
		w.publish(mainURI, mark, func(p *bench.Publish) bool { return p.Version == 3 && noErrors(p) })
	})

	t.Run("positions", func(t *testing.T) {
		// The range of the error, in UTF-16, on a line with CRLF ends and an emoji before it.
		bad := strings.Replace(text, `util.Greet("world")`, `util.Greet(1)`, 1)
		w.change(mainURI, bad, 4)
		line, col := column(bad, "util.Greet(1)")
		var h protocol.Hover
		w.call("textDocument/hover", w.at(mainURI, line, col+len("util.G")), &h)
		contents := h.Contents.Value
		if !strings.Contains(contents, "func util.Greet(name string) string") || !strings.Contains(contents, "Greet says hello.") {
			t.Errorf("hover on Greet: %s", contents)
		}
		// The range of the hover is the selector, counted in UTF-16 units.
		_, scol := column(bad, "Greet(1)")
		if int(h.Range.Start.Character) != scol || h.Range.Start.Line != uint32(line) {
			t.Errorf("hover range %+v, want line %d column %d", h.Range, line, scol)
		}
		w.change(mainURI, text, 5)
	})

	t.Run("definition and references", func(t *testing.T) {
		line, col := column(text, "Greet")
		locs := w.locations("textDocument/definition", w.at(mainURI, line, col+1))
		if len(locs) != 1 || locs[0].URI != utilURI {
			t.Fatalf("definition of util.Greet: %+v, want %s (the case of the disk, the name with ü and ß)", locs, utilURI)
		}
		dl, dc := column(strings.ReplaceAll(wlUtil, "\n", "\r\n"), "Greet(name")
		if r := locs[0].Range; int(r.Start.Line) != dl || int(r.Start.Character) != dc {
			t.Errorf("definition range %+v, want %d:%d", r, dl, dc)
		}
		refs := w.locations("textDocument/references", map[string]any{
			"textDocument": map[string]any{"uri": mainURI}, "position": position{Line: uint32(line), Character: uint32(col + 1)},
			"context": map[string]any{"includeDeclaration": true}})
		got := map[protocol.DocumentURI]int{}
		for _, l := range refs {
			got[l.URI]++
		}
		if len(refs) != 2 || got[utilURI] != 1 || got[canonical(t, mainURI)] != 1 {
			t.Errorf("references of Greet: %+v, want one in each of %s and %s", refs, utilURI, mainURI)
		}
	})

	t.Run("rename", func(t *testing.T) {
		line, col := column(text, "Greet")
		var we protocol.WorkspaceEdit
		w.call("textDocument/rename", map[string]any{"textDocument": map[string]any{"uri": mainURI}, "position": position{Line: uint32(line), Character: uint32(col + 1)}, "newName": "Welcome"}, &we)
		if len(we.Changes) != 2 || len(we.Changes[utilURI]) != 1 || len(we.Changes[canonical(t, mainURI)]) != 1 {
			t.Fatalf("rename edits: %+v, want one in %s and one in %s", we.Changes, utilURI, mainURI)
		}
		e := we.Changes[canonical(t, mainURI)][0]
		if e.NewText != "Welcome" || int(e.Range.Start.Character) != col {
			t.Errorf("rename edit in main.gp: %+v, want Welcome at column %d", e, col)
		}
	})

	t.Run("another spelling of an open file", func(t *testing.T) {
		// The editor opens the library under a spelling that is not the disk's (the folder typed in upper case), changes
		// it with a lower-case one, and the server finds one document: main sees the unsaved text.
		upper := fileURI(strings.ToUpper(grüß), true)
		lower := fileURI(strings.ToLower(grüß), true)
		two := strings.ReplaceAll(strings.Replace(wlUtil, "(name string)", "(name string, n int)", 1), "\n", "\r\n")
		mark := w.c.Seq()
		w.open(upper, two, 1)
		w.publish(mainURI, mark, hasError("not enough arguments in call to util.Greet"))
		mark = w.c.Seq()
		w.change(lower, strings.ReplaceAll(wlUtil, "\n", "\r\n"), 2)
		w.publish(mainURI, mark, noErrors)
		// The answers about the open document use the editor's spelling.
		line, col := column(text, "Greet")
		locs := w.locations("textDocument/definition", w.at(mainURI, line, col+1))
		if len(locs) != 1 || locs[0].URI != canonical(t, upper) {
			t.Errorf("definition with the library open under another spelling: %+v, want %s", locs, canonical(t, upper))
		}
		if err := w.c.Notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": lower}}); err != nil {
			t.Fatal(err)
		}
		locs = w.locations("textDocument/definition", w.at(mainURI, line, col+1))
		if len(locs) != 1 || locs[0].URI != utilURI {
			t.Errorf("definition after the close: %+v, want %s", locs, utilURI)
		}
	})

	t.Run("files changed on disk", func(t *testing.T) {
		// The editor's watcher reports the change, with a spelling of the path that the disk does not have.
		two := strings.Replace(wlUtil, "(name string)", "(name string, n int)", 1)
		mark := w.c.Seq()
		w.ctl(w.write("util/grüß.gp", two))
		w.watched(2, fileURI(strings.ToLower(grüß), true))
		w.publish(mainURI, mark, hasError("not enough arguments in call to util.Greet"))
		mark = w.c.Seq()
		w.ctl(w.write("util/grüß.gp", wlUtil))
		w.watched(2, fileURI(strings.ToUpper(grüß), true))
		w.publish(mainURI, mark, noErrors)

		// A new file: main uses a function that is not there, then it is.
		fresh := strings.Replace(text, `util.Greet("world")`, `util.Fresh()`, 1)
		mark = w.c.Seq()
		w.change(mainURI, fresh, 6)
		w.publish(mainURI, mark, hasError("undefined: util.Fresh"))
		mark = w.c.Seq()
		w.ctl(w.write("util/Fresh.gp", "package util\n\nfunc Fresh() string { return \"f\" }\n"))
		w.watched(1, fileURI(strings.ToLower(w.root+`\util\fresh.gp`), true))
		w.publish(mainURI, mark, func(p *bench.Publish) bool { return p.Version == 6 && noErrors(p) })

		// A rename of the file (the editor reports a delete and a create), and a delete.
		mark = w.c.Seq()
		w.ctl(ctlOp{Op: "rename", Path: w.root + `\util\Fresh.gp`, To: w.root + `\util\other.gp`})
		w.watched(3, fileURI(w.root+`\util\Fresh.gp`, true))
		w.watched(1, fileURI(w.root+`\util\other.gp`, true))
		time.Sleep(300 * time.Millisecond)
		w.ctl(ctlOp{Op: "remove", Path: w.root + `\util\other.gp`})
		w.watched(3, fileURI(w.root+`\util\other.gp`, true))
		w.publish(mainURI, mark, hasError("undefined: util.Fresh"))
		w.change(mainURI, text, 7)
		w.publish(mainURI, w.c.Seq()-1, func(p *bench.Publish) bool { return p.Version == 7 && noErrors(p) })
	})

	t.Run("byte order mark", func(t *testing.T) {
		// A file on disk with a BOM: the editor shows it without, so the server's columns on line 0 must too.
		src := strings.Replace(text, `util.Greet("world")`, `util.Bom()`, 1)
		mark := w.c.Seq()
		w.change(mainURI, src, 8)
		w.publish(mainURI, mark, func(p *bench.Publish) bool { return p.Version == 8 && noErrors(p) })
		line, col := column(src, "Bom()")
		locs := w.locations("textDocument/definition", w.at(mainURI, line, col+1))
		bomURI := canonical(t, fileURI(w.root+`\util\bom.gp`, false))
		_, want := column(wlBOM, "Bom()")
		if len(locs) != 1 || locs[0].URI != bomURI || locs[0].Range.Start.Line != 0 || int(locs[0].Range.Start.Character) != want {
			t.Errorf("definition of util.Bom: %+v, want %s line 0 column %d", locs, bomURI, want)
		}
		w.change(mainURI, text, 9)
	})

	t.Run("workspace symbols", func(t *testing.T) {
		var syms []protocol.SymbolInformation
		w.call("workspace/symbol", map[string]any{"query": "Greet"}, &syms)
		if len(syms) != 1 || syms[0].Location.URI != utilURI || syms[0].Name != "Greet" {
			t.Errorf("workspace symbols for Greet: %+v, want one in %s", syms, utilURI)
		}
	})

	// The same file under names that are not the one the editor used first: a short name of a folder (ADMINI~1), a UNC
	// name of the same folder (the administrative share), and a path of more than 260 characters.
	closeDoc := func(uri string) {
		if err := w.c.Notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": uri}}); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("short name", func(t *testing.T) {
		closeDoc(mainURI)
		if !strings.EqualFold(w.n.home, `C:\Users\Administrator`) {
			t.Skipf("the home folder %s is not the one with the short name ADMINI~1", w.n.home)
		}
		short := `C:\Users\ADMINI~1` + strings.TrimPrefix(mainP, w.n.home)
		uri := fileURI(short, false)
		mark := w.c.Seq()
		w.open(uri, text, 1)
		w.publish(uri, mark, noErrors)
		line, col := column(text, "Greet")
		locs := w.locations("textDocument/definition", w.at(uri, line, col+1))
		if len(locs) != 1 || locs[0].URI != utilURI {
			t.Errorf("definition from a file opened by its short name: %+v, want %s (the long name, in the case of the disk)", locs, utilURI)
		}
		closeDoc(uri)
	})

	t.Run("UNC name", func(t *testing.T) {
		// \\localhost\c$ is the administrative share of the drive; VS Code writes it file://localhost/c%24/...
		rest := strings.ReplaceAll(strings.TrimPrefix(mainP, `C:`), `\`, "/")
		uri := "file://localhost/c%24" + (&url.URL{Path: rest}).EscapedPath()
		if !strings.HasPrefix(mainP, `C:\`) {
			t.Skipf("the run folder %s is not on C:", mainP)
		}
		mark := w.c.Seq()
		w.open(uri, text, 1)
		w.publish(uri, mark, noErrors)
		bad := strings.Replace(text, `util.Greet("world")`, `util.Greet(1)`, 1)
		mark = w.c.Seq()
		w.change(uri, bad, 2)
		w.publish(uri, mark, hasError("cannot use 1"))
		line, col := column(text, "Greet")
		var h protocol.Hover
		w.call("textDocument/hover", w.at(uri, line, col+1), &h)
		if !strings.Contains(h.Contents.Value, "func util.Greet(name string) string") {
			t.Errorf("hover in a file opened by its UNC name: %q", h.Contents.Value)
		}
		closeDoc(uri)
	})

	t.Run("long path", func(t *testing.T) {
		deep := w.root
		for i := 0; i < 6; i++ {
			deep += `\` + strings.Repeat(string(rune('a'+i)), 50)
		}
		src := "package main\n\nfunc main() {\n\tvar x int = \"s\"\n\t_ = x\n}\n"
		w.ctl(
			ctlOp{Op: "write", Path: deep + `\gocpp.mod`, Text: "module example.com/deep\n"},
			ctlOp{Op: "write", Path: deep + `\main.gp`, Text: strings.ReplaceAll(src, "\n", "\r\n")},
		)
		t.Cleanup(func() { w.ctl(ctlOp{Op: "removeall", Path: w.root + `\` + strings.Repeat("a", 50)}) })
		if len(deep) < 300 {
			t.Fatalf("the folder is only %d characters long", len(deep))
		}
		uri := fileURI(deep+`\main.gp`, true)
		mark := w.c.Seq()
		w.open(uri, strings.ReplaceAll(src, "\n", "\r\n"), 1)
		p := w.publish(uri, mark, hasError("cannot use"))
		if errs := errorsOf(p); len(errs) == 0 || errs[0].Line != 3 {
			t.Errorf("diagnostics at a path of %d characters: %+v", len(deep), errs)
		}
		closeDoc(uri)
	})

	t.Run("header import", func(t *testing.T) {
		// C++ headers in a folder with spaces and characters outside the ANSI code page: the namespace probe starts the
		// compiler there (g++ reads its command line in the ANSI code page: the server hands it a short name). A system
		// header (std) and one of the package; "stdd" is a typo that only the probe can tell from a name.
		hdr := "#pragma once\ninline int answer() { return 42; }\n"
		loc := "package cpp\n\nimport \"./native.hpp\"\n\nfunc Answer() int { return int(native.answer()) }\n"
		cpp := "package cpp\n\nimport \"<vector>\"\n\nfunc F() {\n\tvar v std.vector[int]\n\tvar w stdd.vector[int]\n\t_, _ = v, w\n}\n"
		w.ctl(w.write("cpp/native.hpp", hdr), w.write("cpp/loc.gp", loc), w.write("cpp/cpp.gp", cpp))
		uri := fileURI(w.root+`\cpp\cpp.gp`, true)
		mark := w.c.Seq()
		w.open(uri, strings.ReplaceAll(cpp, "\n", "\r\n"), 1)
		// The first diagnostics come without the probe; the probe, in the background, brings the error of the typo, and
		// nothing about the headers themselves.
		p := w.publish(uri, mark, hasError("undefined: stdd"))
		if errs := errorsOf(p); len(errs) != 1 || errs[0].Line != 6 {
			t.Errorf("diagnostics with the probe: %+v, want only undefined: stdd on line 6", errs)
		}
		closeDoc(uri)
	})

	t.Run("formatting keeps CRLF", func(t *testing.T) {
		messy := strings.ReplaceAll("package main\n\nimport \"fmt\"\n\nfunc main() {\nfmt.Println(  \"x\" )\n\tvar   a = 1\n\t_ = a\n}\n", "\n", "\r\n")
		uri := fileURI(w.root+`\messy.gp`, true)
		w.ctl(ctlOp{Op: "write", Path: w.root + `\messy.gp`, Text: messy})
		w.open(uri, messy, 1)
		var edits []protocol.TextEdit
		w.call("textDocument/formatting", map[string]any{"textDocument": map[string]any{"uri": uri}, "options": map[string]any{"tabSize": 4, "insertSpaces": false}}, &edits)
		m := protocol.NewMapper(messy)
		out := messy
		for i := len(edits) - 1; i >= 0; i-- {
			s, e, err := m.RangeOffsets(edits[i].Range)
			if err != nil {
				t.Fatal(err)
			}
			out = out[:s] + edits[i].NewText + out[e:]
		}
		want := strings.ReplaceAll("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"x\")\n\tvar a = 1\n\t_ = a\n}\n", "\n", "\r\n")
		if out != want {
			t.Errorf("formatted:\n%q\nwant\n%q", out, want)
		}
	})

	t.Run("shutdown", func(t *testing.T) {
		w.call("shutdown", nil, nil)
		if err := w.c.Notify("exit", nil); err != nil {
			t.Fatal(err)
		}
		w.c.Close()
		code, err := w.p.Wait(w.ctx())
		if err != nil || code != 0 {
			t.Errorf("gocpp lsp exited with %d (%v); stderr:\n%s", code, err, w.p.Stderr())
		}
		if log := w.serverLog(); regexp.MustCompile(`panic|goroutine \d+ \[`).MatchString(log) {
			t.Errorf("the server's log has a panic:\n%s", log)
		}
	})
}

// TestWindowsLSPNonASCIIEnv starts the server with a TEMP and a LOCALAPPDATA that the C++ compiler cannot read (a user
// folder with Japanese characters and a space: step 14.2's environment rules replace them with short names or a private
// folder below ProgramData), and checks that the namespace probe, which starts the compiler, still works, and that the
// summaries of the packages are written to the (replaced) cache folder.
func TestWindowsLSPNonASCIIEnv(t *testing.T) {
	n := nativeRemote(t)
	run := n.abs("")
	w := startWindowsLSP(t, n, lspOpts{
		rootURI: func(root string) string { return fileURI(root, true) },
		env: map[string]string{
			"TEMP": run + `\tmp 日本`, "TMP": run + `\tmp 日本`, "LOCALAPPDATA": run + `\lad 日本`,
		},
	})
	hdr := "#pragma once\ninline int answer() { return 42; }\n"
	loc := "package cpp\n\nimport \"./native.hpp\"\n\nfunc Answer() int { return int(native.answer()) }\n"
	cpp := "package cpp\n\nimport \"<vector>\"\n\nfunc F() {\n\tvar v std.vector[int]\n\tvar w stdd.vector[int]\n\t_, _ = v, w\n}\n"
	w.ctl(w.write("cpp/native.hpp", hdr), w.write("cpp/loc.gp", loc), w.write("cpp/cpp.gp", cpp), w.write("gocpp.mod", "module example.com/ws\n\ngo 1.25\n"))
	uri := fileURI(w.root+`\cpp\cpp.gp`, true)
	mark := w.c.Seq()
	w.open(uri, strings.ReplaceAll(cpp, "\n", "\r\n"), 1)
	p := w.publish(uri, mark, hasError("undefined: stdd"))
	if errs := errorsOf(p); len(errs) != 1 {
		t.Errorf("diagnostics with the probe under a TEMP the compiler cannot read: %+v, want only undefined: stdd", errs)
	}
	log := w.serverLog()
	t.Logf("the server's log:\n%s", log)
	if strings.Contains(log, "?") && regexp.MustCompile(`cache of summaries is in [^\n]*\?`).MatchString(log) {
		t.Errorf("the cache folder has a character the log cannot show: %s", log)
	}
}

// TestWindowsLSPStdinClosed: an editor that dies closes the server's input without shutdown or exit: the server ends (status 1,
// as `gocpp lsp -h` says), and the compiler processes it started are gone with it.
func TestWindowsLSPStdinClosed(t *testing.T) {
	n := nativeRemote(t)
	w := startWindowsLSP(t, n, lspOpts{rootURI: func(root string) string { return fileURI(root, true) }})
	mainURI := fileURI(w.root+`\main.gp`, true)
	mark := w.c.Seq()
	w.open(mainURI, strings.ReplaceAll(wlMain, "\n", "\r\n"), 1)
	w.publish(mainURI, mark, noErrors)
	w.c.Close() // the input ends
	code, err := w.p.Wait(w.ctx())
	if err != nil || code != 1 {
		t.Errorf("gocpp lsp after the end of its input: exit %d (%v), want 1; stderr:\n%s", code, err, w.p.Stderr())
	}
}

// TestWindowsLSPLockedFile: a program holds a byte-range lock on a file for longer than the server repeats a read (about 0.6 s):
// Go's ReadFile gets ERROR_LOCK_VIOLATION. The server must not stay without the file: it tells the user, reads the file again
// in the background, and diagnoses when it can, with no new event (Phase 14 step 14.5, ADR-096).
func TestWindowsLSPLockedFile(t *testing.T) {
	n := nativeRemote(t)
	w := startWindowsLSP(t, n, lspOpts{rootURI: func(root string) string { return fileURI(root, true) }})
	mainURI := fileURI(w.root+`\main.gp`, true)
	text := strings.ReplaceAll(wlMain, "\n", "\r\n")
	mark := w.c.Seq()
	w.open(mainURI, text, 1)
	w.publish(mainURI, mark, noErrors)

	// The library gets a second parameter on disk, and is then locked for 6 s.
	grüß := w.root + `\util\grüß.gp`
	two := strings.Replace(wlUtil, "(name string)", "(name string, n int)", 1)
	w.ctl(w.write("util/grüß.gp", two))
	script := w.ctlScript(ctlOp{Op: "lockfor", Path: grüß, Ms: 6000})
	locked := make(chan winssh.Result, 1)
	go func() {
		res, _ := n.r.Exec(context.Background(), time.Minute, n.env, `winlspctl.exe `+script)
		locked <- res
	}()
	time.Sleep(1500 * time.Millisecond) // the lock is taken (ssh and a process start)
	mark = w.c.Seq()
	w.watched(2, fileURI(grüß, true))
	line, col := column(text, "Greet")
	start := time.Now()
	var h protocol.Hover
	w.call("textDocument/hover", w.at(mainURI, line, col+1), &h) // must answer, with or without the new file
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("hover took %v while the file was locked", d)
	}
	// After the lock is gone the server has read the file by itself.
	w.publish(mainURI, mark, hasError("not enough arguments in call to util.Greet"))
	if res := <-locked; res.Code != 0 {
		t.Errorf("winlspctl lockfor: exit %d\n%s\n%s", res.Code, res.Stdout, res.Stderr)
	}
	log := w.serverLog()
	if !strings.Contains(log, "cannot read") || !strings.Contains(log, "can be read again") {
		t.Errorf("the server's log does not tell about the file that was locked:\n%s", log)
	}
}
