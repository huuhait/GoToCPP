package fuzz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/lsp/server"
	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/tools/fuzz/fuzzkit"
)

// FuzzLSP drives the language server (internal/lsp/server) with a client that the fuzzer writes.
// The input is a program, a NUL byte, and a script: each three bytes of it are one step (an edit
// of the document, or a request at a position, or a cancel, a save, a close and an open). The
// server must not panic, must answer every request (a deadlock is a hang of the target), must
// not answer with an internal error, and must end with exit code 0 after shutdown.
//
// The server runs in the process (server.New: no cache on disk), over pipes.

func FuzzLSP(f *testing.F) {
	for _, s := range fuzzkit.GPSeeds()[:8] {
		f.Add(append(append(append([]byte{}, s...), 0), lspScript(s)...))
	}
	for _, s := range genSeeds()[:6] {
		f.Add(append(append(append([]byte{}, s...), 0), lspScript(s)...))
	}
	f.Add([]byte("package main\n\nfunc main() {\n\tx := \n}\n\x00\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f"))
	f.Add([]byte("package main\nimport \"fmt\"\nfunc main() { fmt.Println(1) }\n\x00\x03\x00\x40\x05\x08\x10\x07\x00\x00\x08\x04\x20\x0a\x00\x00"))
	f.Fuzz(func(t *testing.T, in []byte) {
		src, script, _ := bytes.Cut(in, []byte{0})
		if len(src) > 32<<10 || len(script) > 3*96 {
			return
		}
		fuzzkit.Guard("FuzzLSP", func() { runLSP(t, src, script) })
	})
}

// lspScript returns a script that asks every request at every kind of place of src: the seeds of
// the target.
func lspScript(src []byte) []byte {
	var out []byte
	for k := byte(3); k <= 17; k++ {
		for cat := byte(0); cat < 6; cat++ {
			out = append(out, k, byte(int(k)*7+int(cat)*13+len(src))%251, cat)
		}
	}
	return out
}

// An lspClient is the fuzzer's client: it writes messages to the server and keeps what the server
// said.
type lspClient struct {
	in      io.WriteCloser
	mu      sync.Mutex
	replies map[int]*lspReply
	cond    *sync.Cond
	bad     []string
	nextID  int
	done    chan struct{}
}

type lspReply struct {
	code   int64
	errMsg string
}

func (c *lspClient) send(body string) {
	fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

func (c *lspClient) notify(method, params string) {
	c.send(fmt.Sprintf(`{"jsonrpc":"2.0","method":%q,"params":%s}`, method, params))
}

// request sends a request and returns its ID.
func (c *lspClient) request(method, params string) int {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()
	c.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%q,"params":%s}`, id, method, params))
	return id
}

// await waits for the response to the request id.
func (c *lspClient) await(id int) *lspReply {
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.replies[id] == nil {
		select {
		case <-c.done:
			if c.replies[id] == nil {
				return nil
			}
		default:
		}
		c.cond.Wait()
	}
	return c.replies[id]
}

// read reads what the server writes: responses are kept, a request of the server is answered with
// null (the client supports what the server asks for), notifications are dropped.
func (c *lspClient) read(out io.Reader) {
	defer func() {
		c.mu.Lock()
		close(c.done)
		c.cond.Broadcast()
		c.mu.Unlock()
	}()
	r := protocol.NewReader(out)
	for {
		body, err := r.Read()
		if err != nil {
			return
		}
		var m struct {
			ID     *json.RawMessage `json:"id"`
			Method string           `json:"method"`
			Error  *struct {
				Code    int64  `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &m); err != nil {
			c.mu.Lock()
			c.bad = append(c.bad, fmt.Sprintf("the server wrote %q: %v", body, err))
			c.mu.Unlock()
			continue
		}
		switch {
		case m.Method != "" && m.ID != nil:
			c.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":null}`, *m.ID))
		case m.Method == "" && m.ID != nil:
			var id int
			if _, err := fmt.Sscan(string(*m.ID), &id); err != nil {
				continue
			}
			rep := &lspReply{}
			if m.Error != nil {
				rep.code, rep.errMsg = m.Error.Code, m.Error.Message
			}
			c.mu.Lock()
			c.replies[id] = rep
			c.cond.Broadcast()
			c.mu.Unlock()
		}
	}
}

const lspURI = "file:///fuzzws/main.gp"

// runLSP plays the script against a new server.
func runLSP(t *testing.T, src, script []byte) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := &lspClient{in: inW, replies: map[int]*lspReply{}, done: make(chan struct{})}
	c.cond = sync.NewCond(&c.mu)
	var logs bytes.Buffer
	var logMu sync.Mutex
	s := server.New(log.New(lockedWriter{&logMu, &logs}, "", 0))
	code := make(chan int, 1)
	go func() {
		code <- s.Serve(context.Background(), inR, outW)
		inR.Close()
		outW.Close()
	}()
	go c.read(outR)
	// However the run ends, the server and the readers end with it.
	defer func() {
		inW.Close()
		select {
		case <-code:
		case <-time.After(20 * time.Second):
			t.Errorf("the server did not end after its input closed")
		}
		outR.Close()
	}()

	id := c.request("initialize", `{"processId":null,"rootUri":"file:///fuzzws","capabilities":{"general":{"positionEncodings":["utf-16"]}}}`)
	if r := c.await(id); r == nil || r.code != 0 {
		t.Fatalf("initialize: %+v", r)
	}
	c.notify("initialized", `{}`)

	text := string(src)
	version := 1
	open := true
	c.notify("textDocument/didOpen", fmt.Sprintf(`{"textDocument":{"uri":%q,"languageId":"gocpp","version":1,"text":%s}}`, lspURI, quote(text)))

	var tg lspTargets
	var pending []int
	var methods []string
	check := func(id int, method string) {
		r := c.await(id)
		if r == nil {
			t.Fatalf("%s: the server ended without answering", method)
		}
		// -32603 is the code of an internal error; the others (invalid params, content modified,
		// request cancelled, server cancelled) are answers.
		if r.code == int64(protocol.InternalError) {
			t.Fatalf("%s: internal error: %s", method, r.errMsg)
		}
	}
	for i := 0; i+3 <= len(script); i += 3 {
		kind, a, b := script[i]%21, int(script[i+1]), int(script[i+2])
		// The place: a token of the current text, chosen by the low bits of b and the index a (the
		// declarations, the uses, the selectors, the labels, the operators, the literals), or
		// arithmetic on lines and columns for the other values.
		line, col := lspPick(&tg, text, a, b)
		pos := fmt.Sprintf(`{"line":%d,"character":%d}`, line, col)
		doc := fmt.Sprintf(`{"uri":%q}`, lspURI)
		tdp := fmt.Sprintf(`{"textDocument":%s,"position":%s}`, doc, pos)
		var method, params string
		switch kind {
		case 0, 1, 2: // an edit: insert, delete, replace everything
			if !open {
				continue
			}
			version++
			var ch string
			switch kind {
			case 0:
				ins := lspInserts[(a+b)%len(lspInserts)]
				ch = fmt.Sprintf(`{"range":{"start":%s,"end":%s},"text":%s}`, pos, pos, quote(ins))
				text = lspApply(text, line, col, line, col, ins)
			case 1:
				l2, c2 := lspPick(&tg, text, a+b, b+3)
				if l2 < line || l2 == line && c2 < col {
					l2, c2 = line, col
				}
				ch = fmt.Sprintf(`{"range":{"start":%s,"end":{"line":%d,"character":%d}},"text":""}`, pos, l2, c2)
				text = lspApply(text, line, col, l2, c2, "")
			default:
				text = lspInserts[(a+b)%len(lspInserts)] + text
				ch = fmt.Sprintf(`{"text":%s}`, quote(text))
			}
			c.notify("textDocument/didChange", fmt.Sprintf(`{"textDocument":{"uri":%q,"version":%d},"contentChanges":[%s]}`, lspURI, version, ch))
			continue
		case 3:
			method, params = "textDocument/hover", tdp
		case 4:
			method, params = "textDocument/definition", tdp
		case 5:
			method, params = "textDocument/completion", tdp
		case 6:
			method, params = "textDocument/documentSymbol", fmt.Sprintf(`{"textDocument":%s}`, doc)
		case 7:
			method, params = "textDocument/formatting", fmt.Sprintf(`{"textDocument":%s,"options":{"tabSize":4,"insertSpaces":false}}`, doc)
		case 8:
			method, params = "textDocument/references", fmt.Sprintf(`{"textDocument":%s,"position":%s,"context":{"includeDeclaration":true}}`, doc, pos)
		case 9:
			method, params = "textDocument/rename", fmt.Sprintf(`{"textDocument":%s,"position":%s,"newName":%s}`, doc, pos, quote(lspNames[(a+b)%len(lspNames)]))
		case 10:
			method, params = "textDocument/semanticTokens/full", fmt.Sprintf(`{"textDocument":%s}`, doc)
		case 11:
			method, params = "textDocument/signatureHelp", tdp
		case 12:
			method, params = "textDocument/codeAction", fmt.Sprintf(`{"textDocument":%s,"range":{"start":%s,"end":%s},"context":{"diagnostics":[]}}`, doc, pos, pos)
		case 13:
			method, params = "textDocument/documentHighlight", tdp
		case 14:
			method, params = "textDocument/inlayHint", fmt.Sprintf(`{"textDocument":%s,"range":{"start":{"line":0,"character":0},"end":{"line":%d,"character":0}}}`, doc, line+1)
		case 15:
			method, params = "textDocument/prepareRename", tdp
		case 16:
			method, params = "workspace/symbol", fmt.Sprintf(`{"query":%s}`, quote(lspNames[(a+b)%len(lspNames)]))
		case 17:
			method, params = "textDocument/typeDefinition", tdp
		case 18: // cancel the last request that is not answered
			if n := len(pending); n > 0 {
				c.notify("$/cancelRequest", fmt.Sprintf(`{"id":%d}`, pending[n-1]))
			}
			continue
		case 19: // save, or close and open again
			if open && a%2 == 0 {
				c.notify("textDocument/didSave", fmt.Sprintf(`{"textDocument":%s}`, doc))
			} else if open {
				c.notify("textDocument/didClose", fmt.Sprintf(`{"textDocument":%s}`, doc))
				open = false
			} else {
				version++
				c.notify("textDocument/didOpen", fmt.Sprintf(`{"textDocument":{"uri":%q,"languageId":"gocpp","version":%d,"text":%s}}`, lspURI, version, quote(text)))
				open = true
			}
			continue
		default: // a request that does not exist, and one with params of the wrong shape
			method, params = [2]string{"textDocument/nonesuch", "textDocument/hover"}[a%2], `[1,"x"]`
		}
		rid := c.request(method, params)
		pending = append(pending, rid)
		methods = append(methods, method)
		// Some of the requests are answered before the next step, others run while it is sent.
		if a%3 != 0 {
			check(rid, method)
			pending = pending[:len(pending)-1]
		}
	}
	// Every request is answered, whether it was cancelled or not.
	for i, rid := range pending {
		check(rid, methods[len(methods)-len(pending)+i])
	}
	id = c.request("shutdown", `null`)
	check(id, "shutdown")
	c.notify("exit", `null`)
	select {
	case n := <-code:
		code <- n
		if n != 0 {
			t.Fatalf("exit code %d after shutdown and exit", n)
		}
	case <-time.After(20 * time.Second):
		t.Fatalf("the server does not end after exit")
	}
	c.mu.Lock()
	bad := c.bad
	c.mu.Unlock()
	if len(bad) > 0 {
		t.Fatal(strings.Join(bad, "\n"))
	}
}

type lockedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (l lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

var lspInserts = []string{"x", "(", ")", "{", "}", "\"", ".", ":=", "\n", "\tfunc f() {}\n", "go ", "<-", "[]int{", "*", "lazy ", "!", "macro M() {}\n", "#M()\n", "é", "\x00", "𝄞"}

var lspNames = []string{"x", "y", "main", "fmt", "_", "", "1x", "func", "Foo"}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// lspPos returns the position for the bytes a and b: a line of the text (or one past it) and a
// column in it (UTF-16 units, sometimes too big).
func lspPos(text string, a, b int) (line, col int) {
	lines := strings.Split(text, "\n")
	line = a % (len(lines) + 1)
	if line < len(lines) {
		col = b % (len([]rune(lines[line])) + 3)
	} else {
		col = b % 3
	}
	return line, col
}

// lspApply replaces the text between two positions (as lspPos makes them) with s. A position
// outside the text is clamped.
func lspApply(text string, l1, c1, l2, c2 int, s string) string {
	off := func(l, c int) int {
		lines := strings.SplitAfter(text, "\n")
		o := 0
		for i := 0; i < l && i < len(lines); i++ {
			o += len(lines[i])
		}
		if l >= len(lines) {
			return len(text)
		}
		rs := []rune(strings.TrimSuffix(lines[l], "\n"))
		if c > len(rs) {
			c = len(rs)
		}
		return min(o+len(string(rs[:c])), len(text)) // (an invalid byte is three bytes as a rune)
	}
	a, b := off(l1, c1), off(l2, c2)
	if b < a {
		b = a
	}
	return text[:a] + s + text[b:]
}

// lspTargets are the places of a text that the requests aim at, found by the parser: the places
// are (line, UTF-16 column) pairs, by kind. The text is parsed again when it changed.
type lspTargets struct {
	text                           string
	ok                             bool
	use, decl, sel, label, op, lit [][2]int
}

func (t *lspTargets) parse(text string) {
	*t = lspTargets{text: text, ok: true}
	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "x.gp", []byte(text), parser.AllErrors|parser.SkipObjectResolution|parser.Phase10Syntax)
	if file == nil {
		return
	}
	tf := fset.File(file.Pos())
	if tf == nil {
		return
	}
	lines := strings.Split(text, "\n")
	at := func(p token.Pos) ([2]int, bool) {
		if !p.IsValid() || int(p)-tf.Base() > len(text) {
			return [2]int{}, false
		}
		pos := fset.Position(p)
		if pos.Line < 1 || pos.Line > len(lines) {
			return [2]int{}, false
		}
		l := lines[pos.Line-1]
		off := min(pos.Column-1, len(l))
		return [2]int{pos.Line - 1, len(utf16.Encode([]rune(l[:off])))}, true
	}
	add := func(dst *[][2]int, p token.Pos) {
		if v, ok := at(p); ok {
			*dst = append(*dst, v)
		}
	}
	decl := func(id *ast.Ident) {
		if id != nil {
			add(&t.decl, id.Pos())
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			add(&t.use, n.Pos())
		case *ast.SelectorExpr:
			add(&t.sel, n.Sel.Pos())
		case *ast.LabeledStmt:
			add(&t.label, n.Label.Pos())
		case *ast.BranchStmt:
			if n.Label != nil {
				add(&t.label, n.Label.Pos())
			}
		case *ast.BinaryExpr:
			add(&t.op, n.OpPos)
		case *ast.UnaryExpr:
			add(&t.op, n.OpPos)
		case *ast.SendStmt:
			add(&t.op, n.Arrow)
		case *ast.AssignStmt:
			add(&t.op, n.TokPos)
			if n.Tok == token.DEFINE {
				for _, l := range n.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						decl(id)
					}
				}
			}
		case *ast.BasicLit:
			add(&t.lit, n.Pos())
		case *ast.FuncDecl:
			decl(n.Name)
		case *ast.TypeSpec:
			decl(n.Name)
		case *ast.ValueSpec:
			for _, id := range n.Names {
				decl(id)
			}
		case *ast.Field:
			for _, id := range n.Names {
				decl(id)
			}
		}
		return true
	})
}

// lspPick returns a place of the text for the numbers a and b.
func lspPick(t *lspTargets, text string, a, b int) (line, col int) {
	if !t.ok || t.text != text {
		t.parse(text)
	}
	lists := [][][2]int{t.use, t.decl, t.sel, t.label, t.op, t.lit}
	if cat := b & 7; cat < len(lists) && len(lists[cat]) > 0 {
		l := lists[cat]
		p := l[a%len(l)]
		return p[0], p[1]
	}
	return lspPos(text, a, b>>3)
}
