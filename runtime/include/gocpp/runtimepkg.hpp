// gocpp/runtimepkg.hpp: the native functions of the standard packages runtime and runtime/debug
// (Phase 08, step 8.10; ADR-038).
//
// Go's runtime package is written in .gp (stdlib/runtime), and its functions that need the machine
// are these, named by //gocpp:native lines (ADR-027):
//
//     //gocpp:native gp::runtime::gosched
//     func Gosched()                        // the call is ::gp::runtime::gosched()
//
// Types are the C++ types of the Go parameters: int is int64_t, uintptr is uint64_t, a func value
// is gp::func, and several results are a std::tuple. What the package does with goroutines is in
// gocpp/goroutine.hpp (gp::num_goroutine, gp::goexit in gocpp/panic.hpp).
//
// A goroutine is an OS thread (ADR-007), which makes some of the package trivial and some of it a
// stand-in; docs/spec/cpp-mapping.md ("runtime") says which, and so does each function below:
//
//   - Gosched yields the CPU (sched_yield). It is a hint, not Go's "run another goroutine first":
//     the OS decides, so a program must not depend on the order that Go's scheduler gives it.
//   - GOMAXPROCS remembers the value and nothing uses it; its default is Go 1.25's: the CPUs the
//     process may run on (sched_getaffinity), lowered by the cgroup's CPU limit (rounded up, at
//     least 2), and the GOMAXPROCS environment variable overrides it.
//   - LockOSThread and UnlockOSThread do nothing: every goroutine has a thread of its own.
//   - GC does nothing, SetFinalizer never runs the finalizer, KeepAlive does nothing: memory is
//     reference counted (ADR-004) and is freed when the last reference goes (a cycle is never
//     freed).
//   - ReadMemStats reports what the allocator knows (the live bytes, the bytes from the system);
//     the counts of allocations and collections are 0, because nothing counts them.
//   - Caller, Callers, FuncForPC and Stack read the program's own debug information (the symbol
//     table and the DWARF line table, as a panic's trace does: gocpp/trace.hpp): names are Go's
//     (main.f, main.(*T).M, main.main.func1), files and lines are those of the .gp source.
//
// Like every runtime type, nothing here has plain-named members: these are free functions.
#pragma once

#include <gocpp/func.hpp>
#include <gocpp/iface.hpp>
#include <gocpp/slice.hpp>
#include <gocpp/string.hpp>

#include <cstdint>
#include <tuple>

namespace gp::runtime {

// --- goroutines and processors ---

// gosched is runtime.Gosched: it yields the processor.
void gosched() noexcept;

// num_goroutine is runtime.NumGoroutine (gp::num_goroutine).
int64_t num_goroutine() noexcept;

// gomaxprocs is runtime.GOMAXPROCS(n): it returns the previous setting, and sets it to n when n >=
// 1 (a smaller n only asks). The first call (or the first call of any function here that needs it)
// finds the default: the GOMAXPROCS environment variable if it is a positive number, else
// num_cpu() lowered by the cgroup's CPU limit. Nothing in gocpp uses the value for scheduling.
int64_t gomaxprocs(int64_t n) noexcept;

// set_default_gomaxprocs is runtime.SetDefaultGOMAXPROCS: the default again, ignoring the
// environment variable.
void set_default_gomaxprocs() noexcept;

// default_gomaxprocs is the default that gomaxprocs starts with, ignoring the environment
// variable, and cpu_limit and ncpu are what it is made of: the number of CPUs of a cgroup limit
// of limit CPUs (0: none) on a machine of ncpu CPUs. They are here for tests.
int64_t default_gomaxprocs() noexcept;
int64_t adjust_for_cgroup(int64_t ncpu, double limit) noexcept;

// num_cpu is runtime.NumCPU: the CPUs the process may run on, read once.
int64_t num_cpu() noexcept;

// num_cgo_call is runtime.NumCgoCall: 0, calls of C and C++ are not counted.
int64_t num_cgo_call() noexcept;

// lock_os_thread and unlock_os_thread are runtime.LockOSThread and UnlockOSThread: nothing, except
// that the goroutine remembers the count, which Go's deadlock report shows ("locked to thread").
void lock_os_thread() noexcept;
void unlock_os_thread() noexcept;

// --- memory ---

// gc is runtime.GC: nothing (memory is reference counted).
void gc() noexcept;

// set_finalizer is runtime.SetFinalizer and keep_alive is runtime.KeepAlive: nothing. A finalizer
// never runs.
void set_finalizer(const gp::any& obj, const gp::any& finalizer) noexcept;
void keep_alive(const gp::any& x) noexcept;

// mem_info gives the numbers that ReadMemStats can fill in: the bytes of live allocations, the
// bytes that the allocator got from the system, and the free bytes it holds (os::mem_info).
std::tuple<uint64_t, uint64_t, uint64_t> mem_info() noexcept;

// set_mutex_profile_fraction is runtime.SetMutexProfileFraction: it keeps the rate and returns
// the previous one (profiles are not made).
int64_t set_mutex_profile_fraction(int64_t rate) noexcept;

// --- what the program is ---

// version is runtime.Version: the Go release whose standard library gocpp follows ("go1.25.14").
gp::string version();

// goroot is runtime.GOROOT: the GOROOT environment variable, or "".
gp::string goroot();

// --- the stack ---

// caller is runtime.Caller(skip): the return address (the pc), the file and the line of the call,
// in the Go function skip levels above the one that called it (0 is that function itself); ok is
// false when the stack is shorter, or the program's debug information can't be read. Frames of the
// runtime's own code are not counted.
std::tuple<uint64_t, gp::string, int64_t, bool> caller(int64_t skip);

// callers is runtime.Callers(skip, pcs): it fills pcs with return addresses, innermost first,
// where skip 0 is Callers itself (the pc of a pseudo frame named runtime.Callers) and 1 the caller
// of Callers, and returns how many it wrote.
int64_t callers(int64_t skip, gp::slice<uint64_t> pcs);

// pc_infos describes the addresses pcs, for FuncForPC, Func.FileLine and CallersFrames: names[i],
// files[i], lines[i] and entries[i] are filled for each i that all four slices have; an address
// that is in no function of the program gets entry 0 (and "" and 0 for the rest). With call
// set the addresses are return addresses and the call before each is described.
void pc_infos(gp::slice<uint64_t> pcs, bool call, gp::slice<gp::string> names,
              gp::slice<gp::string> files, gp::slice<int64_t> lines, gp::slice<uint64_t> entries);

// funcs_lock and funcs_unlock guard the table of Func values in stdlib/runtime (FuncForPC returns
// the same *Func for a function every time, as Go does).
void funcs_lock() noexcept;
void funcs_unlock() noexcept;

// stack is runtime.Stack(buf, all): it writes "goroutine N [running]:" and the calling
// goroutine's trace in Go's format into buf (cut when buf is too short) and returns the number of
// bytes. all is not honoured: the other goroutines' stacks are not known (they are threads),
// so only the calling goroutine's trace is written.
int64_t stack(gp::slice<uint8_t> buf, bool all);

// --- the values of the panics of the runtime ---

// set_panic_values is called by the package runtime's init (stdlib/runtime): nil_error is the
// *runtime.PanicNilError that panic(nil) panics with, and assertion makes the
// *runtime.TypeAssertionError of a failed type assertion from its text. With them, a program
// that imports runtime can name the types of the panics that the runtime makes
// (r.(*runtime.PanicNilError)): the descriptors are the package's, not the ones the runtime
// has for programs that don't import it (src/panicval.cpp).
void set_panic_values(const gp::any& nil_error, gp::func<gp::any(gp::string)> assertion);

// --- runtime/debug ---

// set_gc_percent is debug.SetGCPercent: it keeps the value (the default is 100, or GOGC) and
// returns the previous one; no collector uses it.
int64_t set_gc_percent(int64_t percent) noexcept;

// set_memory_limit is debug.SetMemoryLimit: a negative limit only asks. The default is
// math.MaxInt64, or GOMEMLIMIT. Nothing enforces it.
int64_t set_memory_limit(int64_t limit) noexcept;

// set_max_stack and set_max_threads are debug.SetMaxStack and SetMaxThreads: they keep the value
// and return the previous one (1000000000 and 10000 at first). The stack overflow check uses the
// size of the stack of the thread, not this, and the thread limit is the system's.
int64_t set_max_stack(int64_t bytes) noexcept;
int64_t set_max_threads(int64_t threads) noexcept;

// set_panic_on_fault is debug.SetPanicOnFault: it keeps the flag. (A fault is always a fatal
// signal here; the panic of a nil dereference is made by a check.)
bool set_panic_on_fault(bool enabled) noexcept;

// set_traceback is debug.SetTraceback: nothing.
void set_traceback(const gp::string& level) noexcept;

// free_os_memory is debug.FreeOSMemory: it gives the allocator's free memory back to the system.
void free_os_memory() noexcept;

// runtime_rand is runtime.rand (math/rand and math/rand/v2 call it for their global generator): a
// uniformly random uint64 from a generator of the thread, seeded from the system
// (std::random_device) the first time the thread asks. It is not Go's ChaCha8 stream; it needs no
// lock.
uint64_t runtime_rand() noexcept;

// stack_room returns how many bytes of the calling thread's stack can still be used before a
// reserve (a sixteenth of the stack, between 128 KiB and 512 KiB) that is kept for what runs after
// the check: negative when a recursive function must stop. encoding/json calls it at every level of
// its recursion (a stack is fixed here, D24). A thread whose stack the runtime does not know (a
// thread of a C library) has all the room there is: INT64_MAX.
// stack_floor is the lowest address that the code of this thread may use before the reserve (0
// until stack_room_slow has worked it out).
extern thread_local uintptr_t stack_floor;
int64_t stack_room_slow() noexcept;
inline int64_t stack_room() noexcept {
    uintptr_t floor = stack_floor;
    if (floor == 0) {
        return stack_room_slow();
    }
    return static_cast<int64_t>(reinterpret_cast<uintptr_t>(__builtin_frame_address(0))) -
           static_cast<int64_t>(floor);
}

// splitmix64 advances the state and returns the next value of the generator that runtime_rand uses
// (Vigna's SplitMix64). It is here so that a test can check the mixing against known answers.
uint64_t splitmix64(uint64_t& state) noexcept;

// write_stderr writes the bytes to standard error with one write (debug.PrintStack).
void write_stderr(gp::slice<uint8_t> bytes) noexcept;

} // namespace gp::runtime
