// phase: 09
// Go 1.25's strings/builder_test.go (Builder: Write*, Grow, Reset, the copy check, String sharing, invalid runes) through a small harness; the allocation counts are not checked; expected output from real Go.

package main

import (
	"bytes"
	"fmt"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)


func check(t *testT, b *strings.Builder, want string) {
	t.Helper()
	got := b.String()
	atomic.AddInt64(&checks, 1)
	if got != want {
		t.Errorf("String: got %#q; want %#q", got, want)
		return
	}
	atomic.AddInt64(&checks, 1)
	if n := b.Len(); n != len(got) {
		t.Errorf("Len: got %d; but len(String()) is %d", n, len(got))
	}
	atomic.AddInt64(&checks, 1)
	if n := b.Cap(); n < len(got) {
		t.Errorf("Cap: got %d; but len(String()) is %d", n, len(got))
	}
}

func TestBuilder(t *testT) {
	var b strings.Builder
	check(t, &b, "")
	n, err := b.WriteString("hello")
	atomic.AddInt64(&checks, 1)
	if err != nil || n != 5 {
		t.Errorf("WriteString: got %d,%s; want 5,nil", n, err)
	}
	check(t, &b, "hello")
	atomic.AddInt64(&checks, 1)
	if err = b.WriteByte(' '); err != nil {
		t.Errorf("WriteByte: %s", err)
	}
	check(t, &b, "hello ")
	n, err = b.WriteString("world")
	atomic.AddInt64(&checks, 1)
	if err != nil || n != 5 {
		t.Errorf("WriteString: got %d,%s; want 5,nil", n, err)
	}
	check(t, &b, "hello world")
}

func TestBuilderString(t *testT) {
	var b strings.Builder
	b.WriteString("alpha")
	check(t, &b, "alpha")
	s1 := b.String()
	b.WriteString("beta")
	check(t, &b, "alphabeta")
	s2 := b.String()
	b.WriteString("gamma")
	check(t, &b, "alphabetagamma")
	s3 := b.String()

	// Check that subsequent operations didn't change the returned strings.
	atomic.AddInt64(&checks, 1)
	if want := "alpha"; s1 != want {
		t.Errorf("first String result is now %q; want %q", s1, want)
	}
	atomic.AddInt64(&checks, 1)
	if want := "alphabeta"; s2 != want {
		t.Errorf("second String result is now %q; want %q", s2, want)
	}
	atomic.AddInt64(&checks, 1)
	if want := "alphabetagamma"; s3 != want {
		t.Errorf("third String result is now %q; want %q", s3, want)
	}
}

func TestBuilderReset(t *testT) {
	var b strings.Builder
	check(t, &b, "")
	b.WriteString("aaa")
	s := b.String()
	check(t, &b, "aaa")
	b.Reset()
	check(t, &b, "")

	// Ensure that writing after Reset doesn't alter
	// previously returned strings.
	b.WriteString("bbb")
	check(t, &b, "bbb")
	atomic.AddInt64(&checks, 1)
	if want := "aaa"; s != want {
		t.Errorf("previous String result changed after Reset: got %q; want %q", s, want)
	}
}

func TestBuilderGrow(t *testT) {
	for _, growLen := range []int{0, 100, 1000, 10000, 100000} {
		atomic.AddInt64(&checks, 1)
		if false {
			t.Logf("skipping allocs check for growLen %d: extra allocs with -asan; see #70079", growLen)
			continue
		}
		p := bytes.Repeat([]byte{'a'}, growLen)
		allocs := allocsPerRun(100, func() {
			var b strings.Builder
			b.Grow(growLen) // should be only alloc, when growLen > 0
			atomic.AddInt64(&checks, 1)
			if b.Cap() < growLen {
				t.Fatalf("growLen=%d: Cap() is lower than growLen", growLen)
			}
			b.Write(p)
			atomic.AddInt64(&checks, 1)
			if b.String() != string(p) {
				t.Fatalf("growLen=%d: bad data written after Grow", growLen)
			}
		})
		_ = allocs // gocpp does not count heap allocations
	}
	// when growLen < 0, should panic
	var a strings.Builder
	n := -1
	defer func() {
		atomic.AddInt64(&checks, 1)
		if r := recover(); r == nil {
			t.Errorf("a.Grow(%d) should panic()", n)
		}
	}()
	a.Grow(n)
}

func TestBuilderWrite2(t *testT) {
	const s0 = "hello 世界"
	for _, tt := range []struct {
		name string
		fn   func(b *strings.Builder) (int, error)
		n    int
		want string
	}{
		{
			"Write",
			func(b *strings.Builder) (int, error) { return b.Write([]byte(s0)) },
			len(s0),
			s0,
		},
		{
			"WriteRune",
			func(b *strings.Builder) (int, error) { return b.WriteRune('a') },
			1,
			"a",
		},
		{
			"WriteRuneWide",
			func(b *strings.Builder) (int, error) { return b.WriteRune('世') },
			3,
			"世",
		},
		{
			"WriteString",
			func(b *strings.Builder) (int, error) { return b.WriteString(s0) },
			len(s0),
			s0,
		},
	} {
		t.Run(tt.name, func(t *testT) {
			var b strings.Builder
			n, err := tt.fn(&b)
			atomic.AddInt64(&checks, 1)
			if err != nil {
				t.Fatalf("first call: got %s", err)
			}
			atomic.AddInt64(&checks, 1)
			if n != tt.n {
				t.Errorf("first call: got n=%d; want %d", n, tt.n)
			}
			check(t, &b, tt.want)

			n, err = tt.fn(&b)
			atomic.AddInt64(&checks, 1)
			if err != nil {
				t.Fatalf("second call: got %s", err)
			}
			atomic.AddInt64(&checks, 1)
			if n != tt.n {
				t.Errorf("second call: got n=%d; want %d", n, tt.n)
			}
			check(t, &b, tt.want+tt.want)
		})
	}
}

func TestBuilderWriteByte(t *testT) {
	var b strings.Builder
	atomic.AddInt64(&checks, 1)
	if err := b.WriteByte('a'); err != nil {
		t.Error(err)
	}
	atomic.AddInt64(&checks, 1)
	if err := b.WriteByte(0); err != nil {
		t.Error(err)
	}
	check(t, &b, "a\x00")
}


func TestBuilderCopyPanic(t *testT) {
	tests := []struct {
		name      string
		fn        func()
		wantPanic bool
	}{
		{
			name:      "String",
			wantPanic: false,
			fn: func() {
				var a strings.Builder
				a.WriteByte('x')
				b := a
				_ = b.String() // appease vet
			},
		},
		{
			name:      "Len",
			wantPanic: false,
			fn: func() {
				var a strings.Builder
				a.WriteByte('x')
				b := a
				b.Len()
			},
		},
		{
			name:      "Cap",
			wantPanic: false,
			fn: func() {
				var a strings.Builder
				a.WriteByte('x')
				b := a
				b.Cap()
			},
		},
		{
			name:      "Reset",
			wantPanic: false,
			fn: func() {
				var a strings.Builder
				a.WriteByte('x')
				b := a
				b.Reset()
				b.WriteByte('y')
			},
		},
		{
			name:      "Write",
			wantPanic: true,
			fn: func() {
				var a strings.Builder
				a.Write([]byte("x"))
				b := a
				b.Write([]byte("y"))
			},
		},
		{
			name:      "WriteByte",
			wantPanic: true,
			fn: func() {
				var a strings.Builder
				a.WriteByte('x')
				b := a
				b.WriteByte('y')
			},
		},
		{
			name:      "WriteString",
			wantPanic: true,
			fn: func() {
				var a strings.Builder
				a.WriteString("x")
				b := a
				b.WriteString("y")
			},
		},
		{
			name:      "WriteRune",
			wantPanic: true,
			fn: func() {
				var a strings.Builder
				a.WriteRune('x')
				b := a
				b.WriteRune('y')
			},
		},
		{
			name:      "Grow",
			wantPanic: true,
			fn: func() {
				var a strings.Builder
				a.Grow(1)
				b := a
				b.Grow(2)
			},
		},
	}
	for _, tt := range tests {
		didPanic := make(chan bool)
		go func() {
			defer func() { didPanic <- recover() != nil }()
			tt.fn()
		}()
		atomic.AddInt64(&checks, 1)
		if got := <-didPanic; got != tt.wantPanic {
			t.Errorf("%s: panicked = %v; want %v", tt.name, got, tt.wantPanic)
		}
	}
}

func TestBuilderWriteInvalidRune(t *testT) {
	// Invalid runes, including negative ones, should be written as
	// utf8.RuneError.
	for _, r := range []rune{-1, utf8.MaxRune + 1} {
		var b strings.Builder
		b.WriteRune(r)
		check(t, &b, "\uFFFD")
	}
}

var someBytes = []byte("some bytes sdljlk jsklj3lkjlk djlkjw")

var sinkS string

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
		{"TestBuilder", TestBuilder},
		{"TestBuilderString", TestBuilderString},
		{"TestBuilderReset", TestBuilderReset},
		{"TestBuilderGrow", TestBuilderGrow},
		{"TestBuilderWrite2", TestBuilderWrite2},
		{"TestBuilderWriteByte", TestBuilderWriteByte},
		{"TestBuilderCopyPanic", TestBuilderCopyPanic},
		{"TestBuilderWriteInvalidRune", TestBuilderWriteInvalidRune},
	}
	runTests(tests_)
}

// Output:
// ok TestBuilder subtests: 0 checks: 15
// ok TestBuilderString subtests: 0 checks: 12
// ok TestBuilderReset subtests: 0 checks: 13
// ok TestBuilderGrow subtests: 0 checks: 1006
// ok TestBuilderWrite2 subtests: 4 checks: 40
// ok TestBuilderWriteByte subtests: 0 checks: 5
// ok TestBuilderCopyPanic subtests: 0 checks: 9
// ok TestBuilderWriteInvalidRune subtests: 0 checks: 6
// tests: 8 failed: 0
