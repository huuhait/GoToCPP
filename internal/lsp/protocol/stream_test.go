package protocol

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
)

// frame returns body with the LSP framing.
func frame(body string) string {
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

// readAll reads messages from r until the end.
func readAll(r *Reader) ([]string, error) {
	var bodies []string
	for {
		body, err := r.Read()
		if err != nil {
			return bodies, err
		}
		bodies = append(bodies, string(body))
	}
}

// The messages of a stream that uses every allowed form of the framing.
var streamBodies = []string{
	`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
	`{"jsonrpc":"2.0","method":"initialized","params":{}}`,
	``, // an empty body is framed fine (it is not JSON, a later problem)
	`{"jsonrpc":"2.0","method":"x","params":{"text":"emoji 😀 é 中 \r\n"}}`,
	strings.Repeat("x", 70000), // longer than the reader's buffer
	// Over 1 MiB: read as it comes (Reader.Read), and the message after
	// it must still be read right.
	`{"text":"` + strings.Repeat("y😀\n", 400000) + `"}`,
	`{"after":"the big one"}`,
}

func stream() string {
	var b strings.Builder
	b.WriteString(frame(streamBodies[0]))
	// Content-Type and the name in other cases.
	fmt.Fprintf(&b, "content-length: %d\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n%s",
		len(streamBodies[1]), streamBodies[1])
	// Content-Type first, a space before the value, and a lone \n.
	fmt.Fprintf(&b, "Content-Type: x\nContent-Length:   %d  \n\n%s", len(streamBodies[2]), streamBodies[2])
	// A blank line between two messages.
	b.WriteString("\r\n")
	b.WriteString(frame(streamBodies[3]))
	b.WriteString(frame(streamBodies[4]))
	b.WriteString(frame(streamBodies[5]))
	b.WriteString(frame(streamBodies[6]))
	return b.String()
}

func TestReadMessages(t *testing.T) {
	readers := map[string]func(io.Reader) io.Reader{
		"whole":    func(r io.Reader) io.Reader { return r },
		"one byte": iotest.OneByteReader,
		"half":     iotest.HalfReader,
		"random":   func(r io.Reader) io.Reader { return &chunkReader{r: r, rnd: rand.New(rand.NewSource(1))} },
	}
	for name, wrap := range readers {
		t.Run(name, func(t *testing.T) {
			got, err := readAll(NewReader(wrap(strings.NewReader(stream()))))
			if err != io.EOF {
				t.Errorf("the error at the end is %v, want io.EOF", err)
			}
			if len(got) != len(streamBodies) {
				t.Fatalf("read %d messages, want %d: %q", len(got), len(streamBodies), got)
			}
			for i := range got {
				if got[i] != streamBodies[i] {
					t.Errorf("message %d is %.80q, want %.80q", i, got[i], streamBodies[i])
				}
			}
		})
	}
}

// A chunkReader returns the data of r in pieces of random sizes.
type chunkReader struct {
	r   io.Reader
	rnd *rand.Rand
}

func (c *chunkReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1+c.rnd.Intn(min(len(p), 17))]
	}
	return c.r.Read(p)
}

func TestReadEmpty(t *testing.T) {
	for _, in := range []string{"", "\r\n", "\n\r\n"} {
		if _, err := NewReader(strings.NewReader(in)).Read(); err != io.EOF {
			t.Errorf("Read(%q) = %v, want io.EOF", in, err)
		}
	}
}

func TestReadBadHeaders(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string // the HeaderError's message; "" for io.ErrUnexpectedEOF
	}{
		{"Content-Length 5\r\n\r\nhello", `"Content-Length 5" is not a header line`},
		{"Content-Type: x\r\n\r\nhello", "no Content-Length header"},
		{"Content-Length: five\r\n\r\nhello", `bad Content-Length "five"`},
		{"Content-Length: -1\r\n\r\n", `bad Content-Length "-1"`},
		{"Content-Length: 0x10\r\n\r\n", `bad Content-Length "0x10"`},
		{"Content-Length: 99999999999999999999\r\n\r\n", `bad Content-Length "99999999999999999999"`},
		{"Content-Length: 268435457\r\n\r\n", "Content-Length 268435457 is more than 268435456"},
		{"Content-Length: 1\r\nContent-Length: 1\r\n\r\nx", "two Content-Length headers"},
		{"X-Long: " + strings.Repeat("a", 70<<10) + "\r\n", "a header line is too long"},
		{"Content-Length: 5\r\n", ""},                   // the end in the header part
		{"Content-Length: 5", ""},                       // the end in a header line
		{"Content-Length: 5\r\n\r\nhel", ""},            // the end in the body
		{"Content-Length: 2000000\r\n\r\nshort", ""},    // the end in a big body
		{frame("{}") + "Content-Length: 3\r\n\r\n", ""}, // the second message
	} {
		r := NewReader(strings.NewReader(test.in))
		var err error
		for err == nil {
			_, err = r.Read()
		}
		var he *HeaderError
		switch {
		case test.want == "":
			if err != io.ErrUnexpectedEOF {
				t.Errorf("Read(%.60q) = %v, want io.ErrUnexpectedEOF", test.in, err)
			}
		case !errors.As(err, &he) || he.Msg != test.want:
			t.Errorf("Read(%.60q) = %v, want the header error %q", test.in, err, test.want)
		}
	}
}

func TestWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	for _, body := range []string{`{"a":1}`, "", "é😀"} {
		if err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	want := frame(`{"a":1}`) + frame("") + frame("é😀")
	if buf.String() != want {
		t.Errorf("wrote %q, want %q", buf.String(), want)
	}
}

// writeCounter counts the Write calls.
type writeCounter struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	calls int
}

func (w *writeCounter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls++
	return w.buf.Write(p)
}

// TestWriterConcurrent writes from many goroutines: every message must be
// whole, written in one Write call.
func TestWriterConcurrent(t *testing.T) {
	out := &writeCounter{}
	w := NewWriter(out)
	const goroutines, each = 8, 50
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range each {
				body := fmt.Sprintf(`{"g":%d,"i":%d,"pad":%q}`, g, i, strings.Repeat("p", i*37))
				if err := w.Write([]byte(body)); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	if out.calls != goroutines*each {
		t.Errorf("%d Write calls, want %d", out.calls, goroutines*each)
	}
	got, err := readAll(NewReader(&out.buf))
	if err != io.EOF || len(got) != goroutines*each {
		t.Fatalf("read back %d messages (error %v), want %d", len(got), err, goroutines*each)
	}
	seen := make(map[string]bool)
	for _, body := range got {
		var g, i int
		if _, err := fmt.Sscanf(body, `{"g":%d,"i":%d,`, &g, &i); err != nil {
			t.Fatalf("a message is broken: %.80q", body)
		}
		want := fmt.Sprintf(`{"g":%d,"i":%d,"pad":%q}`, g, i, strings.Repeat("p", i*37))
		if body != want || seen[body] {
			t.Fatalf("a message is broken or repeated: %.80q", body)
		}
		seen[body] = true
	}
}

// TestWriteParts: the frame of a body given in parts is the frame of the whole body, for one part,
// several, an empty one and none.
func TestWriteParts(t *testing.T) {
	for name, parts := range map[string][][]byte{
		"one part":    {[]byte(`{"a":1}`)},
		"three parts": {[]byte(`{"a":`), []byte(`[1,2]`), []byte(`}`)},
		"empty part":  {[]byte(`{`), nil, []byte(`}`)},
		"no part":     nil,
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			if err := NewWriter(&out).WriteParts(parts...); err != nil {
				t.Fatal(err)
			}
			want := frame(string(bytes.Join(parts, nil)))
			if out.String() != want {
				t.Fatalf("frame %q, want %q", out.String(), want)
			}
		})
	}
}
