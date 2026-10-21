package server

import (
	"sync/atomic"
	"testing"
	"time"
)

// testGC is an idleGC with a heap that the test sets and a collector that counts.
func testGC(objects, live uint64) (*idleGC, *atomic.Int64) {
	var n atomic.Int64
	g := newIdleGC(false, nil) // (true would also set the memory limit of the test process)
	g.disabled = false
	g.heap = func() (uint64, uint64) { return objects, live }
	g.collect = func() { n.Add(1) }
	return g, &n
}

const mib = 1 << 20

func TestIdleGCThreshold(t *testing.T) {
	for _, tc := range []struct {
		name          string
		objects, live uint64
		want          int64
	}{
		{"nothing allocated", 100 * mib, 100 * mib, 0},
		{"less than the minimum", 20 * mib, 14 * mib, 0},
		{"minimum reached, but under a quarter of a big heap", 108 * mib, 100 * mib, 0},
		{"a quarter of the live heap", 125 * mib, 100 * mib, 1},
		{"the minimum of a small heap", 10 * mib, 2 * mib, 1},
		{"objects under live (the numbers are not from the same moment)", 50 * mib, 100 * mib, 0},
	} {
		g, n := testGC(tc.objects, tc.live)
		g.idle(g.gen)
		if n.Load() != tc.want {
			t.Errorf("%s: %d collections, want %d", tc.name, n.Load(), tc.want)
		}
	}
}

// Work in flight, work that came after the timer started, a run that waits
// and the pause between two cycles all keep the collector from running.
func TestIdleGCNotIdle(t *testing.T) {
	g, n := testGC(300*mib, 100*mib)
	g.begin()
	g.idle(g.gen)
	if n.Load() != 0 {
		t.Error("collected while a request was in flight")
	}
	gen := g.gen
	g.end()
	g.begin() // a new piece of work after the timer of the first end started
	g.idle(gen)
	g.end()
	if n.Load() != 0 {
		t.Error("collected for a timer that was late")
	}

	g, n = testGC(300*mib, 100*mib)
	busy := true
	g.busy = func() bool { return busy }
	g.idle(g.gen)
	if n.Load() != 0 {
		t.Error("collected while a diagnostics run waited")
	}
	busy = false
	g.idle(g.gen)
	g.idle(g.gen)
	if n.Load() != 1 {
		t.Errorf("%d collections for two timers within the pause, want 1", n.Load())
	}
	g.last = time.Now().Add(-time.Second)
	g.idle(g.gen)
	if n.Load() != 2 {
		t.Errorf("%d collections after the pause, want 2", n.Load())
	}
}

// The real timer: the end of the last piece of work collects the garbage once the quiet time has passed.
func TestIdleGCTimer(t *testing.T) {
	g, n := testGC(300*mib, 100*mib)
	done := make(chan struct{}, 1)
	g.collect = func() { n.Add(1); done <- struct{}{} }
	g.begin()
	g.end()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("no collection after the end of the work")
	}
	if n.Load() != 1 {
		t.Errorf("%d collections", n.Load())
	}
}

// A timer rearmed after an earlier idle cycle must use the new generation.
func TestIdleGCRearm(t *testing.T) {
	g, n := testGC(300*mib, 100*mib)
	done := make(chan struct{}, 3)
	g.collect = func() {
		n.Add(1)
		done <- struct{}{}
	}
	t.Cleanup(func() {
		g.mu.Lock()
		g.disabled = true
		if g.timer != nil {
			g.timer.Stop()
		}
		g.mu.Unlock()
	})
	for cycle := int64(1); cycle <= 3; cycle++ {
		g.mu.Lock()
		g.last = time.Now().Add(-time.Second)
		g.mu.Unlock()
		g.begin()
		g.end()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("no collection for idle cycle %d", cycle)
		}
		if got := n.Load(); got != cycle {
			t.Fatalf("%d collections after cycle %d", got, cycle)
		}
	}
}

// A server that is off, or a nil one, does nothing.
func TestIdleGCOff(t *testing.T) {
	var nilGC *idleGC
	nilGC.begin()
	nilGC.end()
	g, n := testGC(300*mib, 100*mib)
	g.disabled = true
	g.begin()
	g.end()
	time.Sleep(3 * idleGCDelay)
	if n.Load() != 0 {
		t.Error("a disabled collector collected")
	}
}

func TestReadHeap(t *testing.T) {
	objects, live := readHeap()
	if objects == 0 {
		t.Error("no heap objects reported")
	}
	_ = live
}

func TestMemoryLimitFor(t *testing.T) {
	for _, tc := range []struct {
		live uint64
		want int64 // MiB
	}{
		{0, 256},
		{50, 256},
		{120, 256},   // 1.4 * 120 + 64 = 232: the floor
		{155, 281},   // a project of 90,000 lines: 1.4 * 155 + 64
		{200, 344},   // 1.4 * 200 + 64
		{1000, 1464}, // a huge one: the limit stays above the live heap
	} {
		got := memoryLimitFor(tc.live * mib)
		if got/mib != tc.want {
			t.Errorf("live %d MiB: limit %d MiB, want %d", tc.live, got/mib, tc.want)
		}
		if got < int64(tc.live*mib) {
			t.Errorf("live %d MiB: the limit is under the live heap", tc.live)
		}
	}
}

func TestGCPercentFor(t *testing.T) {
	for _, tc := range []struct {
		live uint64 // MiB
		want int
	}{
		{0, 800},    // not known yet: the most room
		{4, 800},    // 96 MiB of room would be 2,400%: the bound
		{12, 800},   // exactly the bound
		{24, 400},   // a small project: 96 MiB above its live heap
		{48, 200},   // 96 MiB above 48
		{96, 100},   // the headroom is the live heap itself
		{155, 100},  // a project of 90,000 lines: GOGC 100
		{1000, 100}, // a huge one
	} {
		if got := gcPercentFor(tc.live * mib); got != tc.want {
			t.Errorf("live %d MiB: GC percent %d, want %d", tc.live, got, tc.want)
		}
	}
}

func TestGCPercentLeavesTheHeadroomAbove(t *testing.T) {
	for _, live := range []uint64{16, 24, 40, 64, 90} {
		percent := gcPercentFor(live * mib)

		room := live * mib * uint64(percent) / 100

		if room < gcHeadroom-mib || room > gcHeadroom+mib {
			t.Errorf("live %d MiB: %d%% leaves %d MiB, want about %d",
				live, percent, room/mib, gcHeadroom/mib)
		}
	}
}

// recordHeapPolicy replaces the runtime's settings by recorders and returns them.
func recordHeapPolicy(t *testing.T) (limit *int64, percent *int) {
	t.Helper()
	limit, percent = new(int64), new(int)
	*limit, *percent = -1, -1
	saved := heapPolicy
	t.Cleanup(func() { heapPolicy = saved })
	heapPolicy.setMemoryLimit = func(n int64) int64 { *limit = n; return 0 }
	heapPolicy.setGCPercent = func(n int) int { *percent = n; return 0 }
	return limit, percent
}

func TestApplyHeapPolicySetsBoth(t *testing.T) {
	t.Setenv("GOMEMLIMIT", "")
	t.Setenv("GOGC", "")
	limit, percent := recordHeapPolicy(t)

	applyHeapPolicy(24 * mib)

	if *limit != memoryLimitFor(24*mib) || *percent != 400 {
		t.Errorf("limit %d, percent %d, want %d and 400", *limit, *percent, memoryLimitFor(24*mib))
	}
}

func TestApplyHeapPolicyKeepsTheUsersGOGC(t *testing.T) {
	t.Setenv("GOMEMLIMIT", "")
	t.Setenv("GOGC", "50")
	limit, percent := recordHeapPolicy(t)

	applyHeapPolicy(24 * mib)

	if *percent != -1 {
		t.Errorf("the GC percent was set to %d with GOGC in the environment", *percent)
	}
	if *limit == -1 {
		t.Error("the memory limit was left alone with only GOGC set")
	}
}

func TestApplyHeapPolicyKeepsTheUsersMemoryLimit(t *testing.T) {
	t.Setenv("GOMEMLIMIT", "1GiB")
	t.Setenv("GOGC", "")
	limit, percent := recordHeapPolicy(t)

	applyHeapPolicy(24 * mib)

	if *limit != -1 {
		t.Errorf("the memory limit was set to %d with GOMEMLIMIT in the environment", *limit)
	}
	if *percent != 400 {
		t.Errorf("GC percent %d, want 400", *percent)
	}
}

func TestFollowsLiveHeapUnlessBothAreTheUsers(t *testing.T) {
	for _, tc := range []struct {
		limit, percent string
		want           bool
	}{
		{"", "", true},
		{"1GiB", "", true},
		{"", "50", true},
		{"1GiB", "50", false},
	} {
		t.Setenv("GOMEMLIMIT", tc.limit)
		t.Setenv("GOGC", tc.percent)
		if got := followsLiveHeap(); got != tc.want {
			t.Errorf("GOMEMLIMIT=%q GOGC=%q: %v, want %v", tc.limit, tc.percent, got, tc.want)
		}
	}
}
