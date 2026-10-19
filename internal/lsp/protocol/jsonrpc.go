// gocpp's own code (not copied from gopls, see COPIED.md): the JSON-RPC 2.0
// messages of the LSP base protocol.

package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// An ID is the id of a request: an integer or a string (JSON-RPC 2.0 §4).
// The zero ID is the integer 0. IDs are comparable, so they can be map keys.
type ID struct {
	name     string
	number   int64
	isString bool
}

// IntID returns the integer ID n.
func IntID(n int64) ID { return ID{number: n} }

// StringID returns the string ID s.
func StringID(s string) ID { return ID{name: s, isString: true} }

// String returns the ID as it is written in JSON: 7 or "seven".
func (id ID) String() string {
	if id.isString {
		return strconv.Quote(id.name)
	}
	return strconv.FormatInt(id.number, 10)
}

// MarshalJSON writes the ID as a JSON number or string.
func (id ID) MarshalJSON() ([]byte, error) {
	if id.isString {
		return json.Marshal(id.name)
	}
	return strconv.AppendInt(nil, id.number, 10), nil
}

// UnmarshalJSON reads an integer or a string. Anything else (null, 1.5, an
// object) is an error.
func (id *ID) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*id = StringID(s)
		return nil
	}
	n, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return fmt.Errorf("id %s is not an integer or a string", data)
	}
	*id = IntID(n)
	return nil
}

// A ResponseError is the error of a response: JSON-RPC 2.0's error object,
// the LSP's ResponseError. It is also a Go error, so a method handler can
// return one to choose the code of its error response.
type ResponseError struct {
	Code    int64           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("%s (code %d)", e.Message, e.Code)
}

// Errorf returns a ResponseError with the code and the formatted message.
// The codes are those of tsprotocol.go: ErrorCodes (the JSON-RPC 2.0 codes:
// ParseError, MethodNotFound, ServerNotInitialized, ...) and LSPErrorCodes
// (RequestCancelled, ContentModified, ...).
func Errorf[C ErrorCodes | LSPErrorCodes](code C, format string, args ...any) *ResponseError {
	return &ResponseError{Code: int64(code), Message: fmt.Sprintf(format, args...)}
}

// A Message is one JSON-RPC 2.0 message: a *Request (a request or a
// notification) or a *Response.
type Message interface {
	isMessage()
}

// A Request is a request or, without an ID, a notification.
type Request struct {
	ID     *ID // nil for a notification
	Method string
	Params json.RawMessage // nil when the message has no params
}

// IsNotification reports whether the request is a notification: it has no
// ID, and it gets no response.
func (r *Request) IsNotification() bool { return r.ID == nil }

// A Response answers a request. Exactly one of Result and Error is set; a
// result of null is the JSON null, not nil.
type Response struct {
	ID     *ID // nil only in an error response to a message whose ID can't be read
	Result json.RawMessage
	Error  *ResponseError

	// trusted is set by the connection when Result is the output of
	// encoding/json or of a type of this package that encodes itself: valid
	// JSON, already compact, so EncodeMessage need not scan it again.
	trusted bool
}

func (*Request) isMessage()  {}
func (*Response) isMessage() {}

// wire is a message as it is on the wire. A RawMessage field is nil when the
// member is missing and "null" when it is null.
type wire struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  *string         `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

// errInvalid is the error a decode error wraps when the body is JSON but not
// a JSON-RPC 2.0 message (CodeInvalidRequest); other decode errors mean the
// body is not JSON (ParseError).
var errInvalid = errors.New("invalid JSON-RPC 2.0 message")

// A DecodeError is an error of DecodeMessage. Code is ParseError for a body
// that is not JSON and InvalidRequest for JSON that is not a JSON-RPC 2.0
// message. ID is the message's ID when it could be read, for the error
// response.
type DecodeError struct {
	Code ErrorCodes
	ID   *ID
	Err  error
}

func (e *DecodeError) Error() string { return e.Err.Error() }
func (e *DecodeError) Unwrap() error { return e.Err }

// DecodeMessage decodes one JSON-RPC 2.0 message. It doesn't decode batches
// (a JSON array): LSP doesn't use them. An error is a *DecodeError.
func DecodeMessage(data []byte) (Message, error) {
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		code := ParseError
		if json.Valid(data) {
			// Valid JSON of the wrong shape: an array (a batch), a
			// string, or a member of the wrong type.
			code = InvalidRequest
			if t := bytes.TrimLeft(data, " \t\r\n"); len(t) > 0 && t[0] == '[' {
				err = fmt.Errorf("%w: batches are not supported", errInvalid)
			} else {
				err = fmt.Errorf("%w: %v", errInvalid, err)
			}
		}
		return nil, &DecodeError{Code: code, Err: err}
	}
	var id *ID
	nullID := string(w.ID) == "null" // allowed only in an error response
	if w.ID != nil && !nullID {
		var v ID
		if err := v.UnmarshalJSON(w.ID); err != nil {
			return nil, &DecodeError{Code: InvalidRequest, Err: fmt.Errorf("%w: %v", errInvalid, err)}
		}
		id = &v
	}
	invalid := func(msg string) error {
		return &DecodeError{Code: InvalidRequest, ID: id, Err: fmt.Errorf("%w: %s", errInvalid, msg)}
	}
	if w.JSONRPC != "2.0" {
		return nil, invalid(fmt.Sprintf("jsonrpc is %q, want \"2.0\"", w.JSONRPC))
	}
	if w.Method != nil {
		if w.Result != nil || w.Error != nil {
			return nil, invalid("a request with a result or an error")
		}
		if nullID {
			return nil, invalid("a request with the id null")
		}
		params := w.Params
		switch {
		case string(params) == "null":
			// Not allowed by JSON-RPC 2.0, but some clients send it for a
			// request without params (shutdown); it means no params.
			params = nil
		case params != nil && params[0] != '{' && params[0] != '[':
			return nil, invalid("params must be an object or an array")
		}
		return &Request{ID: id, Method: *w.Method, Params: params}, nil
	}
	switch {
	case w.Result != nil && w.Error != nil:
		return nil, invalid("a response with both a result and an error")
	case w.Result == nil && w.Error == nil:
		return nil, invalid("no method, result or error")
	}
	if w.ID == nil || nullID && w.Error == nil {
		return nil, invalid("a response without an id")
	}
	return &Response{ID: id, Result: w.Result, Error: w.Error}, nil
}

// EncodeMessage encodes a message as JSON. A Response with neither a result
// nor an error gets the result null.
func EncodeMessage(m Message) ([]byte, error) {
	w := wire{JSONRPC: "2.0"}
	switch m := m.(type) {
	case *Request:
		w.Method = &m.Method
		w.Params = m.Params
		if m.ID != nil {
			w.ID, _ = m.ID.MarshalJSON()
		}
	case *Response:
		if m.trusted && m.Error == nil && len(m.Result) > 0 {
			// The result of a big answer (thousands of references) is not
			// scanned and copied again by json.Marshal: the message is
			// put together, in the order and form that wire gives.
			id := []byte("null")
			if m.ID != nil {
				id, _ = m.ID.MarshalJSON()
			}
			b := make([]byte, 0, len(m.Result)+len(id)+48)
			b = append(b, `{"jsonrpc":"2.0","id":`...)
			b = append(b, id...)
			b = append(b, `,"result":`...)
			b = append(b, m.Result...)
			return append(b, '}'), nil
		}
		w.ID = json.RawMessage("null") // JSON-RPC 2.0: null when the id is unknown
		if m.ID != nil {
			w.ID, _ = m.ID.MarshalJSON()
		}
		w.Error = m.Error
		if m.Error == nil {
			w.Result = m.Result
			if w.Result == nil {
				w.Result = json.RawMessage("null")
			}
		}
	default:
		return nil, fmt.Errorf("protocol: unknown message type %T", m)
	}
	return json.Marshal(w)
}
