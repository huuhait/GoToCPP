// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ported by hand from go1.25.14's src/runtime (extern.go, mprof.go, mstats.go, panic.go, proc.go,
// symtab.go, ...): only the part of the package that gocpp has, with its documentation; PORT.md
// lists what is left out and what differs. The functions that need the machine are
// //gocpp:native (ADR-027, gocpp/runtimepkg.hpp).

/*
Package runtime contains operations that interact with Go's runtime system,
such as functions to control goroutines. It also includes the low-level type information
used by the reflect package; see [reflect]'s documentation for the programmable
interface to the run-time type system.

# Environment Variables

The following environment variables ($name or %name%, depending on the host
operating system) control the run-time behavior of Go programs. The meanings
and use may change from release to release.

The GOGC variable sets the initial garbage collection target percentage.
A collection is triggered when the ratio of freshly allocated data to live data
remaining after the previous collection reaches this percentage. The default
is GOGC=100. Setting GOGC=off disables the garbage collector entirely.
[runtime/debug.SetGCPercent] allows changing this percentage at run time.

The GOMEMLIMIT variable sets a soft memory limit for the runtime. This memory limit
includes the Go heap and all other memory managed by the runtime, and excludes
external memory sources such as mappings of the binary itself, memory managed in
other languages, and memory held by the operating system on behalf of the Go
program. GOMEMLIMIT is a numeric value in bytes with an optional unit suffix.
The supported suffixes include B, KiB, MiB, GiB, and TiB. These suffixes
represent quantities of bytes as defined by the IEC 80000-13 standard. That is,
they are based on powers of two: KiB means 2^10 bytes, MiB means 2^20 bytes,
and so on. The default setting is [math.MaxInt64], which effectively disables the
memory limit. [runtime/debug.SetMemoryLimit] allows changing this limit at run
time.

The GODEBUG variable controls debugging variables within the runtime.
It is a comma-separated list of name=val pairs setting these named variables:

	clobberfree: setting clobberfree=1 causes the garbage collector to
	clobber the memory content of an object with bad content when it frees
	the object.

	cpu.*: cpu.all=off disables the use of all optional instruction set extensions.
	cpu.extension=off disables use of instructions from the specified instruction set extension.
	extension is the lower case name for the instruction set extension such as sse41 or avx
	as listed in internal/cpu package. As an example cpu.avx=off disables runtime detection
	and thereby use of AVX instructions.

	cgocheck: setting cgocheck=0 disables all checks for packages
	using cgo to incorrectly pass Go pointers to non-Go code.
	Setting cgocheck=1 (the default) enables relatively cheap
	checks that may miss some errors. A more complete, but slow,
	cgocheck mode can be enabled using GOEXPERIMENT (which
	requires a rebuild), see https://pkg.go.dev/internal/goexperiment for details.

	checkfinalizers: setting checkfinalizers=1 causes the garbage collector to run
	multiple partial non-parallel stop-the-world collections to identify common issues with
	finalizers and cleanups, like those listed at
	https://go.dev/doc/gc-guide#Finalizers_cleanups_and_weak_pointers. If a potential issue
	is found, the program will terminate with a description of all potential issues, the
	associated values, and a list of those values' finalizers and cleanups, including where
	they were created. It also adds tracking for tiny blocks to help diagnose issues with
	those as well. The analysis performed during the partial collection is conservative.
	Notably, it flags any path back to the original object from the cleanup function,
	cleanup arguments, or finalizer function as a potential issue, even if that path might
	be severed sometime later during execution (though this is not a recommended pattern).
	This mode also produces one line of output to stderr every GC cycle with information
	about the finalizer and cleanup queue lengths. Lines produced by this mode start with
	"checkfinalizers:".

	decoratemappings: controls whether the Go runtime annotates OS
	anonymous memory mappings with context about their purpose. These
	annotations appear in /proc/self/maps and /proc/self/smaps as
	"[anon: Go: ...]". This setting is only used on Linux. For Go 1.25, it
	defaults to `decoratemappings=1`, enabling annotations. Using
	`decoratemappings=0` reverts to the pre-Go 1.25 behavior.

	disablethp: setting disablethp=1 on Linux disables transparent huge pages for the heap.
	It has no effect on other platforms. disablethp is meant for compatibility with versions
	of Go before 1.21, which stopped working around a Linux kernel default that can result
	in significant memory overuse. See https://go.dev/issue/64332. This setting will be
	removed in a future release, so operators should tweak their Linux configuration to suit
	their needs before then. See https://go.dev/doc/gc-guide#Linux_transparent_huge_pages.

	dontfreezetheworld: by default, the start of a fatal panic or throw
	"freezes the world", preempting all threads to stop all running
	goroutines, which makes it possible to traceback all goroutines, and
	keeps their state close to the point of panic. Setting
	dontfreezetheworld=1 disables this preemption, allowing goroutines to
	continue executing during panic processing. Note that goroutines that
	naturally enter the scheduler will still stop. This can be useful when
	debugging the runtime scheduler, as freezetheworld perturbs scheduler
	state and thus may hide problems.

	efence: setting efence=1 causes the allocator to run in a mode
	where each object is allocated on a unique page and addresses are
	never recycled.

	gccheckmark: setting gccheckmark=1 enables verification of the
	garbage collector's concurrent mark phase by performing a
	second mark pass while the world is stopped.  If the second
	pass finds a reachable object that was not found by concurrent
	mark, the garbage collector will panic.

	gcpacertrace: setting gcpacertrace=1 causes the garbage collector to
	print information about the internal state of the concurrent pacer.

	gcshrinkstackoff: setting gcshrinkstackoff=1 disables moving goroutines
	onto smaller stacks. In this mode, a goroutine's stack can only grow.

	gcstoptheworld: setting gcstoptheworld=1 disables concurrent garbage collection,
	making every garbage collection a stop-the-world event. Setting gcstoptheworld=2
	also disables concurrent sweeping after the garbage collection finishes.

	gctrace: setting gctrace=1 causes the garbage collector to emit a single line to standard
	error at each collection, summarizing the amount of memory collected and the
	length of the pause. The format of this line is subject to change. Included in
	the explanation below is also the relevant runtime/metrics metric for each field.
	Currently, it is:
		gc # @#s #%: #+#+# ms clock, #+#/#/#+# ms cpu, #->#-># MB, # MB goal, # MB stacks, #MB globals, # P
	where the fields are as follows:
		gc #         the GC number, incremented at each GC
		@#s          time in seconds since program start
		#%           percentage of time spent in GC since program start
		#+...+#      wall-clock/CPU times for the phases of the GC
		#->#-># MB   heap size at GC start, at GC end, and live heap, or /gc/scan/heap:bytes
		# MB goal    goal heap size, or /gc/heap/goal:bytes
		# MB stacks  estimated scannable stack size, or /gc/scan/stack:bytes
		# MB globals scannable global size, or /gc/scan/globals:bytes
		# P          number of processors used, or /sched/gomaxprocs:threads
	The phases are stop-the-world (STW) sweep termination, concurrent
	mark and scan, and STW mark termination. The CPU times
	for mark/scan are broken down in to assist time (GC performed in
	line with allocation), background GC time, and idle GC time.
	If the line ends with "(forced)", this GC was forced by a
	runtime.GC() call.

	harddecommit: setting harddecommit=1 causes memory that is returned to the OS to
	also have protections removed on it. This is the only mode of operation on Windows,
	but is helpful in debugging scavenger-related issues on other platforms. Currently,
	only supported on Linux.

	inittrace: setting inittrace=1 causes the runtime to emit a single line to standard
	error for each package with init work, summarizing the execution time and memory
	allocation. No information is printed for inits executed as part of plugin loading
	and for packages without both user defined and compiler generated init work.
	The format of this line is subject to change. Currently, it is:
		init # @#ms, # ms clock, # bytes, # allocs
	where the fields are as follows:
		init #      the package name
		@# ms       time in milliseconds when the init started since program start
		# clock     wall-clock time for package initialization work
		# bytes     memory allocated on the heap
		# allocs    number of heap allocations

	madvdontneed: setting madvdontneed=0 will use MADV_FREE
	instead of MADV_DONTNEED on Linux when returning memory to the
	kernel. This is more efficient, but means RSS numbers will
	drop only when the OS is under memory pressure. On the BSDs and
	Illumos/Solaris, setting madvdontneed=1 will use MADV_DONTNEED instead
	of MADV_FREE. This is less efficient, but causes RSS numbers to drop
	more quickly.

	memprofilerate: setting memprofilerate=X will update the value of runtime.MemProfileRate.
	When set to 0 memory profiling is disabled.  Refer to the description of
	MemProfileRate for the default value.

	profstackdepth: profstackdepth=128 (the default) will set the maximum stack
	depth used by all pprof profilers except for the CPU profiler to 128 frames.
	Stack traces that exceed this limit will be truncated to the limit starting
	from the leaf frame. Setting profstackdepth to any value above 1024 will
	silently default to 1024. Future versions of Go may remove this limitation
	and extend profstackdepth to apply to the CPU profiler and execution tracer.

	pagetrace: setting pagetrace=/path/to/file will write out a trace of page events
	that can be viewed, analyzed, and visualized using the x/debug/cmd/pagetrace tool.
	Build your program with GOEXPERIMENT=pagetrace to enable this functionality. Do not
	enable this functionality if your program is a setuid binary as it introduces a security
	risk in that scenario. Currently not supported on Windows, plan9 or js/wasm. Setting this
	option for some applications can produce large traces, so use with care.

	panicnil: setting panicnil=1 disables the runtime error when calling panic with nil
	interface value or an untyped nil.

	invalidptr: invalidptr=1 (the default) causes the garbage collector and stack
	copier to crash the program if an invalid pointer value (for example, 1)
	is found in a pointer-typed location. Setting invalidptr=0 disables this check.
	This should only be used as a temporary workaround to diagnose buggy code.
	The real fix is to not store integers in pointer-typed locations.

	sbrk: setting sbrk=1 replaces the memory allocator and garbage collector
	with a trivial allocator that obtains memory from the operating system and
	never reclaims any memory.

	scavtrace: setting scavtrace=1 causes the runtime to emit a single line to standard
	error, roughly once per GC cycle, summarizing the amount of work done by the
	scavenger as well as the total amount of memory returned to the operating system
	and an estimate of physical memory utilization. The format of this line is subject
	to change, but currently it is:
		scav # KiB work (bg), # KiB work (eager), # KiB total, #% util
	where the fields are as follows:
		# KiB work (bg)    the amount of memory returned to the OS in the background since
		                   the last line
		# KiB work (eager) the amount of memory returned to the OS eagerly since the last line
		# KiB now          the amount of address space currently returned to the OS
		#% util            the fraction of all unscavenged heap memory which is in-use
	If the line ends with "(forced)", then scavenging was forced by a
	debug.FreeOSMemory() call.

	scheddetail: setting schedtrace=X and scheddetail=1 causes the scheduler to emit
	detailed multiline info every X milliseconds, describing state of the scheduler,
	processors, threads and goroutines.

	schedtrace: setting schedtrace=X causes the scheduler to emit a single line to standard
	error every X milliseconds, summarizing the scheduler state.

	tracebackancestors: setting tracebackancestors=N extends tracebacks with the stacks at
	which goroutines were created, where N limits the number of ancestor goroutines to
	report. This also extends the information returned by runtime.Stack.
	Setting N to 0 will report no ancestry information.

	tracefpunwindoff: setting tracefpunwindoff=1 forces the execution tracer to
	use the runtime's default stack unwinder instead of frame pointer unwinding.
	This increases tracer overhead, but could be helpful as a workaround or for
	debugging unexpected regressions caused by frame pointer unwinding.

	traceadvanceperiod: the approximate period in nanoseconds between trace generations. Only
	applies if a program is built with GOEXPERIMENT=exectracer2. Used primarily for testing
	and debugging the execution tracer.

	tracecheckstackownership: setting tracecheckstackownership=1 enables a debug check in the
	execution tracer to double-check stack ownership before taking a stack trace.

	asyncpreemptoff: asyncpreemptoff=1 disables signal-based
	asynchronous goroutine preemption. This makes some loops
	non-preemptible for long periods, which may delay GC and
	goroutine scheduling. This is useful for debugging GC issues
	because it also disables the conservative stack scanning used
	for asynchronously preempted goroutines.

The [net] and [net/http] packages also refer to debugging variables in GODEBUG.
See the documentation for those packages for details.

The GOMAXPROCS variable limits the number of operating system threads that
can execute user-level Go code simultaneously. There is no limit to the number of threads
that can be blocked in system calls on behalf of Go code; those do not count against
the GOMAXPROCS limit. This package's [GOMAXPROCS] function queries and changes
the limit.

The GORACE variable configures the race detector, for programs built using -race.
See the [Race Detector article] for details.

The GOTRACEBACK variable controls the amount of output generated when a Go
program fails due to an unrecovered panic or an unexpected runtime condition.
By default, a failure prints a stack trace for the current goroutine,
eliding functions internal to the run-time system, and then exits with exit code 2.
The failure prints stack traces for all goroutines if there is no current goroutine
or the failure is internal to the run-time.
GOTRACEBACK=none omits the goroutine stack traces entirely.
GOTRACEBACK=single (the default) behaves as described above.
GOTRACEBACK=all adds stack traces for all user-created goroutines.
GOTRACEBACK=system is like “all” but adds stack frames for run-time functions
and shows goroutines created internally by the run-time.
GOTRACEBACK=crash is like “system” but crashes in an operating system-specific
manner instead of exiting. For example, on Unix systems, the crash raises
SIGABRT to trigger a core dump.
GOTRACEBACK=wer is like “crash” but doesn't disable Windows Error Reporting (WER).
For historical reasons, the GOTRACEBACK settings 0, 1, and 2 are synonyms for
none, all, and system, respectively.
The [runtime/debug.SetTraceback] function allows increasing the
amount of output at run time, but it cannot reduce the amount below that
specified by the environment variable.

The GOARCH, GOOS, GOPATH, and GOROOT environment variables complete
the set of Go environment variables. They influence the building of Go programs
(see [cmd/go] and [go/build]).
GOARCH, GOOS, and GOROOT are recorded at compile time and made available by
constants or functions in this package, but they do not influence the execution
of the run-time system.

# Security

On Unix platforms, Go's runtime system behaves slightly differently when a
binary is setuid/setgid or executed with setuid/setgid-like properties, in order
to prevent dangerous behaviors. On Linux this is determined by checking for the
AT_SECURE flag in the auxiliary vector, on the BSDs and Solaris/Illumos it is
determined by checking the issetugid syscall, and on AIX it is determined by
checking if the uid/gid match the effective uid/gid.

When the runtime determines the binary is setuid/setgid-like, it does three main
things:
  - The standard input/output file descriptors (0, 1, 2) are checked to be open.
    If any of them are closed, they are opened pointing at /dev/null.
  - The value of the GOTRACEBACK environment variable is set to 'none'.
  - When a signal is received that terminates the program, or the program
    encounters an unrecoverable panic that would otherwise override the value
    of GOTRACEBACK, the goroutine stack, registers, and other memory related
    information are omitted.

[Race Detector article]: https://go.dev/doc/articles/race_detector
*/
package runtime

import (
	"internal/goarch"
	"internal/goos"
)

// Compiler is the name of the compiler toolchain that built the
// running binary. Known toolchains are:
//
//	gc      Also known as cmd/compile.
//	gccgo   The gccgo front end, part of the GCC compiler suite.
const Compiler = "gc"

// SetCPUProfileRate sets the CPU profiling rate to hz samples per second.
// If hz <= 0, SetCPUProfileRate turns off profiling.
// If the profiler is on, the rate cannot be changed without first turning it off.
//
// Most clients should use the [runtime/pprof] package or
// the [testing] package's -test.cpuprofile flag instead of calling
// SetCPUProfileRate directly.
func SetCPUProfileRate(hz int) {}

// GOMAXPROCS sets the maximum number of CPUs that can be executing
// simultaneously and returns the previous setting. If n < 1, it does not change
// the current setting.
//
// # Default
//
// If the GOMAXPROCS environment variable is set to a positive whole number,
// GOMAXPROCS defaults to that value.
//
// Otherwise, the Go runtime selects an appropriate default value from a combination of
//   - the number of logical CPUs on the machine,
//   - the process’s CPU affinity mask,
//   - and, on Linux, the process’s average CPU throughput limit based on cgroup CPU
//     quota, if any.
//
// If GODEBUG=containermaxprocs=0 is set and GOMAXPROCS is not set by the
// environment variable, then GOMAXPROCS instead defaults to the value of
// [runtime.NumCPU]. Note that GODEBUG=containermaxprocs=0 is [default] for
// language version 1.24 and below.
//
// # Updates
//
// The Go runtime periodically updates the default value based on changes to
// the total logical CPU count, the CPU affinity mask, or cgroup quota. Setting
// a custom value with the GOMAXPROCS environment variable or by calling
// GOMAXPROCS disables automatic updates. The default value and automatic
// updates can be restored by calling [SetDefaultGOMAXPROCS].
//
// If GODEBUG=updatemaxprocs=0 is set, the Go runtime does not perform
// automatic GOMAXPROCS updating. Note that GODEBUG=updatemaxprocs=0 is
// [default] for language version 1.24 and below.
//
// # Compatibility
//
// Note that the default GOMAXPROCS behavior may change as the scheduler
// improves, especially the implementation detail below.
//
// # Implementation details
//
// When computing default GOMAXPROCS via cgroups, the Go runtime computes the
// "average CPU throughput limit" as the cgroup CPU quota / period. In cgroup
// v2, these values come from the cpu.max file. In cgroup v1, they come from
// cpu.cfs_quota_us and cpu.cfs_period_us, respectively. In container runtimes
// that allow configuring CPU limits, this value usually corresponds to the
// "CPU limit" option, not "CPU request".
//
// The Go runtime typically selects the default GOMAXPROCS as the minimum of
// the logical CPU count, the CPU affinity mask count, or the cgroup CPU
// throughput limit. However, it will never set GOMAXPROCS less than 2 unless
// the logical CPU count or CPU affinity mask count are below 2.
//
// If the cgroup CPU throughput limit is not a whole number, the Go runtime
// rounds up to the next whole number.
//
// GOMAXPROCS updates are performed up to once per second, or less if the
// application is idle.
//
// [default]: https://go.dev/doc/godebug#default
//
//gocpp:native gp::runtime::gomaxprocs
func GOMAXPROCS(n int) int

// SetDefaultGOMAXPROCS updates the GOMAXPROCS setting to the runtime
// default, as described by [GOMAXPROCS], ignoring the GOMAXPROCS
// environment variable.
//
// SetDefaultGOMAXPROCS can be used to enable the default automatic updating
// GOMAXPROCS behavior if it has been disabled by the GOMAXPROCS
// environment variable or a prior call to [GOMAXPROCS], or to force an immediate
// update if the caller is aware of a change to the total logical CPU count, CPU
// affinity mask or cgroup quota.
//
//gocpp:native gp::runtime::set_default_gomaxprocs
func SetDefaultGOMAXPROCS()

// NumCPU returns the number of logical CPUs usable by the current process.
//
// The set of available CPUs is checked by querying the operating system
// at process startup. Changes to operating system CPU allocation after
// process startup are not reflected.
//
//gocpp:native gp::runtime::num_cpu
func NumCPU() int

// NumCgoCall returns the number of cgo calls made by the current process.
//
//gocpp:native gp::runtime::num_cgo_call
func NumCgoCall() int64

// NumGoroutine returns the number of goroutines that currently exist.
//
//gocpp:native gp::runtime::num_goroutine
func NumGoroutine() int

// Error identifies a runtime error used in panic.
//
// The Go runtime triggers panics for a variety of cases, as described by the
// Go Language Spec, such as out-of-bounds slice/array access, close of nil
// channels, type assertion failures, etc.
//
// When these cases occur, the Go runtime panics with an error that implements
// Error. This can be useful when recovering from panics to distinguish between
// custom application panics and fundamental runtime panics.
//
// Packages outside of the Go standard library should not implement Error.
type Error interface {
	error
	// RuntimeError is a no-op function but
	// serves to distinguish types that are runtime
	// errors from ordinary errors: a type is a
	// runtime error if it has a RuntimeError method.
	RuntimeError()
}

// Caller reports file and line number information about function invocations on
// the calling goroutine's stack. The argument skip is the number of stack frames
// to ascend, with 0 identifying the caller of Caller. (For historical reasons the
// meaning of skip differs between Caller and [Callers].) The return values report
// the program counter, the file name (using forward slashes as path separator, even
// on Windows), and the line number within the file of the corresponding call.
// The boolean ok is false if it was not possible to recover the information.
//
//gocpp:native gp::runtime::caller
func Caller(skip int) (pc uintptr, file string, line int, ok bool)

// Callers fills the slice pc with the return program counters of function invocations
// on the calling goroutine's stack. The argument skip is the number of stack frames
// to skip before recording in pc, with 0 identifying the frame for Callers itself and
// 1 identifying the caller of Callers.
// It returns the number of entries written to pc.
//
// To translate these PCs into symbolic information such as function
// names and line numbers, use [CallersFrames]. CallersFrames accounts
// for inlined functions and adjusts the return program counters into
// call program counters. Iterating over the returned slice of PCs
// directly is discouraged, as is using [FuncForPC] on any of the
// returned PCs, since these cannot account for inlining or return
// program counter adjustment.
//
//gocpp:native gp::runtime::callers
func Callers(skip int, pc []uintptr) int

// GOROOT returns the root of the Go tree. It uses the
// GOROOT environment variable, if set at process start,
// or else the root used during the Go build.
//
// Deprecated: The root used during the Go build will not be
// meaningful if the binary is copied to another machine.
// Use the system path to locate the “go” binary, and use
// “go env GOROOT” to find its GOROOT.
//
//gocpp:native gp::runtime::goroot
func GOROOT() string

// Version returns the Go tree's version string.
// It is either the commit hash and date at the time of the build or,
// when possible, a release tag like "go1.3".
//
//gocpp:native gp::runtime::version
func Version() string

// GOOS is the running program's operating system target:
// one of darwin, freebsd, linux, and so on.
// To view possible combinations of GOOS and GOARCH, run "go tool dist list".
const GOOS string = goos.GOOS

// GOARCH is the running program's architecture target:
// one of 386, amd64, arm, s390x, and so on.
const GOARCH string = goarch.GOARCH

// SetFinalizer sets the finalizer associated with obj to the provided
// finalizer function. When the garbage collector finds an unreachable block
// with an associated finalizer, it clears the association and runs
// finalizer(obj) in a separate goroutine. This makes obj reachable again,
// but now without an associated finalizer. Assuming that SetFinalizer
// is not called again, the next time the garbage collector sees
// that obj is unreachable, it will free obj.
//
// SetFinalizer(obj, nil) clears any finalizer associated with obj.
//
// New Go code should consider using [AddCleanup] instead, which is much
// less error-prone than SetFinalizer.
//
// The argument obj must be a pointer to an object allocated by calling
// new, by taking the address of a composite literal, or by taking the
// address of a local variable.
// The argument finalizer must be a function that takes a single argument
// to which obj's type can be assigned, and can have arbitrary ignored return
// values. If either of these is not true, SetFinalizer may abort the
// program.
//
// Finalizers are run in dependency order: if A points at B, both have
// finalizers, and they are otherwise unreachable, only the finalizer
// for A runs; once A is freed, the finalizer for B can run.
// If a cyclic structure includes a block with a finalizer, that
// cycle is not guaranteed to be garbage collected and the finalizer
// is not guaranteed to run, because there is no ordering that
// respects the dependencies.
//
// The finalizer is scheduled to run at some arbitrary time after the
// program can no longer reach the object to which obj points.
// There is no guarantee that finalizers will run before a program exits,
// so typically they are useful only for releasing non-memory resources
// associated with an object during a long-running program.
// For example, an [os.File] object could use a finalizer to close the
// associated operating system file descriptor when a program discards
// an os.File without calling Close, but it would be a mistake
// to depend on a finalizer to flush an in-memory I/O buffer such as a
// [bufio.Writer], because the buffer would not be flushed at program exit.
//
// It is not guaranteed that a finalizer will run if the size of *obj is
// zero bytes, because it may share same address with other zero-size
// objects in memory. See https://go.dev/ref/spec#Size_and_alignment_guarantees.
//
// It is not guaranteed that a finalizer will run for objects allocated
// in initializers for package-level variables. Such objects may be
// linker-allocated, not heap-allocated.
//
// Note that because finalizers may execute arbitrarily far into the future
// after an object is no longer referenced, the runtime is allowed to perform
// a space-saving optimization that batches objects together in a single
// allocation slot. The finalizer for an unreferenced object in such an
// allocation may never run if it always exists in the same batch as a
// referenced object. Typically, this batching only happens for tiny
// (on the order of 16 bytes or less) and pointer-free objects.
//
// A finalizer may run as soon as an object becomes unreachable.
// In order to use finalizers correctly, the program must ensure that
// the object is reachable until it is no longer required.
// Objects stored in global variables, or that can be found by tracing
// pointers from a global variable, are reachable. A function argument or
// receiver may become unreachable at the last point where the function
// mentions it. To make an unreachable object reachable, pass the object
// to a call of the [KeepAlive] function to mark the last point in the
// function where the object must be reachable.
//
// For example, if p points to a struct, such as os.File, that contains
// a file descriptor d, and p has a finalizer that closes that file
// descriptor, and if the last use of p in a function is a call to
// syscall.Write(p.d, buf, size), then p may be unreachable as soon as
// the program enters [syscall.Write]. The finalizer may run at that moment,
// closing p.d, causing syscall.Write to fail because it is writing to
// a closed file descriptor (or, worse, to an entirely different
// file descriptor opened by a different goroutine). To avoid this problem,
// call KeepAlive(p) after the call to syscall.Write.
//
// A single goroutine runs all finalizers for a program, sequentially.
// If a finalizer must run for a long time, it should do so by starting
// a new goroutine.
//
// In the terminology of the Go memory model, a call
// SetFinalizer(x, f) “synchronizes before” the finalization call f(x).
// However, there is no guarantee that KeepAlive(x) or any other use of x
// “synchronizes before” f(x), so in general a finalizer should use a mutex
// or other synchronization mechanism if it needs to access mutable state in x.
// For example, consider a finalizer that inspects a mutable field in x
// that is modified from time to time in the main program before x
// becomes unreachable and the finalizer is invoked.
// The modifications in the main program and the inspection in the finalizer
// need to use appropriate synchronization, such as mutexes or atomic updates,
// to avoid read-write races.
//
//gocpp:native gp::runtime::set_finalizer
func SetFinalizer(obj any, finalizer any)

// KeepAlive marks its argument as currently reachable.
// This ensures that the object is not freed, and its finalizer is not run,
// before the point in the program where KeepAlive is called.
//
// A very simplified example showing where KeepAlive is required:
//
//	type File struct { d int }
//	d, err := syscall.Open("/file/path", syscall.O_RDONLY, 0)
//	// ... do something if err != nil ...
//	p := &File{d}
//	runtime.SetFinalizer(p, func(p *File) { syscall.Close(p.d) })
//	var buf [10]byte
//	n, err := syscall.Read(p.d, buf[:])
//	// Ensure p is not finalized until Read returns.
//	runtime.KeepAlive(p)
//	// No more uses of p after this point.
//
// Without the KeepAlive call, the finalizer could run at the start of
// [syscall.Read], closing the file descriptor before syscall.Read makes
// the actual system call.
//
// Note: KeepAlive should only be used to prevent finalizers from
// running prematurely. In particular, when used with [unsafe.Pointer],
// the rules for valid uses of unsafe.Pointer still apply.
//
//gocpp:native gp::runtime::keep_alive
func KeepAlive(x any)

// GC runs a garbage collection and blocks the caller until the
// garbage collection is complete. It may also block the entire
// program.
//
//gocpp:native gp::runtime::gc
func GC()

// SetBlockProfileRate controls the fraction of goroutine blocking events
// that are reported in the blocking profile. The profiler aims to sample
// an average of one blocking event per rate nanoseconds spent blocked.
//
// To include every blocking event in the profile, pass rate = 1.
// To turn off profiling entirely, pass rate <= 0.
func SetBlockProfileRate(rate int) {}

// SetMutexProfileFraction controls the fraction of mutex contention events
// that are reported in the mutex profile. On average 1/rate events are
// reported. The previous rate is returned.
//
// To turn off profiling entirely, pass rate 0.
// To just read the current rate, pass rate < 0.
// (For n>1 the details of sampling may change.)
//
//gocpp:native gp::runtime::set_mutex_profile_fraction
func SetMutexProfileFraction(rate int) int

// Stack formats a stack trace of the calling goroutine into buf
// and returns the number of bytes written to buf.
// If all is true, Stack formats stack traces of all other goroutines
// into buf after the trace for the current goroutine.
//
//gocpp:native gp::runtime::stack
func Stack(buf []byte, all bool) int

// A MemStats records statistics about the memory allocator.
type MemStats struct {
	// Alloc is bytes of allocated heap objects.
	//
	// This is the same as HeapAlloc (see below).
	Alloc uint64
	// TotalAlloc is cumulative bytes allocated for heap objects.
	//
	// TotalAlloc increases as heap objects are allocated, but
	// unlike Alloc and HeapAlloc, it does not decrease when
	// objects are freed.
	TotalAlloc uint64
	// Sys is the total bytes of memory obtained from the OS.
	//
	// Sys is the sum of the XSys fields below. Sys measures the
	// virtual address space reserved by the Go runtime for the
	// heap, stacks, and other internal data structures. It's
	// likely that not all of the virtual address space is backed
	// by physical memory at any given moment, though in general
	// it all was at some point.
	Sys uint64
	// Lookups is the number of pointer lookups performed by the
	// runtime.
	//
	// This is primarily useful for debugging runtime internals.
	Lookups uint64
	// Mallocs is the cumulative count of heap objects allocated.
	// The number of live objects is Mallocs - Frees.
	Mallocs uint64
	// Frees is the cumulative count of heap objects freed.
	Frees uint64
	// HeapAlloc is bytes of allocated heap objects.
	//
	// "Allocated" heap objects include all reachable objects, as
	// well as unreachable objects that the garbage collector has
	// not yet freed. Specifically, HeapAlloc increases as heap
	// objects are allocated and decreases as the heap is swept
	// and unreachable objects are freed. Sweeping occurs
	// incrementally between GC cycles, so these two processes
	// occur simultaneously, and as a result HeapAlloc tends to
	// change smoothly (in contrast with the sawtooth that is
	// typical of stop-the-world garbage collectors).
	HeapAlloc uint64
	// HeapSys is bytes of heap memory obtained from the OS.
	//
	// HeapSys measures the amount of virtual address space
	// reserved for the heap. This includes virtual address space
	// that has been reserved but not yet used, which consumes no
	// physical memory, but tends to be small, as well as virtual
	// address space for which the physical memory has been
	// returned to the OS after it became unused (see HeapReleased
	// for a measure of the latter).
	//
	// HeapSys estimates the largest size the heap has had.
	HeapSys uint64
	// HeapIdle is bytes in idle (unused) spans.
	//
	// Idle spans have no objects in them. These spans could be
	// (and may already have been) returned to the OS, or they can
	// be reused for heap allocations, or they can be reused as
	// stack memory.
	//
	// HeapIdle minus HeapReleased estimates the amount of memory
	// that could be returned to the OS, but is being retained by
	// the runtime so it can grow the heap without requesting more
	// memory from the OS. If this difference is significantly
	// larger than the heap size, it indicates there was a recent
	// transient spike in live heap size.
	HeapIdle uint64
	// HeapInuse is bytes in in-use spans.
	//
	// In-use spans have at least one object in them. These spans
	// can only be used for other objects of roughly the same
	// size.
	//
	// HeapInuse minus HeapAlloc estimates the amount of memory
	// that has been dedicated to particular size classes, but is
	// not currently being used. This is an upper bound on
	// fragmentation, but in general this memory can be reused
	// efficiently.
	HeapInuse uint64
	// HeapReleased is bytes of physical memory returned to the OS.
	//
	// This counts heap memory from idle spans that was returned
	// to the OS and has not yet been reacquired for the heap.
	HeapReleased uint64
	// HeapObjects is the number of allocated heap objects.
	//
	// Like HeapAlloc, this increases as objects are allocated and
	// decreases as the heap is swept and unreachable objects are
	// freed.
	HeapObjects uint64
	// StackInuse is bytes in stack spans.
	//
	// In-use stack spans have at least one stack in them. These
	// spans can only be used for other stacks of the same size.
	//
	// There is no StackIdle because unused stack spans are
	// returned to the heap (and hence counted toward HeapIdle).
	StackInuse uint64
	// StackSys is bytes of stack memory obtained from the OS.
	//
	// StackSys is StackInuse, plus any memory obtained directly
	// from the OS for OS thread stacks.
	//
	// In non-cgo programs this metric is currently equal to StackInuse
	// (but this should not be relied upon, and the value may change in
	// the future).
	//
	// In cgo programs this metric includes OS thread stacks allocated
	// directly from the OS. Currently, this only accounts for one stack in
	// c-shared and c-archive build modes and other sources of stacks from
	// the OS (notably, any allocated by C code) are not currently measured.
	// Note this too may change in the future.
	StackSys uint64
	// MSpanInuse is bytes of allocated mspan structures.
	MSpanInuse uint64
	// MSpanSys is bytes of memory obtained from the OS for mspan
	// structures.
	MSpanSys uint64
	// MCacheInuse is bytes of allocated mcache structures.
	MCacheInuse uint64
	// MCacheSys is bytes of memory obtained from the OS for
	// mcache structures.
	MCacheSys uint64
	// BuckHashSys is bytes of memory in profiling bucket hash tables.
	BuckHashSys uint64
	// GCSys is bytes of memory in garbage collection metadata.
	GCSys uint64
	// OtherSys is bytes of memory in miscellaneous off-heap
	// runtime allocations.
	OtherSys uint64
	// NextGC is the target heap size of the next GC cycle.
	//
	// The garbage collector's goal is to keep HeapAlloc ≤ NextGC.
	// At the end of each GC cycle, the target for the next cycle
	// is computed based on the amount of reachable data and the
	// value of GOGC.
	NextGC uint64
	// LastGC is the time the last garbage collection finished, as
	// nanoseconds since 1970 (the UNIX epoch).
	LastGC uint64
	// PauseTotalNs is the cumulative nanoseconds in GC
	// stop-the-world pauses since the program started.
	//
	// During a stop-the-world pause, all goroutines are paused
	// and only the garbage collector can run.
	PauseTotalNs uint64
	// PauseNs is a circular buffer of recent GC stop-the-world
	// pause times in nanoseconds.
	//
	// The most recent pause is at PauseNs[(NumGC+255)%256]. In
	// general, PauseNs[N%256] records the time paused in the most
	// recent N%256th GC cycle. There may be multiple pauses per
	// GC cycle; this is the sum of all pauses during a cycle.
	PauseNs [256]uint64
	// PauseEnd is a circular buffer of recent GC pause end times,
	// as nanoseconds since 1970 (the UNIX epoch).
	//
	// This buffer is filled the same way as PauseNs. There may be
	// multiple pauses per GC cycle; this records the end of the
	// last pause in a cycle.
	PauseEnd [256]uint64
	// NumGC is the number of completed GC cycles.
	NumGC uint32
	// NumForcedGC is the number of GC cycles that were forced by
	// the application calling the GC function.
	NumForcedGC uint32
	// GCCPUFraction is the fraction of this program's available
	// CPU time used by the GC since the program started.
	//
	// GCCPUFraction is expressed as a number between 0 and 1,
	// where 0 means GC has consumed none of this program's CPU. A
	// program's available CPU time is defined as the integral of
	// GOMAXPROCS since the program started. That is, if
	// GOMAXPROCS is 2 and a program has been running for 10
	// seconds, its "available CPU" is 20 seconds. GCCPUFraction
	// does not include CPU time used for write barrier activity.
	//
	// This is the same as the fraction of CPU reported by
	// GODEBUG=gctrace=1.
	GCCPUFraction float64
	// EnableGC indicates that GC is enabled. It is always true,
	// even if GOGC=off.
	EnableGC bool
	// DebugGC is currently unused.
	DebugGC bool
	// BySize reports per-size class allocation statistics.
	//
	// BySize[N] gives statistics for allocations of size S where
	// BySize[N-1].Size < S ≤ BySize[N].Size.
	//
	// This does not report allocations larger than BySize[60].Size.
	BySize [61]struct {
		// Size is the maximum byte size of an object in this
		// size class.
		Size uint32

		// Mallocs is the cumulative count of heap objects
		// allocated in this size class. The cumulative bytes
		// of allocation is Size*Mallocs. The number of live
		// objects in this size class is Mallocs - Frees.
		Mallocs uint64

		// Frees is the cumulative count of heap objects freed
		// in this size class.
		Frees uint64
	}
}

// ReadMemStats populates m with memory allocator statistics.
//
// The returned memory allocator statistics are up to date as of the
// call to ReadMemStats. This is in contrast with a heap profile,
// which is a snapshot as of the most recently completed garbage
// collection cycle.
func ReadMemStats(m *MemStats) {
	inUse, sys, idle := memInfo()
	*m = MemStats{}
	m.Alloc = inUse
	m.HeapAlloc = inUse
	m.HeapInuse = inUse
	m.Sys = sys
	m.HeapSys = sys
	m.HeapIdle = idle
	m.TotalAlloc = totalAlloc(inUse)
	m.NumGC = gcCount()
	m.NumForcedGC = m.NumGC // every cycle is a call of GC
	m.EnableGC = true
}

// Goexit terminates the goroutine that calls it. No other goroutine is affected.
// Goexit runs all deferred calls before terminating the goroutine. Because Goexit
// is not a panic, any recover calls in those deferred functions will return nil.
//
// Calling Goexit from the main goroutine terminates that goroutine
// without func main returning. Since func main has not returned,
// the program continues execution of other goroutines.
// If all other goroutines exit, the program crashes.
//
// It crashes if called from a thread not created by the Go runtime.
//
//gocpp:native gp::goexit
func Goexit()

// Gosched yields the processor, allowing other goroutines to run. It does not
// suspend the current goroutine, so execution resumes automatically.
//
//gocpp:native gp::runtime::gosched
func Gosched()

// LockOSThread wires the calling goroutine to its current operating system thread.
// The calling goroutine will always execute in that thread,
// and no other goroutine will execute in it,
// until the calling goroutine has made as many calls to
// [UnlockOSThread] as to LockOSThread.
// If the calling goroutine exits without unlocking the thread,
// the thread will be terminated.
//
// All init functions are run on the startup thread. Calling LockOSThread
// from an init function will cause the main function to be invoked on
// that thread.
//
// A goroutine should call LockOSThread before calling OS services or
// non-Go library functions that depend on per-thread state.
//
//gocpp:native gp::runtime::lock_os_thread
func LockOSThread()

// UnlockOSThread undoes an earlier call to LockOSThread.
// If this drops the number of active LockOSThread calls on the
// calling goroutine to zero, it unwires the calling goroutine from
// its fixed operating system thread.
// If there are no active LockOSThread calls, this is a no-op.
//
// Before calling UnlockOSThread, the caller must ensure that the OS
// thread is suitable for running other goroutines. If the caller made
// any permanent changes to the state of the thread that would affect
// other goroutines, it should not call this function and thus leave
// the goroutine locked to the OS thread until the goroutine (and
// hence the thread) exits.
//
//gocpp:native gp::runtime::unlock_os_thread
func UnlockOSThread()

// MemProfileRate controls the fraction of memory allocations
// that are recorded and reported in the memory profile.
// The profiler aims to sample an average of
// one allocation per MemProfileRate bytes allocated.
//
// gocpp makes no memory profile: the variable is only kept.
var MemProfileRate int = 512 * 1024

// A TypeAssertionError explains a failed type assertion.
type TypeAssertionError struct {
	msg string
}

func (*TypeAssertionError) RuntimeError() {}

func (e *TypeAssertionError) Error() string {
	return e.msg
}

// A PanicNilError happens when code calls panic(nil).
//
// Before Go 1.21, programs that called panic(nil) observed recover returning nil.
// Starting in Go 1.21, programs that call panic(nil) observe recover returning a *PanicNilError.
// Programs can change back to the old behavior by setting GODEBUG=panicnil=1.
type PanicNilError struct {
	// This field makes PanicNilError structurally different from
	// any other struct in this package, and the _ makes it different
	// from any struct in other packages too.
	// This avoids any accidental conversions being possible
	// between this struct and some other struct sharing the same fields,
	// like happened in go.dev/issue/56603.
	_ [0]*PanicNilError
}

func (*PanicNilError) Error() string { return "panic called with nil argument" }
func (*PanicNilError) RuntimeError() {}

// setPanicValues gives the C++ runtime the values of this package's own types for the panics that
// it makes itself (panic(nil), a failed type assertion): the descriptors of this package are the
// identity of *PanicNilError and *TypeAssertionError, so `r.(*runtime.PanicNilError)` holds for
// them (gocpp/runtimepkg.hpp).
//
//gocpp:native gp::runtime::set_panic_values
func setPanicValues(nilError any, assertion func(msg string) any)

func init() {
	setPanicValues(new(PanicNilError), func(msg string) any { return &TypeAssertionError{msg} })
}

// pcInfos describes the addresses pcs: the names of their functions, the files and lines of
// their code and the entry points of their functions (0 for an address that is in no function).
// With call, the addresses are return addresses, and the call before each one is described.
// The four slices are as long as pcs.
//
//gocpp:native gp::runtime::pc_infos
func pcInfos(pcs []uintptr, call bool, names []string, files []string, lines []int, entries []uintptr)

// pcInfo is pcInfos for one address.
func pcInfo(pc uintptr, call bool) (name, file string, line int, entry uintptr) {
	names, files, lines, entries := make([]string, 1), make([]string, 1), make([]int, 1), make([]uintptr, 1)
	pcInfos([]uintptr{pc}, call, names, files, lines, entries)
	return names[0], files[0], lines[0], entries[0]
}

// funcsLock and funcsUnlock guard funcs.
//
//gocpp:native gp::runtime::funcs_lock
func funcsLock()

//gocpp:native gp::runtime::funcs_unlock
func funcsUnlock()

// funcs holds the Func of each function that FuncForPC or CallersFrames has described, by entry
// address and name, so that a function has one *Func (as in Go, where FuncForPC(pc1) ==
// FuncForPC(pc2) for two addresses of one function). The name is in the key because FuncForPC
// of a call that the compiler inlined names the inlined function with the entry of the function
// it is inlined into, as Go's does.
var funcs = map[funcKey]*Func{}

type funcKey struct {
	entry uintptr
	name  string
}

// funcOf returns the *Func of the function with this name and entry address.
func funcOf(name string, entry uintptr) *Func {
	k := funcKey{entry, name}
	funcsLock()
	f := funcs[k]
	if f == nil {
		f = &Func{name: name, entry: entry}
		funcs[k] = f
	}
	funcsUnlock()
	return f
}

// inlinedPC reports whether pc is an address that Callers made up for a call that the compiler
// inlined (a -release -trace program: runtime/src/frames.cpp, inlined_tag, bit 62). Go's
// CallersFrames gives the frame of an inlined call no Func, and so does gocpp's.
func inlinedPC(pc uintptr) bool {
	return uint64(pc)&(uint64(1)<<62) != 0
}

// Frames may be used to get function/file/line information for a
// slice of PC values returned by [Callers].
type Frames struct {
	frames []Frame
}

// Frame is the information returned by [Frames] for each call frame.
type Frame struct {
	// PC is the program counter for the location in this frame.
	// For a frame that calls another frame, this will be the
	// program counter of a call instruction. Because of inlining,
	// multiple frames may have the same PC value, but different
	// symbolic information.
	PC uintptr
	// Func is the Func value of this call frame. This may be nil
	// for non-Go code or fully inlined functions.
	Func *Func
	// Function is the package path-qualified function name of
	// this call frame. If non-empty, this string uniquely
	// identifies a single function in the program.
	// This may be the empty string if not known.
	// If Func is not nil then Function == Func.Name().
	Function string
	// File and Line are the file name and line number of the
	// location in this frame. For non-leaf frames, this will be
	// the location of a call. These may be the empty string and
	// zero, respectively, if not known. The file name uses
	// forward slashes, even on Windows.
	File string
	Line int
	// Entry point program counter for the function; may be zero
	// if not known. If Func is not nil then Entry ==
	// Func.Entry().
	Entry     uintptr
	startLine int
}

// CallersFrames takes a slice of PC values returned by [Callers] and
// prepares to return function/file/line information.
// Do not change the slice until you are done with the [Frames].
func CallersFrames(callers []uintptr) *Frames {
	n := len(callers)
	names, files, lines, entries := make([]string, n), make([]string, n), make([]int, n), make([]uintptr, n)
	pcInfos(callers, true, names, files, lines, entries)
	ci := &Frames{}
	for i, pc := range callers {
		if entries[i] == 0 {
			continue // an address that is in no function of the program: Go skips it too
		}
		if pc > entries[i] {
			// The pc of the instruction after the call is what Callers gives, and what
			// FuncForPC wants; the frame has the pc of the call.
			pc--
		}
		var fn *Func
		if !inlinedPC(callers[i]) {
			fn = funcOf(names[i], entries[i])
		}
		ci.frames = append(ci.frames, Frame{
			PC:       pc,
			Func:     fn,
			Function: names[i],
			File:     files[i],
			Line:     lines[i],
			Entry:    entries[i],
		})
	}
	return ci
}

// Next returns a [Frame] representing the next call frame in the slice
// of PC values. If it has already returned all call frames, Next
// returns a zero [Frame].
//
// The more result indicates whether the next call to Next will return
// a valid [Frame]. It does not necessarily indicate whether this call
// returned one.
//
// See the [Frames] example for idiomatic usage.
func (ci *Frames) Next() (frame Frame, more bool) {
	if len(ci.frames) == 0 {
		return Frame{}, false
	}
	frame = ci.frames[0]
	ci.frames = ci.frames[1:]
	return frame, len(ci.frames) > 0
}

// A Func represents a Go function in the running binary.
type Func struct {
	name  string
	entry uintptr
}

// FuncForPC returns a *[Func] describing the function that contains the
// given program counter address, or else nil.
//
// If pc represents multiple functions because of inlining, it returns
// the *Func describing the innermost function, but with an entry of
// the outermost function.
func FuncForPC(pc uintptr) *Func {
	name, _, _, entry := pcInfo(pc, false)
	if entry == 0 {
		return nil
	}
	return funcOf(name, entry)
}

// Name returns the name of the function.
func (f *Func) Name() string {
	if f == nil {
		return ""
	}
	return f.name
}

// Entry returns the entry address of the function.
func (f *Func) Entry() uintptr {
	return f.entry
}

// FileLine returns the file name and line number of the
// source code corresponding to the program counter pc.
// The result will not be accurate if pc is not a program
// counter within f.
func (f *Func) FileLine(pc uintptr) (file string, line int) {
	_, file, line, _ = pcInfo(pc, false)
	return file, line
}

// memInfo is what the allocator knows: the bytes of live allocations, the bytes it got from the
// system and the free bytes it holds.
//
//gocpp:native gp::runtime::mem_info
func memInfo() (inUse, sys, idle uint64)

// totalAlloc is the highest number of live bytes that ReadMemStats has seen, including inUse: a
// TotalAlloc that does not decrease and is never below Alloc (nothing counts the allocations).
//
//gocpp:native gp::runtime::total_alloc
func totalAlloc(inUse uint64) uint64

// gcCount is the number of calls of GC: each one is a completed collection cycle for NumGC.
//
//gocpp:native gp::runtime::gc_count
func gcCount() uint32

// A Pinner is a set of Go objects each pinned to a fixed location in memory. The
// [Pinner.Pin] method pins one object, while [Pinner.Unpin] unpins all pinned
// objects. See their comments for more information.
//
// Objects never move in gocpp, so Pin only keeps the object alive until Unpin.
type Pinner struct {
	*pinner
}

type pinner struct {
	refs []any
}

// Pin pins a Go object, preventing it from being moved or freed by the garbage
// collector until the [Pinner.Unpin] method has been called.
//
// gocpp does not check that pointer is a pointer to an object allocated by Go.
func (p *Pinner) Pin(pointer any) {
	if p.pinner == nil {
		p.pinner = new(pinner)
	}
	p.refs = append(p.refs, pointer)
}

// Unpin unpins all pinned objects of the [Pinner].
func (p *Pinner) Unpin() {
	if p.pinner != nil {
		p.refs = nil
	}
}
