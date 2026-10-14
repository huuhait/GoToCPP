// gocpp's own code (not copied from gopls, see COPIED.md): a JSON-RPC 2.0
// connection that dispatches requests and notifications to a Handler.

package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
)

// A Handler handles the requests and notifications of a Conn.
type Handler interface {
	// Handle handles one request or notification. For a request, the
	// result, or the error, is the response; a *ResponseError chooses the
	// error code, any other error is an InternalError. For a notification,
	// the result is dropped and an error is only logged.
	//
	// ErrStop ends Serve: after a request's response (with the result) is
	// written, or after the notification.
	Handle(ctx context.Context, req *Request) (result any, err error)
}

// HandlerFunc is a function that is a Handler.
type HandlerFunc func(ctx context.Context, req *Request) (any, error)

// Handle calls f.
func (f HandlerFunc) Handle(ctx context.Context, req *Request) (any, error) { return f(ctx, req) }

// ErrStop is the error a Handler returns to end Serve (the "exit"
// notification).
var ErrStop = errors.New("protocol: stop serving")

// A Conn is a JSON-RPC 2.0 connection over a byte stream with the LSP
// framing (stdin and stdout for "gocpp lsp").
type Conn struct {
	r   *Reader
	w   *Writer
	log *log.Logger
}

// NewConn returns a connection that reads messages from r and writes them
// to w. It logs problems with the messages to logger, which may be nil.
func NewConn(r io.Reader, w io.Writer, logger *log.Logger) *Conn {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Conn{r: NewReader(r), w: NewWriter(w), log: logger}
}

// Serve reads the messages and hands each request and notification to h,
// one at a time and in order, until h returns ErrStop (then Serve returns
// nil), the input ends (io.EOF), or the stream breaks (a *HeaderError, a
// read or write error). ctx is the context of every Handle call. Serve
// checks it only between two messages: a Serve that waits for input doesn't
// see ctx end before the next message (or the end of the input) comes.
//
// A body that is not JSON gets a ParseError response, and JSON that is not a
// JSON-RPC 2.0 message gets an InvalidRequest response; Serve goes on after
// both. A response from the client is dropped: the server sends no requests
// yet.
//
// Since one message is handled at a time, a $/cancelRequest always comes
// after the response to its request. Step 11.2 keeps this read loop in
// order but runs requests in goroutines, with a context per request ID that
// $/cancelRequest cancels (the answer is then RequestCancelled, or
// ContentModified when a change made the result stale).
func (c *Conn) Serve(ctx context.Context, h Handler) error {
	for {
		if err := ctx.Err(); err != nil {
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
				return err
			}
			continue
		}
		var req *Request
		switch m := msg.(type) {
		case *Response:
			c.log.Printf("dropped a response to request %v: the server sends no requests", m.ID)
			continue
		case *Request:
			req = m
		}
		result, herr := h.Handle(ctx, req)
		stop := errors.Is(herr, ErrStop)
		if stop {
			herr = nil
		}
		if req.IsNotification() {
			if herr != nil {
				c.log.Printf("%s: %v", req.Method, herr)
			}
		} else if err := c.reply(req, result, herr); err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
}

// reply writes the response to req.
func (c *Conn) reply(req *Request, result any, err error) error {
	resp := &Response{ID: req.ID}
	if err == nil {
		resp.Result, err = json.Marshal(result)
		if err != nil {
			c.log.Printf("%s: cannot encode the result: %v", req.Method, err)
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
	return c.write(resp)
}

// Notify sends a notification to the client. It is safe for concurrent use.
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

func (c *Conn) write(m Message) error {
	data, err := EncodeMessage(m)
	if err != nil {
		return err
	}
	return c.w.Write(data)
}
