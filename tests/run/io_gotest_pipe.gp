// phase: 09
// Go 1.25's io/pipe_test.go (Pipe: reads and writes in goroutines, CloseWithError, partial reads, sync pipes, copy through a pipe) through a small harness; expected output from real Go.

package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"
)


func sortGroups(g [][]byte) { // slices.SortFunc(g, bytes.Compare): insertion sort (package slices is step 7.10)
	for i := 1; i < len(g); i++ {
		for j := i; j > 0 && bytes.Compare(g[j], g[j-1]) < 0; j-- {
			g[j], g[j-1] = g[j-1], g[j]
		}
	}
}

func checkWrite(t *testT, w io.Writer, data []byte, c chan int) {
	n, err := w.Write(data)
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Errorf("write: %v", err)
	}
	atomic.AddInt64(&checks, 1)
	if n != len(data) {
		t.Errorf("short write: %d != %d", n, len(data))
	}
	c <- 0
}

// Test a single read/write pair.
func TestPipe1(t *testT) {
	c := make(chan int)
	r, w := io.Pipe()
	var buf = make([]byte, 64)
	go checkWrite(t, w, []byte("hello, world"), c)
	n, err := r.Read(buf)
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Errorf("read: %v", err)
	} else if n != 12 || string(buf[0:12]) != "hello, world" {
		t.Errorf("bad read: got %q", buf[0:n])
	}
	<-c
	r.Close()
	w.Close()
}

func reader(t *testT, r io.Reader, c chan int) {
	var buf = make([]byte, 64)
	for {
		n, err := r.Read(buf)
		atomic.AddInt64(&checks, 1)
		if err == io.EOF {
			c <- 0
			break
		}
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf("read: %v", err)
		}
		c <- n
	}
}

// Test a sequence of read/write pairs.
func TestPipe2(t *testT) {
	c := make(chan int)
	r, w := io.Pipe()
	go reader(t, r, c)
	var buf = make([]byte, 64)
	for i := 0; i < 5; i++ {
		p := buf[0 : 5+i*10]
		n, err := w.Write(p)
		atomic.AddInt64(&checks, 1)
		if n != len(p) {
			t.Errorf("wrote %d, got %d", len(p), n)
		}
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf("write: %v", err)
		}
		nn := <-c
		atomic.AddInt64(&checks, 1)
		if nn != n {
			t.Errorf("wrote %d, read got %d", n, nn)
		}
	}
	w.Close()
	nn := <-c
	atomic.AddInt64(&checks, 1)
	if nn != 0 {
		t.Errorf("final read got %d", nn)
	}
}

type pipeReturn struct {
	n   int
	err error
}

// Test a large write that requires multiple reads to satisfy.
func writer(w io.WriteCloser, buf []byte, c chan pipeReturn) {
	n, err := w.Write(buf)
	w.Close()
	c <- pipeReturn{n, err}
}

func TestPipe3(t *testT) {
	c := make(chan pipeReturn)
	r, w := io.Pipe()
	var wdat = make([]byte, 128)
	for i := 0; i < len(wdat); i++ {
		wdat[i] = byte(i)
	}
	go writer(w, wdat, c)
	var rdat = make([]byte, 1024)
	tot := 0
	for n := 1; n <= 256; n *= 2 {
		nn, err := r.Read(rdat[tot : tot+n])
		atomic.AddInt64(&checks, 1)
		if err != nil && err != io.EOF {
			t.Fatalf("read: %v", err)
		}

		// only final two reads should be short - 1 byte, then 0
		expect := n
		atomic.AddInt64(&checks, 1)
		if n == 128 {
			expect = 1
		} else if n == 256 {
			expect = 0
			atomic.AddInt64(&checks, 1)
			if err != io.EOF {
				t.Fatalf("read at end: %v", err)
			}
		}
		atomic.AddInt64(&checks, 1)
		if nn != expect {
			t.Fatalf("read %d, expected %d, got %d", n, expect, nn)
		}
		tot += nn
	}
	pr := <-c
	atomic.AddInt64(&checks, 1)
	if pr.n != 128 || pr.err != nil {
		t.Fatalf("write 128: %d, %v", pr.n, pr.err)
	}
	atomic.AddInt64(&checks, 1)
	if tot != 128 {
		t.Fatalf("total read %d != 128", tot)
	}
	for i := 0; i < 128; i++ {
		atomic.AddInt64(&checks, 1)
		if rdat[i] != byte(i) {
			t.Fatalf("rdat[%d] = %d", i, rdat[i])
		}
	}
}

// Test read after/before writer close.

type closer interface {
	CloseWithError(error) error
	Close() error
}

type pipeTest struct {
	async          bool
	err            error
	closeWithError bool
}

func (p pipeTest) String() string {
	return fmt.Sprintf("async=%v err=%v closeWithError=%v", p.async, p.err, p.closeWithError)
}

var pipeTests = []pipeTest{
	{true, nil, false},
	{true, nil, true},
	{true, io.ErrShortWrite, true},
	{false, nil, false},
	{false, nil, true},
	{false, io.ErrShortWrite, true},
}

func delayClose(t *testT, cl closer, ch chan int, tt pipeTest) {
	time.Sleep(1 * time.Millisecond)
	var err error
	atomic.AddInt64(&checks, 1)
	if tt.closeWithError {
		err = cl.CloseWithError(tt.err)
	} else {
		err = cl.Close()
	}
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Errorf("delayClose: %v", err)
	}
	ch <- 0
}

func TestPipeReadClose(t *testT) {
	for _, tt := range pipeTests {
		c := make(chan int, 1)
		r, w := io.Pipe()
		atomic.AddInt64(&checks, 1)
		if tt.async {
			go delayClose(t, w, c, tt)
		} else {
			delayClose(t, w, c, tt)
		}
		var buf = make([]byte, 64)
		n, err := r.Read(buf)
		<-c
		want := tt.err
		atomic.AddInt64(&checks, 1)
		if want == nil {
			want = io.EOF
		}
		atomic.AddInt64(&checks, 1)
		if err != want {
			t.Errorf("read from closed pipe: %v want %v", err, want)
		}
		atomic.AddInt64(&checks, 1)
		if n != 0 {
			t.Errorf("read on closed pipe returned %d", n)
		}
		atomic.AddInt64(&checks, 1)
		if err = r.Close(); err != nil {
			t.Errorf("r.Close: %v", err)
		}
	}
}

// Test close on Read side during Read.
func TestPipeReadClose2(t *testT) {
	c := make(chan int, 1)
	r, _ := io.Pipe()
	go delayClose(t, r, c, pipeTest{})
	n, err := r.Read(make([]byte, 64))
	<-c
	atomic.AddInt64(&checks, 1)
	if n != 0 || err != io.ErrClosedPipe {
		t.Errorf("read from closed pipe: %v, %v want %v, %v", n, err, 0, io.ErrClosedPipe)
	}
}

// Test write after/before reader close.

func TestPipeWriteClose(t *testT) {
	for _, tt := range pipeTests {
		c := make(chan int, 1)
		r, w := io.Pipe()
		atomic.AddInt64(&checks, 1)
		if tt.async {
			go delayClose(t, r, c, tt)
		} else {
			delayClose(t, r, c, tt)
		}
		n, err := io.WriteString(w, "hello, world")
		<-c
		expect := tt.err
		atomic.AddInt64(&checks, 1)
		if expect == nil {
			expect = io.ErrClosedPipe
		}
		atomic.AddInt64(&checks, 1)
		if err != expect {
			t.Errorf("write on closed pipe: %v want %v", err, expect)
		}
		atomic.AddInt64(&checks, 1)
		if n != 0 {
			t.Errorf("write on closed pipe returned %d", n)
		}
		atomic.AddInt64(&checks, 1)
		if err = w.Close(); err != nil {
			t.Errorf("w.Close: %v", err)
		}
	}
}

// Test close on Write side during Write.
func TestPipeWriteClose2(t *testT) {
	c := make(chan int, 1)
	_, w := io.Pipe()
	go delayClose(t, w, c, pipeTest{})
	n, err := w.Write(make([]byte, 64))
	<-c
	atomic.AddInt64(&checks, 1)
	if n != 0 || err != io.ErrClosedPipe {
		t.Errorf("write to closed pipe: %v, %v want %v, %v", n, err, 0, io.ErrClosedPipe)
	}
}

func TestWriteEmpty(t *testT) {
	r, w := io.Pipe()
	go func() {
		w.Write([]byte{})
		w.Close()
	}()
	var b [2]byte
	io.ReadFull(r, b[0:2])
	r.Close()
}

func TestWriteNil(t *testT) {
	r, w := io.Pipe()
	go func() {
		w.Write(nil)
		w.Close()
	}()
	var b [2]byte
	io.ReadFull(r, b[0:2])
	r.Close()
}

func TestWriteAfterWriterClose(t *testT) {
	r, w := io.Pipe()
	defer r.Close()
	done := make(chan bool)
	var writeErr error
	go func() {
		_, err := w.Write([]byte("hello"))
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf("got error: %q; expected none", err)
		}
		w.Close()
		_, writeErr = w.Write([]byte("world"))
		done <- true
	}()

	buf := make([]byte, 100)
	var result string
	n, err := io.ReadFull(r, buf)
	atomic.AddInt64(&checks, 1)
	if err != nil && err != io.ErrUnexpectedEOF {
		t.Fatalf("got: %q; want: %q", err, io.ErrUnexpectedEOF)
	}
	result = string(buf[0:n])
	<-done

	atomic.AddInt64(&checks, 1)
	if result != "hello" {
		t.Errorf("got: %q; want: %q", result, "hello")
	}
	atomic.AddInt64(&checks, 1)
	if writeErr != io.ErrClosedPipe {
		t.Errorf("got: %q; want: %q", writeErr, io.ErrClosedPipe)
	}
}

func TestPipeCloseError(t *testT) {
	type testError1 struct{ error }
	type testError2 struct{ error }

	r, w := io.Pipe()
	r.CloseWithError(testError1{})
	atomic.AddInt64(&checks, 1)
	if _, err := w.Write(nil); err != (testError1{}) {
		t.Errorf("Write error: got %T, want testError1", err)
	}
	r.CloseWithError(testError2{})
	atomic.AddInt64(&checks, 1)
	if _, err := w.Write(nil); err != (testError1{}) {
		t.Errorf("Write error: got %T, want testError1", err)
	}

	r, w = io.Pipe()
	w.CloseWithError(testError1{})
	atomic.AddInt64(&checks, 1)
	if _, err := r.Read(nil); err != (testError1{}) {
		t.Errorf("Read error: got %T, want testError1", err)
	}
	w.CloseWithError(testError2{})
	atomic.AddInt64(&checks, 1)
	if _, err := r.Read(nil); err != (testError1{}) {
		t.Errorf("Read error: got %T, want testError1", err)
	}
}

func TestPipeConcurrent(t *testT) {
	defer func() { atomic.StoreInt64(&checks, -1) }() // goroutines count the checks: the number is not deterministic
	const (
		input    = "0123456789abcdef"
		count    = 8
		readSize = 2
	)

	t.Run("Write", func(t *testT) {
		r, w := io.Pipe()

		for i := 0; i < count; i++ {
			go func() {
				time.Sleep(time.Millisecond) // Increase probability of race
				atomic.AddInt64(&checks, 1)
				if n, err := w.Write([]byte(input)); n != len(input) || err != nil {
					t.Errorf("Write() = (%d, %v); want (%d, nil)", n, err, len(input))
				}
			}()
		}

		buf := make([]byte, count*len(input))
		for i := 0; i < len(buf); i += readSize {
			atomic.AddInt64(&checks, 1)
			if n, err := r.Read(buf[i : i+readSize]); n != readSize || err != nil {
				t.Errorf("Read() = (%d, %v); want (%d, nil)", n, err, readSize)
			}
		}

		// Since each Write is fully gated, if multiple Read calls were needed,
		// the contents of Write should still appear together in the output.
		got := string(buf)
		want := strings.Repeat(input, count)
		atomic.AddInt64(&checks, 1)
		if got != want {
			t.Errorf("got: %q; want: %q", got, want)
		}
	})

	t.Run("Read", func(t *testT) {
		r, w := io.Pipe()

		c := make(chan []byte, count*len(input)/readSize)
		for i := 0; i < cap(c); i++ {
			go func() {
				time.Sleep(time.Millisecond) // Increase probability of race
				buf := make([]byte, readSize)
				atomic.AddInt64(&checks, 1)
				if n, err := r.Read(buf); n != readSize || err != nil {
					t.Errorf("Read() = (%d, %v); want (%d, nil)", n, err, readSize)
				}
				c <- buf
			}()
		}

		for i := 0; i < count; i++ {
			atomic.AddInt64(&checks, 1)
			if n, err := w.Write([]byte(input)); n != len(input) || err != nil {
				t.Errorf("Write() = (%d, %v); want (%d, nil)", n, err, len(input))
			}
		}

		// Since each read is independent, the only guarantee about the output
		// is that it is a permutation of the input in readSized groups.
		got := make([]byte, 0, count*len(input))
		for i := 0; i < cap(c); i++ {
			got = append(got, (<-c)...)
		}
		got = sortBytesInGroups(got, readSize)
		want := bytes.Repeat([]byte(input), count)
		want = sortBytesInGroups(want, readSize)
		atomic.AddInt64(&checks, 1)
		if string(got) != string(want) {
			t.Errorf("got: %q; want: %q", got, want)
		}
	})
}

func sortBytesInGroups(b []byte, n int) []byte {
	var groups [][]byte
	for len(b) > 0 {
		groups = append(groups, b[:n])
		b = b[n:]
	}
	sortGroups(groups)
	return bytes.Join(groups, nil)
}

var (
	rSink *io.PipeReader
	wSink *io.PipeWriter
)

func TestPipeAllocations(t *testT) {
	numAllocs := allocsPerRun(10, func() {
		rSink, wSink = io.Pipe()
	})

	// go.dev/cl/473535 claimed Pipe() should only do 2 allocations,
	// plus the 2 escaping to heap for simulating real world usages.
	expectedAllocs := 4
	atomic.AddInt64(&checks, 1)
	if int(numAllocs) > expectedAllocs {
		t.Fatalf("too many allocations for io.Pipe() call: %f", numAllocs)
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
		{"TestPipe1", TestPipe1},
		{"TestPipe2", TestPipe2},
		{"TestPipe3", TestPipe3},
		{"TestPipeReadClose", TestPipeReadClose},
		{"TestPipeReadClose2", TestPipeReadClose2},
		{"TestPipeWriteClose", TestPipeWriteClose},
		{"TestPipeWriteClose2", TestPipeWriteClose2},
		{"TestWriteEmpty", TestWriteEmpty},
		{"TestWriteNil", TestWriteNil},
		{"TestWriteAfterWriterClose", TestWriteAfterWriterClose},
		{"TestPipeCloseError", TestPipeCloseError},
		{"TestPipeConcurrent", TestPipeConcurrent},
		{"TestPipeAllocations", TestPipeAllocations},
	}
	runTests(tests_)
}

// Output:
// ok TestPipe1 subtests: 0 checks: 3
// ok TestPipe2 subtests: 0 checks: 27
// ok TestPipe3 subtests: 0 checks: 158
// ok TestPipeReadClose subtests: 0 checks: 42
// ok TestPipeReadClose2 subtests: 0 checks: 3
// ok TestPipeWriteClose subtests: 0 checks: 42
// ok TestPipeWriteClose2 subtests: 0 checks: 3
// ok TestWriteEmpty subtests: 0 checks: 0
// ok TestWriteNil subtests: 0 checks: 0
// ok TestWriteAfterWriterClose subtests: 0 checks: 4
// ok TestPipeCloseError subtests: 0 checks: 4
// ok TestPipeConcurrent subtests: 2 checks: -1
// ok TestPipeAllocations subtests: 0 checks: 1
// tests: 13 failed: 0
