// gocpp's own code (not copied from gopls, see COPIED.md): a JSON-RPC 2.0
// connection that dispatches requests and notifications to a Handler.

package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"slices"
	"sync"
	"time"
)

// A Handler handles the requests and notifications of a Conn.
//
// Both methods run on the Conn's read goroutine, one message at a time and
// in the order the messages came: a request sees every notification that
// came before it and none that came after. So they must be quick (applying
// a didChange is): the next message is read only when they return. The
// slow part of a request is its Work, which runs on a goroutine of its
// own, at the same time as the next messages.
type Handler interface {
	// Notify handles a notification. An error is only logged, except
	// ErrStop, which ends Serve after the notification ("exit").
	Notify(ctx context.Context, req *Request) error

	// Call starts a request: it does what must happen in order (checks
	// of the lifecycle, taking the current snapshot) and returns the rest
	// of the work. ctx is the request's context, the one the Work gets.
	Call(ctx context.Context, req *Request) Work
}

// Work is the part of a request that runs on its own goroutine. Its result,
// or its error, is the response: a *ResponseError chooses the error code,
// any other error is an InternalError. A nil Work answers null.
//
// ctx is done when the request is cancelled ($/cancelRequest or
// Conn.Cancel; the response has been sent then, and what the Work returns
// is dropped) or when Serve ends (Serve says what is answered then). A
// Work should check ctx while it works, and return soon after ctx is done.
type Work func(ctx context.Context) (result any, err error)

// Reply returns a Work that answers with result and err: for a request
// that Call answers at once.
func Reply(result any, err error) Work {
	return func(context.Context) (any, error) { return result, err }
}

// ErrStop is the error Notify returns to end Serve (the "exit"
// notification).
var ErrStop = errors.New("protocol: stop serving")

// stopWait is how long Serve waits at its end for the Work of the requests
// that are still running (their contexts are cancelled then).
const stopWait = time.Second

// A Conn is a JSON-RPC 2.0 connection over a byte stream with the LSP
// framing (stdin and stdout for "gocpp lsp").
//
// The requests run at the same time, each Work on its own goroutine with a
// context per request ID. That is on purpose, and unlike gopls: gopls's
// jsonrpc2.AsyncHandler runs one request after the other, and only a
// handler that calls jsonrpc2.Async (diagnostics, a few slow commands)
// lets the next one start. Here the order comes from Handler.Call, which
// takes the request's snapshot on the read loop; the Work then runs beside
// the others. There is no limit on how many run at once: the client sends
// a few at a time, and the expensive steps behind them (type-checking,
// step 11.3) limit their own parallelism.
type Conn struct {
	r   *Reader
	w   *Writer
	log *log.Logger

	mu       sync.Mutex
	calls    map[ID]*call // the requests whose answer is not written yet
	ended    bool         // the read loop has ended: no answer is written any more
	writeErr error        // the first error writing a response, a *WriteError
	onEnd    func()       // OnEnd
	works    sync.WaitGroup
	sent     map[ID]string // the requests of the server whose answer has not come, by ID: their methods
	lastID   int64         // the number of the last ID of a request of the server
}

// A call is a request in flight.
type call struct {
	id     ID
	method string
	cancel context.CancelCauseFunc
	// claimed: the answer is being written (finish), by the Work's
	// goroutine or by Cancel; under Conn.mu.
	claimed bool
	done    chan struct{} // closed when the answer is written
}

// NewConn returns a connection that reads messages from r and writes them
// to w. It logs problems with the messages to logger, which may be nil.
func NewConn(r io.Reader, w io.Writer, logger *log.Logger) *Conn {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Conn{r: NewReader(r), w: NewWriter(w), log: logger, calls: make(map[ID]*call)}
}

// ErrClosed is the error of Notify after the connection has ended (Serve):
// nothing is written to the client any more.
var ErrClosed = errors.New("protocol: the connection has ended")

// A WriteError is an error writing a message to the client: the client
// has stopped reading (a closed pipe), or the output broke.
type WriteError struct {
	Err error
}

func (e *WriteError) Error() string { return "writing to the client: " + e.Err.Error() }
func (e *WriteError) Unwrap() error { return e.Err }

// errEnded is the cause of the contexts of the requests that are still
// running when Serve ends.
var errEnded = Errorf(RequestCancelled, "the connection ended")

// Serve reads the messages and hands them to h (Handler says how) until
// h.Notify returns ErrStop (then Serve returns nil), the input ends
// (io.EOF), or the stream breaks (a *HeaderError, a read or write error).
// ctx is the parent of every request's context. Serve checks it only
// between two messages: a Serve that waits for input doesn't see ctx end
// before the next message (or the end of the input) comes.
//
// A $/cancelRequest is the Conn's own: it cancels the request with that ID
// (Cancel with RequestCancelled), and an unknown ID is ignored. A body that
// is not JSON gets a ParseError response, JSON that is not a JSON-RPC 2.0
// message gets an InvalidRequest response, and so does a request whose ID
// is the ID of a request still in flight; Serve goes on after them. A
// response from the client to a request of the server (Request) is logged
// when it is an error, and dropped; another response is dropped too.
//
// A request whose context is cancelled while its Work runs, and whose Work
// then returns the context's error (context.Canceled, maybe wrapped), is
// answered with the cause of the cancellation when that is a
// *ResponseError (RequestCancelled, ContentModified), and with
// RequestCancelled when not. Another error, or a result, is the answer as
// usual.
//
// When Serve ends, it cancels the requests that are still running and
// waits up to a second for their Work to return (so that they let go of
// what they hold). After ErrStop ("exit") or the end or break of the
// stream, nothing more is written: the requests in flight get no answer,
// like gopls, which closes the connection at exit. Only when ctx ends are
// they answered RequestCancelled, since the client may still be there.
// Once the answers have ended (at once after ErrStop or the end or break of
// the stream, after the wait when ctx ends), nothing is written any more:
// Notify returns ErrClosed (a write that has started still finishes). A
// write error is Serve's error, a *WriteError, except after ErrStop: then
// Serve returns nil.
//
// The function of OnEnd runs as soon as the read loop ends, before the
// wait.
func (c *Conn) Serve(ctx context.Context, h Handler) error {
	err := c.serve(ctx, h)
	quiet := ctx.Err() == nil || err != ctx.Err()
	c.mu.Lock()
	if quiet {
		c.end()
	}
	running := make([]*call, 0, len(c.calls))
	for _, cl := range c.calls {
		running = append(running, cl)
	}
	onEnd := c.onEnd
	c.mu.Unlock()
	if onEnd != nil {
		onEnd()
	}
	for _, cl := range running {
		cl.cancel(errEnded)
	}
	done := make(chan struct{})
	go func() {
		c.works.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(stopWait):
		c.log.Printf("requests still run %v after the end of the connection", stopWait)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.end()
	clear(c.calls)
	if err != nil && c.writeErr != nil {
		return c.writeErr
	}
	return err
}

// end marks the end of the answers: the requests whose answer nobody has
// claimed get none (finish fails from now on). c.mu is held.
func (c *Conn) end() {
	c.ended = true
	for _, cl := range c.calls {
		if !cl.claimed {
			cl.claimed = true
			close(cl.done)
		}
	}
}

func (c *Conn) serve(ctx context.Context, h Handler) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := c.writeError(); err != nil {
			return err
		}
		body, err := c.r.Read()
		if err != nil {
			return err
		}
		msg, err := DecodeMessage(body)
		if err != nil {
			var de *DecodeError
			errors.As(err, &de)
			c.log.Printf("bad message: %v", err)
			resp := &Response{ID: de.ID, Error: &ResponseError{Code: int64(de.Code), Message: err.Error()}}
			if err := c.write(resp); err != nil {
				return &WriteError{err}
			}
			continue
		}
		var req *Request
		switch m := msg.(type) {
		case *Response:
			c.answered(m)
			continue
		case *Request:
			req = m
		}
		if req.IsNotification() {
			if req.Method == "$/cancelRequest" {
				c.cancelRequest(req.Params)
				continue
			}
			if err := h.Notify(ctx, req); errors.Is(err, ErrStop) {
				return nil
			} else if err != nil {
				c.log.Printf("%s: %v", req.Method, err)
			}
			continue
		}
		if err := c.start(ctx, h, req); err != nil {
			return err
		}
	}
}

// start starts the request req: h.Call on this goroutine, the Work on a new
// one.
func (c *Conn) start(ctx context.Context, h Handler, req *Request) error {
	rctx, cancel := context.WithCancelCause(ctx)
	cl := &call{id: *req.ID, method: req.Method, cancel: cancel, done: make(chan struct{})}
	c.mu.Lock()
	// An ID whose answer is being written is free: the client may have
	// read the answer already.
	old := c.calls[cl.id]
	dup := old != nil && !old.claimed
	if !dup {
		c.calls[cl.id] = cl
	}
	c.mu.Unlock()
	if dup {
		cancel(nil)
		err := Errorf(InvalidRequest, "%s: the request ID %v is in use by a request in flight", req.Method, cl.id)
		c.log.Print(err.Message)
		if err := c.write(&Response{ID: req.ID, Error: err}); err != nil {
			return &WriteError{err}
		}
		return nil
	}
	work := h.Call(rctx, req)
	c.works.Add(1)
	go func() {
		defer c.works.Done()
		defer cancel(nil)
		var result any
		var err error
		if work != nil {
			result, err = work(rctx)
		}
		if errors.Is(err, context.Canceled) && rctx.Err() != nil {
			var re *ResponseError
			if !errors.As(context.Cause(rctx), &re) {
				// Serve's own ctx was cancelled.
				re = Errorf(RequestCancelled, "request %v cancelled", cl.id)
			}
			err = re
		}
		if c.finish(cl) {
			c.reply(cl, result, err)
		}
	}()
	return nil
}

// finish claims the answer to the request cl: the caller must write it
// with reply. It reports false when the answer was claimed already
// (cancelled) or the answers have ended (Serve): then nothing must be
// written.
func (c *Conn) finish(cl *call) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ended || c.calls[cl.id] != cl || cl.claimed {
		return false
	}
	cl.claimed = true
	return true
}

// cancelRequest handles a $/cancelRequest notification.
func (c *Conn) cancelRequest(params json.RawMessage) {
	var p struct {
		ID *ID `json:"id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.ID == nil {
		c.log.Printf("$/cancelRequest: bad params %s", params)
		return
	}
	c.Cancel(*p.ID, Errorf(RequestCancelled, "request %v cancelled", *p.ID))
}

// Cancel cancels the request id that is in flight: it answers the request
// with the error err (RequestCancelled, or ContentModified when a change
// made the result stale) and cancels its context, with err as the cause
// (context.Cause). What its Work returns later is dropped. Cancel reports
// false, and does nothing, when no request id is in flight (it is answered
// already, or never came). It is safe for concurrent use.
func (c *Conn) Cancel(id ID, err *ResponseError) bool {
	c.mu.Lock()
	cl := c.calls[id]
	c.mu.Unlock()
	return cl != nil && c.cancelCall(cl, err)
}

func (c *Conn) cancelCall(cl *call, err *ResponseError) bool {
	if !c.finish(cl) {
		return false
	}
	cl.cancel(err)
	c.reply(cl, nil, err)
	return true
}

// CancelAll cancels every request in flight but the ones in except, like
// Cancel with err.
func (c *Conn) CancelAll(err *ResponseError, except ...ID) {
	for _, cl := range c.inFlight(except) {
		c.cancelCall(cl, err)
	}
}

// Wait waits until the requests in flight when it is called, but the ones
// in except, are answered (their answer is written, or will never be), or
// until ctx is done. It reports whether they all were answered. A request
// that comes later is not waited for.
func (c *Conn) Wait(ctx context.Context, except ...ID) bool {
	for _, cl := range c.inFlight(except) {
		select {
		case <-cl.done:
		case <-ctx.Done():
			return false
		}
	}
	return true
}

// InFlight returns the number of requests whose Work runs and whose answer
// is not on its way yet.
func (c *Conn) InFlight() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, cl := range c.calls {
		if !cl.claimed {
			n++
		}
	}
	return n
}

// inFlight returns the requests in flight but the ones in except.
func (c *Conn) inFlight(except []ID) []*call {
	c.mu.Lock()
	defer c.mu.Unlock()
	calls := make([]*call, 0, len(c.calls))
	for id, cl := range c.calls {
		if !slices.Contains(except, id) {
			calls = append(calls, cl)
		}
	}
	return calls
}

// reply writes the response to the request cl, whose answer the caller has
// claimed (finish), and then marks cl answered: it leaves the requests in
// flight (Wait). A write error is kept: the read loop stops at it, and
// Serve returns it.
func (c *Conn) reply(cl *call, result any, err error) {
	defer func() {
		c.mu.Lock()
		if c.calls[cl.id] == cl {
			delete(c.calls, cl.id)
		}
		c.mu.Unlock()
		close(cl.done)
	}()
	id := cl.id
	resp := &Response{ID: &id}
	if err == nil {
		resp.Result, err = json.Marshal(result)
		if err != nil {
			c.log.Printf("%s: cannot encode the result: %v", cl.method, err)
			resp.Result = nil
		}
	}
	if err != nil {
		var re *ResponseError
		if !errors.As(err, &re) {
			re = &ResponseError{Code: int64(InternalError), Message: err.Error()}
		}
		resp.Error = re
	}
	if werr := c.write(resp); werr != nil && werr != ErrClosed {
		c.mu.Lock()
		first := c.writeErr == nil
		if first {
			c.writeErr = &WriteError{werr}
		}
		c.mu.Unlock()
		if first {
			c.log.Printf("%s: cannot write the answer: %v", cl.method, werr)
		}
	}
}

func (c *Conn) writeError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writeErr
}

// OnEnd sets a function that Serve calls once, as soon as its read loop
// ends (exit, the end of the input, a broken stream or ctx), before it
// waits for the requests: the server stops its background work there, so
// that it doesn't try to write to the client any more. Call it before
// Serve.
func (c *Conn) OnEnd(f func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEnd = f
}

// Notify sends a notification to the client. It is safe for concurrent use.
// After the end of the connection it returns ErrClosed and writes nothing
// (Serve).
func (c *Conn) Notify(method string, params any) error {
	var data json.RawMessage
	if params != nil {
		var err error
		if data, err = json.Marshal(params); err != nil {
			return err
		}
	}
	return c.write(&Request{Method: method, Params: data})
}

// Request sends a request to the client, with an ID of the server's own
// ("gocpp-1", "gocpp-2", ...), and returns without waiting for the answer:
// when it comes, an error is logged and a result dropped (the server's
// requests, such as client/registerCapability, need no result). It is safe
// for concurrent use. After the end of the connection it returns ErrClosed
// and writes nothing (Serve).
func (c *Conn) Request(method string, params any) error {
	var data json.RawMessage
	if params != nil {
		var err error
		if data, err = json.Marshal(params); err != nil {
			return err
		}
	}
	c.mu.Lock()
	c.lastID++
	id := StringID(fmt.Sprintf("gocpp-%d", c.lastID))
	if c.sent == nil {
		c.sent = make(map[ID]string)
	}
	c.sent[id] = method
	c.mu.Unlock()
	err := c.write(&Request{ID: &id, Method: method, Params: data})
	if err != nil {
		c.mu.Lock()
		delete(c.sent, id)
		c.mu.Unlock()
	}
	return err
}

// answered handles a response from the client.
func (c *Conn) answered(m *Response) {
	var method string
	ok := false
	if m.ID != nil {
		c.mu.Lock()
		method, ok = c.sent[*m.ID]
		delete(c.sent, *m.ID)
		c.mu.Unlock()
	}
	switch {
	case !ok:
		c.log.Printf("dropped a response to request %v: the server sent no such request", m.ID)
	case m.Error != nil:
		c.log.Printf("%s: the client answered with an error: %v", method, m.Error)
	}
}

func (c *Conn) write(m Message) error {
	c.mu.Lock()
	ended := c.ended
	c.mu.Unlock()
	if ended {
		return ErrClosed
	}
	data, err := EncodeMessage(m)
	if err != nil {
		return err
	}
	return c.w.Write(data)
}
