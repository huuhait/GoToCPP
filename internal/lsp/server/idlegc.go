package server

import (
	"os"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"sync"
	"time"
)

// The idle garbage collector (off unless gocpp lsp -idle-gc: measured with the
// keystroke benchmarks, alternating runs with it on and off, it did not lower the p95 of a
// keystroke, and costs the CPU of more collections). A check allocates megabytes of garbage that
// the Go runtime collects in the middle of the next piece of work (its
// pacer starts a cycle when the heap has doubled): the write barriers and
// the mutator assists of the cycle show in the p95 of the keystroke, while
// the user types more slowly than the server answers and the cores are idle
// between two keys. So the server starts the cycle itself, when it has been
// quiet for idleGCDelay and has allocated enough since the last cycle that
// the pacer would soon do it anyway (idleGCGrowth of the live heap, at least
// idleGCMinBytes). The pacer's own cycles stay (gcPercentFor sets their percent): a burst of requests
// that never pauses, or a request that allocates more than the threshold, is
// collected as before. The memory of an editing session is lower than with
// the pacer alone: the heap is collected at 1.25 times the live heap, not at 2
// times.
const (
	// idleGCDelay is the quiet time after the last piece of work.
	idleGCDelay = 5 * time.Millisecond
	// idleGCGrowth is the share of the live heap that the garbage must have
	// reached, as a fraction: 1/idleGCGrowth.
	idleGCGrowth = 4
	// idleGCMinBytes is the least garbage that is worth a cycle.
	idleGCMinBytes = 8 << 20
	// idleGCEvery is the shortest time between two idle cycles.
	idleGCEvery = 100 * time.Millisecond
)

// An idleGC watches the work of a server (begin and end of every piece of
// it) and collects the garbage in the gaps.
type idleGC struct {
	// busy says that something else is waiting to run (the debouncer has a
	// diagnostics run that is due): the server is not idle. May be nil.
	busy func() bool
	// heap returns the bytes of the heap objects (live and not swept yet)
	// and the live heap of the last cycle; collect runs a cycle. The tests
	// replace them.
	heap    func() (objects, live uint64)
	collect func()

	mu       sync.Mutex
	disabled bool
	inflight int
	gen      uint64 // changes at every begin and end, so a timer can tell it is late
	timer    *time.Timer
	last     time.Time
	cycles   int // the cycles it started
}

func newIdleGC(enabled bool, busy func() bool) *idleGC {
	return &idleGC{disabled: !enabled, busy: busy, heap: readHeap, collect: runtime.GC}
}

var memoryOnce sync.Once

var heapSamples = sync.Pool{New: func() any {
	return []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}, {Name: "/gc/heap/live:bytes"}}
}}

// readHeap is idleGC.heap for the running process.
func readHeap() (objects, live uint64) {
	s := heapSamples.Get().([]metrics.Sample)
	metrics.Read(s)
	if s[0].Value.Kind() == metrics.KindUint64 {
		objects = s[0].Value.Uint64()
	}
	if s[1].Value.Kind() == metrics.KindUint64 {
		live = s[1].Value.Uint64()
	}
	heapSamples.Put(s)
	return objects, live
}

// begin notes that a piece of work starts.
func (g *idleGC) begin() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.inflight++
	g.gen++
	if g.timer != nil {
		g.timer.Stop()
	}
	g.mu.Unlock()
}

// quiet reports whether no piece of work is in flight.
func (g *idleGC) quiet() bool {
	if g == nil {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.inflight == 0
}

// end notes that a piece of work is over: when nothing else runs, a timer
// starts that looks at the heap once the quiet time has passed.
func (g *idleGC) end() {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.inflight--
	g.gen++
	if g.inflight > 0 || g.disabled {
		return
	}
	gen := g.gen
	if g.timer != nil {
		g.timer.Stop()
	}
	// Reset would reuse the first callback's captured generation forever.
	// Each opportunity needs its own generation; a callback already running
	// is still rejected by idle's generation checks.
	g.timer = time.AfterFunc(idleGCDelay, func() { g.idle(gen) })
}

// idle is the timer: if nothing happened since gen, and the garbage is
// worth it, it collects.
func (g *idleGC) idle(gen uint64) {
	g.mu.Lock()
	if g.gen != gen || g.inflight > 0 || time.Since(g.last) < idleGCEvery {
		g.mu.Unlock()
		return
	}
	g.mu.Unlock()
	if g.busy != nil && g.busy() {
		// A run is waiting for its turn: look again after it.
		return
	}
	objects, live := g.heap()
	if objects < live || objects-live < max(idleGCMinBytes, live/idleGCGrowth) {
		return
	}
	g.mu.Lock()
	if g.gen != gen || g.inflight > 0 {
		g.mu.Unlock()
		return
	}
	g.last = time.Now()
	g.cycles++
	g.mu.Unlock()
	g.collect()
}

// The memory limit. The Go pacer lets the heap grow to twice the live data
// before it collects, and the resident set of a big project ends well over that
// (garbage that is not swept yet, fragmentation): a project of 90,000 lines has
// 155 MiB of live data and 430 MiB resident. A soft limit makes the runtime
// collect sooner as the total approaches it. It follows the live heap: memFloor for
// the projects whose pacer goal is under it anyway (the budget of the speed contract
// is 400 MiB for 50,000 lines), and above that memFactor times the live heap and
// memSlack, which is the room for garbage, plus the memory of the runtime that is no
// heap object (runtimeOverhead).
//
// The limit counts all of the runtime's memory (ADR-142), and that overhead grows with GOMAXPROCS
// (the per-P caches of spans, fragmentation, metadata): on a machine of 192 threads it
// was 86 to 93 MiB for a live heap of 150 to 200 MiB on the large benchmark project, so a
// limit from the live heap alone left the heap about 10 MiB of room (gctrace: a goal of
// 156 MiB for 146 MiB live) and the runtime collected every keystroke or two, a cycle of
// 70 ms each time (keystroke-imported p95 4.7 times gopls's; with GOMAXPROCS=8 the same
// build had no such cycles). The overhead counts at most once more the live heap, so the
// limit stays under what the pacer's own goal needs.
const (
	memFloor  = 256 << 20
	memFactor = 14 // tenths
	memSlack  = 64 << 20
)

// memoryLimitFor returns the soft limit for a live heap of live bytes and a runtime overhead
// of overhead bytes (runtimeOverhead).
func memoryLimitFor(live, overhead uint64) int64 {
	return int64(max(memFloor, live/10*memFactor+memSlack+min(overhead, live)))
}

var overheadSamples = sync.Pool{New: func() any {
	return []metrics.Sample{
		{Name: "/memory/classes/total:bytes"},
		{Name: "/memory/classes/heap/released:bytes"},
		{Name: "/memory/classes/heap/free:bytes"},
		{Name: "/memory/classes/heap/objects:bytes"},
	}
}}

// runtimeOverhead returns the memory that the soft limit counts and that is neither a heap
// object (live, or dead and not swept yet) nor free: fragmentation, stacks, metadata, the
// per-P caches.
func runtimeOverhead() uint64 {
	s := overheadSamples.Get().([]metrics.Sample)
	defer overheadSamples.Put(s)
	metrics.Read(s)
	var v [4]uint64
	for i := range s {
		if s[i].Value.Kind() != metrics.KindUint64 {
			return 0
		}
		v[i] = s[i].Value.Uint64()
	}
	total, released, free, objects := v[0], v[1], v[2], v[3]
	if used := released + free + objects; total > used {
		return total - used
	}
	return 0
}

// The pacer's GC percent. GOGC 100 collects when the heap has doubled, which for a small project is
// every dozen keystrokes (a check allocates about a megabyte): the cycle's write barriers and mutator
// assists then show as a spike in one keystroke of twelve, which is the p95 of the typing rows. The
// server asks for a fixed amount of room above the live heap instead (gcHeadroom) wherever that is
// more than the live heap itself, so a project whose live heap is under it collects every few dozen
// keystrokes, and a project over it keeps GOGC 100 and the soft limit above. The headroom is memory
// the process may use above the live data, not a leak: the next cycle gives it back.
const (
	// gcHeadroom is the garbage that the pacer lets accumulate above the live heap of a small project.
	gcHeadroom = 96 << 20
	// gcMaxPercent bounds the percent for a tiny live heap.
	gcMaxPercent = 800
)

// gcPercentFor returns the GC percent for a live heap of live bytes: the one that leaves gcHeadroom
// above it, at least 100 and at most gcMaxPercent.
func gcPercentFor(live uint64) int {
	if live == 0 {
		return gcMaxPercent
	}
	return int(min(gcMaxPercent, max(100, gcHeadroom*100/live)))
}

// heapPolicy is what followLiveHeap sets: the functions are replaced by the tests.
var heapPolicy = struct {
	setMemoryLimit func(int64) int64
	setGCPercent   func(int) int
}{debug.SetMemoryLimit, debug.SetGCPercent}

// applyHeapPolicy sets the soft memory limit and the GC percent for a live heap of live bytes and
// a runtime overhead of overhead bytes. The user's GOMEMLIMIT and GOGC are theirs: the matching
// setting is left alone.
func applyHeapPolicy(live, overhead uint64) {
	if os.Getenv("GOMEMLIMIT") == "" {
		heapPolicy.setMemoryLimit(memoryLimitFor(live, overhead))
	}
	if os.Getenv("GOGC") == "" {
		heapPolicy.setGCPercent(gcPercentFor(live))
	}
}

// gcSentinel is the object whose finalizer tells that a collection ended.
type gcSentinel struct{ _ [16]byte }

// followsLiveHeap reports whether the server sets anything: not when the user set both
// GOMEMLIMIT and GOGC.
func followsLiveHeap() bool {
	return os.Getenv("GOMEMLIMIT") == "" || os.Getenv("GOGC") == ""
}

// followLiveHeap sets the memory limit and the GC percent now and after each collection, from the
// live heap of the cycle that ended, unless the user set GOMEMLIMIT or GOGC: then that setting is
// theirs.
func followLiveHeap() {
	if !followsLiveHeap() {
		return
	}
	applyHeapPolicy(0, 0)
	var arm func()
	arm = func() {
		runtime.SetFinalizer(&gcSentinel{}, func(*gcSentinel) {
			_, live := readHeap()
			applyHeapPolicy(live, runtimeOverhead())
			arm()
		})
	}
	arm()
}
