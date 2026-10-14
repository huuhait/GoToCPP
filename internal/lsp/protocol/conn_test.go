package protocol

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// funcs is a Handler made of two functions; a nil one does nothing.
type funcs struct {
	notify func(ctx context.Context, req *Request) error
	call   func(ctx context.Context, req *Request) Work
}

func (h funcs) Notify(ctx context.Context, req *Request) error {
	if h.notify == nil {
		return nil
	}
	return h.notify(ctx, req)
}

func (h funcs) Call(ctx context.Context, req *Request) Work {
	if h.call == nil {
		return nil
	}
	return h.call(ctx, req)
}

// afterAnswers is an input that, when it is first read, waits until the
// Conn has written the answers to the requests in flight, and then reads
// rest. Serve writes no answer after the end of its input, so a test
// input that wants its answers ends with one.
type afterAnswers struct {
	c    *Conn
	rest io.Reader
	once sync.Once
}

func (r *afterAnswers) Read(p []byte) (int, error) {
	r.once.Do(func() { r.c.Wait(context.Background()) })
	return r.rest.Read(p)
}

// newConn returns a Conn that reads in, then waits for the answers, then
// reads rest (afterAnswers).
func newConn(in, rest string, w io.Writer) *Conn {
	wait := &afterAnswers{rest: strings.NewReader(rest)}
	wait.c = NewConn(io.MultiReader(strings.NewReader(in), wait), w, nil)
	return wait.c
}

// serve runs a Conn over the input in, followed by rest after the answers,
// with the handler h and returns the bodies it wrote, sorted (the requests
// answer in any order), and the error of Serve.
func serve(t *testing.T, in, rest string, h Handler) ([]string, error) {
	t.Helper()
	var out bytes.Buffer
	err := newConn(in, rest, &out).Serve(context.Background(), h)
	bodies, rerr := readAll(NewReader(&out))
	if rerr != io.EOF {
		t.Fatalf("the output of Serve is broken: %v", rerr)
	}
	slices.Sort(bodies)
	return bodies, err
}

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

func TestServe(t *testing.T) {
	checkGoroutines(t)
	var calls []string // on the read loop: no lock
	h := funcs{
		notify: func(ctx context.Context, req *Request) error {
			calls = append(calls, req.Method)
			switch req.Method {
			case "stop":
				return ErrStop
			case "note":
				return errors.New("a notification's error is only logged")
			}
			return nil
		},
		call: func(ctx context.Context, req *Request) Work {
			calls = append(calls, req.Method)
			switch req.Method {
			case "echo":
				return Reply(req.Params, nil)
			case "fail":
				return Reply(nil, Errorf(InvalidParams, "wrong"))
			case "crash":
				return func(context.Context) (any, error) { return nil, errors.New("something broke") }
			case "chan":
				return Reply(make(chan int), nil) // can't be encoded
			}
			return nil
		},
	}
	in := frame(`{"jsonrpc":"2.0","id":1,"method":"echo","params":{"x":[1,2]}}`) +
		frame(`{"jsonrpc":"2.0","method":"note","params":{}}`) +
		frame(`{"jsonrpc":"2.0","id":"two","method":"fail"}`) +
		frame(`not json`) +
		frame(`{"jsonrpc":"2.0","id":3,"method":"crash"}`) +
		frame(`{"jsonrpc":"2.0","id":4,"result":{}}`) + // a response from the client: dropped
		frame(`{"jsonrpc":"2.0","id":5}`) +
		frame(`{"jsonrpc":"2.0","id":6,"method":"chan"}`) +
		frame(`{"jsonrpc":"2.0","id":7,"method":"nothing"}`) +
		frame(`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":99}}`) // the Conn's own
	rest := frame(`{"jsonrpc":"2.0","method":"stop"}`) +
		frame(`{"jsonrpc":"2.0","id":9,"method":"echo"}`) // after the stop: never read
	out, err := serve(t, in, rest, h)
	if err != nil {
		t.Errorf("Serve = %v, want nil after ErrStop", err)
	}
	want := []string{
		`{"jsonrpc":"2.0","id":"two","error":{"code":-32602,"message":"wrong"}}`,
		`{"jsonrpc":"2.0","id":1,"result":{"x":[1,2]}}`,
		`{"jsonrpc":"2.0","id":3,"error":{"code":-32603,"message":"something broke"}}`,
		`{"jsonrpc":"2.0","id":5,"error":{"code":-32600,"message":"invalid JSON-RPC 2.0 message: no method, result or error"}}`,
		`{"jsonrpc":"2.0","id":6,"error":{"code":-32603,"message":"json: unsupported type: chan int"}}`,
		`{"jsonrpc":"2.0","id":7,"result":null}`,
		`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"invalid character 'o' in literal null (expecting 'u')"}}`,
	}
	if strings.Join(out, "\n") != strings.Join(want, "\n") {
		t.Errorf("Serve wrote\n%s\nwant\n%s", strings.Join(out, "\n"), strings.Join(want, "\n"))
	}
	if got := strings.Join(calls, " "); got != "echo note fail crash chan nothing stop" {
		t.Errorf("the handler got %s", got)
	}
}

func TestServeEnd(t *testing.T) {
	checkGoroutines(t)
	nop := funcs{}
	// The end of the input.
	if _, err := serve(t, frame(`{"jsonrpc":"2.0","method":"n"}`), "", nop); err != io.EOF {
		t.Errorf("Serve at the end of the input = %v, want io.EOF", err)
	}
	// A bad header: Serve stops; what came before is answered.
	out, err := serve(t, frame(`{"jsonrpc":"2.0","id":1,"method":"m"}`), "Content-Length: x\r\n\r\n", nop)
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
	checkGoroutines(t)
	// The responses are written on the requests' goroutines; Serve returns
	// the first write error, not the end of the input.
	in := frame(`{"jsonrpc":"2.0","id":1,"method":"m"}`) + frame(`{"jsonrpc":"2.0","id":2,"method":"m"}`)
	err := newConn(in, "", errWriter{}).Serve(context.Background(), funcs{})
	if we := (*WriteError)(nil); !errors.As(err, &we) || we.Err.Error() != "broken pipe" {
		t.Errorf("Serve = %v, want the write error", err)
	}
	// A write error stops the read loop at the next message.
	inR, inW := io.Pipe()
	defer inW.Close()
	done := make(chan error, 1)
	go func() {
		err := NewConn(inR, errWriter{}, nil).Serve(context.Background(), funcs{})
		inR.Close() // the writes below fail now
		done <- err
	}()
	fmt.Fprint(inW, frame(`{"jsonrpc":"2.0","id":1,"method":"m"}`))
	// The response to 1 fails sometime; the next message stops Serve.
	for {
		if _, err := fmt.Fprint(inW, frame(`{"jsonrpc":"2.0","method":"n"}`)); err != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err, we := <-done, (*WriteError)(nil); !errors.As(err, &we) || err.Error() != "writing to the client: broken pipe" {
		t.Errorf("Serve = %v, want the write error", err)
	}
}

// A peer is the client's end of a Conn served over pipes.
type peer struct {
	t     *testing.T
	conn  *Conn
	in    *io.PipeWriter
	out   *Reader
	done  chan error // the error of Serve
	msgs  chan Message
	logs  *bytes.Buffer
	logMu *sync.Mutex
}

func startPeer(t *testing.T, h Handler) *peer {
	t.Helper()
	return startPeerCtx(t, context.Background(), h)
}

// startPeerSetup is startPeer with a setup of the Conn before Serve.
func startPeerSetup(t *testing.T, setup func(*Conn), h Handler) *peer {
	t.Helper()
	return startPeerFull(t, context.Background(), setup, h)
}

// startPeerCtx is startPeer with the context of Serve.
func startPeerCtx(t *testing.T, ctx context.Context, h Handler) *peer {
	t.Helper()
	return startPeerFull(t, ctx, nil, h)
}

func startPeerFull(t *testing.T, ctx context.Context, setup func(*Conn), h Handler) *peer {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	var logMu sync.Mutex
	logs := &bytes.Buffer{}
	logger := log.New(writerFunc(func(p []byte) (int, error) {
		logMu.Lock()
		defer logMu.Unlock()
		return logs.Write(p)
	}), "", 0)
	p := &peer{t: t, conn: NewConn(inR, outW, logger), in: inW, out: NewReader(outR),
		done: make(chan error, 1), msgs: make(chan Message, 100), logs: logs, logMu: &logMu}
	if setup != nil {
		setup(p.conn)
	}
	go func() {
		err := p.conn.Serve(ctx, h)
		inR.Close()
		outW.Close()
		p.done <- err
	}()
	go func() {
		defer close(p.msgs)
		for {
			body, err := p.out.Read()
			if err != nil {
				return
			}
			m, err := DecodeMessage(body)
			if err != nil {
				t.Errorf("the conn wrote %q: %v", body, err)
				return
			}
			p.msgs <- m
		}
	}()
	t.Cleanup(func() { inW.Close() })
	return p
}

func (p *peer) send(body string) {
	p.t.Helper()
	if _, err := io.WriteString(p.in, frame(body)); err != nil {
		p.t.Fatalf("writing to the conn: %v", err)
	}
}

// read returns the next response.
func (p *peer) read() *Response {
	p.t.Helper()
	select {
	case m, ok := <-p.msgs:
		if !ok {
			p.t.Fatal("the conn closed its output")
		}
		r, ok := m.(*Response)
		if !ok {
			p.t.Fatalf("got %+v, want a response", m)
		}
		return r
	case <-time.After(10 * time.Second):
		p.t.Fatal("no message in 10 s")
	}
	return nil
}

// end closes the input and returns the error of Serve, and the responses
// written after the input closed (to the requests that still ran).
func (p *peer) end() ([]*Response, error) {
	p.t.Helper()
	p.in.Close()
	select {
	case err := <-p.done:
		var late []*Response
		for m := range p.msgs {
			if r, ok := m.(*Response); ok {
				late = append(late, r)
			} else {
				p.t.Errorf("after the end, the conn wrote %+v", m)
			}
		}
		return late, err
	case <-time.After(10 * time.Second):
		p.t.Fatal("Serve didn't end in 10 s")
	}
	return nil, nil
}

// endQuiet is end when nothing must come after the input closed.
func (p *peer) endQuiet() error {
	p.t.Helper()
	late, err := p.end()
	for _, r := range late {
		p.t.Errorf("after the end, the conn wrote %+v", r)
	}
	return err
}

func (p *peer) log() string {
	p.logMu.Lock()
	defer p.logMu.Unlock()
	return p.logs.String()
}

// wantID checks that r is the response to the request id.
func wantID(t *testing.T, r *Response, id ID) {
	t.Helper()
	if r.ID == nil || *r.ID != id {
		t.Fatalf("got the response %+v, want the response to %v", r, id)
	}
}

func wantCode[C ErrorCodes | LSPErrorCodes](t *testing.T, r *Response, code C) {
	t.Helper()
	if r.Error == nil || r.Error.Code != int64(code) {
		t.Errorf("got %+v (error %+v), want the error code %d", r, r.Error, code)
	}
}

// blocking is a handler whose requests "block" wait until their context is
// done, report the cause on causes and return ctx.Err(); "free" answers at
// once with its params. The notification "stop" stops Serve.
func blocking(causes chan<- error) funcs {
	return funcs{notify: func(_ context.Context, req *Request) error {
		if req.Method == "stop" {
			return ErrStop
		}
		return nil
	}, call: func(_ context.Context, req *Request) Work {
		switch req.Method {
		case "block":
			return func(ctx context.Context) (any, error) {
				<-ctx.Done()
				causes <- context.Cause(ctx)
				return nil, ctx.Err()
			}
		case "free":
			return Reply(req.Params, nil)
		}
		return nil
	}}
}

func TestConcurrentRequests(t *testing.T) {
	checkGoroutines(t)
	causes := make(chan error, 10)
	p := startPeer(t, blocking(causes))
	// While 1 and 2 block, 3 is answered: the requests run concurrently.
	p.send(`{"jsonrpc":"2.0","id":1,"method":"block"}`)
	p.send(`{"jsonrpc":"2.0","id":"two","method":"block"}`)
	p.send(`{"jsonrpc":"2.0","id":3,"method":"free","params":[3]}`)
	r := p.read()
	wantID(t, r, IntID(3))
	if string(r.Result) != "[3]" {
		t.Errorf("result %s", r.Result)
	}
	if n := p.conn.InFlight(); n != 2 {
		t.Errorf("%d requests in flight, want 2", n)
	}
	// An ID in use gets an error and doesn't run.
	p.send(`{"jsonrpc":"2.0","id":1,"method":"free"}`)
	r = p.read()
	wantID(t, r, IntID(1))
	wantCode(t, r, InvalidRequest)
	// $/cancelRequest answers at once with RequestCancelled; the cause is
	// the context's.
	p.send(`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":"two"}}`)
	r = p.read()
	wantID(t, r, StringID("two"))
	wantCode(t, r, RequestCancelled)
	var re *ResponseError
	if err := <-causes; !errors.As(err, &re) || re.Code != int64(RequestCancelled) {
		t.Errorf("the cause is %v", err)
	}
	// Unknown IDs and bad params are ignored; a second cancel too.
	p.send(`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":"two"}}`)
	p.send(`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":42}}`)
	p.send(`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":null}}`)
	p.send(`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{}}`)
	// Conn.Cancel with another code.
	p.send(`{"jsonrpc":"2.0","id":4,"method":"free"}`)
	wantID(t, p.read(), IntID(4)) // the notifications before 4 are handled
	if !p.conn.Cancel(IntID(1), Errorf(ContentModified, "stale")) {
		t.Error("Cancel(1) = false")
	}
	if p.conn.Cancel(IntID(1), Errorf(ContentModified, "stale")) {
		t.Error("the second Cancel(1) = true")
	}
	r = p.read()
	wantID(t, r, IntID(1))
	if r.Error == nil || r.Error.Code != int64(ContentModified) || r.Error.Message != "stale" {
		t.Errorf("got %+v", r.Error)
	}
	<-causes
	if n := p.conn.InFlight(); n != 0 {
		t.Errorf("%d requests in flight, want 0", n)
	}
	if err := p.endQuiet(); err != io.EOF {
		t.Errorf("Serve = %v", err)
	}
	if logs := p.log(); strings.Count(logs, "$/cancelRequest: bad params") != 2 || !strings.Contains(logs, "the request ID 1 is in use") {
		t.Errorf("the log:\n%s", logs)
	}
}

// TestServeEndInFlight: the requests that run at the end of the input are
// cancelled and get no answer, and Serve waits for their Work.
func TestServeEndInFlight(t *testing.T) {
	checkGoroutines(t)
	for _, end := range []string{"eof", "stop"} {
		causes := make(chan error, 10)
		p := startPeer(t, blocking(causes))
		for i := range 5 {
			p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"block"}`, i))
		}
		p.send(`{"jsonrpc":"2.0","id":9,"method":"free"}`)
		wantID(t, p.read(), IntID(9))
		want := io.EOF
		if end == "stop" {
			p.send(`{"jsonrpc":"2.0","method":"stop"}`)
			want = nil
		}
		late, err := p.end()
		if err != want {
			t.Errorf("%s: Serve = %v, want %v", end, err, want)
		}
		// Serve has returned, so every Work has returned; none is
		// answered.
		if len(causes) != 5 || len(late) != 0 {
			t.Fatalf("%s: %d of the 5 requests returned, %d answered (want 5 and 0)", end, len(causes), len(late))
		}
		for range 5 {
			if err := <-causes; err != errEnded {
				t.Errorf("%s: the cause is %v, want errEnded", end, err)
			}
		}
	}
}

// TestServeContext: the end of Serve's context cancels the requests in
// flight (RequestCancelled), and Serve returns at the next message.
func TestServeContext(t *testing.T) {
	checkGoroutines(t)
	causes := make(chan error, 10)
	ctx, cancel := context.WithCancel(context.Background())
	p := startPeerCtx(t, ctx, blocking(causes))
	p.send(`{"jsonrpc":"2.0","id":1,"method":"block"}`)
	p.send(`{"jsonrpc":"2.0","id":2,"method":"free"}`)
	wantID(t, p.read(), IntID(2)) // 1 runs now
	cancel()
	r := p.read()
	wantID(t, r, IntID(1))
	wantCode(t, r, RequestCancelled)
	if err := <-causes; err != context.Canceled {
		t.Errorf("the cause is %v", err)
	}
	p.send(`{"jsonrpc":"2.0","method":"n"}`)
	if err := p.endQuiet(); err != context.Canceled {
		t.Errorf("Serve = %v, want context.Canceled", err)
	}
}

func TestWaitAndCancelAll(t *testing.T) {
	checkGoroutines(t)
	release := make(chan struct{})
	p := startPeer(t, funcs{call: func(_ context.Context, req *Request) Work {
		return func(ctx context.Context) (any, error) {
			if req.Method == "slow" {
				select {
				case <-release:
				case <-ctx.Done():
				}
			}
			return req.Method, nil
		}
	}})
	p.send(`{"jsonrpc":"2.0","id":1,"method":"slow"}`)
	p.send(`{"jsonrpc":"2.0","id":2,"method":"slow"}`)
	p.send(`{"jsonrpc":"2.0","id":3,"method":"slow"}`)
	p.send(`{"jsonrpc":"2.0","id":4,"method":"fast"}`)
	wantID(t, p.read(), IntID(4))
	// Wait times out while they run.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	if p.conn.Wait(ctx, IntID(3)) {
		t.Error("Wait = true while 1 and 2 run")
	}
	cancel()
	// It returns when all but the exception have answered.
	waited := make(chan bool)
	go func() { waited <- p.conn.Wait(context.Background(), IntID(3)) }()
	close(release)
	got := map[ID]string{}
	for range 3 {
		r := p.read()
		got[*r.ID] = string(r.Result)
	}
	if !<-waited || len(got) != 3 {
		t.Errorf("Wait returned false, or %v", got)
	}
	// CancelAll spares the exception.
	release = make(chan struct{})
	p.send(`{"jsonrpc":"2.0","id":5,"method":"slow"}`)
	p.send(`{"jsonrpc":"2.0","id":6,"method":"slow"}`)
	p.send(`{"jsonrpc":"2.0","id":7,"method":"fast"}`)
	wantID(t, p.read(), IntID(7))
	p.conn.CancelAll(Errorf(RequestCancelled, "all"), IntID(6))
	r := p.read()
	wantID(t, r, IntID(5))
	wantCode(t, r, RequestCancelled)
	close(release)
	r = p.read()
	wantID(t, r, IntID(6))
	if string(r.Result) != `"slow"` {
		t.Errorf("6 answered %s", r.Result)
	}
	if err := p.endQuiet(); err != io.EOF {
		t.Errorf("Serve = %v", err)
	}
}

// TestCancelRace cancels requests while they answer (run it with -race):
// each gets exactly one response.
func TestCancelRace(t *testing.T) {
	checkGoroutines(t)
	p := startPeer(t, funcs{call: func(_ context.Context, req *Request) Work {
		return func(ctx context.Context) (any, error) { return 1, nil }
	}})
	const n = 300
	go func() {
		for i := range n {
			io.WriteString(p.in, frame(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"m"}`, i)))
			io.WriteString(p.in, frame(fmt.Sprintf(`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":%d}}`, i)))
		}
	}()
	seen := map[ID]bool{}
	for range n {
		r := p.read()
		if seen[*r.ID] {
			t.Fatalf("two responses to %v", *r.ID)
		}
		seen[*r.ID] = true
		if r.Error != nil && r.Error.Code != int64(RequestCancelled) {
			t.Errorf("response %+v", r.Error)
		}
	}
	if err := p.endQuiet(); err != io.EOF {
		t.Errorf("Serve = %v", err)
	}
}

// TestNotify sends notifications from other goroutines while Serve answers
// requests: every message comes out whole.
func TestNotify(t *testing.T) {
	checkGoroutines(t)
	var out bytes.Buffer
	var mu sync.Mutex
	w := writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return out.Write(p)
	})
	var in strings.Builder
	for i := range 100 {
		in.WriteString(frame(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"m"}`, i)))
	}
	// The input ends when the notifications are sent and the requests
	// answered.
	var wg sync.WaitGroup
	wait := &afterAnswers{rest: readerFunc(func([]byte) (int, error) { wg.Wait(); return 0, io.EOF })}
	c := NewConn(io.MultiReader(strings.NewReader(in.String()), wait), w, nil)
	wait.c = c
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
	err := c.Serve(context.Background(), funcs{call: func(context.Context, *Request) Work { return Reply(1, nil) }})
	wg.Wait()
	if err != io.EOF {
		t.Fatal(err)
	}
	// After the end, nothing is written.
	if err := c.Notify("n", nil); err != ErrClosed {
		t.Errorf("Notify after Serve = %v, want ErrClosed", err)
	}
	bodies, err := readAll(NewReader(&out))
	if err != io.EOF || len(bodies) != 200 {
		t.Fatalf("read %d messages (%v), want 200", len(bodies), err)
	}
	counts := make(map[string]int)
	for _, b := range bodies {
		if strings.Contains(b, `"result":1`) {
			b = "result"
		}
		counts[b]++
	}
	if counts["result"] != 100 ||
		counts[`{"jsonrpc":"2.0","method":"window/logMessage","params":{"type":3,"message":"hi"}}`] != 100 {
		t.Errorf("wrong messages: %v", counts)
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

// TestOnEnd: the OnEnd function runs when the read loop ends, before the
// running requests are cancelled; after the end, Notify writes nothing.
func TestOnEnd(t *testing.T) {
	checkGoroutines(t)
	var ended atomic.Bool
	results := make(chan string, 1)
	var p *peer
	p = startPeerSetup(t, func(c *Conn) { c.OnEnd(func() { ended.Store(true) }) }, funcs{call: func(context.Context, *Request) Work {
		return func(ctx context.Context) (any, error) {
			<-ctx.Done()
			results <- fmt.Sprintf("ended %v, notify %v", ended.Load(), p.conn.Notify("late", nil))
			return nil, ctx.Err()
		}
	}})
	p.send(`{"jsonrpc":"2.0","id":1,"method":"m"}`)
	waitInFlight(t, p.conn, 1)
	if err := p.endQuiet(); err != io.EOF {
		t.Errorf("Serve = %v", err)
	}
	if got := <-results; got != "ended true, notify "+ErrClosed.Error() {
		t.Errorf("the Work saw %s", got)
	}
}

// waitInFlight waits until c has n requests in flight.
func waitInFlight(t *testing.T, c *Conn, n int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for c.InFlight() != n {
		if time.Now().After(deadline) {
			t.Fatalf("%d requests in flight, want %d", c.InFlight(), n)
		}
		time.Sleep(100 * time.Microsecond)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
