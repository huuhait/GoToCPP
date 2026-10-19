package bench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// A Client talks to a language server over a byte stream, as an editor
// does: it sends requests and notifications, collects the diagnostics the
// server publishes, and answers the requests that servers make of clients
// (gopls asks for its configuration, registers capabilities, and creates
// progress tokens). Timestamps are taken by the goroutine that reads the
// server's output, the moment a message is decoded, so the time of a wait
// is not the time the waiting goroutine got scheduled.
type Client struct {
	w  *protocol.Writer
	cl io.Closer // closes the write side

	mu       sync.Mutex
	nextID   int64
	pending  map[int64]chan *reply
	pubs     map[protocol.DocumentURI][]*Publish
	seq      int64 // the number of publishes so far
	cond     *sync.Cond
	readErr  error
	done     bool
	settings json.RawMessage // the answer to workspace/configuration (one item), or nil
}

type reply struct {
	result json.RawMessage
	err    *protocol.ResponseError
	at     time.Time
}

// A Publish is one textDocument/publishDiagnostics.
type Publish struct {
	URI     protocol.DocumentURI
	Version int32 // 0 when the server sent none
	Diags   []Diag
	At      time.Time
	Seq     int64 // 1, 2, ...: the order of all publishes
}

// A Diag is a diagnostic: its severity (1 error, 2 warning) and message.
type Diag struct {
	Severity int
	Message  string
	Line     int
}

// Errors returns the number of diagnostics of severity error (or without
// a severity) of the publish.
func (p *Publish) Errors() int {
	n := 0
	for _, d := range p.Diags {
		if d.Severity <= 1 {
			n++
		}
	}
	return n
}

// firstError returns the message of the first error of the publish.
func (p *Publish) firstError() string {
	for _, d := range p.Diags {
		if d.Severity <= 1 {
			return d.Message
		}
	}
	return ""
}

// NewClient returns a client that reads the server's output from r and
// writes to w, and starts reading. settings, if not nil, is the JSON
// answer to workspace/configuration (for each item asked for).
func NewClient(r io.Reader, w io.WriteCloser, settings json.RawMessage) *Client {
	c := &Client{w: protocol.NewWriter(w), cl: w, pending: make(map[int64]chan *reply),
		pubs: make(map[protocol.DocumentURI][]*Publish), settings: settings}
	c.cond = sync.NewCond(&c.mu)
	go c.readLoop(protocol.NewReader(r))
	return c
}

func (c *Client) readLoop(r *protocol.Reader) {
	for {
		body, err := r.Read()
		if err == nil {
			var m protocol.Message
			if m, err = protocol.DecodeMessage(body); err == nil {
				c.handle(m, time.Now())
				continue
			}
		}
		c.mu.Lock()
		c.readErr, c.done = err, true
		for id, ch := range c.pending {
			ch <- &reply{err: &protocol.ResponseError{Code: -1, Message: "the server ended: " + err.Error()}, at: time.Now()}
			delete(c.pending, id)
		}
		c.cond.Broadcast()
		c.mu.Unlock()
		return
	}
}

func (c *Client) handle(m protocol.Message, at time.Time) {
	switch m := m.(type) {
	case *protocol.Response:
		if m.ID == nil {
			return
		}
		var id int64
		if _, err := fmt.Sscanf(m.ID.String(), "%d", &id); err != nil {
			return
		}
		c.mu.Lock()
		ch := c.pending[id]
		delete(c.pending, id)
		c.mu.Unlock()
		if ch != nil {
			ch <- &reply{result: m.Result, err: m.Error, at: at}
		}
	case *protocol.Request:
		if m.IsNotification() {
			if m.Method == "textDocument/publishDiagnostics" {
				c.publish(m.Params, at)
			}
			return
		}
		// A request of the server: answer it as a client with nothing to
		// say does.
		var result any
		if m.Method == "workspace/configuration" {
			var p struct{ Items []json.RawMessage }
			json.Unmarshal(m.Params, &p)
			items := make([]json.RawMessage, len(p.Items))
			for i := range items {
				items[i] = json.RawMessage("null")
				if c.settings != nil {
					items[i] = c.settings
				}
			}
			result = items
		}
		resp := &protocol.Response{ID: m.ID}
		if data, err := json.Marshal(result); err == nil {
			resp.Result = data
		}
		if body, err := protocol.EncodeMessage(resp); err == nil {
			c.w.Write(body)
		}
	}
}

func (c *Client) publish(params json.RawMessage, at time.Time) {
	var p struct {
		URI         protocol.DocumentURI `json:"uri"`
		Version     int32                `json:"version"`
		Diagnostics []struct {
			Severity int    `json:"severity"`
			Message  string `json:"message"`
			Range    struct {
				Start struct{ Line int } `json:"start"`
			} `json:"range"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return
	}
	pub := &Publish{URI: p.URI, Version: p.Version, At: at}
	for _, d := range p.Diagnostics {
		pub.Diags = append(pub.Diags, Diag{Severity: d.Severity, Message: d.Message, Line: d.Range.Start.Line})
	}
	c.mu.Lock()
	c.seq++
	pub.Seq = c.seq
	list := append(c.pubs[p.URI], pub)
	if len(list) > 64 {
		list = list[len(list)-32:]
	}
	c.pubs[p.URI] = list
	c.cond.Broadcast()
	c.mu.Unlock()
}

// Seq returns the number of publishes so far: a mark for WaitPublish.
func (c *Client) Seq() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seq
}

// ErrServerEnded is the error of a wait when the server's output ended.
var ErrServerEnded = errors.New("the server ended")

// WaitPublish waits for a publish of uri after the mark since (Seq) for
// which ok is true (nil: any) and returns it. It stops with ctx.Err() when
// ctx is done.
func (c *Client) WaitPublish(ctx context.Context, uri protocol.DocumentURI, since int64, ok func(*Publish) bool) (*Publish, error) {
	stop := context.AfterFunc(ctx, func() {
		c.mu.Lock()
		c.cond.Broadcast()
		c.mu.Unlock()
	})
	defer stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	for {
		for _, p := range c.pubs[uri] {
			if p.Seq > since && (ok == nil || ok(p)) {
				return p, nil
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if c.done {
			return nil, fmt.Errorf("%w: %v", ErrServerEnded, c.readErr)
		}
		c.cond.Wait()
	}
}

// LastPublish returns the last publish of uri, or nil.
func (c *Client) LastPublish(uri protocol.DocumentURI) *Publish {
	c.mu.Lock()
	defer c.mu.Unlock()
	if l := c.pubs[uri]; len(l) > 0 {
		return l[len(l)-1]
	}
	return nil
}

// Call sends a request and waits for its response; the result is decoded
// into result when it is not nil. It returns the time from just before the
// request was written to the moment its response was read.
func (c *Client) Call(ctx context.Context, method string, params, result any) (time.Duration, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return 0, err
	}
	c.mu.Lock()
	if c.done {
		c.mu.Unlock()
		return 0, fmt.Errorf("%w: %v", ErrServerEnded, c.readErr)
	}
	c.nextID++
	id := c.nextID
	ch := make(chan *reply, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	pid := protocol.IntID(id)
	body, err := protocol.EncodeMessage(&protocol.Request{ID: &pid, Method: method, Params: data})
	if err != nil {
		return 0, err
	}
	start := time.Now()
	if err := c.w.Write(body); err != nil {
		return 0, err
	}
	select {
	case r := <-ch:
		d := r.at.Sub(start)
		if r.err != nil {
			return d, fmt.Errorf("%s: %w", method, r.err)
		}
		if result != nil && len(r.result) > 0 && string(r.result) != "null" {
			if err := json.Unmarshal(r.result, result); err != nil {
				return d, fmt.Errorf("%s: %v", method, err)
			}
		}
		return d, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return 0, fmt.Errorf("%s: %w", method, ctx.Err())
	}
}

// Notify sends a notification.
func (c *Client) Notify(method string, params any) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	body, err := protocol.EncodeMessage(&protocol.Request{Method: method, Params: data})
	if err != nil {
		return err
	}
	return c.w.Write(body)
}

// Close closes the write side: the server sees the end of its input.
func (c *Client) Close() error { return c.cl.Close() }

// Done reports whether the server's output has ended.
func (c *Client) Done() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.done
}
