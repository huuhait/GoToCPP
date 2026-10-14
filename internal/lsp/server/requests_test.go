package server

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// checkGoroutines fails the test if, at its end, more goroutines run than
// at its start (after a few seconds to let them end).
func checkGoroutines(t *testing.T) {
	t.Helper()
	before := runtime.NumGoroutine()
	t.Cleanup(func() {
		deadline := time.Now().Add(5 * time.Second)
		for {
			n := runtime.NumGoroutine()
			if n <= before {
				return
			}
			if time.Now().After(deadline) {
				buf := make([]byte, 1<<16)
				buf = buf[:runtime.Stack(buf, true)]
				t.Errorf("%d goroutines at the start, %d at the end:\n%s", before, n, buf)
				return
			}
			time.Sleep(time.Millisecond)
		}
	})
}

// A textResult is the answer of the test methods: what the snapshot of the
// request had.
type textResult struct {
	Snapshot uint64 `json:"snapshot"`
	Text     string `json:"text"`
	Version  int32  `json:"version"`
}

// The test methods. Their params are {"textDocument":{"uri":...}}.
//   - test/text answers at once with the text of the document;
//   - test/block says it started on started, then waits until it gets a
//     token from release (and answers like test/text) or its context is
//     done (and answers the context's error);
//   - test/stale is test/block, but a change of its document makes it
//     stale (method.contentModified).
type testMethods struct {
	started  chan string // the method, when it starts to wait
	release  chan struct{}
	returned chan string // the method, when its Work returns
}

func newTestMethods() *testMethods {
	return &testMethods{started: make(chan string, 100), release: make(chan struct{}), returned: make(chan string, 100)}
}

func (m *testMethods) setup(s *Server) {
	text := func(e env, params json.RawMessage) (any, error) {
		snap := e.snapshot
		var p protocol.TextDocumentIdentifier
		var wrap struct {
			TextDocument *protocol.TextDocumentIdentifier `json:"textDocument"`
		}
		wrap.TextDocument = &p
		if err := decode(params, &wrap); err != nil {
			return nil, err
		}
		r := textResult{Snapshot: snap.ID()}
		if o := snap.Overlay(p.URI); o != nil {
			r.Text, r.Version = o.Text(), o.Version()
		}
		return r, nil
	}
	block := func(name string) func(context.Context, env, json.RawMessage) (any, error) {
		return func(ctx context.Context, e env, params json.RawMessage) (any, error) {
			defer func() { m.returned <- name }()
			m.started <- name
			select {
			case <-m.release:
				return text(e, params)
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	s.requests["test/text"] = method{run: func(_ context.Context, e env, params json.RawMessage) (any, error) {
		return text(e, params)
	}}
	s.requests["test/block"] = method{run: block("test/block")}
	s.requests["test/stale"] = method{run: block("test/stale"), contentModified: true}
}

// settled waits until the server holds no request any more: one live
// snapshot (the current one) and no stale request. A Work lets go of its
// snapshot after it returns, which may be after its answer (a cancelled
// request is answered at once), so the test polls, for up to 10 s.
func (c *client) settled() {
	c.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		c.s.mu.Lock()
		stale := len(c.s.stale)
		c.s.mu.Unlock()
		live := c.s.Session().Live()
		if live == 1 && stale == 0 {
			return
		}
		if time.Now().After(deadline) {
			c.t.Fatalf("after 10 s: %d snapshots live (want 1), %d documents with stale requests (want 0)", live, stale)
		}
		time.Sleep(100 * time.Microsecond)
	}
}

// wait returns the next value of ch, or fails after 10 s.
func wait[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatal("nothing came in 10 s")
	}
	var zero T
	return zero
}

func docParams(uri protocol.DocumentURI) string {
	return fmt.Sprintf(`{"textDocument":{"uri":%q}}`, uri)
}

func fullChange(uri protocol.DocumentURI, version int, text string) string {
	t, _ := json.Marshal(text)
	return fmt.Sprintf(`{"textDocument":{"uri":%q,"version":%d},"contentChanges":[{"text":%s}]}`, uri, version, t)
}

// request sends a request without waiting for its answer and returns its
// ID.
func (c *client) request(method, params string) protocol.ID {
	c.t.Helper()
	c.id++
	c.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%q,"params":%s}`, c.id, method, params))
	return protocol.IntID(int64(c.id))
}

// response reads the next message, which must be the response to id.
func (c *client) response(id protocol.ID) *protocol.Response {
	c.t.Helper()
	m := c.read()
	resp, ok := m.(*protocol.Response)
	if !ok || resp.ID == nil || *resp.ID != id {
		c.t.Fatalf("got %+v, want the response to %v", m, id)
	}
	return resp
}

func textOf(t *testing.T, resp *protocol.Response) textResult {
	t.Helper()
	var r textResult
	if resp.Error != nil || json.Unmarshal(resp.Result, &r) != nil {
		t.Fatalf("got %+v (error %+v), want a text result", resp, resp.Error)
	}
	return r
}

// TestRequestsSeeChangesBefore sends didChange and requests one after the
// other without waiting (run it with -race): a request sees exactly the
// changes that came before it, however the requests interleave.
func TestRequestsSeeChangesBefore(t *testing.T) {
	checkGoroutines(t)
	m := newTestMethods()
	c := startWith(t, m.setup)
	c.call("initialize", initParams)
	c.notify("textDocument/didOpen", didOpen(uriA, 1, "v1"))
	c.notify("textDocument/didOpen", didOpen(uriB, 1, "b"))
	const n = 400
	sent := make(chan error, 1)
	go func() {
		// Not c.send: t.Fatal must not be called on this goroutine.
		w := func(body string) error {
			_, err := fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n%s", len(body), body)
			return err
		}
		for v := 2; v <= n; v++ {
			for _, body := range []string{
				`{"jsonrpc":"2.0","method":"textDocument/didChange","params":` + fullChange(uriA, v, fmt.Sprintf("v%d", v)) + `}`,
				fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"test/text","params":%s}`, v, docParams(uriA)),
				`{"jsonrpc":"2.0","method":"textDocument/didChange","params":` + fullChange(uriB, v, fmt.Sprintf("b%d", v)) + `}`,
			} {
				if err := w(body); err != nil {
					sent <- err
					return
				}
			}
		}
		sent <- nil
	}()
	seen := make(map[int64]bool)
	for range n - 1 {
		m := c.read()
		resp, ok := m.(*protocol.Response)
		if !ok || resp.ID == nil {
			t.Fatalf("got %+v, want a response", m)
		}
		var id int64
		fmt.Sscan(resp.ID.String(), &id)
		if seen[id] {
			t.Fatalf("two responses to %d", id)
		}
		seen[id] = true
		r := textOf(t, resp)
		// Snapshot: 0, 2 opens, then 2 changes per round, A's first.
		if want := fmt.Sprintf("v%d", id); r.Text != want || r.Version != int32(id) || r.Snapshot != uint64(2*id-1) {
			t.Fatalf("request %d saw %+v, want the text %q of snapshot %d", id, r, want, 2*id-1)
		}
	}
	if err := wait(t, sent); err != nil {
		t.Fatal(err)
	}
	c.call("shutdown", "")
	c.settled() // the requests released their snapshots
	c.notify("exit", "")
	if code := c.wait(); code != 0 {
		t.Errorf("exit code %d", code)
	}
}

// TestCancelRequest: $/cancelRequest answers RequestCancelled, the Work
// stops and the snapshot is released.
func TestCancelRequest(t *testing.T) {
	checkGoroutines(t)
	m := newTestMethods()
	c := startWith(t, m.setup)
	c.call("initialize", initParams)
	c.notify("textDocument/didOpen", didOpen(uriA, 1, "a"))
	id := c.request("test/block", docParams(uriA))
	wait(t, m.started)
	// Other requests answer meanwhile.
	if r := textOf(t, c.call("test/text", docParams(uriA))); r.Text != "a" {
		t.Errorf("test/text: %+v", r)
	}
	c.notify("$/cancelRequest", fmt.Sprintf(`{"id":%s}`, id))
	wantError(t, c.response(id), protocol.RequestCancelled, "cancelled")
	wait(t, m.returned)
	c.notify("textDocument/didChange", fullChange(uriA, 2, "a2"))
	c.call("test/text", docParams(uriA)) // the didChange is handled
	c.settled()
	c.call("shutdown", "")
	c.notify("exit", "")
	c.wait()
}

// TestContentModified: a request finishes on its snapshot after a change of
// its document, unless the change makes it stale (contentModified): then it
// gets ContentModified. A change of another document doesn't.
func TestContentModified(t *testing.T) {
	checkGoroutines(t)
	m := newTestMethods()
	c := startWith(t, m.setup)
	c.call("initialize", initParams)
	c.notify("textDocument/didOpen", didOpen(uriA, 1, "old"))
	c.notify("textDocument/didOpen", didOpen(uriB, 1, "b"))
	block := c.request("test/block", docParams(uriA))
	staleA := c.request("test/stale", docParams(uriA))
	staleB := c.request("test/stale", docParams(uriB))
	for range 3 {
		wait(t, m.started)
	}
	c.notify("textDocument/didChange", fullChange(uriA, 2, "new"))
	resp := c.response(staleA)
	wantError(t, resp, protocol.ContentModified, "file:///w/a.gp changed")
	wait(t, m.returned)
	// The others still wait; let them go.
	if r := textOf(t, c.call("test/text", docParams(uriA))); r.Text != "new" {
		t.Errorf("after the change: %+v", r)
	}
	m.release <- struct{}{}
	m.release <- struct{}{}
	got := map[protocol.ID]textResult{}
	for range 2 {
		r := c.read().(*protocol.Response)
		got[*r.ID] = textOf(t, r)
	}
	if got[block].Text != "old" || got[block].Snapshot != 2 || got[staleB].Text != "b" {
		t.Errorf("got %+v; want test/block to answer from its snapshot (old text) and test/stale of b.gp to finish", got)
	}
	// didClose and a second didOpen of a document make it stale too, and
	// so does a failed didChange (the text is broken then).
	backward := `{"textDocument":{"uri":"file:///w/a.gp","version":4},"contentChanges":[
		{"range":{"start":{"line":0,"character":2},"end":{"line":0,"character":1}},"text":"x"}]}`
	for _, test := range []struct {
		method, params string
		uri, other     protocol.DocumentURI
	}{
		{"textDocument/didClose", docParams(uriB), uriB, uriA},
		{"textDocument/didOpen", didOpen(uriA, 3, "again"), uriA, uriB},
		{"textDocument/didChange", backward, uriA, uriB},
	} {
		if c.overlay(uriB) == nil {
			c.notify("textDocument/didOpen", didOpen(uriB, 1, "b"))
		}
		stale := c.request("test/stale", docParams(test.uri))
		other := c.request("test/stale", docParams(test.other))
		wait(t, m.started)
		wait(t, m.started)
		c.notify(test.method, test.params)
		wantError(t, c.response(stale), protocol.ContentModified, string(test.uri)+" changed")
		wait(t, m.returned)
		if test.method == "textDocument/didChange" {
			c.readNotification("window/showMessage")
		}
		// The other one still runs.
		c.notify("$/cancelRequest", fmt.Sprintf(`{"id":%s}`, other))
		wantError(t, c.response(other), protocol.RequestCancelled, "")
		wait(t, m.returned)
	}
	if o := c.overlay(uriA); o == nil || !o.Broken() || o.Text() != "again" {
		t.Errorf("a.gp: %+v, want the broken text \"again\"", o)
	}
	c.call("shutdown", "")
	c.settled()
	c.notify("exit", "")
	c.wait()
}

// TestShutdownWaits: shutdown answers after the requests in flight; one
// that doesn't finish in time is cancelled.
func TestShutdownWaits(t *testing.T) {
	checkGoroutines(t)
	m := newTestMethods()
	c := startWith(t, func(s *Server) {
		m.setup(s)
		s.shutdownWait = 10 * time.Second
	})
	c.call("initialize", initParams)
	c.notify("textDocument/didOpen", didOpen(uriA, 1, "a"))
	// A request that finishes during the wait answers before shutdown.
	slow := c.request("test/block", docParams(uriA))
	wait(t, m.started)
	shutdown := c.request("shutdown", "{}")
	// After shutdown, a request gets an error at once.
	late := c.request("test/text", docParams(uriA))
	wantError(t, c.response(late), protocol.InvalidRequest, "test/text after shutdown")
	m.release <- struct{}{}
	if r := textOf(t, c.response(slow)); r.Text != "a" {
		t.Errorf("the slow request: %+v", r)
	}
	wantResult(t, c.response(shutdown), "null")
	c.notify("exit", "")
	if code := c.wait(); code != 0 {
		t.Errorf("exit code %d, want 0", code)
	}

	// A request that ignores the wait is cancelled.
	m = newTestMethods()
	c = startWith(t, func(s *Server) {
		m.setup(s)
		s.shutdownWait = 50 * time.Millisecond
	})
	c.call("initialize", initParams)
	slow = c.request("test/block", docParams(uriA))
	wait(t, m.started)
	shutdown = c.request("shutdown", "{}")
	wantError(t, c.response(slow), protocol.RequestCancelled, "the server shuts down")
	wantResult(t, c.response(shutdown), "null")
	wait(t, m.returned)
	c.notify("exit", "")
	if code := c.wait(); code != 0 {
		t.Errorf("exit code %d, want 0", code)
	}
	if !strings.Contains(c.logs.String(), "shutdown: cancelled the requests still running after 50ms") {
		t.Errorf("the log:\n%s", c.logs)
	}
}

// TestExitInFlight: exit (or the end of the input) cancels the requests in
// flight, they get no answer (nothing is written after exit), and Serve
// returns after their Work.
func TestExitInFlight(t *testing.T) {
	checkGoroutines(t)
	for _, end := range []string{"exit", "eof"} {
		m := newTestMethods()
		c := startWith(t, m.setup)
		c.call("initialize", initParams)
		c.request("test/block", docParams(uriA))
		c.request("test/stale", docParams(uriA))
		wait(t, m.started)
		wait(t, m.started)
		if end == "exit" {
			c.notify("exit", "")
		}
		if code := c.wait(); code != 1 { // it checks that nothing more came
			t.Errorf("%s: exit code %d, want 1", end, code)
		}
		if len(m.returned) != 2 {
			t.Errorf("%s: %d Works returned before Serve, want 2", end, len(m.returned))
		}
		c.settled()
	}
}

// TestShutdownAnswersLast sends requests and shutdown without waiting for
// the answers: shutdown is answered after every request before it, and
// none of them is cancelled. An exit sent right after shutdown, without
// waiting for its answer (a script), still lets those answers out first.
func TestShutdownAnswersLast(t *testing.T) {
	checkGoroutines(t)
	for _, exitAtOnce := range []bool{false, true} {
		m := newTestMethods()
		c := startWith(t, m.setup)
		init := c.request("initialize", initParams)
		c.notify("textDocument/didOpen", didOpen(uriA, 1, "a"))
		const n = 50
		for range n {
			c.request("test/text", docParams(uriA))
		}
		shutdown := c.request("shutdown", "{}")
		if exitAtOnce {
			c.notify("exit", "")
		}
		answered := 0
		for {
			r := c.read().(*protocol.Response)
			if r.Error != nil {
				t.Fatalf("request %v: %v", r.ID, r.Error)
			}
			if *r.ID == shutdown {
				break
			}
			if *r.ID != init {
				textOf(t, r)
			}
			answered++
		}
		if answered != n+1 {
			t.Errorf("%d answers before shutdown's, want %d", answered, n+1)
		}
		if !exitAtOnce {
			c.notify("exit", "")
		}
		if code := c.wait(); code != 0 {
			t.Errorf("exit code %d, want 0", code)
		}
	}
}

// TestNothingAfterExit: a Work that notifies the client after exit writes
// nothing (ErrClosed), and the background context of its snapshot is done
// by the time its own context is: the session shut down when the read loop
// ended.
func TestNothingAfterExit(t *testing.T) {
	checkGoroutines(t)
	started := make(chan struct{}, 1)
	seen := make(chan string, 1)
	c := startWith(t, func(s *Server) {
		s.requests["test/late"] = method{run: func(ctx context.Context, e env, _ json.RawMessage) (any, error) {
			started <- struct{}{}
			<-ctx.Done()
			bg := e.snapshot.BackgroundContext().Err()
			err := e.notify("window/logMessage", &protocol.LogMessageParams{Type: protocol.Info, Message: "late"})
			seen <- fmt.Sprintf("background %v, notify %v", bg, err)
			return nil, ctx.Err()
		}}
	})
	c.call("initialize", initParams)
	c.request("test/late", "{}")
	wait(t, started)
	c.notify("exit", "")             // without shutdown: exit at once
	if code := c.wait(); code != 1 { // it checks that nothing more came
		t.Errorf("exit code %d, want 1", code)
	}
	if got, want := wait(t, seen), "background context canceled, notify "+protocol.ErrClosed.Error(); got != want {
		t.Errorf("the Work saw %q, want %q", got, want)
	}
}
