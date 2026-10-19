// phase: 09
// Go 1.25's strings/reader_test.go (Reader: Read, ReadAt, Seek, ReadByte/UnreadByte, ReadRune/UnreadRune, WriteTo, Reset, Len/Size, the concurrency check) through a small harness; expected output from real Go.

package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
)


func TestReader(t *testT) {
	r := strings.NewReader("0123456789")
	tests := []struct {
		off     int64
		seek    int
		n       int
		want    string
		wantpos int64
		readerr error
		seekerr string
	}{
		{seek: io.SeekStart, off: 0, n: 20, want: "0123456789"},
		{seek: io.SeekStart, off: 1, n: 1, want: "1"},
		{seek: io.SeekCurrent, off: 1, wantpos: 3, n: 2, want: "34"},
		{seek: io.SeekStart, off: -1, seekerr: "strings.Reader.Seek: negative position"},
		{seek: io.SeekStart, off: 1 << 33, wantpos: 1 << 33, readerr: io.EOF},
		{seek: io.SeekCurrent, off: 1, wantpos: 1<<33 + 1, readerr: io.EOF},
		{seek: io.SeekStart, n: 5, want: "01234"},
		{seek: io.SeekCurrent, n: 5, want: "56789"},
		{seek: io.SeekEnd, off: -1, n: 1, wantpos: 9, want: "9"},
	}

	for i, tt := range tests {
		pos, err := r.Seek(tt.off, tt.seek)
		atomic.AddInt64(&checks, 1)
		if err == nil && tt.seekerr != "" {
			t.Errorf("%d. want seek error %q", i, tt.seekerr)
			continue
		}
		atomic.AddInt64(&checks, 1)
		if err != nil && err.Error() != tt.seekerr {
			t.Errorf("%d. seek error = %q; want %q", i, err.Error(), tt.seekerr)
			continue
		}
		atomic.AddInt64(&checks, 1)
		if tt.wantpos != 0 && tt.wantpos != pos {
			t.Errorf("%d. pos = %d, want %d", i, pos, tt.wantpos)
		}
		buf := make([]byte, tt.n)
		n, err := r.Read(buf)
		atomic.AddInt64(&checks, 1)
		if err != tt.readerr {
			t.Errorf("%d. read = %v; want %v", i, err, tt.readerr)
			continue
		}
		got := string(buf[:n])
		atomic.AddInt64(&checks, 1)
		if got != tt.want {
			t.Errorf("%d. got %q; want %q", i, got, tt.want)
		}
	}
}

func TestReadAfterBigSeek(t *testT) {
	r := strings.NewReader("0123456789")
	atomic.AddInt64(&checks, 1)
	if _, err := r.Seek(1<<31+5, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	atomic.AddInt64(&checks, 1)
	if n, err := r.Read(make([]byte, 10)); n != 0 || err != io.EOF {
		t.Errorf("Read = %d, %v; want 0, EOF", n, err)
	}
}

func TestReaderAt(t *testT) {
	r := strings.NewReader("0123456789")
	tests := []struct {
		off     int64
		n       int
		want    string
		wanterr any
	}{
		{0, 10, "0123456789", nil},
		{1, 10, "123456789", io.EOF},
		{1, 9, "123456789", nil},
		{11, 10, "", io.EOF},
		{0, 0, "", nil},
		{-1, 0, "", "strings.Reader.ReadAt: negative offset"},
	}
	for i, tt := range tests {
		b := make([]byte, tt.n)
		rn, err := r.ReadAt(b, tt.off)
		got := string(b[:rn])
		atomic.AddInt64(&checks, 1)
		if got != tt.want {
			t.Errorf("%d. got %q; want %q", i, got, tt.want)
		}
		atomic.AddInt64(&checks, 1)
		if fmt.Sprintf("%v", err) != fmt.Sprintf("%v", tt.wanterr) {
			t.Errorf("%d. got error = %v; want %v", i, err, tt.wanterr)
		}
	}
}

func TestReaderAtConcurrent(t *testT) {
	// Test for the race detector, to verify ReadAt doesn't mutate
	// any state.
	r := strings.NewReader("0123456789")
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var buf [1]byte
			r.ReadAt(buf[:], int64(i))
		}(i)
	}
	wg.Wait()
}

func TestEmptyReaderConcurrent(t *testT) {
	// Test for the race detector, to verify a Read that doesn't yield any bytes
	// is okay to use from multiple goroutines. This was our historic behavior.
	// See golang.org/issue/7856
	r := strings.NewReader("")
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			var buf [1]byte
			r.Read(buf[:])
		}()
		go func() {
			defer wg.Done()
			r.Read(nil)
		}()
	}
	wg.Wait()
}

func TestWriteTo(t *testT) {
	const str = "0123456789"
	for i := 0; i <= len(str); i++ {
		s := str[i:]
		r := strings.NewReader(s)
		var b bytes.Buffer
		n, err := r.WriteTo(&b)
		atomic.AddInt64(&checks, 1)
		if expect := int64(len(s)); n != expect {
			t.Errorf("got %v; want %v", n, expect)
		}
		atomic.AddInt64(&checks, 1)
		if err != nil {
			t.Errorf("for length %d: got error = %v; want nil", len(s), err)
		}
		atomic.AddInt64(&checks, 1)
		if b.String() != s {
			t.Errorf("got string %q; want %q", b.String(), s)
		}
		atomic.AddInt64(&checks, 1)
		if r.Len() != 0 {
			t.Errorf("reader contains %v bytes; want 0", r.Len())
		}
	}
}

// tests that Len is affected by reads, but Size is not.
func TestReaderLenSize(t *testT) {
	r := strings.NewReader("abc")
	io.CopyN(io.Discard, r, 1)
	atomic.AddInt64(&checks, 1)
	if r.Len() != 2 {
		t.Errorf("Len = %d; want 2", r.Len())
	}
	atomic.AddInt64(&checks, 1)
	if r.Size() != 3 {
		t.Errorf("Size = %d; want 3", r.Size())
	}
}

func TestReaderReset(t *testT) {
	r := strings.NewReader("世界")
	atomic.AddInt64(&checks, 1)
	if _, _, err := r.ReadRune(); err != nil {
		t.Errorf("ReadRune: unexpected error: %v", err)
	}

	const want = "abcdef"
	r.Reset(want)
	atomic.AddInt64(&checks, 1)
	if err := r.UnreadRune(); err == nil {
		t.Errorf("UnreadRune: expected error, got nil")
	}
	buf, err := io.ReadAll(r)
	atomic.AddInt64(&checks, 1)
	if err != nil {
		t.Errorf("ReadAll: unexpected error: %v", err)
	}
	atomic.AddInt64(&checks, 1)
	if got := string(buf); got != want {
		t.Errorf("ReadAll: got %q, want %q", got, want)
	}
}

func TestReaderZero(t *testT) {
	atomic.AddInt64(&checks, 1)
	if l := (&strings.Reader{}).Len(); l != 0 {
		t.Errorf("Len: got %d, want 0", l)
	}

	atomic.AddInt64(&checks, 1)
	if n, err := (&strings.Reader{}).Read(nil); n != 0 || err != io.EOF {
		t.Errorf("Read: got %d, %v; want 0, io.EOF", n, err)
	}

	atomic.AddInt64(&checks, 1)
	if n, err := (&strings.Reader{}).ReadAt(nil, 11); n != 0 || err != io.EOF {
		t.Errorf("ReadAt: got %d, %v; want 0, io.EOF", n, err)
	}

	atomic.AddInt64(&checks, 1)
	if b, err := (&strings.Reader{}).ReadByte(); b != 0 || err != io.EOF {
		t.Errorf("ReadByte: got %d, %v; want 0, io.EOF", b, err)
	}

	atomic.AddInt64(&checks, 1)
	if ch, size, err := (&strings.Reader{}).ReadRune(); ch != 0 || size != 0 || err != io.EOF {
		t.Errorf("ReadRune: got %d, %d, %v; want 0, 0, io.EOF", ch, size, err)
	}

	atomic.AddInt64(&checks, 1)
	if offset, err := (&strings.Reader{}).Seek(11, io.SeekStart); offset != 11 || err != nil {
		t.Errorf("Seek: got %d, %v; want 11, nil", offset, err)
	}

	atomic.AddInt64(&checks, 1)
	if s := (&strings.Reader{}).Size(); s != 0 {
		t.Errorf("Size: got %d, want 0", s)
	}

	atomic.AddInt64(&checks, 1)
	if (&strings.Reader{}).UnreadByte() == nil {
		t.Errorf("UnreadByte: got nil, want error")
	}

	atomic.AddInt64(&checks, 1)
	if (&strings.Reader{}).UnreadRune() == nil {
		t.Errorf("UnreadRune: got nil, want error")
	}

	atomic.AddInt64(&checks, 1)
	if n, err := (&strings.Reader{}).WriteTo(io.Discard); n != 0 || err != nil {
		t.Errorf("WriteTo: got %d, %v; want 0, nil", n, err)
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
		{"TestReader", TestReader},
		{"TestReadAfterBigSeek", TestReadAfterBigSeek},
		{"TestReaderAt", TestReaderAt},
		{"TestReaderAtConcurrent", TestReaderAtConcurrent},
		{"TestEmptyReaderConcurrent", TestEmptyReaderConcurrent},
		{"TestWriteTo", TestWriteTo},
		{"TestReaderLenSize", TestReaderLenSize},
		{"TestReaderReset", TestReaderReset},
		{"TestReaderZero", TestReaderZero},
	}
	runTests(tests_)
}

// Output:
// ok TestReader subtests: 0 checks: 45
// ok TestReadAfterBigSeek subtests: 0 checks: 2
// ok TestReaderAt subtests: 0 checks: 12
// ok TestReaderAtConcurrent subtests: 0 checks: 0
// ok TestEmptyReaderConcurrent subtests: 0 checks: 0
// ok TestWriteTo subtests: 0 checks: 44
// ok TestReaderLenSize subtests: 0 checks: 2
// ok TestReaderReset subtests: 0 checks: 4
// ok TestReaderZero subtests: 0 checks: 10
// tests: 9 failed: 0
