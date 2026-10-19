// phase: 09
// Go 1.25's iter/pull_test.go (Pull and Pull2: values, early stop, double next/yield, panics and runtime.Goexit propagation, runtime.NumGoroutine before and after) through a small harness; expected output from real Go.

package main

import (
	"fmt"
	"iter"
	"runtime"
	"sync/atomic"
)


func count(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := range n {
			atomic.AddInt64(&checks, 1)
			if !yield(i) {
				break
			}
		}
	}
}

func squares(n int) iter.Seq2[int, int64] {
	return func(yield func(int, int64) bool) {
		for i := range n {
			atomic.AddInt64(&checks, 1)
			if !yield(i, int64(i)*int64(i)) {
				break
			}
		}
	}
}

func TestPull(t *testT) {
	for end := 0; end <= 3; end++ {
		t.Run(fmt.Sprint(end), func(t *testT) {
			ng := stableNumGoroutine()
			wantNG := func(want int) {
				atomic.AddInt64(&checks, 1)
				if xg := runtime.NumGoroutine() - ng; xg != want {
					t.Helper()
					t.Errorf("have %d extra goroutines, want %d", xg, want)
				}
			}
			wantNG(0)
			next, stop := iter.Pull(count(3))
			wantNG(1)
			for i := range end {
				v, ok := next()
				atomic.AddInt64(&checks, 1)
				if v != i || ok != true {
					t.Fatalf("next() = %d, %v, want %d, %v", v, ok, i, true)
				}
				wantNG(1)
			}
			wantNG(1)
			atomic.AddInt64(&checks, 1)
			if end < 3 {
				stop()
				wantNG(0)
			}
			for range 2 {
				v, ok := next()
				atomic.AddInt64(&checks, 1)
				if v != 0 || ok != false {
					t.Fatalf("next() = %d, %v, want %d, %v", v, ok, 0, false)
				}
				wantNG(0)
			}
			wantNG(0)

			stop()
			stop()
			stop()
			wantNG(0)
		})
	}
}

func TestPull2(t *testT) {
	for end := 0; end <= 3; end++ {
		t.Run(fmt.Sprint(end), func(t *testT) {
			ng := stableNumGoroutine()
			wantNG := func(want int) {
				atomic.AddInt64(&checks, 1)
				if xg := runtime.NumGoroutine() - ng; xg != want {
					t.Helper()
					t.Errorf("have %d extra goroutines, want %d", xg, want)
				}
			}
			wantNG(0)
			next, stop := iter.Pull2(squares(3))
			wantNG(1)
			for i := range end {
				k, v, ok := next()
				atomic.AddInt64(&checks, 1)
				if k != i || v != int64(i*i) || ok != true {
					t.Fatalf("next() = %d, %d, %v, want %d, %d, %v", k, v, ok, i, i*i, true)
				}
				wantNG(1)
			}
			wantNG(1)
			atomic.AddInt64(&checks, 1)
			if end < 3 {
				stop()
				wantNG(0)
			}
			for range 2 {
				k, v, ok := next()
				atomic.AddInt64(&checks, 1)
				if v != 0 || ok != false {
					t.Fatalf("next() = %d, %d, %v, want %d, %d, %v", k, v, ok, 0, 0, false)
				}
				wantNG(0)
			}
			wantNG(0)

			stop()
			stop()
			stop()
			wantNG(0)
		})
	}
}

// stableNumGoroutine is like NumGoroutine but tries to ensure stability of
// the value by letting any exiting goroutines finish exiting.
func stableNumGoroutine() int {
	// The idea behind stablizing the value of NumGoroutine is to
	// see the same value enough times in a row in between calls to
	// runtime.Gosched. With GOMAXPROCS=1, we're trying to make sure
	// that other goroutines run, so that they reach a stable point.
	// It's not guaranteed, because it is still possible for a goroutine
	// to Gosched back into itself, so we require NumGoroutine to be
	// the same 100 times in a row. This should be more than enough to
	// ensure all goroutines get a chance to run to completion (or to
	// some block point) for a small group of test goroutines.
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))

	c := 0
	ng := runtime.NumGoroutine()
	for i := 0; i < 1000; i++ {
		nng := runtime.NumGoroutine()
		atomic.AddInt64(&checks, 1)
		if nng == ng {
			c++
		} else {
			c = 0
			ng = nng
		}
		atomic.AddInt64(&checks, 1)
		if c >= 100 {
			// The same value 100 times in a row is good enough.
			return ng
		}
		runtime.Gosched()
	}
	panic("failed to stabilize NumGoroutine after 1000 iterations")
}

func TestPullDoubleNext(t *testT) {
	next, _ := iter.Pull(doDoubleNext())
	nextSlot = next
	next()
	atomic.AddInt64(&checks, 1)
	if nextSlot != nil {
		t.Fatal("double next did not fail")
	}
}

var nextSlot func() (int, bool)

func doDoubleNext() iter.Seq[int] {
	return func(_ func(int) bool) {
		defer func() {
			atomic.AddInt64(&checks, 1)
			if recover() != nil {
				nextSlot = nil
			}
		}()
		nextSlot()
	}
}

func TestPullDoubleNext2(t *testT) {
	next, _ := iter.Pull2(doDoubleNext2())
	nextSlot2 = next
	next()
	atomic.AddInt64(&checks, 1)
	if nextSlot2 != nil {
		t.Fatal("double next did not fail")
	}
}

var nextSlot2 func() (int, int, bool)

func doDoubleNext2() iter.Seq2[int, int] {
	return func(_ func(int, int) bool) {
		defer func() {
			atomic.AddInt64(&checks, 1)
			if recover() != nil {
				nextSlot2 = nil
			}
		}()
		nextSlot2()
	}
}

func TestPullDoubleYield(t *testT) {
	next, stop := iter.Pull(storeYield())
	next()
	atomic.AddInt64(&checks, 1)
	if yieldSlot == nil {
		t.Fatal("yield failed")
	}
	defer func() {
		atomic.AddInt64(&checks, 1)
		if recover() != nil {
			yieldSlot = nil
		}
		stop()
	}()
	yieldSlot(5)
	atomic.AddInt64(&checks, 1)
	if yieldSlot != nil {
		t.Fatal("double yield did not fail")
	}
}

func storeYield() iter.Seq[int] {
	return func(yield func(int) bool) {
		yieldSlot = yield
		atomic.AddInt64(&checks, 1)
		if !yield(5) {
			return
		}
	}
}

var yieldSlot func(int) bool

func TestPullDoubleYield2(t *testT) {
	next, stop := iter.Pull2(storeYield2())
	next()
	atomic.AddInt64(&checks, 1)
	if yieldSlot2 == nil {
		t.Fatal("yield failed")
	}
	defer func() {
		atomic.AddInt64(&checks, 1)
		if recover() != nil {
			yieldSlot2 = nil
		}
		stop()
	}()
	yieldSlot2(23, 77)
	atomic.AddInt64(&checks, 1)
	if yieldSlot2 != nil {
		t.Fatal("double yield did not fail")
	}
}

func storeYield2() iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		yieldSlot2 = yield
		atomic.AddInt64(&checks, 1)
		if !yield(23, 77) {
			return
		}
	}
}

var yieldSlot2 func(int, int) bool

func TestPullPanic(t *testT) {
	t.Run("next", func(t *testT) {
		next, stop := iter.Pull(panicSeq())
		atomic.AddInt64(&checks, 1)
		if !panicsWith("boom", func() { next() }) {
			t.Fatal("failed to propagate panic on first next")
		}
		// Make sure we don't panic again if we try to call next or stop.
		atomic.AddInt64(&checks, 1)
		if _, ok := next(); ok {
			t.Fatal("next returned true after iterator panicked")
		}
		// Calling stop again should be a no-op.
		stop()
	})
	t.Run("stop", func(t *testT) {
		next, stop := iter.Pull(panicCleanupSeq())
		x, ok := next()
		atomic.AddInt64(&checks, 1)
		if !ok || x != 55 {
			t.Fatalf("expected (55, true) from next, got (%d, %t)", x, ok)
		}
		atomic.AddInt64(&checks, 1)
		if !panicsWith("boom", func() { stop() }) {
			t.Fatal("failed to propagate panic on stop")
		}
		// Make sure we don't panic again if we try to call next or stop.
		atomic.AddInt64(&checks, 1)
		if _, ok := next(); ok {
			t.Fatal("next returned true after iterator panicked")
		}
		// Calling stop again should be a no-op.
		stop()
	})
}

func panicSeq() iter.Seq[int] {
	return func(yield func(int) bool) {
		panic("boom")
	}
}

func panicCleanupSeq() iter.Seq[int] {
	return func(yield func(int) bool) {
		for {
			atomic.AddInt64(&checks, 1)
			if !yield(55) {
				panic("boom")
			}
		}
	}
}

func TestPull2Panic(t *testT) {
	t.Run("next", func(t *testT) {
		next, stop := iter.Pull2(panicSeq2())
		atomic.AddInt64(&checks, 1)
		if !panicsWith("boom", func() { next() }) {
			t.Fatal("failed to propagate panic on first next")
		}
		// Make sure we don't panic again if we try to call next or stop.
		atomic.AddInt64(&checks, 1)
		if _, _, ok := next(); ok {
			t.Fatal("next returned true after iterator panicked")
		}
		// Calling stop again should be a no-op.
		stop()
	})
	t.Run("stop", func(t *testT) {
		next, stop := iter.Pull2(panicCleanupSeq2())
		x, y, ok := next()
		atomic.AddInt64(&checks, 1)
		if !ok || x != 55 || y != 100 {
			t.Fatalf("expected (55, 100, true) from next, got (%d, %d, %t)", x, y, ok)
		}
		atomic.AddInt64(&checks, 1)
		if !panicsWith("boom", func() { stop() }) {
			t.Fatal("failed to propagate panic on stop")
		}
		// Make sure we don't panic again if we try to call next or stop.
		atomic.AddInt64(&checks, 1)
		if _, _, ok := next(); ok {
			t.Fatal("next returned true after iterator panicked")
		}
		// Calling stop again should be a no-op.
		stop()
	})
}

func panicSeq2() iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		panic("boom")
	}
}

func panicCleanupSeq2() iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		for {
			atomic.AddInt64(&checks, 1)
			if !yield(55, 100) {
				panic("boom")
			}
		}
	}
}

func panicsWith(v any, f func()) (panicked bool) {
	defer func() {
		atomic.AddInt64(&checks, 1)
		if r := recover(); r != nil {
			atomic.AddInt64(&checks, 1)
			if r != v {
				panic(r)
			}
			panicked = true
		}
	}()
	f()
	return
}

func TestPullGoexit(t *testT) {
	t.Run("next", func(t *testT) {
		var next func() (int, bool)
		var stop func()
		atomic.AddInt64(&checks, 1)
		if !goexits(t, func() {
			next, stop = iter.Pull(goexitSeq())
			next()
		}) {
			t.Fatal("failed to Goexit from next")
		}
		atomic.AddInt64(&checks, 1)
		if x, ok := next(); x != 0 || ok {
			t.Fatal("iterator returned valid value after iterator Goexited")
		}
		stop()
	})
	t.Run("stop", func(t *testT) {
		next, stop := iter.Pull(goexitCleanupSeq())
		x, ok := next()
		atomic.AddInt64(&checks, 1)
		if !ok || x != 55 {
			t.Fatalf("expected (55, true) from next, got (%d, %t)", x, ok)
		}
		atomic.AddInt64(&checks, 1)
		if !goexits(t, func() {
			stop()
		}) {
			t.Fatal("failed to Goexit from stop")
		}
		// Make sure we don't panic again if we try to call next or stop.
		atomic.AddInt64(&checks, 1)
		if x, ok := next(); x != 0 || ok {
			t.Fatal("next returned true or non-zero value after iterator Goexited")
		}
		// Calling stop again should be a no-op.
		stop()
	})
}

func goexitSeq() iter.Seq[int] {
	return func(yield func(int) bool) {
		runtime.Goexit()
	}
}

func goexitCleanupSeq() iter.Seq[int] {
	return func(yield func(int) bool) {
		for {
			atomic.AddInt64(&checks, 1)
			if !yield(55) {
				runtime.Goexit()
			}
		}
	}
}

func TestPull2Goexit(t *testT) {
	t.Run("next", func(t *testT) {
		var next func() (int, int, bool)
		var stop func()
		atomic.AddInt64(&checks, 1)
		if !goexits(t, func() {
			next, stop = iter.Pull2(goexitSeq2())
			next()
		}) {
			t.Fatal("failed to Goexit from next")
		}
		atomic.AddInt64(&checks, 1)
		if x, y, ok := next(); x != 0 || y != 0 || ok {
			t.Fatal("iterator returned valid value after iterator Goexited")
		}
		stop()
	})
	t.Run("stop", func(t *testT) {
		next, stop := iter.Pull2(goexitCleanupSeq2())
		x, y, ok := next()
		atomic.AddInt64(&checks, 1)
		if !ok || x != 55 || y != 100 {
			t.Fatalf("expected (55, 100, true) from next, got (%d, %d, %t)", x, y, ok)
		}
		atomic.AddInt64(&checks, 1)
		if !goexits(t, func() {
			stop()
		}) {
			t.Fatal("failed to Goexit from stop")
		}
		// Make sure we don't panic again if we try to call next or stop.
		atomic.AddInt64(&checks, 1)
		if x, y, ok := next(); x != 0 || y != 0 || ok {
			t.Fatal("next returned true or non-zero after iterator Goexited")
		}
		// Calling stop again should be a no-op.
		stop()
	})
}

func goexitSeq2() iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		runtime.Goexit()
	}
}

func goexitCleanupSeq2() iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		for {
			atomic.AddInt64(&checks, 1)
			if !yield(55, 100) {
				runtime.Goexit()
			}
		}
	}
}

func goexits(t *testT, f func()) bool {
	t.Helper()

	exit := make(chan bool)
	go func() {
		cleanExit := false
		defer func() {
			exit <- recover() == nil && !cleanExit
		}()
		f()
		cleanExit = true
	}()
	return <-exit
}

func TestPullImmediateStop(t *testT) {
	next, stop := iter.Pull(panicSeq())
	stop()
	// Make sure we don't panic if we try to call next or stop.
	atomic.AddInt64(&checks, 1)
	if _, ok := next(); ok {
		t.Fatal("next returned true after iterator was stopped")
	}
}

func TestPull2ImmediateStop(t *testT) {
	next, stop := iter.Pull2(panicSeq2())
	stop()
	// Make sure we don't panic if we try to call next or stop.
	atomic.AddInt64(&checks, 1)
	if _, _, ok := next(); ok {
		t.Fatal("next returned true after iterator was stopped")
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
		{"TestPull", TestPull},
		{"TestPull2", TestPull2},
		{"TestPullDoubleNext", TestPullDoubleNext},
		{"TestPullDoubleNext2", TestPullDoubleNext2},
		{"TestPullDoubleYield", TestPullDoubleYield},
		{"TestPullDoubleYield2", TestPullDoubleYield2},
		{"TestPullPanic", TestPullPanic},
		{"TestPull2Panic", TestPull2Panic},
		{"TestPullGoexit", TestPullGoexit},
		{"TestPull2Goexit", TestPull2Goexit},
		{"TestPullImmediateStop", TestPullImmediateStop},
		{"TestPull2ImmediateStop", TestPull2ImmediateStop},
	}
	runTests(tests_)
}

// Output:
// ok TestPull subtests: 4 checks: 861
// ok TestPull2 subtests: 4 checks: 861
// ok TestPullDoubleNext subtests: 0 checks: 2
// ok TestPullDoubleNext2 subtests: 0 checks: 2
// ok TestPullDoubleYield subtests: 0 checks: 3
// ok TestPullDoubleYield2 subtests: 0 checks: 3
// ok TestPullPanic subtests: 2 checks: 10
// ok TestPull2Panic subtests: 2 checks: 10
// ok TestPullGoexit subtests: 2 checks: 6
// ok TestPull2Goexit subtests: 2 checks: 6
// ok TestPullImmediateStop subtests: 0 checks: 1
// ok TestPull2ImmediateStop subtests: 0 checks: 1
// tests: 12 failed: 0
