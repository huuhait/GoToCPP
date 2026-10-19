// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"fmt"
	"time"
)

// The benchmarks: Go's testing/benchmark.go, with the differences that PORT.md lists. Durations
// are int64 nanoseconds (package time is ported, but testing does not use it yet), the clock is libNow,
// and there is no memory profile: the allocation numbers count the calls of C++ operator new.

// InternalBenchmark is an internal type but exported because it is cross-package;
// it is part of the implementation of the "gocpp test" command.
type InternalBenchmark struct {
	Name string
	F    func(b *B)
}

const (
	loopPoisonTimer = uint64(1) << 63
	loopPoisonMask  = ^((uint64(1) << 62) - 1)
)

// B is a type passed to [Benchmark] functions to manage benchmark
// timing and control the number of iterations.
//
// A benchmark ends when its Benchmark function returns or calls any of the methods
// FailNow, Fatal, Fatalf, SkipNow, Skip, or Skipf. Those methods must be called
// only from the goroutine running the Benchmark function.
// The other reporting methods, such as the variations of Log and Error,
// may be called simultaneously from multiple goroutines.
//
// Like in tests, benchmark logs are accumulated during execution
// and dumped to standard output when done. Unlike in tests, benchmark logs
// are always printed, so as not to hide output whose existence may be
// affecting benchmark results.
type B struct {
	common
	importPath       string // import path of the package containing the benchmark
	bstate           *benchState
	N                int
	previousN        int   // number of iterations in the previous run
	previousDuration int64 // total duration of the previous run
	benchFunc        func(b *B)
	benchN           int   // -test.benchtime=Nx: the count, or 0
	benchD           int64 // -test.benchtime=1s: the duration, when benchN is 0
	bytes            int64
	missingBytes     bool // one of the subbenchmarks does not have bytes set.
	timerOn          bool
	startAllocs      int // the allocation counters when the timer started
	startBytes       int
	netAllocs        uint64 // the allocations while the timer was on
	netBytes         uint64
	showAllocResult  bool
	result           BenchmarkResult
	parallelism      int // RunParallel creates parallelism*GOMAXPROCS goroutines

	// extra records additional metrics reported by ReportMetric.
	extra map[string]float64

	// loop tracks the state of B.Loop
	loop struct {
		// n is the target number of iterations. It gets bumped up as we go.
		// When the benchmark loop is done, we commit this to b.N so users can
		// do reporting based on it, but we avoid exposing it until then.
		n uint64
		// i is the current Loop iteration. It's strictly monotonically
		// increasing toward n.
		//
		// The high bits of i are used for the loopPoison constants.
		i uint64

		done bool // set when B.Loop return false
	}
}

// StartTimer starts timing a test. This function is called automatically
// before a benchmark starts, but it can also be used to resume timing after
// a call to [B.StopTimer].
func (b *B) StartTimer() {
	if !b.timerOn {
		b.startAllocs, b.startBytes = libAllocs()
		b.start = libNow()
		b.timerOn = true
		b.loop.i &^= loopPoisonTimer
	}
}

// StopTimer stops timing a test. This can be used to pause the timer
// while performing steps that you don't want to measure.
func (b *B) StopTimer() {
	if b.timerOn {
		b.duration += libNow() - b.start
		allocs, bytes := libAllocs()
		b.netAllocs += uint64(allocs - b.startAllocs)
		b.netBytes += uint64(bytes - b.startBytes)
		b.timerOn = false
		b.loop.i |= loopPoisonTimer
	}
}

// ResetTimer zeroes the elapsed benchmark time and memory allocation counters
// and deletes user-reported metrics.
// It does not affect whether the timer is running.
func (b *B) ResetTimer() {
	if b.extra == nil {
		b.extra = make(map[string]float64, 16)
	} else {
		clear(b.extra)
	}
	if b.timerOn {
		b.startAllocs, b.startBytes = libAllocs()
		b.start = libNow()
	}
	b.duration = 0
	b.netAllocs = 0
	b.netBytes = 0
}

// SetBytes records the number of bytes processed in a single operation.
// If this is called, the benchmark will report ns/op and MB/s.
func (b *B) SetBytes(n int64) { b.bytes = n }

// ReportAllocs enables malloc statistics for this benchmark.
// It is equivalent to setting -test.benchmem, but it only affects the
// benchmark function that calls ReportAllocs. The numbers are the calls of C++ operator new in the
// timed part (testing.cpp), not the allocations of Go's compiler.
func (b *B) ReportAllocs() {
	b.showAllocResult = true
}

// runN runs a single benchmark for the specified number of iterations.
func (b *B) runN(n int) {
	libAllocCounting(1)
	defer func() {
		libAllocCounting(-1)
		b.runCleanup(normalPanic)
	}()
	b.N = n
	b.loop.n = 0
	b.loop.i = 0
	b.loop.done = false
	b.parallelism = 1
	b.ResetTimer()
	b.StartTimer()
	b.benchFunc(b)
	b.StopTimer()
	b.previousN = n
	b.previousDuration = b.duration

	if b.loop.n > 0 && !b.loop.done && !b.failed {
		b.Error("benchmark function returned without B.Loop() == false (break or return in loop?)")
	}
}

// run1 runs the first iteration of benchFunc. It reports whether more
// iterations of this benchmarks should be run.
func (b *B) run1() bool {
	if bstate := b.bstate; bstate != nil {
		// Extend maxLen, if needed.
		if n := len(b.name) + bstate.extLen + 1; n > bstate.maxLen {
			bstate.maxLen = n + 8 // Add additional slack to avoid too many jumps in size.
		}
	}
	go func() {
		// Signal that we're done whether we return normally
		// or by FailNow's runtime.Goexit.
		defer func() {
			b.signal <- true
		}()

		b.runN(1)
	}()
	<-b.signal
	if b.failed {
		writef(b.w, "--- FAIL: %s\n%s", b.name, string(b.output))
		return false
	}
	// Only print the output if we know we are not going to proceed.
	// Otherwise it is printed in processBench.
	libLock()
	finished := b.finished
	hasSub := b.hasSub
	libUnlock()
	if hasSub || finished {
		tag := "BENCH"
		if b.skipped {
			tag = "SKIP"
		}
		if b.chatty != nil && (len(b.output) > 0 || finished) {
			b.trimOutput()
			writef(b.w, "--- %s: %s\n%s", tag, b.name, string(b.output))
		}
		return false
	}
	return true
}

var labelsPrinted bool

// run executes the benchmark in a separate goroutine, including all of its
// subbenchmarks. b must not have subbenchmarks.
func (b *B) run() {
	if !labelsPrinted {
		labelsPrinted = true
		writef(b.w, "goos: %s\n", libGOOS())
		writef(b.w, "goarch: %s\n", libGOARCH())
		if b.importPath != "" {
			writef(b.w, "pkg: %s\n", b.importPath)
		}
		if cpu := libCPUName(); cpu != "" {
			writef(b.w, "cpu: %s\n", cpu)
		}
	}
	if b.bstate != nil {
		// Running go test --test.bench
		b.bstate.processBench(b) // Must call doBench.
	} else {
		// Running func Benchmark.
		b.doBench()
	}
}

func (b *B) doBench() BenchmarkResult {
	go b.launch()
	<-b.signal
	return b.result
}

// predictN predicts how many iterations are needed to run for goalns nanoseconds.
func predictN(goalns int64, prevIters int64, prevns int64, last int64) int {
	if prevns == 0 {
		// Round up to dodge divide by zero. See https://go.dev/issue/70709.
		prevns = 1
	}

	// Order of operations matters.
	// For very fast benchmarks, prevIters ~= prevns.
	// If you divide first, you get 0 or 1,
	// which can hide an order of magnitude in execution time.
	// So multiply first, then divide.
	n := goalns * prevIters / prevns
	// Run more iterations than we think we'll need (1.2x).
	n += n / 5
	// Don't grow too fast in case we had timing errors previously.
	n = min(n, 100*last)
	// Be sure to run at least one more than last time.
	n = max(n, last+1)
	// Don't run more than 1e9 times. (This also keeps n in int range on 32 bit platforms.)
	n = min(n, 1000000000)
	return int(n)
}

// launch launches the benchmark function. It gradually increases the number
// of benchmark iterations until the benchmark runs for the requested benchtime.
// launch is run by the doBench function as a separate goroutine.
// run1 must have been called on b.
func (b *B) launch() {
	// Signal that we're done whether we return normally
	// or by FailNow's runtime.Goexit.
	defer func() {
		b.signal <- true
	}()

	// B.Loop does its own ramp-up logic so we just need to run the benchmark once.
	// If b.loop.n is non zero, it means B.Loop has already run.
	if b.loop.n == 0 {
		// Run the benchmark for at least the specified amount of time.
		if b.benchN > 0 {
			// We already ran a single iteration in run1.
			// If -benchtime=1x was requested, use that result.
			// See https://golang.org/issue/32051.
			if b.benchN > 1 {
				b.runN(b.benchN)
			}
		} else {
			d := b.benchD
			for n := int64(1); !b.failed && b.duration < d && n < 1000000000; {
				last := n
				// Predict required iterations.
				goalns := d
				prevIters := int64(b.N)
				n = int64(predictN(goalns, prevIters, b.duration, last))
				b.runN(int(n))
			}
		}
	}
	b.result = BenchmarkResult{N: b.N, T: time.Duration(b.duration), Bytes: b.bytes, MemAllocs: b.netAllocs, MemBytes: b.netBytes, Extra: b.extra}
}

// Elapsed returns the measured elapsed time of the benchmark.
// The duration reported by Elapsed matches the one measured by
// [B.StartTimer], [B.StopTimer], and [B.ResetTimer].
func (b *B) Elapsed() time.Duration {
	return time.Duration(b.elapsed())
}

// elapsed is Elapsed in nanoseconds.
func (b *B) elapsed() int64 {
	d := b.duration
	if b.timerOn {
		d += libNow() - b.start
	}
	return d
}

// ReportMetric adds "n unit" to the reported benchmark results.
// If the metric is per-iteration, the caller should divide by b.N,
// and by convention units should end in "/op".
// ReportMetric overrides any previously reported value for the same unit.
// ReportMetric panics if unit is the empty string or if unit contains
// any whitespace.
// If unit is a unit normally reported by the benchmark framework itself
// (such as "allocs/op"), ReportMetric will override that metric.
// Setting "ns/op" to 0 will suppress that built-in metric.
func (b *B) ReportMetric(n float64, unit string) {
	if unit == "" {
		panic("metric unit must not be empty")
	}
	for i := 0; i < len(unit); i++ {
		if isSpaceByte(unit[i]) {
			panic("metric unit must not contain whitespace")
		}
	}
	b.extra[unit] = n
}

// stopOrScaleBLoop determines whether the B.Loop should continue, and if so
// scales the target iteration count.
func (b *B) stopOrScaleBLoop() bool {
	t := b.elapsed()
	if t >= b.benchD {
		// We've reached the target
		return false
	}
	// Loop scaling
	goalns := b.benchD
	prevIters := int64(b.loop.n)
	b.loop.n = uint64(predictN(goalns, prevIters, t, prevIters))
	if b.loop.n&loopPoisonMask != 0 {
		// The iteration count should never get this high, but if it did we'd be
		// in big trouble.
		panic("loop iteration target overflow")
	}
	// predictN may have grown the target, so loop is still running.
	return true
}

// loopSlowPath is a separate slow-path function for Loop.
func (b *B) loopSlowPath() bool {
	// Consistency checks
	if !b.timerOn {
		b.Fatal("B.Loop called with timer stopped")
	}
	if b.loop.i&loopPoisonMask != 0 {
		panic(fmt.Sprintf("unknown loop stop condition: %#x", b.loop.i))
	}

	if b.loop.n == 0 {
		// It's the first call to b.Loop() in the benchmark function.
		// Allows more precise measurement of benchmark loop cost counts.
		// Also initialize b.N to 1 to kick start loop scaling.
		if b.benchN > 0 {
			b.loop.n = uint64(b.benchN)
		} else {
			b.loop.n = 1
		}
		b.N = 0
		b.ResetTimer()

		// Start the next iteration.
		b.loop.i++
		return true
	}

	// Handle fixed iteration count requests.
	var more bool
	if b.benchN > 0 {
		// Fixed iteration count.
		if b.loop.i != uint64(b.benchN) {
			panic(fmt.Sprintf("iteration count %d < fixed target %d", b.loop.i, b.benchN))
		}
		more = false
	} else {
		// Handle time-based loop scaling.
		more = b.stopOrScaleBLoop()
	}
	if !more {
		b.StopTimer()
		// Commit iteration count
		b.N = int(b.loop.n)
		b.loop.done = true
		return false
	}

	// Start the next iteration.
	b.loop.i++
	return true
}

// Loop returns true as long as the benchmark should continue running.
//
// A typical benchmark is structured like:
//
//	func Benchmark(b *testing.B) {
//		... setup ...
//		for b.Loop() {
//			... code to measure ...
//		}
//		... cleanup ...
//	}
//
// Loop resets the benchmark timer the first time it is called in a benchmark,
// so any setup performed prior to starting the benchmark loop does not count
// toward the benchmark measurement. Likewise, when it returns false, it stops
// the timer so cleanup code is not measured.
func (b *B) Loop() bool {
	// This is written such that the fast path is as fast as possible and can be
	// inlined.
	//
	// There are three cases where we'll fall out of the fast path:
	//
	// - On the first call, both i and n are 0.
	//
	// - If the loop reaches the n'th iteration, then i == n and we need
	//   to figure out the new target iteration count or if we're done.
	//
	// - If the timer is stopped, it poisons the top bit of i so the slow
	//   path can do consistency checks and fail.
	if b.loop.i < b.loop.n {
		b.loop.i++
		return true
	}
	return b.loopSlowPath()
}

// The results of a benchmark run.
//
// The allocation counters are the calls of C++ operator new (testing.cpp), not Go's allocations.
type BenchmarkResult struct {
	N         int           // The number of iterations.
	T         time.Duration // The total time taken.
	Bytes     int64         // Bytes processed in one iteration.
	MemAllocs uint64        // The total number of memory allocations.
	MemBytes  uint64        // The total number of bytes allocated.

	// Extra records additional metrics reported by ReportMetric.
	Extra map[string]float64
}

// NsPerOp returns the "ns/op" metric.
func (r BenchmarkResult) NsPerOp() int64 {
	if v, ok := r.Extra["ns/op"]; ok {
		return int64(v)
	}
	if r.N <= 0 {
		return 0
	}
	return int64(r.T) / int64(r.N)
}

// mbPerSec returns the "MB/s" metric.
func (r BenchmarkResult) mbPerSec() float64 {
	if v, ok := r.Extra["MB/s"]; ok {
		return v
	}
	if r.Bytes <= 0 || r.T <= 0 || r.N <= 0 {
		return 0
	}
	return (float64(r.Bytes) * float64(r.N) / 1e6) / (float64(r.T) / 1e9)
}

// AllocsPerOp returns the "allocs/op" metric,
// which is calculated as r.MemAllocs / r.N.
func (r BenchmarkResult) AllocsPerOp() int64 {
	if v, ok := r.Extra["allocs/op"]; ok {
		return int64(v)
	}
	if r.N <= 0 {
		return 0
	}
	return int64(r.MemAllocs) / int64(r.N)
}

// AllocedBytesPerOp returns the "B/op" metric,
// which is calculated as r.MemBytes / r.N.
func (r BenchmarkResult) AllocedBytesPerOp() int64 {
	if v, ok := r.Extra["B/op"]; ok {
		return int64(v)
	}
	if r.N <= 0 {
		return 0
	}
	return int64(r.MemBytes) / int64(r.N)
}

// String returns a summary of the benchmark results.
// It follows the benchmark result line format from
// https://golang.org/design/14313-benchmark-format, not including the
// benchmark name.
// Extra metrics override built-in metrics of the same name.
// String does not include allocs/op or B/op, since those are reported
// by [BenchmarkResult.MemString].
func (r BenchmarkResult) String() string {
	buf := fmt.Sprintf("%8d", r.N)

	// Get ns/op as a float.
	ns, ok := r.Extra["ns/op"]
	if !ok {
		ns = float64(r.T) / float64(r.N)
	}
	if ns != 0 {
		buf += "\t" + prettyPrint(ns, "ns/op")
	}

	if mbs := r.mbPerSec(); mbs != 0 {
		buf += fmt.Sprintf("\t%7.2f MB/s", mbs)
	}

	// Print extra metrics that aren't represented in the standard
	// metrics.
	var extraKeys []string
	for k := range r.Extra {
		switch k {
		case "ns/op", "MB/s", "B/op", "allocs/op":
			// Built-in metrics reported elsewhere.
			continue
		}
		extraKeys = append(extraKeys, k)
	}
	sortStrings(extraKeys)
	for _, k := range extraKeys {
		buf += "\t" + prettyPrint(r.Extra[k], k)
	}
	return buf
}

func prettyPrint(x float64, unit string) string {
	// Print all numbers with 10 places before the decimal point
	// and small numbers with four sig figs. Field widths are
	// chosen to fit the whole part in 10 places while aligning
	// the decimal point of all fractional formats.
	var format string
	y := x
	if y < 0 {
		y = -y
	}
	switch {
	case y == 0 || y >= 999.95:
		format = "%10.0f %s"
	case y >= 99.995:
		format = "%12.1f %s"
	case y >= 9.9995:
		format = "%13.2f %s"
	case y >= 0.99995:
		format = "%14.3f %s"
	case y >= 0.099995:
		format = "%15.4f %s"
	case y >= 0.0099995:
		format = "%16.5f %s"
	case y >= 0.00099995:
		format = "%17.6f %s"
	default:
		format = "%18.7f %s"
	}
	return fmt.Sprintf(format, x, unit)
}

// MemString returns r.AllocedBytesPerOp and r.AllocsPerOp in the same format as 'go test'.
func (r BenchmarkResult) MemString() string {
	return fmt.Sprintf("%8d B/op\t%8d allocs/op",
		r.AllocedBytesPerOp(), r.AllocsPerOp())
}

// benchmarkName returns full name of benchmark including procs suffix.
func benchmarkName(name string, n int) string {
	if n != 1 {
		return fmt.Sprintf("%s-%d", name, n)
	}
	return name
}

type benchState struct {
	match *matcher

	maxLen int // The largest recorded benchmark name.
	extLen int // Maximum extension length.
}

// RunBenchmarks is an internal function but exported because it is cross-package;
// it is part of the implementation of the "gocpp test" command.
func RunBenchmarks(matchString func(pat, str string) (bool, error), benchmarks []InternalBenchmark) {
	Init()
	runBenchmarks("", matchString, benchmarks)
}

func runBenchmarks(importPath string, matchString func(pat, str string) (bool, error), benchmarks []InternalBenchmark) bool {
	// If no flag was specified, don't run benchmarks.
	if len(matchBench) == 0 {
		return true
	}
	// Collect matching benchmarks and determine longest name.
	maxprocs := 1
	for _, procs := range cpuList {
		if procs > maxprocs {
			maxprocs = procs
		}
	}
	bstate := &benchState{
		match:  newMatcher(matchString, matchBench, "-test.bench", skipPattern),
		extLen: len(benchmarkName("", maxprocs)),
	}
	var bs []InternalBenchmark
	for _, Benchmark := range benchmarks {
		if _, matched, _ := bstate.match.fullName(nil, Benchmark.Name); matched {
			bs = append(bs, Benchmark)
			benchName := benchmarkName(Benchmark.Name, maxprocs)
			if l := len(benchName) + bstate.extLen + 1; l > bstate.maxLen {
				bstate.maxLen = l
			}
		}
	}
	main := &B{
		common: common{
			name:  "Main",
			w:     stdoutWriter{},
			bench: true,
		},
		importPath: importPath,
		benchFunc: func(b *B) {
			for _, Benchmark := range bs {
				b.Run(Benchmark.Name, Benchmark.F)
			}
		},
		benchN: benchTimeN,
		benchD: benchTimeD,
		bstate: bstate,
	}
	if Verbose() {
		main.chatty = chatty
	}
	main.runN(1)
	return !main.failed
}

// processBench runs bench b for the configured CPU counts and prints the results.
func (s *benchState) processBench(b *B) {
	for i, procs := range cpuList {
		for j := 0; j < testCount; j++ {
			benchName := benchmarkName(b.name, procs)

			// If it's chatty, we've already printed this information.
			if b.chatty == nil {
				writef(b.w, "%s\t", padRight(benchName, s.maxLen))
			}
			// Recompute the running time for all but the first iteration.
			if i > 0 || j > 0 {
				b = &B{
					common: common{
						signal: make(chan bool),
						name:   b.name,
						w:      b.w,
						chatty: b.chatty,
						bench:  true,
					},
					benchFunc: b.benchFunc,
					benchN:    b.benchN,
					benchD:    b.benchD,
				}
				b.run1()
			}
			r := b.doBench()
			if b.failed {
				// The output could be very long here, but probably isn't.
				// We print it all, regardless, because we don't want to trim the reason
				// the benchmark failed.
				writef(b.w, "--- FAIL: %s\n%s", benchName, string(b.output))
				continue
			}
			results := r.String()
			if b.chatty != nil {
				writef(b.w, "%s\t", padRight(benchName, s.maxLen))
			}
			if benchMem || b.showAllocResult {
				results += "\t" + r.MemString()
			}
			writef(b.w, "%s\n", results)
			// Unlike with tests, we ignore the -chatty flag and always print output for
			// benchmarks since the output generation time will skew the results.
			if len(b.output) > 0 {
				b.trimOutput()
				writef(b.w, "--- BENCH: %s\n%s", benchName, string(b.output))
			}
		}
	}
}

// padRight pads s with spaces on the right to width n, like the verb %-*s.
func padRight(s string, n int) string {
	for len(s) < n {
		s += " "
	}
	return s
}

// writef writes the formatted text to w.
func writef(w writer, format string, args ...any) {
	w.Write([]byte(fmt.Sprintf(format, args...)))
}

// Run benchmarks f as a subbenchmark with the given name. It reports
// whether there were any failures.
//
// A subbenchmark is like any other benchmark. A benchmark that calls Run at
// least once will not be measured itself and will be called once with N=1.
func (b *B) Run(name string, f func(b *B)) bool {
	// Since b has subbenchmarks, we will no longer run it as a benchmark itself.
	// Release the lock and acquire it on exit to ensure locks stay paired.
	libLock()
	b.hasSub = true
	libUnlock()
	benchName, ok, partial := b.name, true, false
	if b.bstate != nil {
		benchName, ok, partial = b.bstate.match.fullName(&b.common, name)
	}
	if !ok {
		return true
	}
	sub := &B{
		common: common{
			signal: make(chan bool),
			name:   benchName,
			parent: &b.common,
			level:  b.level + 1,
			w:      b.w,
			chatty: b.chatty,
			bench:  true,
		},
		importPath: b.importPath,
		benchFunc:  f,
		benchN:     b.benchN,
		benchD:     b.benchD,
		bstate:     b.bstate,
	}
	if partial {
		// Partial name match, like -bench=X/Y matching BenchmarkX.
		// Only process sub-benchmarks, if any.
		sub.hasSub = true
	}

	if b.chatty != nil {
		if !labelsPrinted {
			labelsPrinted = true
			fmt.Printf("goos: %s\n", libGOOS())
			fmt.Printf("goarch: %s\n", libGOARCH())
			if b.importPath != "" {
				fmt.Printf("pkg: %s\n", b.importPath)
			}
			if cpu := libCPUName(); cpu != "" {
				fmt.Printf("cpu: %s\n", cpu)
			}
		}
		fmt.Println(benchName)
	}

	if sub.run1() {
		sub.run()
	}
	b.add(sub.result)
	return !sub.failed
}

// add simulates running benchmarks in sequence in a single iteration. It is
// used to give some meaningful results in case func Benchmark is used in
// combination with Run.
func (b *B) add(other BenchmarkResult) {
	r := &b.result
	// The aggregated BenchmarkResults resemble running all subbenchmarks as
	// in sequence in a single benchmark.
	r.N = 1
	r.T += time.Duration(other.NsPerOp())
	if other.Bytes == 0 {
		// Summing Bytes is meaningless in aggregate if not all subbenchmarks
		// set it.
		b.missingBytes = true
		r.Bytes = 0
	}
	if !b.missingBytes {
		r.Bytes += other.Bytes
	}
	r.MemAllocs += uint64(other.AllocsPerOp())
	r.MemBytes += uint64(other.AllocedBytesPerOp())
}

// trimOutput shortens the output from a benchmark, which can be very long.
func (b *B) trimOutput() {
	// The output is likely to appear multiple times because the benchmark
	// is run multiple times, but at least it will be seen. This is not a big deal
	// because benchmarks rarely print, but just in case, we trim it if it's too long.
	const maxNewlines = 10
	for nlCount, j := 0, 0; j < len(b.output); j++ {
		if b.output[j] == '\n' {
			nlCount++
			if nlCount >= maxNewlines {
				b.output = append(b.output[:j], "\n\t... [output truncated]\n"...)
				break
			}
		}
	}
}

// A PB is used by RunParallel for running parallel benchmarks.
type PB struct {
	b     *B
	grain uint64 // acquire that many iterations from the shared counter at once
	cache uint64 // local cache of acquired iterations
	bN    uint64 // total number of iterations to execute (b.N)
}

// parallelCount is the shared iteration counter of RunParallel (guarded by the lock).
type parallelCount struct {
	n uint64
}

var pbCounters = map[*B]*parallelCount{}

// Next reports whether there are more iterations to execute.
func (pb *PB) Next() bool {
	if pb.cache == 0 {
		libLock()
		c := pbCounters[pb.b]
		c.n += pb.grain
		n := c.n
		libUnlock()
		if n <= pb.bN {
			pb.cache = pb.grain
		} else if n < pb.bN+pb.grain {
			pb.cache = pb.bN + pb.grain - n
		} else {
			return false
		}
	}
	pb.cache--
	return true
}

// RunParallel runs a benchmark in parallel.
// It creates multiple goroutines and distributes b.N iterations among them.
// The number of goroutines defaults to GOMAXPROCS. To increase parallelism for
// non-CPU-bound benchmarks, call [B.SetParallelism] before RunParallel.
// RunParallel is usually used with the go test -cpu flag.
//
// The body function will be run in each goroutine. It should set up any
// goroutine-local state and then iterate until pb.Next returns false.
// It should not use the [B.StartTimer], [B.StopTimer], or [B.ResetTimer] functions,
// because they have global effect. It should also not call [B.Run].
func (b *B) RunParallel(body func(*PB)) {
	if b.N == 0 {
		return // Nothing to do when probing.
	}
	// Calculate grain size as number of iterations that take ~100µs.
	// 100µs is enough to amortize the overhead and provide sufficient
	// dynamic load balancing.
	grain := uint64(0)
	if b.previousN > 0 && b.previousDuration > 0 {
		grain = 100000 * uint64(b.previousN) / uint64(b.previousDuration)
	}
	// Don't allow grain to be too small or too large.
	if grain < 1 {
		grain = 1
	}
	if grain > 10000 {
		grain = 10000
	}

	c := &parallelCount{}
	libLock()
	pbCounters[b] = c
	libUnlock()
	numProcs := b.parallelism * libNCPU()
	done := make(chan bool, numProcs)
	for p := 0; p < numProcs; p++ {
		go func() {
			defer func() { done <- true }()
			pb := &PB{
				b:     b,
				grain: grain,
				bN:    uint64(b.N),
			}
			body(pb)
		}()
	}
	for p := 0; p < numProcs; p++ {
		<-done
	}
	libLock()
	n := c.n
	delete(pbCounters, b)
	libUnlock()
	// Check that body ran until pb.Next returned false, rather than returning early.
	if n <= uint64(b.N) && !b.Failed() {
		b.Fatal("RunParallel: body exited without pb.Next() == false")
	}
}

// SetParallelism sets the number of goroutines used by [B.RunParallel] to p*GOMAXPROCS.
// There is usually no need to call SetParallelism for CPU-bound benchmarks.
// If p is less than 1, this call will have no effect.
func (b *B) SetParallelism(p int) {
	if p >= 1 {
		b.parallelism = p
	}
}

// Benchmark benchmarks a single function. It is useful for creating
// custom benchmarks that do not use the "go test" command.
//
// If f depends on testing flags, then [Init] must be used to register
// those flags before calling Benchmark and before calling [flag.Parse].
//
// If f calls Run, the result will be an estimate of running all its
// subbenchmarks that don't call Run in sequence in a single benchmark.
func Benchmark(f func(b *B)) BenchmarkResult {
	Init()
	b := &B{
		common: common{
			signal: make(chan bool),
			w:      discard{},
		},
		benchFunc: f,
		benchN:    benchTimeN,
		benchD:    benchTimeD,
	}
	if b.run1() {
		b.run()
	}
	return b.result
}

type discard struct{}

func (discard) Write(b []byte) (n int, err error) { return len(b), nil }

// AllocsPerRun returns the average number of allocations during calls to f.
// Although the return value has type float64, it will always be an integral value.
//
// To compute the number of allocations, the function will first be run once as a warm-up.
// The average number of allocations over the specified number of runs will then be measured and returned.
//
// The allocations counted are the calls of C++ operator new that f makes (and any goroutine that
// runs at the same time), not the allocations of Go's compiler: a value that Go keeps on the stack
// can cost one here, and the other way round (testing.cpp).
func AllocsPerRun(runs int, f func()) (avg float64) {
	// Warm up the function
	f()

	libAllocCounting(1)
	defer libAllocCounting(-1)
	// Measure the starting statistics
	mallocs0, _ := libAllocs()

	for i := 0; i < runs; i++ {
		f()
	}

	// Read the final statistics
	mallocs1, _ := libAllocs()

	// Average the mallocs over the runs (not counting the one-time warm-up).
	// Rounding to integer is also important
	return float64((mallocs1 - mallocs0) / runs)
}
