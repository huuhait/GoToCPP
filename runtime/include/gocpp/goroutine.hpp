// gocpp/goroutine.hpp: goroutines, as OS threads (ADR-007).
//
// gp::go(f) starts a detached thread that runs f. There is no scheduler: a goroutine is a
// thread, with a stack of its own (8 MiB by default), an id for the panic message, and a place
// in the count that runtime.NumGoroutine reports. Generated code (step 8.2) evaluates the
// function and the arguments of a `go` statement first, in the calling goroutine, as Go does,
// and passes a closure that holds the values.
//
//     gp::go([jobs, results, wg] { worker(jobs, results, wg); });
//
// An unrecovered panic in a goroutine ends the whole program, as in Go: "panic: <value>", a
// blank line, "goroutine N [running]:", the stack trace of the goroutine, and exit code 2. When
// several goroutines die at once, one of them reports and the others wait for the process to
// end (detail::claim_death). A C++ exception that nothing catches does the same through
// run_main's terminate handler, and a stack overflow prints Go's "fatal error: stack overflow"
// (exit code 2) for every goroutine's stack and for the main thread's, on Linux and not in a
// sanitizer build (src/os/linux/thread.cpp: the guard below the stack, the alternate signal
// stack, the hook that run_main sets).
//
// `go f(x, y)` (step 8.2) is gp::go_fn(f, x, y) for a function f of the program and
// gp::go_call(callable, args...) for anything else (a func literal, a method, a builtin): both
// store a copy of the function and of the arguments, which the calling goroutine has evaluated
// already, and run the call on the new goroutine (docs/spec/cpp-mapping.md, "Goroutines").
//
// A goroutine's thread is reused: when a goroutine ends, its thread waits a few seconds for the
// next `go` statement instead of exiting (detail::idle_limit; src/goroutine.cpp), so a loop that
// starts a million short goroutines creates a few threads, not a million. A thread that is idle
// is no goroutine: it is not counted, and it holds only its stack.
#pragma once

#include <gocpp/ptr.hpp>
#include <gocpp/waiter.hpp>

#include <cstddef>
#include <cstdint>
#include <memory>
#include <tuple>
#include <type_traits>
#include <utility>

namespace gp {

namespace detail {

// task is what a goroutine runs: the closure of a go statement, once.
struct task {
    virtual ~task() = default;
    virtual void run() = 0;
};

template <typename F>
struct task_of final : task {
    explicit task_of(F&& f) : fn(std::move(f)) {}
    void run() override { fn(); }
    F fn;
};

// goroutine is the runtime's record of one goroutine: its id, its waiter (what it parks on, so
// that a channel finds it), and its closure until it runs. The goroutine's thread has a pointer
// to it in its thread slot (os::tls_get). The id 1 is the main goroutine: the thread that first
// asks for its goroutine, which run_main does before anything else.
struct goroutine {
    uint64_t id = 0;
    bool counted = false; // is in num_goroutine()
    waiter park;
    // Why the goroutine is blocked ("chan receive", "chan send", "select", ...): what Go's
    // deadlock report and traceback print in the brackets, "goroutine 1 [chan receive]:". Null
    // while it runs. Written by the goroutine itself (block_scope), read by anyone: use reason()
    // and set_reason() (the compilers' atomic builtins, because <atomic> is not in the headers that
    // every generated file includes).
    const char* wait_reason = nullptr;
    const char* reason() const noexcept { return __atomic_load_n(&wait_reason, __ATOMIC_ACQUIRE); }
    void set_reason(const char* r) noexcept { __atomic_store_n(&wait_reason, r, __ATOMIC_RELEASE); }
    // Is the goroutine counted as blocked (num_blocked)? Set by block_enter; cleared, with the
    // count, by whoever comes first: the goroutine that was woken (block_exit) or the goroutine
    // that wakes it (unblock), so a goroutine that is being woken is not blocked any more.
    bool is_blocked = false;
    // Set by block_enter: this goroutine's block made every live goroutine blocked (read only by
    // the goroutine itself, in park_cell, to decide whether the deadlock detector must look).
    bool last_asleep = false;
    // How many runtime.LockOSThread calls are active (the goroutine always has a thread of its own,
    // so this only shows in Go's reports: "goroutine 1 [chan receive, locked to thread]:"). The
    // main goroutine is locked while the package initializers run (run_main). Written by the
    // goroutine itself (atomically: a report of all goroutines reads it while the goroutine runs).
    int32_t locked = 0;
    // Where it came from, for Go's "created by main.main in goroutine 1": the creator's id (0 for
    // the main goroutine and for a thread that no go statement started) and the address after
    // the call of the go statement, in the creator's code.
    uint64_t parent_id = 0;
    const void* create_pc = nullptr;
    std::unique_ptr<task> body;
    // end_hook(end_arg), when set (by the goroutine itself, before it ends), runs on the
    // goroutine's thread after the goroutine has stopped counting (num_goroutine,
    // running_goroutines), and after its record is freed, so it must not use the record:
    // iter.Pull's coroutine wakes the goroutine that is waiting for it here, so that
    // runtime.NumGoroutine is already back when next or stop returns, as in Go (coroexit).
    void (*end_hook)(void*) noexcept = nullptr;
    void* end_arg = nullptr;
    goroutine* next_adopted = nullptr; // the list of threads that no go statement started
    goroutine* prev_adopted = nullptr;
};

// The default stack of a goroutine is 8 MiB. Stack pages cost memory only once they are touched,
// so a small stack saves nothing on a 64-bit machine (the audit measured the same RSS for 10k
// threads at 64 KiB and at 8 MiB) and a small one overflows on a modest recursion. A program
// may set it down to 1 MiB (what is lower is raised to it): stack_size returns it,
// set_stack_size sets it for the goroutines that start after (0 is the default again). It is
// the main thread's too only through ulimit -s, which the program doesn't control.
inline constexpr std::size_t default_stack_size = std::size_t(8) << 20;
inline constexpr std::size_t min_stack_size = std::size_t(1) << 20;
std::size_t stack_size() noexcept;
void set_stack_size(std::size_t n) noexcept;

// this_goroutine returns the record of the goroutine that runs the calling thread. A thread
// that no go statement started (the main thread, a thread of a C library that calls into
// gocpp, a std::thread) gets a record the first time it asks, with the next id; the first
// thread to ask is the main goroutine (run_main asks first, on the main thread; a program that
// embeds the runtime and asks first from another thread gives the id 1 to that thread). Only the
// main thread's record counts in num_goroutine; the others don't. A record is freed when its
// thread ends (the OS layer's thread-exit hook, os::tls_on_thread_exit), so a library that is
// called from a thread per request holds no record for a thread that is gone.
goroutine& this_goroutine() noexcept;

// release_adopted frees the record p of an adopted thread (the thread-exit hook; also callable
// for tests). adopted_count is how many adopted records are alive.
void release_adopted(void* p) noexcept;
std::size_t adopted_count() noexcept;

// end_threads ends the idle goroutine threads and waits, at most timeout_ns, until every
// goroutine thread has gone (a thread that still runs a goroutine is not waited for longer), and
// forget_adopted frees the records of all adopted threads and stops the thread-exit hook. A
// shared library calls them when it is unloaded (gp::lib::shutdown), because the threads' code and
// the hook's address are in the library.
void end_threads(int64_t timeout_ns) noexcept;
void forget_adopted() noexcept;

// running_goroutines is how many goroutines that go statements started have not ended yet (a
// goroutine that is blocked counts; a thread that is idle between goroutines does not). A shared
// library must not be unloaded while it is above 0: set_go_hook installs fn, which every go
// statement calls first (gp::lib uses it to keep the library mapped; null: none).
int64_t running_goroutines() noexcept;
void set_go_hook(void (*fn)() noexcept) noexcept;

// current_goroutine_id returns the id of the calling thread's goroutine without creating a
// record: 0 for a thread that has none (yet). It only reads the thread's slot, so it is
// async-signal-safe (the stack overflow report uses it).
uint64_t current_goroutine_id() noexcept;

// goroutine_id returns this_goroutine().id, or 0 when there is no memory for a record (it
// doesn't throw: Go's fatal errors print it).
uint64_t goroutine_id() noexcept;

// go_start starts the goroutine that runs body, on a stack of stack bytes (0: the default; the
// OS has a minimum, and rounds up). The goroutine counts from now on, before its thread
// runs: gp::num_goroutine() already includes it when go_start returns. When the OS can't start
// a thread, it is Go's fatal error: "runtime: failed to create new OS thread (have N already;
// errno=E)", "fatal error: newosproc", exit code 2.
// creator is the address in the code of the go statement (the return address of gp::go), for the
// "created by" line of the deadlock report; null: unknown.
void go_start(std::unique_ptr<task> body, std::size_t stack, const void* creator = nullptr);

// The creator that go_start gets for a goroutine that the runtime starts itself, as Go names it in
// "created by time.goFunc" (the function of an AfterFunc timer): the address of this variable
// instead of an address in code.
extern const char creator_gofunc;
// ... and for the coroutine of iter.Pull: "created by iter.Pull[...] in goroutine N".
extern const char creator_pull;

// idle_limit is how many idle threads the runtime keeps for the next `go` statement (0: none,
// every goroutine gets a thread of its own and the thread ends with it). The default is 256.
// An idle thread gives up after a few seconds. set_idle_limit returns the old limit; the idle
// threads above the new limit end at once. stop_idle_threads ends all idle threads, and says how
// many were idle: tests call it before they check that no thread is left (threads_ended), and
// the threads may need a moment to end.
inline constexpr std::size_t default_idle_limit = 256;
std::size_t set_idle_limit(std::size_t n) noexcept;
std::size_t stop_idle_threads() noexcept;
std::size_t idle_threads() noexcept; // how many are idle now

// The goroutines that are blocked in a gocpp operation (a channel, select, and later sync and
// time.Sleep), for the deadlock detector (step 8.11): when every goroutine is blocked and
// nothing can wake one, Go prints "fatal error: all goroutines are asleep - deadlock!". An
// operation that parks the goroutine for a wake-up from another goroutine wraps the park in a
// block_scope; num_blocked() is how many are inside one now. It is not the whole answer (a
// timer, a goroutine in a C call or one that has not started yet can wake one); that is 8.11's.
void block_enter(const char* reason) noexcept;
void block_exit() noexcept;
// unblock is called by the one who wakes a blocked goroutine, before it wakes it: the goroutine
// stops counting as blocked at once (exactly once: block_exit then finds it done), so a deadlock
// detector that reads num_blocked() == num_goroutine() never sees a goroutine that has been handed
// its value as asleep. A block_scope must therefore be entered *before* the goroutine's entry is
// visible to wakers (a channel enters it under its lock, before it queues the entry).
void unblock(goroutine& g) noexcept;
struct block_scope {
    explicit block_scope(const char* reason) noexcept { block_enter(reason); }
    block_scope(const block_scope&) = delete;
    block_scope& operator=(const block_scope&) = delete;
    ~block_scope() { block_exit(); }
};

// --- Deadlock detection (step 8.11, ADR-077) ---
//
// Go's checkdead: when every goroutine is asleep and nothing can wake one, the program ends with
//
//     fatal error: all goroutines are asleep - deadlock!
//
//     goroutine 1 [chan receive]:
//     main.main()
//             /path/main.go:9 +0x71
//
//     goroutine 7 [select (no cases)]:
//     ...
//     created by main.main in goroutine 1
//             /path/main.go:8 +0x65
//
// and exit code 2. Every goroutine is listed, in the order of its id, with its wait reason and its
// stack. "Asleep" is a goroutine inside a block_scope (a channel, select, sync wait); a goroutine
// in time.Sleep, in a system call (file or network IO, a wait for a child) or running is not, so
// the program does not end while any of those can still wake another. Go 1.23's timer rule: an
// armed AfterFunc timer, or a channel timer that a goroutine is blocked on, may still wake one, and
// a Timer or Ticker nobody waits on does not count (gp::detail::timers_can_wake). A thread that no
// go statement started and that entered gocpp (a callback from a C library's thread; the thread of
// a std::thread that used a channel), or a callback that was handed to C as a function pointer,
// turns the detector off for good, as Go's cgo does: such a thread can wake anything, at any time.
// A shared library (gp::lib) never reports a deadlock: the host program runs.
//
// The cost is two atomic loads where a goroutine is about to sleep or has ended (the counts
// existed already), and a store where a worker thread starts and ends a goroutine (the registry:
// src/sched.hpp). run_main turns the detector on; a unit test turns it on and sets a hook to see
// the report instead of the end of the process.
void enable_deadlock_detector(bool on) noexcept;
bool deadlock_detector_enabled() noexcept;

// deadlock_check is called by a goroutine that is about to sleep and by one that has ended:
// when the number of blocked goroutines is the number of live ones, it calls deadlock_look
// (src/deadlock.cpp), which decides and reports.
void deadlock_check() noexcept;
void deadlock_look() noexcept;

// The report goes to this hook instead of ending the process (null: the real thing, to stderr and
// _exit(2)); the detector is idle again when the hook returns. The text is the whole report. For
// tests. set_deadlock_hook returns the old hook.
using deadlock_hook = void (*)(const char* text, std::size_t n) noexcept;
deadlock_hook set_deadlock_hook(deadlock_hook h) noexcept;

// note_foreign_entry says that a thread of C or C++ code may call into gocpp from now on (a
// callback became a C function pointer): the detector is off for good. foreign_entered says whether
// it is (also after a thread that no go statement started asked for its goroutine).
void note_foreign_entry() noexcept;
bool foreign_entered() noexcept;

// adopt_internal says that the calling thread is the runtime's own (the timer thread, which starts
// the goroutines of AfterFunc): the record that it gets when it first asks for one does not make it
// a foreign thread.
void adopt_internal() noexcept;

// main_goexit is the end of the main goroutine's runtime.Goexit (run_main calls it after the
// deferred calls ran): the main goroutine is not counted any more, the program goes on while other
// goroutines run, and ends with Go's fatal error when none is left ("no goroutines (main called
// runtime.Goexit) - deadlock!") or all of them are asleep. The thread waits for ever.
[[noreturn]] void main_goexit() noexcept;

// go_of_nil is the fatal error of `go f()` with a nil func value ("go of nil func value"): it is
// a fatal error at the go statement, in the calling goroutine, not a panic in the new one.
[[noreturn]] void go_of_nil() noexcept;

// block_forever parks the goroutine for ever, as a receive from or a send to a nil channel
// does ("chan receive (nil chan)"): it is a blocked goroutine that nothing wakes.
[[noreturn]] void block_forever(const char* reason) noexcept;

// The panics that are being handled (step 8.2): a panic counts from the moment a deferred-call
// frame takes it (a panic_error's gp_token, gocpp/panic.hpp) until the panic object is gone:
// recovered, swallowed by a catch outside the defer machinery (fmt's %!v(PANIC=...), a callback's
// barrier) or the program dies of it. Nothing needs to be called where a panic is caught: the count
// follows the object. When main returns while a goroutine runs the deferred calls of a panic
// (`defer close(done)` wakes main, and the goroutine goes on to die), Go lets the panic finish:
// settle_panics, which run_main calls before it ends the program with code 0, waits for such
// goroutines (at most 200 ms), and if a goroutine has claimed the death of the program, it never
// returns. Only the return of main waits: os.Exit (gp::exit below) ends the program at once, in Go
// too.
int64_t panicking() noexcept;
void settle_panics() noexcept;

// set_settle_timeout sets the wait of settle_panics (default 200 ms) and returns the old value in
// nanoseconds; settle_waits() counts the times a settle_panics found a panic to wait for. For
// tests.
int64_t set_settle_timeout(int64_t nanoseconds) noexcept;
int64_t settle_waits() noexcept;

// set_idle_timeout sets how long an idle goroutine thread waits for the next `go` statement
// (default 5 seconds) and returns the old value in nanoseconds; for tests of the race between the
// timeout and a `go` statement that takes the thread at that moment.
int64_t set_idle_timeout(int64_t nanoseconds) noexcept;

// call_task is the closure of `go f(a, b)`: the function (or callable) and the arguments that
// the calling goroutine evaluated, and the call on the new goroutine. The arguments are
// rvalues when the call runs, as the arguments of a deferred call are.
template <typename F, typename... A>
struct call_task final : task {
    template <typename G, typename... B>
    explicit call_task(G&& f, B&&... a) : fn(std::forward<G>(f)), args(std::forward<B>(a)...) {}
    void run() override { std::apply(std::move(fn), std::move(args)); }
    F fn;
    std::tuple<A...> args;
};

// claim_death is called by the code that ends the program with a report (an unrecovered panic,
// a fatal error, a stack overflow) before it prints anything. The first thread to call it goes
// on; every other thread blocks until the process ends, so that two goroutines that die at the
// same time don't print two interleaved reports and don't end the program with different
// codes. A thread that already claimed it (a fatal error while the report of a panic is made)
// goes on too. It is async-signal-safe. A thread that waits gives up after 30 seconds and ends
// the program with code 2 (os::exit_now), so a reporter that is stuck (it needs malloc's lock,
// which the waiting thread's own stack overflow interrupted) can't hang the program for ever;
// Go would not hang. Output that C code printf'd and has not flushed is not written by a stack
// overflow's report (it cannot flush in a signal handler), unlike a panic's.
void claim_death() noexcept;

} // namespace detail

// exit ends the program with the exit code, now: the one exit path of os.Exit (Phase 09), of a
// fatal error and of the return of main (gp::run_main). It flushes what C and C++ code has
// printf'd (gocpp's own output is never buffered), and ends the process with _exit, so no C++
// static destructor, atexit function or thread-local destructor runs, and no other goroutine can
// see an object that is destroyed: a goroutine that still runs, one that is blocked in a system
// call and one in a loop that never ends all stop with the process, and nothing waits for them. In
// a build with AddressSanitizer the leak check runs first, as it would at the end of the process.
// When another goroutine is ending the program already (a panic's report), exit waits for it
// (claim_death), so the report is complete and the code is the report's. Never returns.
[[noreturn]] void exit(int code) noexcept;

// num_goroutine is runtime.NumGoroutine: the number of goroutines that are alive: the main
// goroutine, and each goroutine from the go statement until its function has returned and its
// closure is destroyed.
int64_t num_goroutine() noexcept;

// num_blocked is the number of goroutines that wait inside a block_scope (see above).
int64_t num_blocked() noexcept;

// go starts f as a new goroutine, on a thread of its own with the default stack. (go, go_fn and
// go_call are not inlined: each passes its own return address to go_start, which is in the function
// with the go statement, for the "created by" line of a trace.)
template <typename F>
    requires std::is_invocable_v<std::decay_t<F>&>
[[gnu::noinline]] void go(F&& f) {
    detail::go_start(std::make_unique<detail::task_of<std::decay_t<F>>>(std::forward<F>(f)), 0,
                     __builtin_return_address(0));
}

// go_fn(f, a...) is `go f(a...)` for a function f of the program: the arguments are converted to
// f's parameter types here, in the calling goroutine (gp::defer_frame::gp_push is the same for
// defer), and the new goroutine calls f with them.
template <typename R, typename... P>
[[gnu::noinline]] void go_fn(R (*f)(P...), std::type_identity_t<detail::owning_t<P>>... a) {
    detail::go_start(
        std::make_unique<detail::call_task<R (*)(P...), std::decay_t<detail::owning_t<P>>...>>(
            f, static_cast<detail::owning_t<P>&&>(a)...),
        0, __builtin_return_address(0));
}

// go_call(c, a...) is `go c(a...)` for any other callable: a func literal, a func value, a
// method (the receiver is the first argument of a lambda), a builtin or a library function in a
// generic lambda. c and copies of the arguments belong to the goroutine; c gets them as rvalues.
template <typename F, typename... A>
[[gnu::noinline]] void go_call(F c, A&&... a) {
    if constexpr (requires { static_cast<bool>(c == nullptr); }) {
        if (c == nullptr) {
            detail::go_of_nil();
        }
    }
    detail::go_start(std::make_unique<detail::call_task<F, std::decay_t<A>...>>(
                         std::move(c), std::forward<A>(a)...),
                     0, __builtin_return_address(0));
}

} // namespace gp
