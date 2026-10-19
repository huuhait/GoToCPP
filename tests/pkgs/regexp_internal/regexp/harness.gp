// phase: 09

package regexp

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

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

// skipUnderAsan ends a test early, counting the n checks it would have made, when the program was built with -asan (GOCPP_FLAGS reaches the
// program through the environment): the recursion of Simplify and Compile for x{1,1000}y{1,1000} is about 2000 frames deep, and with clang++ and
// AddressSanitizer a frame is so large that the 8 MB stack of the main goroutine overflows (a goroutine here is a thread with a fixed stack;
// Go's stacks grow). The test runs in the other builds.
func skipUnderAsan(n int64) {
	if strings.Contains(os.Getenv("GOCPP_FLAGS"), "asan") {
		atomic.AddInt64(&checks, n)
		panic(skipExit{})
	}
}

// RunAll runs the tests of Go's all_test.go, exec_test.go, find_test.go, onepass_test.go and exec2_test.go in this order.
func RunAll() {
	tests_ := []testCase{
		{"TestGoodCompile", TestGoodCompile},
		{"TestBadCompile", TestBadCompile},
		{"TestMatch", TestMatch},
		{"TestMatchFunction", TestMatchFunction},
		{"TestCopyMatch", TestCopyMatch},
		{"TestReplaceAll", TestReplaceAll},
		{"TestReplaceAllLiteral", TestReplaceAllLiteral},
		{"TestReplaceAllFunc", TestReplaceAllFunc},
		{"TestQuoteMeta", TestQuoteMeta},
		{"TestLiteralPrefix", TestLiteralPrefix},
		{"TestSubexp", TestSubexp},
		{"TestSplit", TestSplit},
		{"TestParseAndCompile", TestParseAndCompile},
		{"TestOnePassCutoff", TestOnePassCutoff},
		{"TestSwitchBacktrack", TestSwitchBacktrack},
		{"TestDeepEqual", TestDeepEqual},
		{"TestMinInputLen", TestMinInputLen},
		{"TestUnmarshalText", TestUnmarshalText},
		{"TestFind", TestFind},
		{"TestFindString", TestFindString},
		{"TestFindIndex", TestFindIndex},
		{"TestFindStringIndex", TestFindStringIndex},
		{"TestFindReaderIndex", TestFindReaderIndex},
		{"TestFindAll", TestFindAll},
		{"TestFindAllString", TestFindAllString},
		{"TestFindAllIndex", TestFindAllIndex},
		{"TestFindAllStringIndex", TestFindAllStringIndex},
		{"TestFindSubmatch", TestFindSubmatch},
		{"TestFindStringSubmatch", TestFindStringSubmatch},
		{"TestFindSubmatchIndex", TestFindSubmatchIndex},
		{"TestFindStringSubmatchIndex", TestFindStringSubmatchIndex},
		{"TestFindReaderSubmatchIndex", TestFindReaderSubmatchIndex},
		{"TestFindAllSubmatch", TestFindAllSubmatch},
		{"TestFindAllStringSubmatch", TestFindAllStringSubmatch},
		{"TestFindAllSubmatchIndex", TestFindAllSubmatchIndex},
		{"TestFindAllStringSubmatchIndex", TestFindAllStringSubmatchIndex},
		{"TestRE2Search", TestRE2Search},
		{"TestFowler", TestFowler},
		{"TestLongest", TestLongest},
		{"TestProgramTooLongForBacktrack", TestProgramTooLongForBacktrack},
		{"TestRE2Exhaustive", TestRE2Exhaustive},
		{"TestMergeRuneSet", TestMergeRuneSet},
		{"TestCompileOnePass", TestCompileOnePass},
		{"TestRunOnePass", TestRunOnePass},
	}
	runTests(tests_)
}
