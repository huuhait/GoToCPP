// phase: 09
// Go 1.25's maps/maps_test.go (Equal, EqualFunc, Clone incl. NaN keys and a map that grows, Copy, DeleteFunc, with a custom map type and a map with NaN keys) through a small harness; expected output from real Go.

package main

import (
	"maps"
	"fmt"
	"math"
	"strconv"
	"sync/atomic"
)


var m1 = map[int]int{1: 2, 2: 4, 4: 8, 8: 16}
var m2 = map[int]string{1: "2", 2: "4", 4: "8", 8: "16"}

func TestEqual(t *testT) {
	atomic.AddInt64(&checks, 1)
	if !maps.Equal(m1, m1) {
		t.Errorf("Equal(%v, %v) = false, want true", m1, m1)
	}
	atomic.AddInt64(&checks, 1)
	if maps.Equal(m1, (map[int]int)(nil)) {
		t.Errorf("Equal(%v, nil) = true, want false", m1)
	}
	atomic.AddInt64(&checks, 1)
	if maps.Equal((map[int]int)(nil), m1) {
		t.Errorf("Equal(nil, %v) = true, want false", m1)
	}
	atomic.AddInt64(&checks, 1)
	if !maps.Equal[map[int]int, map[int]int](nil, nil) {
		t.Error("Equal(nil, nil) = false, want true")
	}
	atomic.AddInt64(&checks, 1)
	if ms := map[int]int{1: 2}; maps.Equal(m1, ms) {
		t.Errorf("Equal(%v, %v) = true, want false", m1, ms)
	}

	// Comparing NaN for equality is expected to fail.
	mf := map[int]float64{1: 0, 2: math.NaN()}
	atomic.AddInt64(&checks, 1)
	if maps.Equal(mf, mf) {
		t.Errorf("Equal(%v, %v) = true, want false", mf, mf)
	}
}

// equal is simply ==.
func equal[T comparable](v1, v2 T) bool {
	return v1 == v2
}

// equalNaN is like == except that all NaNs are equal.
func equalNaN[T comparable](v1, v2 T) bool {
	isNaN := func(f T) bool { return f != f }
	return v1 == v2 || (isNaN(v1) && isNaN(v2))
}

// equalIntStr compares ints and strings.
func equalIntStr(v1 int, v2 string) bool {
	return strconv.Itoa(v1) == v2
}

func TestEqualFunc(t *testT) {
	atomic.AddInt64(&checks, 1)
	if !maps.EqualFunc(m1, m1, equal[int]) {
		t.Errorf("EqualFunc(%v, %v, equal) = false, want true", m1, m1)
	}
	atomic.AddInt64(&checks, 1)
	if maps.EqualFunc(m1, (map[int]int)(nil), equal[int]) {
		t.Errorf("EqualFunc(%v, nil, equal) = true, want false", m1)
	}
	atomic.AddInt64(&checks, 1)
	if maps.EqualFunc((map[int]int)(nil), m1, equal[int]) {
		t.Errorf("EqualFunc(nil, %v, equal) = true, want false", m1)
	}
	atomic.AddInt64(&checks, 1)
	if !maps.EqualFunc[map[int]int, map[int]int](nil, nil, equal[int]) {
		t.Error("EqualFunc(nil, nil, equal) = false, want true")
	}
	atomic.AddInt64(&checks, 1)
	if ms := map[int]int{1: 2}; maps.EqualFunc(m1, ms, equal[int]) {
		t.Errorf("EqualFunc(%v, %v, equal) = true, want false", m1, ms)
	}

	// Comparing NaN for equality is expected to fail.
	mf := map[int]float64{1: 0, 2: math.NaN()}
	atomic.AddInt64(&checks, 1)
	if maps.EqualFunc(mf, mf, equal[float64]) {
		t.Errorf("EqualFunc(%v, %v, equal) = true, want false", mf, mf)
	}
	// But it should succeed using equalNaN.
	atomic.AddInt64(&checks, 1)
	if !maps.EqualFunc(mf, mf, equalNaN[float64]) {
		t.Errorf("EqualFunc(%v, %v, equalNaN) = false, want true", mf, mf)
	}

	atomic.AddInt64(&checks, 1)
	if !maps.EqualFunc(m1, m2, equalIntStr) {
		t.Errorf("EqualFunc(%v, %v, equalIntStr) = false, want true", m1, m2)
	}
}

func TestClone(t *testT) {
	mc := maps.Clone(m1)
	atomic.AddInt64(&checks, 1)
	if !maps.Equal(mc, m1) {
		t.Errorf("Clone(%v) = %v, want %v", m1, mc, m1)
	}
	mc[16] = 32
	atomic.AddInt64(&checks, 1)
	if maps.Equal(mc, m1) {
		t.Errorf("Equal(%v, %v) = true, want false", mc, m1)
	}
}

func TestCloneNil(t *testT) {
	var m1 map[string]int
	mc := maps.Clone(m1)
	atomic.AddInt64(&checks, 1)
	if mc != nil {
		t.Errorf("Clone(%v) = %v, want %v", m1, mc, m1)
	}
}

func TestCopy(t *testT) {
	mc := maps.Clone(m1)
	maps.Copy(mc, mc)
	atomic.AddInt64(&checks, 1)
	if !maps.Equal(mc, m1) {
		t.Errorf("Copy(%v, %v) = %v, want %v", m1, m1, mc, m1)
	}
	maps.Copy(mc, map[int]int{16: 32})
	want := map[int]int{1: 2, 2: 4, 4: 8, 8: 16, 16: 32}
	atomic.AddInt64(&checks, 1)
	if !maps.Equal(mc, want) {
		t.Errorf("Copy result = %v, want %v", mc, want)
	}

	type M1 map[int]bool
	type M2 map[int]bool
	maps.Copy(make(M1), make(M2))
}

func TestDeleteFunc(t *testT) {
	mc := maps.Clone(m1)
	maps.DeleteFunc(mc, func(int, int) bool { return false })
	atomic.AddInt64(&checks, 1)
	if !maps.Equal(mc, m1) {
		t.Errorf("DeleteFunc(%v, true) = %v, want %v", m1, mc, m1)
	}
	maps.DeleteFunc(mc, func(k, v int) bool { return k > 3 })
	want := map[int]int{1: 2, 2: 4}
	atomic.AddInt64(&checks, 1)
	if !maps.Equal(mc, want) {
		t.Errorf("DeleteFunc result = %v, want %v", mc, want)
	}
}

var n map[int]int


func TestCloneWithDelete(t *testT) {
	var m = make(map[int]int)
	for i := 0; i < 32; i++ {
		m[i] = i
	}
	for i := 8; i < 32; i++ {
		delete(m, i)
	}
	m2 := maps.Clone(m)
	atomic.AddInt64(&checks, 1)
	if len(m2) != 8 {
		t.Errorf("len2(m2) = %d, want %d", len(m2), 8)
	}
	for i := 0; i < 8; i++ {
		atomic.AddInt64(&checks, 1)
		if m2[i] != m[i] {
			t.Errorf("m2[%d] = %d, want %d", i, m2[i], m[i])
		}
	}
}

func TestCloneWithMapAssign(t *testT) {
	var m = make(map[int]int)
	const N = 25
	for i := 0; i < N; i++ {
		m[i] = i
	}
	m2 := maps.Clone(m)
	atomic.AddInt64(&checks, 1)
	if len(m2) != N {
		t.Errorf("len2(m2) = %d, want %d", len(m2), N)
	}
	for i := 0; i < N; i++ {
		atomic.AddInt64(&checks, 1)
		if m2[i] != m[i] {
			t.Errorf("m2[%d] = %d, want %d", i, m2[i], m[i])
		}
	}
}

func TestCloneLarge(t *testT) {
	// See issue 64474.
	type K [17]float64 // > 128 bytes
	type V [17]float64

	var zero float64
	negZero := -zero

	for tst := 0; tst < 3; tst++ {
		// Initialize m with a key and value.
		m := map[K]V{}
		var k1 K
		var v1 V
		m[k1] = v1

		switch tst {
		case 0: // nothing, just a 1-entry map
		case 1:
			// Add more entries to make it 2 buckets
			// 1 entry already
			// 7 more fill up 1 bucket
			// 1 more to grow to 2 buckets
			for i := 0; i < 7+1; i++ {
				m[K{float64(i) + 1}] = V{}
			}
		case 2:
			// Capture the map mid-grow
			// 1 entry already
			// 7 more fill up 1 bucket
			// 5 more (13 total) fill up 2 buckets
			// 13 more (26 total) fill up 4 buckets
			// 1 more to start the 4->8 bucket grow
			for i := 0; i < 7+5+13+1; i++ {
				m[K{float64(i) + 1}] = V{}
			}
		}

		// Clone m, which should freeze the map's contents.
		c := maps.Clone(m)

		// Update m with new key and value.
		k2, v2 := k1, v1
		k2[0] = negZero
		v2[0] = 1.0
		m[k2] = v2

		// Make sure c still has its old key and value.
		for k, v := range c {
			atomic.AddInt64(&checks, 1)
			if math.Signbit(k[0]) {
				t.Errorf("tst%d: sign bit of key changed; got %v want %v", tst, k, k1)
			}
			atomic.AddInt64(&checks, 1)
			if v != v1 {
				t.Errorf("tst%d: value changed; got %v want %v", tst, v, v1)
			}
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
		{"TestEqual", TestEqual},
		{"TestEqualFunc", TestEqualFunc},
		{"TestClone", TestClone},
		{"TestCloneNil", TestCloneNil},
		{"TestCopy", TestCopy},
		{"TestDeleteFunc", TestDeleteFunc},
		{"TestCloneWithDelete", TestCloneWithDelete},
		{"TestCloneWithMapAssign", TestCloneWithMapAssign},
		{"TestCloneLarge", TestCloneLarge},
	}
	runTests(tests_)
}

// Output:
// ok TestEqual subtests: 0 checks: 6
// ok TestEqualFunc subtests: 0 checks: 8
// ok TestClone subtests: 0 checks: 2
// ok TestCloneNil subtests: 0 checks: 1
// ok TestCopy subtests: 0 checks: 2
// ok TestDeleteFunc subtests: 0 checks: 2
// ok TestCloneWithDelete subtests: 0 checks: 9
// ok TestCloneWithMapAssign subtests: 0 checks: 26
// ok TestCloneLarge subtests: 0 checks: 74
// tests: 9 failed: 0
