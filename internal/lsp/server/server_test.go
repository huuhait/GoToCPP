package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/version"
)

// A client talks to a Server over pipes, like an editor over stdin and
// stdout. A goroutine reads what the server writes: the diagnostics
// (textDocument/publishDiagnostics, which come whenever a run ends) go to a
// list of their own (publishes), the other messages to msgs, in order.
type client struct {
	t    testing.TB
	s    *Server
	in   *io.PipeWriter // the server's stdin
	out  *protocol.Reader
	code chan int // the exit code of Serve
	logs *syncBuffer
	id   int

	msgs    chan protocol.Message // closed at the end of the output
	readErr error                 // why the output ended; set before msgs is closed

	pubMu     sync.Mutex
	publishes []*publish

	// skipShown: call skips the window/showMessage notifications that come
	// before an answer and keeps them in shown (the features that warn).
	skipShown bool
	shown     []protocol.ShowMessageParams
}

// A publish is one textDocument/publishDiagnostics.
type publish struct {
	protocol.PublishDiagnosticsParams
	raw  string
	seen bool // nextPublish returned it
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func start(t *testing.T) *client {
	t.Helper()
	return startWith(t, nil)
}

// startWith is start with a setup of the server before it serves (tests
// add their own request methods).
func startWith(t *testing.T, setup func(*Server)) *client {
	t.Helper()
	return startServer(t, New(nil), setup)
}

// startServer starts serving s (whose log the client takes over).
func startServer(t testing.TB, s *Server, setup func(*Server)) *client {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := &client{t: t, in: inW, out: protocol.NewReader(outR), code: make(chan int, 1), logs: &syncBuffer{},
		msgs: make(chan protocol.Message, 1000)}
	c.s = s
	s.log = log.New(c.logs, "", 0)
	if setup != nil {
		setup(c.s)
	}
	go c.readLoop()
	go func() {
		code := c.s.Serve(context.Background(), inR, outW)
		inR.Close() // a later write of the client fails, it doesn't hang
		outW.Close()
		c.code <- code
	}()
	t.Cleanup(func() { inW.Close() })
	return c
}

// readLoop reads the server's output until it ends.
func (c *client) readLoop() {
	for {
		body, err := c.out.Read()
		if err != nil {
			c.readErr = err
			close(c.msgs)
			return
		}
		m, err := protocol.DecodeMessage(body)
		if err != nil {
			c.readErr = fmt.Errorf("the server wrote %q: %v", body, err)
			close(c.msgs)
			return
		}
		if req, ok := m.(*protocol.Request); ok && req.IsNotification() && req.Method == "textDocument/publishDiagnostics" {
			p := &publish{raw: string(req.Params)}
			if err := json.Unmarshal(req.Params, &p.PublishDiagnosticsParams); err != nil {
				c.readErr = fmt.Errorf("publishDiagnostics %s: %v", req.Params, err)
				close(c.msgs)
				return
			}
			c.pubMu.Lock()
			c.publishes = append(c.publishes, p)
			c.pubMu.Unlock()
			continue
		}
		c.msgs <- m
	}
}

func (c *client) send(body string) {
	c.t.Helper()
	if _, err := fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		c.t.Fatalf("writing to the server: %v", err)
	}
}

func (c *client) notify(method, params string) {
	c.t.Helper()
	if params == "" {
		c.send(fmt.Sprintf(`{"jsonrpc":"2.0","method":%q}`, method))
		return
	}
	c.send(fmt.Sprintf(`{"jsonrpc":"2.0","method":%q,"params":%s}`, method, params))
}

// call sends a request and returns its response. The next message from the
// server (but the diagnostics) must be that response: a notification
// before it got no response.
func (c *client) call(method, params string) *protocol.Response {
	c.t.Helper()
	c.id++
	if params == "" {
		c.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%q}`, c.id, method))
	} else {
		c.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%q,"params":%s}`, c.id, method, params))
	}
	m := c.read()
	for c.skipShown {
		req, ok := m.(*protocol.Request)
		if !ok || !req.IsNotification() || req.Method != "window/showMessage" {
			break
		}
		var p protocol.ShowMessageParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			c.t.Fatal(err)
		}
		c.shown = append(c.shown, p)
		m = c.read()
	}
	resp, ok := m.(*protocol.Response)
	if !ok || resp.ID == nil || *resp.ID != protocol.IntID(int64(c.id)) {
		c.t.Fatalf("%s: got %+v, want the response to request %d", method, m, c.id)
	}
	return resp
}

// takeShown returns the messages that call skipped, and forgets them.
func (c *client) takeShown() []protocol.ShowMessageParams {
	s := c.shown
	c.shown = nil
	return s
}

// readNotification reads the next message, which must be a notification
// with the method, and returns its params.
func (c *client) readNotification(method string) string {
	c.t.Helper()
	m := c.read()
	if req, ok := m.(*protocol.Request); !ok || !req.IsNotification() || req.Method != method {
		c.t.Fatalf("got %+v, want the notification %s", m, method)
		return ""
	} else {
		return string(req.Params)
	}
}

// read returns the next message but the diagnostics.
func (c *client) read() protocol.Message {
	c.t.Helper()
	select {
	case m, ok := <-c.msgs:
		if !ok {
			c.t.Fatalf("reading from the server: %v", c.readErr)
		}
		return m
	case <-time.After(10 * time.Second):
		c.t.Fatal("no message from the server in 10 s")
	}
	return nil
}

// wait closes the server's stdin and returns its exit code; the server
// must write nothing more (but the diagnostics before the end).
func (c *client) wait() int {
	c.t.Helper()
	c.in.Close()
	select {
	case code := <-c.code:
		select {
		case m, ok := <-c.msgs:
			if ok {
				c.t.Errorf("after the end, the server wrote %+v", m)
			} else if c.readErr != io.EOF {
				c.t.Errorf("the output ended with %v, want EOF", c.readErr)
			}
		case <-time.After(10 * time.Second):
			c.t.Error("the output didn't end in 10 s")
		}
		return code
	case <-time.After(10 * time.Second):
		c.t.Fatal("the server didn't stop in 10 s")
	}
	return -1
}

// nextPublish returns the next publish of the diagnostics of uri that it
// has not returned yet, waiting up to 10 s for it.
func (c *client) nextPublish(uri protocol.DocumentURI) *publish {
	c.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		c.pubMu.Lock()
		for _, p := range c.publishes {
			if p.URI == uri && !p.seen {
				p.seen = true
				c.pubMu.Unlock()
				return p
			}
		}
		c.pubMu.Unlock()
		if time.Now().After(deadline) {
			c.t.Fatalf("no diagnostics of %s in 10 s", uri)
		}
		time.Sleep(200 * time.Microsecond)
	}
}

// unseen returns the publishes that nextPublish has not returned.
func (c *client) unseen() []*publish {
	c.pubMu.Lock()
	defer c.pubMu.Unlock()
	var ps []*publish
	for _, p := range c.publishes {
		if !p.seen {
			ps = append(ps, p)
		}
	}
	return ps
}

// overlay returns the document uri of the server's current snapshot.
func (c *client) overlay(uri protocol.DocumentURI) *cache.Overlay {
	snap, release := c.s.Session().Snapshot()
	defer release()
	return snap.Overlay(uri)
}

// overlays returns the documents of the server's current snapshot.
func (c *client) overlays() []*cache.Overlay {
	snap, release := c.s.Session().Snapshot()
	defer release()
	return snap.Overlays()
}

// wantError checks that resp is an error with the code.
func wantError[C protocol.ErrorCodes | protocol.LSPErrorCodes](t *testing.T, resp *protocol.Response, code C, msg string) {
	t.Helper()
	if resp.Error == nil || resp.Error.Code != int64(code) || !strings.Contains(resp.Error.Message, msg) {
		t.Errorf("got %+v (error %+v), want the error %d %q", resp, resp.Error, code, msg)
	}
}

func wantResult(t *testing.T, resp *protocol.Response, want string) {
	t.Helper()
	if resp.Error != nil || string(resp.Result) != want {
		t.Errorf("got the result %s (error %v), want %s", resp.Result, resp.Error, want)
	}
}

const initParams = `{"processId":123,"clientInfo":{"name":"test","version":"1.0"},"rootUri":"file:///w","capabilities":{"general":{"positionEncodings":["utf-8","utf-16"]}},"trace":"messages"}`

// The whole answer to initialize: the server offers the text sync and the
// features so far (a client without code action literals gets no list of
// code action kinds).
var initResult = `{"capabilities":{"positionEncoding":"utf-16","textDocumentSync":{"openClose":true,"change":2,"save":{}},"completionProvider":{"triggerCharacters":[".","#"],"resolveProvider":true},"hoverProvider":true,"signatureHelpProvider":{"triggerCharacters":["(",","]},"declarationProvider":true,"definitionProvider":true,"typeDefinitionProvider":true,"referencesProvider":true,"documentHighlightProvider":true,"documentSymbolProvider":true,"codeActionProvider":true,"documentLinkProvider":{},"workspaceSymbolProvider":true,"documentFormattingProvider":true,"renameProvider":true,"executeCommandProvider":{"commands":["gocpp.stats","gocpp.showExpansion"]},"semanticTokensProvider":{"legend":{"tokenTypes":["namespace","type","struct","interface","typeParameter","parameter","variable","property","function","method","cppHeader","cppNamespace","cppName","macro","operator","keyword"],"tokenModifiers":["declaration","readonly","defaultLibrary","modification"]},"range":true,"full":true},"inlayHintProvider":{}},"serverInfo":{"name":"gocpp","version":` + strconv.Quote(version.Version) + `}}`

func TestLifecycle(t *testing.T) {
	c := start(t)
	wantResult(t, c.call("initialize", initParams), initResult)
	if tr := c.s.Trace(); tr != protocol.Messages {
		t.Errorf("the trace is %q, want messages", tr)
	}
	c.notify("initialized", `{}`)
	wantResult(t, c.call("shutdown", ""), "null")
	c.notify("exit", "")
	if code := c.wait(); code != 0 {
		t.Errorf("exit after shutdown: exit code %d, want 0", code)
	}
	if strings.Contains(c.logs.String(), "exit status 1") {
		t.Errorf("the log:\n%s", c.logs)
	}
}

func TestExitWithoutShutdown(t *testing.T) {
	c := start(t)
	c.call("initialize", initParams)
	c.notify("exit", "")
	if code := c.wait(); code != 1 {
		t.Errorf("exit without shutdown: exit code %d, want 1", code)
	}
	if !strings.Contains(c.logs.String(), "exit without shutdown: exit status 1") {
		t.Errorf("the log:\n%s", c.logs)
	}
	// Exit is the one notification that works before initialize too.
	c = start(t)
	c.notify("exit", "")
	if code := c.wait(); code != 1 {
		t.Errorf("exit before initialize: exit code %d, want 1", code)
	}
}

func TestEndOfInput(t *testing.T) {
	c := start(t)
	c.call("initialize", initParams)
	if code := c.wait(); code != 1 {
		t.Errorf("the end without shutdown: exit code %d, want 1", code)
	}
	c = start(t)
	c.call("initialize", initParams)
	c.call("shutdown", "")
	if code := c.wait(); code != 0 {
		t.Errorf("the end after shutdown: exit code %d, want 0", code)
	}
	if !strings.Contains(c.logs.String(), "closed the connection without the exit notification") {
		t.Errorf("the log:\n%s", c.logs)
	}
}

func TestBrokenStream(t *testing.T) {
	c := start(t)
	c.call("initialize", initParams)
	c.call("shutdown", "")
	fmt.Fprint(c.in, "Content-Length: x\r\n\r\n")
	if code := c.wait(); code != 1 {
		t.Errorf("a bad header: exit code %d, want 1", code)
	}
	if !strings.Contains(c.logs.String(), "the connection broke: bad message header") {
		t.Errorf("the log:\n%s", c.logs)
	}
}

func TestBeforeInitialize(t *testing.T) {
	c := start(t)
	wantError(t, c.call("textDocument/hover", `{}`), protocol.ServerNotInitialized, "textDocument/hover before initialize")
	wantError(t, c.call("shutdown", ""), protocol.ServerNotInitialized, "shutdown before initialize")
	// A notification is dropped.
	c.notify("textDocument/didOpen", `{"textDocument":{"uri":"file:///w/a.gp","languageId":"gocpp","version":1,"text":"x"}}`)
	c.notify("initialized", `{}`)
	wantResult(t, c.call("initialize", initParams), initResult)
	if all := c.overlays(); len(all) != 0 {
		t.Errorf("a didOpen before initialize opened %s", all[0].URI())
	}
}

// TestPositionEncoding: a client that offers only UTF-8 still gets UTF-16,
// which every client must support (LSP 3.17 "positionEncodings").
func TestPositionEncoding(t *testing.T) {
	c := start(t)
	resp := c.call("initialize", `{"processId":1,"rootUri":null,"capabilities":{"general":{"positionEncodings":["utf-8"]}}}`)
	wantResult(t, resp, initResult)
}

func TestInitializeTwice(t *testing.T) {
	c := start(t)
	// Bad params: an error, and the server is still not initialized.
	wantError(t, c.call("initialize", ""), protocol.InvalidParams, "no params")
	wantError(t, c.call("initialize", `[1]`), protocol.InvalidParams, "bad params")
	wantError(t, c.call("initialize", `{"processId":"x"}`), protocol.InvalidParams, "bad params")
	wantError(t, c.call("textDocument/hover", `{}`), protocol.ServerNotInitialized, "")
	wantResult(t, c.call("initialize", initParams), initResult)
	wantError(t, c.call("initialize", initParams), protocol.InvalidRequest, "initialize was sent twice")
	wantResult(t, c.call("shutdown", ""), "null")
}

func TestAfterShutdown(t *testing.T) {
	c := start(t)
	c.call("initialize", initParams)
	c.call("shutdown", "")
	wantError(t, c.call("shutdown", ""), protocol.InvalidRequest, "shutdown after shutdown")
	wantError(t, c.call("textDocument/hover", `{}`), protocol.InvalidRequest, "after shutdown")
	c.notify("textDocument/didOpen", `{"textDocument":{"uri":"file:///w/a.gp","languageId":"gocpp","version":1,"text":"x"}}`)
	wantError(t, c.call("initialize", initParams), protocol.InvalidRequest, "")
	if all := c.overlays(); len(all) != 0 {
		t.Errorf("a didOpen after shutdown opened %s", all[0].URI())
	}
	c.notify("exit", "")
	if code := c.wait(); code != 0 {
		t.Errorf("exit code %d, want 0", code)
	}
}

func TestOtherMethods(t *testing.T) {
	c := start(t)
	c.call("initialize", initParams)
	c.notify("initialized", `{}`)
	wantError(t, c.call("textDocument/frobnicate", `{}`), protocol.MethodNotFound, "method not found: textDocument/frobnicate")
	wantError(t, c.call("$/unknown", `{}`), protocol.MethodNotFound, "")
	wantError(t, c.call("exit", ""), protocol.MethodNotFound, "") // exit is a notification
	// Dropped without a response.
	c.notify("frob/nicate", `{}`)
	c.notify("$/progress", `{"token":1,"value":{}}`)
	c.notify("$/cancelRequest", `{"id":1}`)
	c.notify("$/cancelRequest", `{"id":"no such request"}`)
	c.notify("workspace/didChangeConfiguration", `{"settings":{"gocpp":{"x":1}}}`)
	c.notify("$/setTrace", `{"value":"verbose"}`)
	wantResult(t, c.call("shutdown", ""), "null")
	if tr := c.s.Trace(); tr != protocol.Verbose {
		t.Errorf("the trace is %q, want verbose", tr)
	}
	logs := c.logs.String()
	if !strings.Contains(logs, "ignored the notification frob/nicate") || strings.Contains(logs, "$/") {
		t.Errorf("the log:\n%s", logs)
	}
}

func TestBadMessages(t *testing.T) {
	c := start(t)
	c.send(`{"jsonrpc":"2.0","id":1,"method":`)
	if resp, ok := c.read().(*protocol.Response); !ok || resp.ID != nil {
		t.Fatalf("a parse error: got %+v", resp)
	} else {
		wantError(t, resp, protocol.ParseError, "")
	}
	c.send(`[]`)
	if resp, ok := c.read().(*protocol.Response); !ok {
		t.Fatal("a batch: no response")
	} else {
		wantError(t, resp, protocol.InvalidRequest, "batches are not supported")
	}
	// The server goes on.
	wantResult(t, c.call("initialize", initParams), initResult)
}

const (
	uriA = protocol.DocumentURI("file:///w/a.gp")
	uriB = protocol.DocumentURI("file:///w/b.gp")
)

func didOpen(uri protocol.DocumentURI, version int, text string) string {
	t, _ := json.Marshal(text)
	return fmt.Sprintf(`{"textDocument":{"uri":%q,"languageId":"gocpp","version":%d,"text":%s}}`, uri, version, t)
}

func TestTextSync(t *testing.T) {
	c := start(t)
	c.call("initialize", initParams)
	c.notify("initialized", `{}`)
	c.notify("textDocument/didOpen", didOpen(uriA, 1, "package main\r\n\r\nfunc main() {}\r\n"))
	c.notify("textDocument/didOpen", didOpen(uriB, 5, "// 😀 é\n"))
	c.notify("textDocument/didChange", `{"textDocument":{"uri":"file:///w/a.gp","version":2},"contentChanges":[
		{"range":{"start":{"line":2,"character":13},"end":{"line":2,"character":13}},"text":"\r\n\tprintln(\"😀\")\r\n"},
		{"range":{"start":{"line":3,"character":10},"end":{"line":3,"character":12}},"text":"🎉"}]}`)
	c.notify("textDocument/didChange", `{"textDocument":{"uri":"file:///w/b.gp","version":6},"contentChanges":[
		{"range":{"start":{"line":0,"character":5},"end":{"line":0,"character":7}},"rangeLength":2,"text":"ü"}]}`)
	c.notify("textDocument/didSave", `{"textDocument":{"uri":"file:///w/a.gp"}}`)
	c.call("shutdown", "") // after the notifications are handled
	if o := c.overlay(uriA); o == nil || o.Version() != 2 || o.Text() != "package main\r\n\r\nfunc main() {\r\n\tprintln(\"🎉\")\r\n}\r\n" {
		t.Errorf("a.gp: %+v %q", o, o.Text())
	}
	if o := c.overlay(uriB); o == nil || o.Version() != 6 || o.Text() != "// 😀ü\n" {
		t.Errorf("b.gp: %+v %q", o, o.Text())
	}
}

func TestTextSyncFullAndClose(t *testing.T) {
	c := start(t)
	c.call("initialize", initParams)
	c.notify("textDocument/didOpen", didOpen(uriA, 1, "old"))
	c.notify("textDocument/didChange", `{"textDocument":{"uri":"file:///w/a.gp","version":2},"contentChanges":[{"text":"new\ntext"}]}`)
	c.call("textDocument/hover", `{}`) // the notifications are handled now
	if o := c.overlay(uriA); o == nil || o.Text() != "new\ntext" {
		t.Errorf("after a full change: %q", o.Text())
	}
	c.notify("textDocument/didClose", `{"textDocument":{"uri":"file:///w/a.gp"}}`)
	// Problems are logged; a notification gets no response. A failed
	// change also gets a window/showMessage, once per document.
	c.notify("textDocument/didChange", `{"textDocument":{"uri":"file:///w/a.gp","version":3},"contentChanges":[{"text":"x"}]}`)
	wantMessage := `{"type":1,"message":"gocpp lsp: a change of file:///w/a.gp failed, so the server's copy of the file no longer matches the editor's. Close the file and open it again."}`
	if got := c.readNotification("window/showMessage"); got != wantMessage {
		t.Errorf("window/showMessage %s, want %s", got, wantMessage)
	}
	c.notify("textDocument/didChange", `{"textDocument":{"uri":"file:///w/a.gp","version":4},"contentChanges":[{"text":"x"}]}`)
	c.notify("textDocument/didClose", `{"textDocument":{"uri":"file:///w/a.gp"}}`)
	c.notify("textDocument/didOpen", `{"textDocument":{"uri":"untitled:1"}}`)
	c.notify("textDocument/didOpen", didOpen(uriB, 1, "b"))
	c.notify("textDocument/didOpen", didOpen(uriB, 1, "b2"))
	c.notify("textDocument/didChange", `{"textDocument":{"uri":"file:///w/b.gp","version":1},"contentChanges":[{"text":"b3"}]}`)
	backward := `{"textDocument":{"uri":"file:///w/b.gp","version":2},"contentChanges":[
		{"range":{"start":{"line":0,"character":2},"end":{"line":0,"character":1}},"text":"x"}]}`
	c.notify("textDocument/didChange", backward)
	if got := c.readNotification("window/showMessage"); !strings.Contains(got, "a change of file:///w/b.gp failed") {
		t.Errorf("window/showMessage %s", got)
	}
	c.notify("textDocument/didChange", backward)
	c.call("textDocument/hover", `{}`) // no second message before the response
	if o := c.overlay(uriB); o == nil || !o.Broken() {
		t.Error("b.gp is not broken")
	}
	// After didOpen, a new failure is told again.
	c.notify("textDocument/didOpen", didOpen(uriB, 1, "b4"))
	c.call("textDocument/hover", `{}`)
	if o := c.overlay(uriB); o == nil || o.Broken() {
		t.Error("b.gp is still broken after didOpen")
	}
	c.notify("textDocument/didChange", backward)
	c.readNotification("window/showMessage")
	c.notify("textDocument/didOpen", didOpen(uriB, 1, "b3"))
	c.call("shutdown", "")
	if o := c.overlay(uriA); o != nil {
		t.Errorf("a.gp is still open: %q", o.Text())
	}
	if o := c.overlay(uriB); o == nil || o.Text() != "b3" || o.Version() != 1 {
		t.Errorf("b.gp: %q", o.Text())
	}
	logs := c.logs.String()
	for _, want := range []string{
		"textDocument/didChange: file:///w/a.gp is not open",
		"textDocument/didClose: file:///w/a.gp is not open",
		"textDocument/didOpen: bad params: DocumentURI scheme is not 'file': untitled:1",
		"didOpen: file:///w/b.gp was open already",
		"didChange: file:///w/b.gp goes from version 1 to 1",
		"textDocument/didChange: file:///w/b.gp: change 0: the range 0:2-0:1 ends before it starts",
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("the log has no %q:\n%s", want, logs)
		}
	}
}
