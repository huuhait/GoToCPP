// phase: 09
// Go 1.25's path/example_test.go (Base, Clean, Dir, Ext, IsAbs, Join, Match, Split) run in order; expected output from real Go.

package main

import (
	"fmt"
	"path"
	"sync/atomic"
)


func ExampleBase() {
	fmt.Println(path.Base("/a/b"))
	fmt.Println(path.Base("/"))
	fmt.Println(path.Base(""))

}

func ExampleClean() {
	paths := []string{
		"a/c",
		"a//c",
		"a/c/.",
		"a/c/b/..",
		"/../a/c",
		"/../a/b/../././/c",
		"",
	}

	for _, p := range paths {
		fmt.Printf("Clean(%q) = %q\n", p, path.Clean(p))
	}

}

func ExampleDir() {
	fmt.Println(path.Dir("/a/b/c"))
	fmt.Println(path.Dir("a/b/c"))
	fmt.Println(path.Dir("/a/"))
	fmt.Println(path.Dir("a/"))
	fmt.Println(path.Dir("/"))
	fmt.Println(path.Dir(""))

}

func ExampleExt() {
	fmt.Println(path.Ext("/a/b/c/bar.css"))
	fmt.Println(path.Ext("/"))
	fmt.Println(path.Ext(""))

}

func ExampleIsAbs() {
	fmt.Println(path.IsAbs("/dev/null"))

}

func ExampleJoin() {
	fmt.Println(path.Join("a", "b", "c"))
	fmt.Println(path.Join("a", "b/c"))
	fmt.Println(path.Join("a/b", "c"))

	fmt.Println(path.Join("a/b", "../../../xyz"))

	fmt.Println(path.Join("", ""))
	fmt.Println(path.Join("a", ""))
	fmt.Println(path.Join("", "a"))

}

func ExampleMatch() {
	fmt.Println(path.Match("abc", "abc"))
	fmt.Println(path.Match("a*", "abc"))
	fmt.Println(path.Match("a*/b", "a/c/b"))

}

func ExampleSplit() {
	split := func(s string) {
		dir, file := path.Split(s)
		fmt.Printf("path.Split(%q) = dir: %q, file: %q\n", s, dir, file)
	}
	split("static/myfile.css")
	split("myfile.css")
	split("")

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
	fmt.Println("== ExampleBase")
	ExampleBase()
	fmt.Println("== ExampleClean")
	ExampleClean()
	fmt.Println("== ExampleDir")
	ExampleDir()
	fmt.Println("== ExampleExt")
	ExampleExt()
	fmt.Println("== ExampleIsAbs")
	ExampleIsAbs()
	fmt.Println("== ExampleJoin")
	ExampleJoin()
	fmt.Println("== ExampleMatch")
	ExampleMatch()
	fmt.Println("== ExampleSplit")
	ExampleSplit()
}

// Output:
// == ExampleBase
// b
// /
// .
// == ExampleClean
// Clean("a/c") = "a/c"
// Clean("a//c") = "a/c"
// Clean("a/c/.") = "a/c"
// Clean("a/c/b/..") = "a/c"
// Clean("/../a/c") = "/a/c"
// Clean("/../a/b/../././/c") = "/a/c"
// Clean("") = "."
// == ExampleDir
// /a/b
// a/b
// /a
// a
// /
// .
// == ExampleExt
// .css
//
//
// == ExampleIsAbs
// true
// == ExampleJoin
// a/b/c
// a/b/c
// a/b/c
// ../xyz
//
// a
// a
// == ExampleMatch
// true <nil>
// true <nil>
// false <nil>
// == ExampleSplit
// path.Split("static/myfile.css") = dir: "static/", file: "myfile.css"
// path.Split("myfile.css") = dir: "", file: "myfile.css"
// path.Split("") = dir: "", file: ""
