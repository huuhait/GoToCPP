// gocpp's own code (not copied from gopls, see COPIED.md): the framing of
// the LSP base protocol.

package protocol

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// MaxMessageSize is the largest message body a Reader accepts: 256 MiB.
// A larger Content-Length is a header error.
const MaxMessageSize = 256 << 20

// A HeaderError is a bad message header. The stream can't be trusted after
// it, because the end of the bad message is not known: the reader stops.
type HeaderError struct {
	Msg string
}

func (e *HeaderError) Error() string { return "bad message header: " + e.Msg }

// A Reader reads the messages of the LSP base protocol: a header part of
// "Name: value\r\n" lines, which must have Content-Length, an empty line,
// and a body of Content-Length bytes (LSP 3.17 "Base Protocol"). Other
// headers, such as Content-Type, are ignored.
type Reader struct {
	r *bufio.Reader
}

// NewReader returns a Reader that reads from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{r: bufio.NewReaderSize(r, 64<<10)}
}

// Read returns the body of the next message. At the end of the input before
// a message starts, it returns io.EOF; in the middle of a message,
// io.ErrUnexpectedEOF. A bad header is a *HeaderError.
func (r *Reader) Read() ([]byte, error) {
	length := -1
	header := false // a header line has been read
	for {
		line, err := r.r.ReadSlice('\n')
		switch {
		case err == io.EOF && !header && len(line) == 0:
			return nil, io.EOF
		case err == io.EOF:
			return nil, io.ErrUnexpectedEOF
		case errors.Is(err, bufio.ErrBufferFull):
			return nil, &HeaderError{"a header line is too long"}
		case err != nil:
			return nil, err
		}
		// The spec ends a header line with \r\n; a lone \n is accepted.
		text := strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r")
		if text == "" {
			if !header {
				continue // a blank line between two messages: skip it
			}
			break // the end of the header part
		}
		header = true
		name, value, ok := strings.Cut(text, ":")
		if !ok {
			return nil, &HeaderError{fmt.Sprintf("%q is not a header line", text)}
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			if length >= 0 {
				return nil, &HeaderError{"two Content-Length headers"}
			}
			n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil || n < 0 {
				return nil, &HeaderError{fmt.Sprintf("bad Content-Length %q", strings.TrimSpace(value))}
			}
			if n > MaxMessageSize {
				return nil, &HeaderError{fmt.Sprintf("Content-Length %d is more than %d", n, MaxMessageSize)}
			}
			length = int(n)
		}
	}
	if length < 0 {
		return nil, &HeaderError{"no Content-Length header"}
	}
	// A small body is read in one piece. A big one grows with the bytes
	// that really come, so a wrong Content-Length can't allocate much.
	if length <= 1<<20 {
		body := make([]byte, length)
		if _, err := io.ReadFull(r.r, body); err != nil {
			return nil, unexpected(err)
		}
		return body, nil
	}
	var buf bytes.Buffer
	if n, err := buf.ReadFrom(io.LimitReader(r.r, int64(length))); err != nil {
		return nil, err
	} else if n < int64(length) {
		return nil, io.ErrUnexpectedEOF
	}
	return buf.Bytes(), nil
}

// unexpected turns io.EOF into io.ErrUnexpectedEOF.
func unexpected(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// A Writer writes messages with the framing of the LSP base protocol. It is
// safe for concurrent use: each message is written whole, in one Write call.
type Writer struct {
	mu  sync.Mutex
	w   io.Writer
	buf []byte
}

// NewWriter returns a Writer that writes to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// Write writes one message with the body.
func (w *Writer) Write(body []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf[:0], "Content-Length: "...)
	w.buf = strconv.AppendInt(w.buf, int64(len(body)), 10)
	w.buf = append(w.buf, "\r\n\r\n"...)
	w.buf = append(w.buf, body...)
	_, err := w.w.Write(w.buf)
	if cap(w.buf) > 1<<20 {
		w.buf = nil // don't keep a big buffer after a big message
	}
	return err
}
