// phase: 09
// Go 1.25's path/path_test.go (Clean, Split, Join, Ext, Base, IsAbs, Dir, including the alloc-free cases) through a small harness; the allocation counts are not checked; expected output from real Go.

package main

import (
	"fmt"
	"path"
	"runtime"
	"sync/atomic"
)


type PathTest struct {
	path, result string
}

var cleantests = []PathTest{
	// Already clean
	{"", "."},
	{"abc", "abc"},
	{"abc/def", "abc/def"},
	{"a/b/c", "a/b/c"},
	{".", "."},
	{"..", ".."},
	{"../..", "../.."},
	{"../../abc", "../../abc"},
	{"/abc", "/abc"},
	{"/", "/"},

	// Remove trailing slash
	{"abc/", "abc"},
	{"abc/def/", "abc/def"},
	{"a/b/c/", "a/b/c"},
	{"./", "."},
	{"../", ".."},
	{"../../", "../.."},
	{"/abc/", "/abc"},

	// Remove doubled slash
	{"abc//def//ghi", "abc/def/ghi"},
	{"//abc", "/abc"},
	{"///abc", "/abc"},
	{"//abc//", "/abc"},
	{"abc//", "abc"},

	// Remove . elements
	{"abc/./def", "abc/def"},
	{"/./abc/def", "/abc/def"},
	{"abc/.", "abc"},

	// Remove .. elements
	{"abc/def/ghi/../jkl", "abc/def/jkl"},
	{"abc/def/../ghi/../jkl", "abc/jkl"},
	{"abc/def/..", "abc"},
	{"abc/def/../..", "."},
	{"/abc/def/../..", "/"},
	{"abc/def/../../..", ".."},
	{"/abc/def/../../..", "/"},
	{"abc/def/../../../ghi/jkl/../../../mno", "../../mno"},

	// Combinations
	{"abc/./../def", "def"},
	{"abc//./../def", "def"},
	{"abc/../../././../def", "../../def"},
}

func TestClean(t *testT) {
	for _, test := range cleantests {
		atomic.AddInt64(&checks, 1)
		if s := path.Clean(test.path); s != test.result {
			t.Errorf("Clean(%q) = %q, want %q", test.path, s, test.result)
		}
		atomic.AddInt64(&checks, 1)
		if s := path.Clean(test.result); s != test.result {
			t.Errorf("Clean(%q) = %q, want %q", test.result, s, test.result)
		}
	}
}

func TestCleanMallocs(t *testT) {
	atomic.AddInt64(&checks, 1)
	if true {
		t.Skip("skipping malloc count in short mode")
	}
	atomic.AddInt64(&checks, 1)
	if runtime.GOMAXPROCS(0) > 1 {
		t.Log("skipping AllocsPerRun checks; GOMAXPROCS>1")
		return
	}

	for _, test := range cleantests {
		allocs := allocsPerRun(100, func() { path.Clean(test.result) })
		atomic.AddInt64(&checks, 1)
		if allocs > 0 {
			t.Errorf("Clean(%q): %v allocs, want zero", test.result, allocs)
		}
	}
}

type SplitTest struct {
	path, dir, file string
}

var splittests = []SplitTest{
	{"a/b", "a/", "b"},
	{"a/b/", "a/b/", ""},
	{"a/", "a/", ""},
	{"a", "", "a"},
	{"/", "/", ""},
}

func TestSplit(t *testT) {
	for _, test := range splittests {
		atomic.AddInt64(&checks, 1)
		if d, f := path.Split(test.path); d != test.dir || f != test.file {
			t.Errorf("Split(%q) = %q, %q, want %q, %q", test.path, d, f, test.dir, test.file)
		}
	}
}

type JoinTest struct {
	elem []string
	path string
}

var jointests = []JoinTest{
	// zero parameters
	{[]string{}, ""},

	// one parameter
	{[]string{""}, ""},
	{[]string{"a"}, "a"},

	// two parameters
	{[]string{"a", "b"}, "a/b"},
	{[]string{"a", ""}, "a"},
	{[]string{"", "b"}, "b"},
	{[]string{"/", "a"}, "/a"},
	{[]string{"/", ""}, "/"},
	{[]string{"a/", "b"}, "a/b"},
	{[]string{"a/", ""}, "a"},
	{[]string{"", ""}, ""},
}

func TestJoin(t *testT) {
	for _, test := range jointests {
		atomic.AddInt64(&checks, 1)
		if p := path.Join(test.elem...); p != test.path {
			t.Errorf("Join(%q) = %q, want %q", test.elem, p, test.path)
		}
	}
}

type ExtTest struct {
	path, ext string
}

var exttests = []ExtTest{
	{"path.go", ".go"},
	{"path.pb.go", ".go"},
	{"a.dir/b", ""},
	{"a.dir/b.go", ".go"},
	{"a.dir/", ""},
}

func TestExt(t *testT) {
	for _, test := range exttests {
		atomic.AddInt64(&checks, 1)
		if x := path.Ext(test.path); x != test.ext {
			t.Errorf("Ext(%q) = %q, want %q", test.path, x, test.ext)
		}
	}
}

var basetests = []PathTest{
	// Already clean
	{"", "."},
	{".", "."},
	{"/.", "."},
	{"/", "/"},
	{"////", "/"},
	{"x/", "x"},
	{"abc", "abc"},
	{"abc/def", "def"},
	{"a/b/.x", ".x"},
	{"a/b/c.", "c."},
	{"a/b/c.x", "c.x"},
}

func TestBase(t *testT) {
	for _, test := range basetests {
		atomic.AddInt64(&checks, 1)
		if s := path.Base(test.path); s != test.result {
			t.Errorf("Base(%q) = %q, want %q", test.path, s, test.result)
		}
	}
}

var dirtests = []PathTest{
	{"", "."},
	{".", "."},
	{"/.", "/"},
	{"/", "/"},
	{"////", "/"},
	{"/foo", "/"},
	{"x/", "x"},
	{"abc", "."},
	{"abc/def", "abc"},
	{"abc////def", "abc"},
	{"a/b/.x", "a/b"},
	{"a/b/c.", "a/b"},
	{"a/b/c.x", "a/b"},
}

func TestDir(t *testT) {
	for _, test := range dirtests {
		atomic.AddInt64(&checks, 1)
		if s := path.Dir(test.path); s != test.result {
			t.Errorf("Dir(%q) = %q, want %q", test.path, s, test.result)
		}
	}
}

type IsAbsTest struct {
	path  string
	isAbs bool
}

var isAbsTests = []IsAbsTest{
	{"", false},
	{"/", true},
	{"/usr/bin/gcc", true},
	{"..", false},
	{"/a/../bb", true},
	{".", false},
	{"./", false},
	{"lala", false},
}

func TestIsAbs(t *testT) {
	for _, test := range isAbsTests {
		atomic.AddInt64(&checks, 1)
		if r := path.IsAbs(test.path); r != test.isAbs {
			t.Errorf("IsAbs(%q) = %v, want %v", test.path, r, test.isAbs)
		}
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
		{"TestClean", TestClean},
		{"TestCleanMallocs", TestCleanMallocs},
		{"TestSplit", TestSplit},
		{"TestJoin", TestJoin},
		{"TestExt", TestExt},
		{"TestBase", TestBase},
		{"TestDir", TestDir},
		{"TestIsAbs", TestIsAbs},
	}
	runTests(tests_)
}

// Output:
// ok TestClean subtests: 0 checks: 72
// ok TestCleanMallocs subtests: 0 checks: 1
// ok TestSplit subtests: 0 checks: 5
// ok TestJoin subtests: 0 checks: 11
// ok TestExt subtests: 0 checks: 5
// ok TestBase subtests: 0 checks: 11
// ok TestDir subtests: 0 checks: 13
// ok TestIsAbs subtests: 0 checks: 8
// tests: 8 failed: 0
