package protocol

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

// serve runs a Conn over the input in with the handler h and returns the
// bodies it wrote and the error of Serve.
func serve(t *testing.T, in string, h Handler) ([]string, error) {
	t.Helper()
	var out bytes.Buffer
	err := NewConn(strings.NewReader(in), &out, nil).Serve(context.Background(), h)
	bodies, rerr := readAll(NewReader(&out))
	if rerr != io.EOF {
		t.Fatalf("the output of Serve is broken: %v", rerr)
	}
	return bodies, err
}

func TestServe(t *testing.T) {
	var calls []string
	h := HandlerFunc(func(ctx context.Context, req *Request) (any, error) {
		calls = append(calls, req.Method)
		switch req.Method {
		case "echo":
			return req.Params, nil
		case "fail":
			return nil, Errorf(InvalidParams, "wrong")
		case "crash":
			return nil, errors.New("something broke")
		case "chan":
			return make(chan int), nil // can't be encoded
		case "stop":
			return "bye", ErrStop
		case "note":
			return "dropped", errors.New("a notification's error is only logged")
		}
		return nil, nil
	})
	in := frame(`{"jsonrpc":"2.0","id":1,"method":"echo","params":{"x":[1,2]}}`) +
		frame(`{"jsonrpc":"2.0","method":"note","params":{}}`) +
		frame(`{"jsonrpc":"2.0","id":"two","method":"fail"}`) +
		frame(`not json`) +
		frame(`{"jsonrpc":"2.0","id":3,"method":"crash"}`) +
		frame(`{"jsonrpc":"2.0","id":4,"result":{}}`) + // a response from the client: dropped
		frame(`{"jsonrpc":"2.0","id":5}`) +
		frame(`{"jsonrpc":"2.0","id":6,"method":"chan"}`) +
		frame(`{"jsonrpc":"2.0","id":7,"method":"nothing"}`) +
		frame(`{"jsonrpc":"2.0","id":8,"method":"stop"}`) +
		frame(`{"jsonrpc":"2.0","id":9,"method":"echo"}`) // after the stop: never read
	out, err := serve(t, in, h)
	if err != nil {
		t.Errorf("Serve = %v, want nil after ErrStop", err)
	}
	want := []string{
		`{"jsonrpc":"2.0","id":1,"result":{"x":[1,2]}}`,
		`{"jsonrpc":"2.0","id":"two","error":{"code":-32602,"message":"wrong"}}`,
		`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"invalid character 'o' in literal null (expecting 'u')"}}`,
		`{"jsonrpc":"2.0","id":3,"error":{"code":-32603,"message":"something broke"}}`,
		`{"jsonrpc":"2.0","id":5,"error":{"code":-32600,"message":"invalid JSON-RPC 2.0 message: no method, result or error"}}`,
		`{"jsonrpc":"2.0","id":6,"error":{"code":-32603,"message":"json: unsupported type: chan int"}}`,
		`{"jsonrpc":"2.0","id":7,"result":null}`,
		`{"jsonrpc":"2.0","id":8,"result":"bye"}`,
	}
	if strings.Join(out, "\n") != strings.Join(want, "\n") {
		t.Errorf("Serve wrote\n%s\nwant\n%s", strings.Join(out, "\n"), strings.Join(want, "\n"))
	}
	if got := strings.Join(calls, " "); got != "echo note fail crash chan nothing stop" {
		t.Errorf("the handler got %s", got)
	}
}

func TestServeEnd(t *testing.T) {
	nop := HandlerFunc(func(context.Context, *Request) (any, error) { return nil, nil })
	// The end of the input.
	if _, err := serve(t, frame(`{"jsonrpc":"2.0","method":"n"}`), nop); err != io.EOF {
		t.Errorf("Serve at the end of the input = %v, want io.EOF", err)
	}
	// A bad header: Serve stops; what came before is answered.
	out, err := serve(t, frame(`{"jsonrpc":"2.0","id":1,"method":"m"}`)+"Content-Length: x\r\n\r\n", nop)
	var he *HeaderError
	if !errors.As(err, &he) || len(out) != 1 {
		t.Errorf("Serve with a bad header = %v and %d responses, want a HeaderError after 1", err, len(out))
	}
	// A cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = NewConn(strings.NewReader(""), io.Discard, nil).Serve(ctx, nop)
	if err != context.Canceled {
		t.Errorf("Serve with a cancelled context = %v", err)
	}
}

// errWriter fails every write.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestServeWriteError(t *testing.T) {
	nop := HandlerFunc(func(context.Context, *Request) (any, error) { return nil, nil })
	in := frame(`{"jsonrpc":"2.0","id":1,"method":"m"}`) + frame(`{"jsonrpc":"2.0","id":2,"method":"m"}`)
	err := NewConn(strings.NewReader(in), errWriter{}, nil).Serve(context.Background(), nop)
	if err == nil || err.Error() != "broken pipe" {
		t.Errorf("Serve = %v, want the write error", err)
	}
}

// TestNotify sends notifications from other goroutines while Serve answers
// requests: every message comes out whole.
func TestNotify(t *testing.T) {
	var out bytes.Buffer
	var mu sync.Mutex
	w := writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return out.Write(p)
	})
	var in strings.Builder
	for range 100 {
		in.WriteString(frame(`{"jsonrpc":"2.0","id":1,"method":"m"}`))
	}
	c := NewConn(strings.NewReader(in.String()), w, nil)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 25 {
				if err := c.Notify("window/logMessage", &LogMessageParams{Type: Info, Message: "hi"}); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	err := c.Serve(context.Background(), HandlerFunc(func(context.Context, *Request) (any, error) { return 1, nil }))
	wg.Wait()
	if err != io.EOF {
		t.Fatal(err)
	}
	if err := c.Notify("n", nil); err != nil {
		t.Fatal(err)
	}
	bodies, err := readAll(NewReader(&out))
	if err != io.EOF || len(bodies) != 201 {
		t.Fatalf("read %d messages (%v), want 201", len(bodies), err)
	}
	counts := make(map[string]int)
	for _, b := range bodies {
		counts[b]++
	}
	if counts[`{"jsonrpc":"2.0","id":1,"result":1}`] != 100 ||
		counts[`{"jsonrpc":"2.0","method":"window/logMessage","params":{"type":3,"message":"hi"}}`] != 100 ||
		counts[`{"jsonrpc":"2.0","method":"n"}`] != 1 {
		t.Errorf("wrong messages: %v", counts)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
