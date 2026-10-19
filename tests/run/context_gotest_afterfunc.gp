// phase: 09
// Go 1.25's context/afterfunc_test.go (AfterFunc: run on cancel, stop before and after, custom Context types) through a small harness; expected output from real Go.

package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)


const (
	shortDuration    = 1 * time.Millisecond // x_test.go
	veryLongDuration = 1000 * time.Hour
)

// afterFuncContext is a context that's not one of the types
// defined in context.go, that supports registering AfterFuncs.
type afterFuncContext struct {
	mu         sync.Mutex
	afterFuncs map[*byte]func()
	done       chan struct{}
	err        error
}

var _ context.Context = (*afterFuncContext)(nil)

func (c *afterFuncContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (c *afterFuncContext) Done() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	atomic.AddInt64(&checks, 1)
	if c.done == nil {
		c.done = make(chan struct{})
	}
	return c.done
}

func (c *afterFuncContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *afterFuncContext) Value(key any) any {
	return nil
}

func (c *afterFuncContext) AfterFunc(f func()) func() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := new(byte)
	atomic.AddInt64(&checks, 1)
	if c.afterFuncs == nil {
		c.afterFuncs = make(map[*byte]func())
	}
	c.afterFuncs[k] = f
	return func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		_, ok := c.afterFuncs[k]
		delete(c.afterFuncs, k)
		return ok
	}
}

func (c *afterFuncContext) cancel(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	atomic.AddInt64(&checks, 1)
	if c.err != nil {
		return
	}
	c.err = err
	for _, f := range c.afterFuncs {
		go f()
	}
	c.afterFuncs = nil
}

func TestCustomContextAfterFuncCancel(t *testT) {
	ctx0 := &afterFuncContext{}
	ctx1, cancel := context.WithCancel(ctx0)
	defer cancel()
	ctx0.cancel(context.Canceled)
	<-ctx1.Done()
}

func TestCustomContextAfterFuncTimeout(t *testT) {
	ctx0 := &afterFuncContext{}
	ctx1, cancel := context.WithTimeout(ctx0, veryLongDuration)
	defer cancel()
	ctx0.cancel(context.Canceled)
	<-ctx1.Done()
}

func TestCustomContextAfterFuncAfterFunc(t *testT) {
	ctx0 := &afterFuncContext{}
	donec := make(chan struct{})
	stop := context.AfterFunc(ctx0, func() {
		close(donec)
	})
	defer stop()
	ctx0.cancel(context.Canceled)
	<-donec
}

func TestCustomContextAfterFuncUnregisterCancel(t *testT) {
	ctx0 := &afterFuncContext{}
	_, cancel1 := context.WithCancel(ctx0)
	_, cancel2 := context.WithCancel(ctx0)
	atomic.AddInt64(&checks, 1)
	if got, want := len(ctx0.afterFuncs), 2; got != want {
		t.Errorf("after WithCancel(ctx0): ctx0 has %v afterFuncs, want %v", got, want)
	}
	cancel1()
	cancel2()
	atomic.AddInt64(&checks, 1)
	if got, want := len(ctx0.afterFuncs), 0; got != want {
		t.Errorf("after canceling WithCancel(ctx0): ctx0 has %v afterFuncs, want %v", got, want)
	}
}

func TestCustomContextAfterFuncUnregisterTimeout(t *testT) {
	ctx0 := &afterFuncContext{}
	_, cancel := context.WithTimeout(ctx0, veryLongDuration)
	atomic.AddInt64(&checks, 1)
	if got, want := len(ctx0.afterFuncs), 1; got != want {
		t.Errorf("after WithTimeout(ctx0, d): ctx0 has %v afterFuncs, want %v", got, want)
	}
	cancel()
	atomic.AddInt64(&checks, 1)
	if got, want := len(ctx0.afterFuncs), 0; got != want {
		t.Errorf("after canceling WithTimeout(ctx0, d): ctx0 has %v afterFuncs, want %v", got, want)
	}
}

func TestCustomContextAfterFuncUnregisterAfterFunc(t *testT) {
	ctx0 := &afterFuncContext{}
	stop := context.AfterFunc(ctx0, func() {})
	atomic.AddInt64(&checks, 1)
	if got, want := len(ctx0.afterFuncs), 1; got != want {
		t.Errorf("after AfterFunc(ctx0, f): ctx0 has %v afterFuncs, want %v", got, want)
	}
	stop()
	atomic.AddInt64(&checks, 1)
	if got, want := len(ctx0.afterFuncs), 0; got != want {
		t.Errorf("after stopping AfterFunc(ctx0, f): ctx0 has %v afterFuncs, want %v", got, want)
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
		{"TestCustomContextAfterFuncCancel", TestCustomContextAfterFuncCancel},
		{"TestCustomContextAfterFuncTimeout", TestCustomContextAfterFuncTimeout},
		{"TestCustomContextAfterFuncAfterFunc", TestCustomContextAfterFuncAfterFunc},
		{"TestCustomContextAfterFuncUnregisterCancel", TestCustomContextAfterFuncUnregisterCancel},
		{"TestCustomContextAfterFuncUnregisterTimeout", TestCustomContextAfterFuncUnregisterTimeout},
		{"TestCustomContextAfterFuncUnregisterAfterFunc", TestCustomContextAfterFuncUnregisterAfterFunc},
	}
	runTests(tests_)
}

// Output:
// ok TestCustomContextAfterFuncCancel subtests: 0 checks: 4
// ok TestCustomContextAfterFuncTimeout subtests: 0 checks: 4
// ok TestCustomContextAfterFuncAfterFunc subtests: 0 checks: 4
// ok TestCustomContextAfterFuncUnregisterCancel subtests: 0 checks: 8
// ok TestCustomContextAfterFuncUnregisterTimeout subtests: 0 checks: 5
// ok TestCustomContextAfterFuncUnregisterAfterFunc subtests: 0 checks: 5
// tests: 6 failed: 0
