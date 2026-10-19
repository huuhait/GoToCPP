package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/version"
)

// An lspProcess is "gocpp lsp" as a process of its own: the test binary as
// gocpp (GOCPP_TEST_MAIN=1, see TestMain), which starts like gocpp with the
// test code on top.
type lspProcess struct {
	t      *testing.T
	cmd    *exec.Cmd
	in     io.WriteCloser
	stdout io.ReadCloser
	out    *protocol.Reader
	stderr lockedBuffer
	start  time.Time
}

// A lockedBuffer is a bytes.Buffer that exec's copying goroutine can write
// while the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *lockedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

func startLSP(t *testing.T, args ...string) *lspProcess {
	t.Helper()
	return startLSPEnv(t, nil, args...)
}

// startLSPEnv is startLSP with more environment variables.
func startLSPEnv(t *testing.T, env []string, args ...string) *lspProcess {
	t.Helper()
	p := &lspProcess{t: t, cmd: exec.Command(os.Args[0], append([]string{"lsp"}, args...)...)}
	p.cmd.Env = append(append(os.Environ(), "GOCPP_TEST_MAIN=1"), env...)
	p.cmd.Stderr = &p.stderr
	var err error
	if p.in, err = p.cmd.StdinPipe(); err != nil {
		t.Fatal(err)
	}
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p.stdout = stdout
	p.out = protocol.NewReader(stdout)
	p.start = time.Now()
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	timer := time.AfterFunc(30*time.Second, func() { p.cmd.Process.Kill() })
	t.Cleanup(func() {
		timer.Stop()
		p.cmd.Process.Kill()
		p.cmd.Wait()
	})
	return p
}

func (p *lspProcess) send(body string) {
	p.t.Helper()
	if _, err := fmt.Fprintf(p.in, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		p.t.Fatal(err)
	}
}

// read returns the next message, but the diagnostics
// (textDocument/publishDiagnostics), which come whenever the server is done
// with a change.
func (p *lspProcess) read() string {
	p.t.Helper()
	for {
		body, err := p.out.Read()
		if err != nil {
			p.t.Fatalf("reading from gocpp lsp: %v; stderr:\n%s", err, p.stderr.String())
		}
		if !strings.Contains(string(body), `"method":"textDocument/publishDiagnostics"`) {
			return string(body)
		}
	}
}

// wait closes stdin and returns the exit code. gocpp lsp must have written
// nothing more to stdout.
func (p *lspProcess) wait() int {
	p.t.Helper()
	p.in.Close()
	if body, err := p.out.Read(); err != io.EOF {
		p.t.Errorf("at the end, gocpp lsp wrote %q (%v)", body, err)
	}
	err := p.cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	} else if err != nil {
		p.t.Fatal(err)
	}
	return 0
}

var (
	lspInitialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"clientInfo":{"name":"lsp_test"},"rootUri":null,"capabilities":{}}}`
	lspInitResult = `{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"positionEncoding":"utf-16","textDocumentSync":{"openClose":true,"change":2,"save":{}},"completionProvider":{"triggerCharacters":["."],"resolveProvider":true},"hoverProvider":true,"signatureHelpProvider":{"triggerCharacters":["(",","]},"definitionProvider":true,"typeDefinitionProvider":true,"referencesProvider":true,"documentHighlightProvider":true,"documentSymbolProvider":true,"codeActionProvider":true,"documentLinkProvider":{},"workspaceSymbolProvider":true,"documentFormattingProvider":true,"renameProvider":true,"executeCommandProvider":{"commands":["gocpp.stats"]},"semanticTokensProvider":{"legend":{"tokenTypes":["namespace","type","struct","interface","typeParameter","parameter","variable","property","function","method","cppHeader","cppNamespace","cppName"],"tokenModifiers":["declaration","readonly","defaultLibrary","modification"]},"range":true,"full":true},"inlayHintProvider":{}},"serverInfo":{"name":"gocpp","version":` + strconv.Quote(version.Version) + `}}}`
	lspShutdown   = `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`
	lspExit       = `{"jsonrpc":"2.0","method":"exit"}`
)

// TestLSPProcess runs gocpp lsp over pipes, like an editor does: it must
// answer initialize well within the budget (under 100 ms from the start of
// the process, phase 11 README §2.1), keep stdout for LSP messages only,
// and exit with status 0 after shutdown and exit.
func TestLSPProcess(t *testing.T) {
	const runs = 5
	var times []time.Duration
	for i := range runs {
		p := startLSP(t)
		p.send(lspInitialize)
		if got := p.read(); got != lspInitResult {
			t.Fatalf("initialize: got %s, want %s", got, lspInitResult)
		}
		times = append(times, time.Since(p.start))
		p.send(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)
		p.send(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///w/a.gp","languageId":"gocpp","version":1,"text":"package main\n"}}}`)
		p.send(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":"file:///w/a.gp","version":2},"contentChanges":[{"range":{"start":{"line":1,"character":0},"end":{"line":1,"character":0}},"text":"// 😀\n"}]}}`)
		p.send(`{"jsonrpc":"2.0","id":"h","method":"textDocument/hover","params":{"textDocument":{"uri":"file:///w/a.gp"},"position":{"line":0,"character":9}}}`)
		if got, want := p.read(), "{\"jsonrpc\":\"2.0\",\"id\":\"h\",\"result\":{\"contents\":{\"kind\":\"markdown\",\"value\":\"```go\\npackage main\\n```\"},\"range\":{\"start\":{\"line\":0,\"character\":8},\"end\":{\"line\":0,\"character\":12}}}}"; got != want {
			t.Errorf("hover: got %s, want %s", got, want)
		}
		p.send(lspShutdown)
		if got, want := p.read(), `{"jsonrpc":"2.0","id":2,"result":null}`; got != want {
			t.Errorf("shutdown: got %s, want %s", got, want)
		}
		p.send(lspExit)
		if code := p.wait(); code != 0 {
			t.Errorf("run %d: exit status %d, want 0; stderr:\n%s", i, code, p.stderr.String())
		}
		if !strings.Contains(p.stderr.String(), "initialize: client lsp_test") {
			t.Errorf("the log on stderr:\n%s", p.stderr.String())
		}
	}
	slices.Sort(times)
	t.Logf("process start → initialize answered, %d runs: min %v, median %v, max %v",
		runs, times[0], times[runs/2], times[runs-1])
	// The fastest run: a busy machine (make ci runs the packages' tests
	// at the same time) can slow some runs down, but not every one.
	if times[0] >= 100*time.Millisecond {
		t.Errorf("initialize was answered after %v at the fastest, want well under 100 ms", times[0])
	}
}

func TestLSPProcessExitWithoutShutdown(t *testing.T) {
	p := startLSP(t)
	p.send(lspInitialize)
	p.read()
	p.send(lspExit)
	if code := p.wait(); code != 1 {
		t.Errorf("exit without shutdown: exit status %d, want 1", code)
	}
	// The end of stdin without exit.
	p = startLSP(t)
	p.send(lspInitialize)
	p.read()
	if code := p.wait(); code != 1 {
		t.Errorf("the end without shutdown: exit status %d, want 1", code)
	}
}

// TestLSPClosedStdout: a client that closes the server's stdout (it stops
// reading) doesn't kill the server with SIGPIPE: the write of an answer
// fails, the server logs it, and exits normally (0 after shutdown).
func TestLSPClosedStdout(t *testing.T) {
	p := startLSP(t)
	p.send(lspInitialize)
	p.read()
	p.stdout.Close()
	// One write, so that it can't fail when the server stops reading
	// after the failed answer to shutdown.
	var both strings.Builder
	for _, body := range []string{lspShutdown, lspExit} {
		fmt.Fprintf(&both, "Content-Length: %d\r\n\r\n%s", len(body), body)
	}
	if _, err := io.WriteString(p.in, both.String()); err != nil {
		t.Fatal(err)
	}
	p.in.Close()
	err := p.cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		t.Errorf("gocpp lsp: %v, want exit status 0; stderr:\n%s", err, p.stderr.String())
	} else if err != nil {
		t.Fatal(err)
	}
	if logs := p.stderr.String(); !strings.Contains(logs, "broken pipe") {
		t.Errorf("the log has no write error:\n%s", logs)
	}
}

func TestLSPLogfile(t *testing.T) {
	logfile := filepath.Join(t.TempDir(), "lsp.log")
	if err := os.WriteFile(logfile, []byte("an older log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := startLSP(t, "-logfile", logfile)
	p.send(lspInitialize)
	p.read()
	p.send(lspShutdown)
	p.read()
	p.send(lspExit)
	if code := p.wait(); code != 0 {
		t.Errorf("exit status %d, want 0", code)
	}
	if p.stderr.Len() != 0 {
		t.Errorf("with -logfile, stderr has:\n%s", p.stderr.String())
	}
	data, err := os.ReadFile(logfile)
	if err != nil {
		t.Fatal(err)
	}
	if log := string(data); !strings.HasPrefix(log, "an older log\n") || !strings.Contains(log, "initialize: client lsp_test") {
		t.Errorf("the log file:\n%s", log)
	}
}

func TestLSPUsage(t *testing.T) {
	code, out, errOut := gocpp("lsp", "extra")
	if code != 2 || out != "" || errOut != "usage: gocpp lsp [-logfile file] [-slow duration] [-profile dir [-exec-trace]] [-trace file]\n" {
		t.Errorf("gocpp lsp extra: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, out, errOut = gocpp("lsp", "-h")
	if code != 0 || out != "" || !strings.HasPrefix(errOut, lspUsage) || !strings.Contains(errOut, "-logfile file") ||
		!strings.Contains(errOut, "GOCPP_LSP_CACHE") {
		t.Errorf("gocpp lsp -h: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, _, errOut = gocpp("lsp", "-nope")
	if code != 2 || !strings.Contains(errOut, "flag provided but not defined: -nope") {
		t.Errorf("gocpp lsp -nope: exit %d, stderr %q", code, errOut)
	}
	missing := filepath.Join(t.TempDir(), "no", "such", "dir", "lsp.log")
	code, out, errOut = gocpp("lsp", "-logfile", missing)
	if code != 2 || out != "" || !strings.HasPrefix(errOut, "gocpp lsp: open "+missing) {
		t.Errorf("gocpp lsp -logfile %s: exit %d, stdout %q, stderr %q", missing, code, out, errOut)
	}
}

// TestLSPInProcess runs the command in the test process with a whole
// session on stdin.
func TestLSPInProcess(t *testing.T) {
	var in strings.Builder
	for _, body := range []string{lspInitialize, lspShutdown, lspExit} {
		fmt.Fprintf(&in, "Content-Length: %d\r\n\r\n%s", len(body), body)
	}
	code, out, errOut := gocppStdin(in.String(), "lsp")
	if code != 0 {
		t.Errorf("exit %d, want 0; stderr:\n%s", code, errOut)
	}
	var bodies []string
	r := protocol.NewReader(strings.NewReader(out))
	for {
		body, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("stdout is not only LSP messages: %v\n%q", err, out)
		}
		bodies = append(bodies, string(body))
	}
	want := []string{lspInitResult, `{"jsonrpc":"2.0","id":2,"result":null}`}
	if !slices.Equal(bodies, want) {
		t.Errorf("stdout has\n%s\nwant\n%s", strings.Join(bodies, "\n"), strings.Join(want, "\n"))
	}
}

// lspTree writes a module for TestLSPFeatures and returns its folder.
func lspTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"gocpp.mod":    "module example.com/e2e\n",
		"lib/lib.gp":   "package lib\n\n// Add adds.\nfunc Add(a, b int) int { return a + b }\n\ntype Shape interface{ Area() int }\n\ntype Sq struct{ S int }\n\nfunc (s Sq) Area() int { return s.S * s.S }\n",
		"main/main.gp": "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/e2e/lib\"\n\t_ \"./local.h\"\n)\n\nfunc main() {\n\ttotal := lib.Add(1, 2)\n\tvar sh lib.Shape = lib.Sq{S: total}\n\tfmt.Println(total, sh.Area(), strings.ToUpper(\"x\"))\n}\n",
		"main/local.h": "int f(void);\n",
	}
	for name, text := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// lspCall sends a request and returns the result of its answer (the next
// message that is not a diagnostic), failing on an error answer.
func (p *lspProcess) call(id int, method, params string) json.RawMessage {
	p.t.Helper()
	p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%q,"params":%s}`, id, method, params))
	var resp struct {
		ID     int             `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	body := p.read()
	for strings.Contains(body, `"method":"window/showMessage"`) {
		body = p.read() // a message to the user (the rename says what else it changed)
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil || resp.ID != id {
		p.t.Fatalf("%s: got %s, want the answer to request %d", method, body, id)
	}
	if resp.Error != nil {
		p.t.Fatalf("%s: error %s", method, resp.Error.Message)
	}
	return resp.Result
}

// TestLSPFeatures runs the features of steps 11.8 and 11.9 through the real
// process, on files on disk: references and rename across two packages,
// symbols, semantic tokens, inlay hints, document links and the quick fix for a
// missing import.
func TestLSPFeatures(t *testing.T) {
	t.Setenv("GOCPP_LSP_CACHE", "off")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	// No C++ compiler: the header import of the fixture starts no probe. (A
	// probe that the server's exit kills leaves the compiler wrapper's
	// temporary files in TMPDIR, which TestMain checks for.) The link to the
	// header file does not need the compiler: it is a local header.
	t.Setenv("GOCPP_CXX", filepath.Join(t.TempDir(), "no-such-c++"))
	root := lspTree(t)
	uri := func(name string) string {
		return string(protocol.URIFromPath(filepath.Join(root, filepath.FromSlash(name))))
	}
	p := startLSP(t)
	p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"rootUri":%q,"workspaceFolders":[{"uri":%q,"name":"e2e"}],"capabilities":{"workspace":{"workspaceEdit":{"documentChanges":true}},"textDocument":{"rename":{"prepareSupport":true},"documentSymbol":{"hierarchicalDocumentSymbolSupport":true}}}}}`, uri(""), uri("")))
	if got := p.read(); !strings.Contains(got, `"renameProvider":{"prepareProvider":true}`) || !strings.Contains(got, `"semanticTokensProvider"`) {
		t.Fatalf("initialize: %s", got)
	}
	p.send(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)
	mainSrc, _ := os.ReadFile(filepath.Join(root, "main", "main.gp"))
	text, _ := json.Marshal(string(mainSrc))
	p.send(fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"languageId":"gocpp","version":3,"text":%s}}}`, uri("main/main.gp"), text))
	pos := func(src, mark string) string {
		i := strings.Index(src, mark)
		if i < 0 {
			t.Fatalf("%q is not in the file", mark)
		}
		line := strings.Count(src[:i], "\n")
		col := len(src[strings.LastIndex(src[:i], "\n")+1 : i])
		return fmt.Sprintf(`{"line":%d,"character":%d}`, line, col)
	}
	main := string(mainSrc)

	// References of lib.Add, from the use in main, with the declaration.
	var locs []protocol.Location
	res := p.call(2, "textDocument/references", fmt.Sprintf(`{"textDocument":{"uri":%q},"position":%s,"context":{"includeDeclaration":true}}`, uri("main/main.gp"), shift(pos(main, "Add(1, 2)"), 1)))
	if err := json.Unmarshal(res, &locs); err != nil || len(locs) != 2 {
		t.Fatalf("references of Add: %s (%v)", res, err)
	}
	if locs[0].URI != protocol.DocumentURI(uri("lib/lib.gp")) || locs[1].URI != protocol.DocumentURI(uri("main/main.gp")) {
		t.Errorf("references of Add: %+v", locs)
	}

	// Rename of the method Area: its interface's method and the call change
	// with it, in versioned edits (the open file has its version 3).
	res = p.call(3, "textDocument/rename", fmt.Sprintf(`{"textDocument":{"uri":%q},"position":%s,"newName":"Size"}`, uri("main/main.gp"), shift(pos(main, "Area(),"), 1)))
	var we protocol.WorkspaceEdit
	if err := json.Unmarshal(res, &we); err != nil || len(we.DocumentChanges) != 2 {
		t.Fatalf("rename of Area: %s (%v)", res, err)
	}
	versions := map[string]int32{}
	n := 0
	for _, dc := range we.DocumentChanges {
		versions[filepath.Base(dc.TextDocumentEdit.TextDocument.URI.Path())] = dc.TextDocumentEdit.TextDocument.Version
		n += len(dc.TextDocumentEdit.Edits)
	}
	if versions["main.gp"] != 3 || versions["lib.gp"] != 0 || n != 3 {
		t.Errorf("rename of Area: versions %v, %d edits, want main.gp at 3, lib.gp at 0 and 3 edits", versions, n)
	}
	// A refused rename is an error with a message.
	p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":4,"method":"textDocument/rename","params":{"textDocument":{"uri":%q},"position":%s,"newName":"x"}}`, uri("main/main.gp"), shift(pos(main, "Println"), 1)))
	if got := p.read(); !strings.Contains(got, `"error"`) || !strings.Contains(got, "standard library") {
		t.Errorf("rename of Println: %s", got)
	}

	// Symbols of a file and of the workspace.
	var syms []protocol.DocumentSymbol
	if err := json.Unmarshal(p.call(5, "textDocument/documentSymbol", fmt.Sprintf(`{"textDocument":{"uri":%q}}`, uri("lib/lib.gp"))), &syms); err != nil || len(syms) != 4 || syms[0].Name != "Add" || syms[2].Name != "Sq" || len(syms[2].Children) != 1 {
		t.Errorf("document symbols: %+v (%v)", syms, err)
	}
	var wsyms []protocol.SymbolInformation
	if err := json.Unmarshal(p.call(6, "workspace/symbol", `{"query":"area"}`), &wsyms); err != nil || len(wsyms) != 2 {
		t.Errorf("workspace symbols for area: %+v (%v)", wsyms, err)
	}

	// Semantic tokens, inlay hints, links.
	var toks protocol.SemanticTokens
	if err := json.Unmarshal(p.call(7, "textDocument/semanticTokens/full", fmt.Sprintf(`{"textDocument":{"uri":%q}}`, uri("main/main.gp"))), &toks); err != nil || len(toks.Data) == 0 || len(toks.Data)%5 != 0 {
		t.Errorf("semantic tokens: %+v (%v)", toks, err)
	}
	var hints []protocol.InlayHint
	if err := json.Unmarshal(p.call(8, "textDocument/inlayHint", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":0,"character":0},"end":{"line":100,"character":0}}}`, uri("main/main.gp"))), &hints); err != nil || len(hints) < 3 {
		t.Errorf("inlay hints: %+v (%v)", hints, err)
	}
	var links []protocol.DocumentLink
	if err := json.Unmarshal(p.call(9, "textDocument/documentLink", fmt.Sprintf(`{"textDocument":{"uri":%q}}`, uri("main/main.gp"))), &links); err != nil || len(links) != 1 || links[0].Target == nil || *links[0].Target != protocol.URI(uri("main/local.h")) {
		t.Errorf("document links: %+v (%v)", links, err)
	}

	// The quick fix for the missing import of strings.
	start := pos(main, "strings.ToUpper")
	var actions []protocol.CodeAction
	res = p.call(10, "textDocument/codeAction", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":%s,"end":%s},"context":{"diagnostics":[{"range":{"start":%s,"end":%s},"message":"undefined: strings"}],"only":["quickfix"]}}`,
		uri("main/main.gp"), start, start, start, shift(start, len("strings"))))
	if err := json.Unmarshal(res, &actions); err != nil || len(actions) != 1 || actions[0].Title != `Add import "strings"` {
		t.Errorf("code actions: %s (%v)", res, err)
	}

	p.send(lspShutdown)
	p.read()
	p.send(lspExit)
	if code := p.wait(); code != 0 {
		t.Errorf("exit status %d; stderr:\n%s", code, p.stderr.String())
	}
}

// shift returns the position (JSON) moved by n characters on its line.
func shift(pos string, n int) string {
	var p protocol.Position
	if err := json.Unmarshal([]byte(pos), &p); err != nil {
		panic(err)
	}
	return fmt.Sprintf(`{"line":%d,"character":%d}`, p.Line, int(p.Character)+n)
}

// TestLSPProfile: -profile, -exec-trace and -trace write their files when
// the server ends (step 11.12), and the stats request answers.
func TestLSPProfile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "prof", "nested")
	traceFile := filepath.Join(t.TempDir(), "spans.json")
	p := startLSP(t, "-profile", dir, "-exec-trace", "-trace", traceFile, "-slow", "1ns")
	p.send(lspInitialize)
	p.read()
	p.send(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///w/a.gp","languageId":"gocpp","version":1,"text":"package main\n"}}}`)
	p.send(`{"jsonrpc":"2.0","id":7,"method":"gocpp/stats"}`)
	var stats struct {
		Result struct {
			Requests map[string]struct{ Count int }
			Memory   struct{ HeapAlloc uint64 }
		}
	}
	if err := json.Unmarshal([]byte(p.read()), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Result.Requests["textDocument/didOpen"].Count != 1 || stats.Result.Memory.HeapAlloc == 0 {
		t.Errorf("stats: %+v", stats)
	}
	p.send(lspShutdown)
	p.read()
	p.send(lspExit)
	if code := p.wait(); code != 0 {
		t.Errorf("exit status %d, want 0; stderr:\n%s", code, p.stderr.String())
	}
	for _, name := range []string{"cpu.pprof", "heap.pprof", "exec.trace"} {
		if fi, err := os.Stat(filepath.Join(dir, name)); err != nil || fi.Size() == 0 {
			t.Errorf("%s: %v", name, err)
		}
	}
	data, err := os.ReadFile(traceFile)
	if err != nil {
		t.Fatal(err)
	}
	var tr struct{ TraceEvents []struct{ Name, Ph string } }
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	found := false
	for _, e := range tr.TraceEvents {
		found = found || e.Ph == "X" && e.Name == "textDocument/didOpen"
	}
	if !found {
		t.Errorf("no span of didOpen in %s", data)
	}
	if !strings.Contains(p.stderr.String(), "slow: textDocument/didOpen took") {
		t.Errorf("the log has no slow line:\n%s", p.stderr.String())
	}
}

// TestLSPProfileSignal: SIGTERM writes the profiles before the process
// exits with status 143.
func TestLSPProfileSignal(t *testing.T) {
	dir := t.TempDir()
	p := startLSP(t, "-profile", dir)
	p.send(lspInitialize)
	p.read()
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	p.in.Close()
	err := p.cmd.Wait()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 143 {
		t.Errorf("exit: %v, want status 143; stderr:\n%s", err, p.stderr.String())
	}
	for _, name := range []string{"cpu.pprof", "heap.pprof"} {
		if fi, err := os.Stat(filepath.Join(dir, name)); err != nil || fi.Size() == 0 {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestLSPProfileUsage(t *testing.T) {
	code, _, errOut := gocpp("lsp", "-exec-trace")
	if code != 2 || !strings.Contains(errOut, "-exec-trace needs -profile dir") {
		t.Errorf("-exec-trace alone: exit %d, stderr %q", code, errOut)
	}
}

// TestLSPGatePhase: gocpp lsp has the phase gates of gocpp check (a feature
// of a later phase is an error), and GOCPP_GATE_PHASE opens them, like it
// does for check.
func TestLSPGatePhase(t *testing.T) {
	dir := t.TempDir()
	// A feature of a phase after the finished one (version.Phase): the
	// first of these that is not built yet.
	var src, gated string
	for _, c := range []struct {
		phase    int
		src, msg string
	}{
		{7, "package main\n\nfunc G[T any](x T) {}\n\nfunc main() {}\n", "generic functions are not supported yet (Phase 07)"},
		{8, "package main\n\nfunc main() { go func() {}() }\n", "go statements are not supported yet (Phase 08)"},
	} {
		if c.phase > version.Phase {
			src, gated = c.src, c.msg
			break
		}
	}
	if src == "" {
		t.Skip("the features of these phases are built")
	}
	if err := os.WriteFile(filepath.Join(dir, "gocpp.mod"), []byte("module example.com/gate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "main.gp")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	uri := string(protocol.URIFromPath(file))
	diagnostics := func(env ...string) string {
		p := startLSPEnv(t, append(env, "GOCPP_LSP_CACHE=off", "XDG_CACHE_HOME="+t.TempDir()))
		p.send(lspInitialize)
		p.read()
		p.send(fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"languageId":"gocpp","version":1,"text":%q}}}`, uri, src))
		for {
			body, err := p.out.Read()
			if err != nil {
				t.Fatalf("reading from gocpp lsp: %v; stderr:\n%s", err, p.stderr.String())
			}
			if strings.Contains(string(body), `"method":"textDocument/publishDiagnostics"`) {
				return string(body)
			}
		}
	}
	if got := diagnostics(); !strings.Contains(got, gated) {
		t.Errorf("the default gates: %s", got)
	}
	if got := diagnostics("GOCPP_GATE_PHASE=15"); !strings.Contains(got, `"diagnostics":[]`) {
		t.Errorf("with GOCPP_GATE_PHASE=15: %s", got)
	}
	t.Setenv("GOCPP_GATE_PHASE", "99")
	code, _, errOut := gocpp("lsp")
	if code != 2 || !strings.Contains(errOut, "GOCPP_GATE_PHASE=99") {
		t.Errorf("a bad GOCPP_GATE_PHASE: exit %d, stderr %q", code, errOut)
	}
}
