// phase: 09
// Go 1.25's maps/iter_test.go (All, Keys, Values, Insert, Collect, and the early exit of each) through a small harness; expected output from real Go.

package main

import (
	"maps"
	"fmt"
	"slices"
	"sync/atomic"
)


func TestAll(t *testT) {
	for size := 0; size < 10; size++ {
		m := make(map[int]int)
		for i := range size {
			m[i] = i
		}
		cnt := 0
		for i, v := range maps.All(m) {
			v1, ok := m[i]
			atomic.AddInt64(&checks, 1)
			if !ok || v != v1 {
				t.Errorf("at iteration %d got %d, %d want %d, %d", cnt, i, v, i, v1)
			}
			cnt++
		}
		atomic.AddInt64(&checks, 1)
		if cnt != size {
			t.Errorf("read %d values expected %d", cnt, size)
		}
	}
}

func TestKeys(t *testT) {
	for size := 0; size < 10; size++ {
		var want []int
		m := make(map[int]int)
		for i := range size {
			m[i] = i
			want = append(want, i)
		}

		var got []int
		for k := range maps.Keys(m) {
			got = append(got, k)
		}
		slices.Sort(got)
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(got, want) {
			t.Errorf("Keys(%v) = %v, want %v", m, got, want)
		}
	}
}

func TestValues(t *testT) {
	for size := 0; size < 10; size++ {
		var want []int
		m := make(map[int]int)
		for i := range size {
			m[i] = i
			want = append(want, i)
		}

		var got []int
		for v := range maps.Values(m) {
			got = append(got, v)
		}
		slices.Sort(got)
		atomic.AddInt64(&checks, 1)
		if !slices.Equal(got, want) {
			t.Errorf("Values(%v) = %v, want %v", m, got, want)
		}
	}
}

func TestInsert(t *testT) {
	got := map[int]int{
		1: 1,
		2: 1,
	}
	maps.Insert(got, func(yield func(int, int) bool) {
		for i := 0; i < 10; i += 2 {
			atomic.AddInt64(&checks, 1)
			if !yield(i, i+1) {
				return
			}
		}
	})

	want := map[int]int{
		1: 1,
		2: 1,
	}
	for i, v := range map[int]int{
		0: 1,
		2: 3,
		4: 5,
		6: 7,
		8: 9,
	} {
		want[i] = v
	}

	atomic.AddInt64(&checks, 1)
	if !maps.Equal(got, want) {
		t.Errorf("Insert got: %v, want: %v", got, want)
	}
}

func TestCollect(t *testT) {
	m := map[int]int{
		0: 1,
		2: 3,
		4: 5,
		6: 7,
		8: 9,
	}
	got := maps.Collect(maps.All(m))
	atomic.AddInt64(&checks, 1)
	if !maps.Equal(got, m) {
		t.Errorf("Collect got: %v, want: %v", got, m)
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
		{"TestAll", TestAll},
		{"TestKeys", TestKeys},
		{"TestValues", TestValues},
		{"TestInsert", TestInsert},
		{"TestCollect", TestCollect},
	}
	runTests(tests_)
}

// Output:
// ok TestAll subtests: 0 checks: 55
// ok TestKeys subtests: 0 checks: 10
// ok TestValues subtests: 0 checks: 10
// ok TestInsert subtests: 0 checks: 6
// ok TestCollect subtests: 0 checks: 1
// tests: 5 failed: 0
