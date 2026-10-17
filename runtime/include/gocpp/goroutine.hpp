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
// This header is not in rt.hpp yet: step 8.2 adds it when the emitter uses it.
#pragma once

#include <gocpp/waiter.hpp>

#include <cstddef>
#include <cstdint>
#include <memory>
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
    std::unique_ptr<task> body;
    goroutine* next_adopted = nullptr; // the list of threads that no go statement started
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
// main thread's record counts in num_goroutine; the others stay (they are never freed: no
// destructor of the C++ library can tell that such a thread ended).
goroutine& this_goroutine() noexcept;

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
void go_start(std::unique_ptr<task> body, std::size_t stack);

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

// num_goroutine is runtime.NumGoroutine: the number of goroutines that are alive: the main
// goroutine, and each goroutine from the go statement until its function has returned and its
// closure is destroyed.
int64_t num_goroutine() noexcept;

// go starts f as a new goroutine, on a thread of its own with the default stack.
template <typename F>
    requires std::is_invocable_v<std::decay_t<F>&>
void go(F&& f) {
    detail::go_start(std::make_unique<detail::task_of<std::decay_t<F>>>(std::forward<F>(f)), 0);
}

} // namespace gp
