// phase: 09
// Go 1.25's io/multi_test.go (MultiReader, MultiWriter: partial reads, errors, nested readers, flattening, the call stack depth of nested readers; the sha1 test compares the bytes written, the allocation count and the finalizer (runtime.AddCleanup) tests are dropped; the two call depth checks only fail when the depth is greater than Go's, because inlining at -O2 makes runtime.Callers see fewer frames) through a small harness; expected output from real Go.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync/atomic"
)


func TestMultiReader(t *testT) {
	var mr io.Reader
	var buf []byte
	nread := 0
	withFooBar := func(tests func()) {
		r1 := strings.NewReader("foo ")
		r2 := strings.NewReader("")
		r3 := strings.NewReader("bar")
		mr = io.MultiReader(r1, r2, r3)
		buf = make([]byte, 20)
		tests()
	}
	expectRead := func(size int, expected string, eerr error) {
		nread++
		n, gerr := mr.Read(buf[0:size])
		atomic.AddInt64(&checks, 1)
		if n != len(expected) {
			t.Errorf("#%d, expected %d bytes; got %d",
				nread, len(expected), n)
		}
		got := string(buf[0:n])
		atomic.AddInt64(&checks, 1)
		if got != expected {
			t.Errorf("#%d, expected %q; got %q",
				nread, expected, got)
		}
		atomic.AddInt64(&checks, 1)
		if gerr != eerr {
			t.Errorf("#%d, expected error %v; got %v",
				nread, eerr, gerr)
		}
		buf = buf[n:]
	}
	withFooBar(func() {
		expectRead(2, "fo", nil)
		expectRead(5, "o ", nil)
		expectRead(5, "bar", nil)
		expectRead(5, "", io.EOF)
	})
	withFooBar(func() {
		expectRead(4, "foo ", nil)
		expectRead(1, "b", nil)
		expectRead(3, "ar", nil)
		expectRead(1, "", io.EOF)
	})
	withFooBar(func() {
		expectRead(5, "foo ", nil)
	})
}

func TestMultiReaderAsWriterTo(t *testT) {
	mr := io.MultiReader(
		strings.NewReader("foo "),
		io.MultiReader( // Tickle the buffer reusing codepath
			strings.NewReader(""),
			strings.NewReader("bar"),
		),
	)
	mrAsWriterTo, ok := mr.(io.WriterTo)
	atomic.AddInt64(&checks, 1)
	if !ok {
		t.Fatalf("expected cast to WriterTo to succeed")
	}
	sink := &strings.Builder{}
	n, err := mrAsWriterTo.WriteTo(sink)
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Fatalf("expected no error; got %v", err)
	}
	atomic.AddInt64(&checks, 1)
	if n != 7 {
		t.Errorf("expected read 7 bytes; got %d", n)
	}
	atomic.AddInt64(&checks, 1)
	if result := sink.String(); result != "foo bar" {
		t.Errorf(`expected "foo bar"; got %q`, result)
	}
}

func TestMultiWriter(t *testT) {
	sink := new(bytes.Buffer)
	// Hide bytes.Buffer's WriteString method:
	testMultiWriter(t, struct {
		io.Writer
		fmt.Stringer
	}{sink, sink})
}

func TestMultiWriter_String(t *testT) {
	testMultiWriter(t, new(bytes.Buffer))
}


type writeStringChecker struct{ called bool }

func (c *writeStringChecker) WriteString(s string) (n int, err error) {
	c.called = true
	return len(s), nil
}

func (c *writeStringChecker) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func TestMultiWriter_StringCheckCall(t *testT) {
	var c writeStringChecker
	mw := io.MultiWriter(&c)
	io.WriteString(mw, "foo")
	atomic.AddInt64(&checks, 1)
	if !c.called {
		t.Error("did not see WriteString call to writeStringChecker")
	}
}

func testMultiWriter(t *testT, sink interface {
	io.Writer
	fmt.Stringer
}) {
	sha1 := new(bytes.Buffer)
	mw := io.MultiWriter(sha1, sink)

	sourceString := "My input text."
	source := strings.NewReader(sourceString)
	written, err := io.Copy(mw, source)

	atomic.AddInt64(&checks, 1)
	if written != int64(len(sourceString)) {
		t.Errorf("short write of %d, not %d", written, len(sourceString))
	}

	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	sha1hex := fmt.Sprintf("%x", sha1.Bytes())
	atomic.AddInt64(&checks, 1)
	if sha1hex != "4d7920696e70757420746578742e" {
		t.Error("incorrect sha1 value")
	}

	atomic.AddInt64(&checks, 1)
	if sink.String() != sourceString {
		t.Errorf("expected %q; got %q", sourceString, sink.String())
	}
}

// writerFunc is a Writer implemented by the underlying func.
type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) {
	return f(p)
}

// Test that MultiWriter properly flattens chained multiWriters.
func TestMultiWriterSingleChainFlatten(t *testT) {
	pc := make([]uintptr, 1000) // 1000 should fit the full stack
	n := runtime.Callers(0, pc)
	var myDepth = callDepth(pc[:n])
	var writeDepth int // will contain the depth from which writerFunc.Writer was called
	var w io.Writer = io.MultiWriter(writerFunc(func(p []byte) (int, error) {
		n := runtime.Callers(1, pc)
		writeDepth += callDepth(pc[:n])
		return 0, nil
	}))

	mw := w
	// chain a bunch of multiWriters
	for i := 0; i < 100; i++ {
		mw = io.MultiWriter(w)
	}

	mw = io.MultiWriter(w, mw, w, mw)
	mw.Write(nil) // don't care about errors, just want to check the call-depth for Write

	atomic.AddInt64(&checks, 1)
	if writeDepth > 4*(myDepth+2) { // 2 should be multiWriter.Write and writerFunc.Write
		t.Errorf("multiWriter did not flatten chained multiWriters: expected writeDepth %d, got %d",
			4*(myDepth+2), writeDepth)
	}
}

func TestMultiWriterError(t *testT) {
	f1 := writerFunc(func(p []byte) (int, error) {
		return len(p) / 2, io.ErrShortWrite
	})
	f2 := writerFunc(func(p []byte) (int, error) {
		t.Errorf("MultiWriter called f2.Write")
		return len(p), nil
	})
	w := io.MultiWriter(f1, f2)
	n, err := w.Write(make([]byte, 100))
	atomic.AddInt64(&checks, 1)
	if n != 50 || err != io.ErrShortWrite {
		t.Errorf("Write = %d, %v, want 50, ErrShortWrite", n, err)
	}
}

// Test that MultiReader copies the input slice and is insulated from future modification.
func TestMultiReaderCopy(t *testT) {
	slice := []io.Reader{strings.NewReader("hello world")}
	r := io.MultiReader(slice...)
	slice[0] = nil
	data, err := io.ReadAll(r)
	atomic.AddInt64(&checks, 1)
	if err != nil || string(data) != "hello world" {
		t.Errorf("ReadAll() = %q, %v, want %q, nil", data, err, "hello world")
	}
}

// Test that MultiWriter copies the input slice and is insulated from future modification.
func TestMultiWriterCopy(t *testT) {
	var buf strings.Builder
	slice := []io.Writer{&buf}
	w := io.MultiWriter(slice...)
	slice[0] = nil
	n, err := w.Write([]byte("hello world"))
	atomic.AddInt64(&checks, 1)
	if err != nil || n != 11 {
		t.Errorf("Write(`hello world`) = %d, %v, want 11, nil", n, err)
	}
	atomic.AddInt64(&checks, 1)
	if buf.String() != "hello world" {
		t.Errorf("buf.String() = %q, want %q", buf.String(), "hello world")
	}
}

// readerFunc is a Reader implemented by the underlying func.
type readerFunc func(p []byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) {
	return f(p)
}

// callDepth returns the logical call depth for the given PCs.
func callDepth(callers []uintptr) (depth int) {
	frames := runtime.CallersFrames(callers)
	more := true
	for more {
		_, more = frames.Next()
		depth++
	}
	return
}

// Test that MultiReader properly flattens chained multiReaders when Read is called
func TestMultiReaderFlatten(t *testT) {
	pc := make([]uintptr, 1000) // 1000 should fit the full stack
	n := runtime.Callers(0, pc)
	var myDepth = callDepth(pc[:n])
	var readDepth int // will contain the depth from which fakeReader.Read was called
	var r io.Reader = io.MultiReader(readerFunc(func(p []byte) (int, error) {
		n := runtime.Callers(1, pc)
		readDepth = callDepth(pc[:n])
		return 0, errors.New("irrelevant")
	}))

	// chain a bunch of multiReaders
	for i := 0; i < 100; i++ {
		r = io.MultiReader(r)
	}

	r.Read(nil) // don't care about errors, just want to check the call-depth for Read

	atomic.AddInt64(&checks, 1)
	if readDepth > myDepth+2 { // 2 should be multiReader.Read and fakeReader.Read
		t.Errorf("multiReader did not flatten chained multiReaders: expected readDepth %d, got %d",
			myDepth+2, readDepth)
	}
}

// byteAndEOFReader is a Reader which reads one byte (the underlying
// byte) and EOF at once in its Read call.
type byteAndEOFReader byte

func (b byteAndEOFReader) Read(p []byte) (n int, err error) {
	atomic.AddInt64(&checks, 1)
	if len(p) == 0 {
		// Read(0 bytes) is useless. We expect no such useless
		// calls in this test.
		panic("unexpected call")
	}
	p[0] = byte(b)
	return 1, io.EOF
}

// This used to yield bytes forever; issue 16795.
func TestMultiReaderSingleByteWithEOF(t *testT) {
	got, err := io.ReadAll(io.LimitReader(io.MultiReader(byteAndEOFReader('a'), byteAndEOFReader('b')), 10))
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Fatal(err)
	}
	const want = "ab"
	atomic.AddInt64(&checks, 1)
	if string(got) != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

// Test that a reader returning (n, EOF) at the end of a MultiReader
// chain continues to return EOF on its final read, rather than
// yielding a (0, EOF).
func TestMultiReaderFinalEOF(t *testT) {
	r := io.MultiReader(bytes.NewReader(nil), byteAndEOFReader('a'))
	buf := make([]byte, 2)
	n, err := r.Read(buf)
	atomic.AddInt64(&checks, 1)
	if n != 1 || err != io.EOF {
		t.Errorf("got %v, %v; want 1, EOF", n, err)
	}
}


func TestInterleavedMultiReader(t *testT) {
	r1 := strings.NewReader("123")
	r2 := strings.NewReader("45678")

	mr1 := io.MultiReader(r1, r2)
	mr2 := io.MultiReader(mr1)

	buf := make([]byte, 4)

	// Have mr2 use mr1's []Readers.
	// Consume r1 (and clear it for GC to handle) and consume part of r2.
	n, err := io.ReadFull(mr2, buf)
	atomic.AddInt64(&checks, 1)
	if got := string(buf[:n]); got != "1234" || err != nil {
		t.Errorf(`ReadFull(mr2) = (%q, %v), want ("1234", nil)`, got, err)
	}

	// Consume the rest of r2 via mr1.
	// This should not panic even though mr2 cleared r1.
	n, err = io.ReadFull(mr1, buf)
	atomic.AddInt64(&checks, 1)
	if got := string(buf[:n]); got != "5678" || err != nil {
		t.Errorf(`ReadFull(mr1) = (%q, %v), want ("5678", nil)`, got, err)
	}
}

// ---- a small harness instead of package testing (step 9.9); Go's tests are below, unchanged except as listed above ----

type testT struct {
	name   string
	failed bool
}

type fatalExit struct{}
type skipExit struct{}

var subtests int

// checks counts the if statements that the tests of the file run: a Test that returns early shows in the expected output.
// It is atomic: some tests run their checks in several goroutines.
var checks int64

func (t *testT) Helper()                          {}
func (t *testT) Parallel()                        {}
func (t *testT) Name() string                     { return t.name }
func (t *testT) Log(args ...any)                  {}
func (t *testT) Logf(format string, args ...any)  {}
func (t *testT) Skip(args ...any)                 { panic(skipExit{}) }
func (t *testT) Skipf(f string, args ...any)      { panic(skipExit{}) }
func (t *testT) report(msg string)                { t.failed = true; fmt.Println("FAIL", t.name+":", msg) }
func (t *testT) Errorf(format string, args ...any) { t.report(fmt.Sprintf(format, args...)) }
func (t *testT) Error(args ...any)                { t.report(fmt.Sprint(args...)) }
func (t *testT) Fatalf(format string, args ...any) {
	t.Errorf(format, args...)
	panic(fatalExit{})
}
func (t *testT) Fatal(args ...any) {
	t.Error(args...)
	panic(fatalExit{})
}

func (t *testT) Run(name string, f func(t *testT)) bool {
	sub := &testT{name: t.name + "/" + name}
	subtests++
	runTest(sub, f)
	if sub.failed {
		t.failed = true
	}
	return !sub.failed
}

func runTest(t *testT, f func(t *testT)) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case fatalExit, skipExit:
			default:
				t.failed = true
				fmt.Println("PANIC", t.name+":", r)
			}
		}
	}()
	f(t)
}

type testCase struct {
	name string
	f    func(t *testT)
}

func runTests(tests []testCase) {
	failed := 0
	for _, tc := range tests {
		t := &testT{name: tc.name}
		subtests = 0
		atomic.StoreInt64(&checks, 0)
		runTest(t, tc.f)
		if t.failed {
			failed++
			fmt.Println("--- FAIL:", tc.name)
		} else {
			fmt.Println("ok", tc.name, "subtests:", subtests, "checks:", atomic.LoadInt64(&checks))
		}
	}
	fmt.Println("tests:", len(tests), "failed:", failed)
	if failed > 0 {
		panic("tests failed")
	}
}

// allocsPerRun runs f runs times and returns 0: gocpp does not count heap allocations (the tests of allocation counts are
// dropped or their comparison is).
func allocsPerRun(runs int, f func()) float64 {
	for i := 0; i < runs; i++ {
		f()
	}
	return 0
}

// A deterministic generator (splitmix64) in place of math/rand, so that Go and gocpp run the same inputs.
type rngT struct{ s uint64 }

var rng = &rngT{0x9E3779B97F4A7C15}

func (r *rngT) Uint64() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
func (r *rngT) Uint32() uint32 { return uint32(r.Uint64() >> 32) }
func (r *rngT) Intn(n int) int { return int(r.Uint64()>>1) % n }
func (r *rngT) Float64() float64 { return float64(r.Uint64()>>11) / (1 << 53) }

func main() {
	tests_ := []testCase{
		{"TestMultiReader", TestMultiReader},
		{"TestMultiReaderAsWriterTo", TestMultiReaderAsWriterTo},
		{"TestMultiWriter", TestMultiWriter},
		{"TestMultiWriter_String", TestMultiWriter_String},
		{"TestMultiWriter_StringCheckCall", TestMultiWriter_StringCheckCall},
		{"TestMultiWriterSingleChainFlatten", TestMultiWriterSingleChainFlatten},
		{"TestMultiWriterError", TestMultiWriterError},
		{"TestMultiReaderCopy", TestMultiReaderCopy},
		{"TestMultiWriterCopy", TestMultiWriterCopy},
		{"TestMultiReaderFlatten", TestMultiReaderFlatten},
		{"TestMultiReaderSingleByteWithEOF", TestMultiReaderSingleByteWithEOF},
		{"TestMultiReaderFinalEOF", TestMultiReaderFinalEOF},
		{"TestInterleavedMultiReader", TestInterleavedMultiReader},
	}
	runTests(tests_)
}

// Output:
// ok TestMultiReader subtests: 0 checks: 27
// ok TestMultiReaderAsWriterTo subtests: 0 checks: 4
// ok TestMultiWriter subtests: 0 checks: 4
// ok TestMultiWriter_String subtests: 0 checks: 4
// ok TestMultiWriter_StringCheckCall subtests: 0 checks: 1
// ok TestMultiWriterSingleChainFlatten subtests: 0 checks: 1
// ok TestMultiWriterError subtests: 0 checks: 1
// ok TestMultiReaderCopy subtests: 0 checks: 1
// ok TestMultiWriterCopy subtests: 0 checks: 2
// ok TestMultiReaderFlatten subtests: 0 checks: 1
// ok TestMultiReaderSingleByteWithEOF subtests: 0 checks: 4
// ok TestMultiReaderFinalEOF subtests: 0 checks: 2
// ok TestInterleavedMultiReader subtests: 0 checks: 2
// tests: 13 failed: 0
