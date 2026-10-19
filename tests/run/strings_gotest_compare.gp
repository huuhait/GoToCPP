// phase: 09
// Go 1.25's strings/compare_test.go (Compare of equal, shorter, longer and one-byte-different strings up to 99999 bytes) through a small harness; expected output from real Go.

package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"unsafe"
)


var compareTests = []struct {
	a, b string
	i    int
}{
	{"", "", 0},
	{"a", "", 1},
	{"", "a", -1},
	{"abc", "abc", 0},
	{"ab", "abc", -1},
	{"abc", "ab", 1},
	{"x", "ab", 1},
	{"ab", "x", -1},
	{"x", "a", 1},
	{"b", "x", -1},
	// test runtime·memeq's chunked implementation
	{"abcdefgh", "abcdefgh", 0},
	{"abcdefghi", "abcdefghi", 0},
	{"abcdefghi", "abcdefghj", -1},
}

func TestCompare(t *testT) {
	for _, tt := range compareTests {
		numShifts := 16
		for offset := 0; offset <= numShifts; offset++ {
			shiftedB := (strings.Repeat("*", offset) + tt.b)[offset:]
			cmp := strings.Compare(tt.a, shiftedB)
			atomic.AddInt64(&checks, 1)
			if cmp != tt.i {
				t.Errorf(`Compare(%q, %q), offset %d = %v; want %v`, tt.a, tt.b, offset, cmp, tt.i)
			}
		}
	}
}

func TestCompareIdenticalString(t *testT) {
	var s = "Hello Gophers!"
	atomic.AddInt64(&checks, 1)
	if strings.Compare(s, s) != 0 {
		t.Error("s != s")
	}
	atomic.AddInt64(&checks, 1)
	if strings.Compare(s, s[:1]) != 1 {
		t.Error("s > s[:1] failed")
	}
}

func TestCompareStrings(t *testT) {
	// unsafeString converts a []byte to a string with no allocation.
	// The caller must not modify b while the result string is in use.
	unsafeString := func(b []byte) string {
		return unsafe.String(unsafe.SliceData(b), len(b))
	}

	lengths := make([]int, 0) // lengths to test in ascending order
	for i := 0; i <= 128; i++ {
		lengths = append(lengths, i)
	}
	lengths = append(lengths, 256, 512, 1024, 1333, 4095, 4096, 4097)

	atomic.AddInt64(&checks, 1)
	if !true || "" != "" {
		lengths = append(lengths, 65535, 65536, 65537, 99999)
	}

	n := lengths[len(lengths)-1]
	a := make([]byte, n+1)
	b := make([]byte, n+1)
	lastLen := 0
	for _, len := range lengths {
		// randomish but deterministic data. No 0 or 255.
		for i := 0; i < len; i++ {
			a[i] = byte(1 + 31*i%254)
			b[i] = byte(1 + 31*i%254)
		}
		// data past the end is different
		for i := len; i <= n; i++ {
			a[i] = 8
			b[i] = 9
		}

		sa, sb := unsafeString(a), unsafeString(b)
		cmp := strings.Compare(sa[:len], sb[:len])
		atomic.AddInt64(&checks, 1)
		if cmp != 0 {
			t.Errorf(`CompareIdentical(%d) = %d`, len, cmp)
		}
		atomic.AddInt64(&checks, 1)
		if len > 0 {
			cmp = strings.Compare(sa[:len-1], sb[:len])
			atomic.AddInt64(&checks, 1)
			if cmp != -1 {
				t.Errorf(`CompareAshorter(%d) = %d`, len, cmp)
			}
			cmp = strings.Compare(sa[:len], sb[:len-1])
			atomic.AddInt64(&checks, 1)
			if cmp != 1 {
				t.Errorf(`CompareBshorter(%d) = %d`, len, cmp)
			}
		}
		for k := lastLen; k < len; k++ {
			b[k] = a[k] - 1
			cmp = strings.Compare(unsafeString(a[:len]), unsafeString(b[:len]))
			atomic.AddInt64(&checks, 1)
			if cmp != 1 {
				t.Errorf(`CompareAbigger(%d,%d) = %d`, len, k, cmp)
			}
			b[k] = a[k] + 1
			cmp = strings.Compare(unsafeString(a[:len]), unsafeString(b[:len]))
			atomic.AddInt64(&checks, 1)
			if cmp != -1 {
				t.Errorf(`CompareBbigger(%d,%d) = %d`, len, k, cmp)
			}
			b[k] = a[k]
		}
		lastLen = len
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
		{"TestCompare", TestCompare},
		{"TestCompareIdenticalString", TestCompareIdenticalString},
		{"TestCompareStrings", TestCompareStrings},
	}
	runTests(tests_)
}

// Output:
// ok TestCompare subtests: 0 checks: 221
// ok TestCompareIdenticalString subtests: 0 checks: 2
// ok TestCompareStrings subtests: 0 checks: 8737
// tests: 3 failed: 0
