// gocpp/os.hpp: the OS layer, the only way the runtime reaches the operating system.
//
// These are only declarations, in standard C++. Each operating system has its own
// implementation in runtime/src/os/<os>/ (Linux now, Windows in Phase 14), and only those
// files may include OS headers such as <unistd.h> or <windows.h>. Everything else in the
// runtime uses standard C++. The thread layer (threads with a stack size, thread-local storage and
// the stack-overflow hook; Phase 08, steps 8.15 and 8.1) is at the end of this file.
#pragma once

#include <cstddef>
#include <cstdint>
#include <string>
#include <typeinfo>
#include <vector>

namespace gp::detail {
struct exe_image; // gocpp/trace.hpp
} // namespace gp::detail

namespace gp::os {

// The file descriptors of standard output and standard error.
inline constexpr int stdout_fd = 1;
inline constexpr int stderr_fd = 2;

// start prepares the process; run_main calls it first, on the main thread. Like Go's runtime,
// it makes a SIGPIPE harmless (a handler that does nothing, not "ignore", so programs that the
// process starts get the default back): a write to a broken pipe then fails with an error, and
// the caller decides (see write_result). It also starts the thread layer for the main thread
// (the stack bounds, and on Linux the SIGSEGV handler on an alternate stack; see "threads"
// below). Windows (Phase 14) doesn't have it yet.
void start() noexcept;

// write_result is what write reports.
enum class write_result {
    ok,
    // The reader of the pipe (or socket) is gone: EPIPE. Like Go, a Go-level write to stdout or
    // stderr (os.Stdout, so fmt.Print) then calls die_of_sigpipe, while the builtin print and
    // println ignore it, and writes to other files get an error.
    broken_pipe,
    // Any other error.
    failed,
};

// write writes all n bytes of data to the file descriptor fd, with as many system calls as it
// takes (one, unless the OS takes only part of it). It retries when a signal interrupts it, and
// waits when fd is non-blocking and full. On an error, the bytes written until then stay
// written.
write_result write(int fd, const char* data, std::size_t n) noexcept;

// die_of_sigpipe ends the program with SIGPIPE, as Go does after a write to a broken pipe on
// stdout or stderr: with the default action and the signal unblocked, whatever the parent set
// (a shell's `trap '' PIPE` leaves it ignored).
[[noreturn]] void die_of_sigpipe() noexcept;

// exit flushes the C stdio streams (C or C++ code that the program calls may have used
// printf) and ends the process with the exit code, at once: it doesn't run C++ static
// destructors or atexit functions, like Go's os.Exit, so threads that still run can't see
// destroyed objects. In a build with AddressSanitizer, it runs LeakSanitizer's check first,
// which the end of the process would run otherwise.
[[noreturn]] void exit(int code) noexcept;

// arguments returns the program's arguments, the program name first, as UTF-8 strings. On
// Linux they are argv; on Windows (Phase 14) they come from the wide command line.
std::vector<std::string> arguments(int argc, char** argv);

// environment returns the process environment as "KEY=value" strings, in the OS's order.
std::vector<std::string> environment();

// --- the stack trace of a panic (gocpp/trace.hpp) ---

// stack writes the return addresses of the calling thread's stack into pcs, innermost first
// (the first one is in the function that called stack), at most max of them, and returns how
// many it wrote (0 when it can't). It runs at every panic, so it only collects addresses:
// read_exe and the DWARF line table turn them into names and lines later, and only for a
// panic that ends the program. On Linux it is glibc's backtrace(); Phase 14 brings Windows
// (CaptureStackBackTrace). Not async-signal-safe: a panic is a C++ exception, not a signal.
int stack(uintptr_t* pcs, int max) noexcept;

// read_exe reads the running program's file into img, and reports whether it could. It reads
// the file at once (a few hundred KB for a small program), so it is for the end of a program
// only. On Linux the file is /proc/self/exe, an ELF file; Windows (PE, Phase 14) has no
// version yet.
bool read_exe(gp::detail::exe_image& img);

// demangle returns the C++ name of a mangled symbol name ("gpkg_main::f(long)"), or "" when
// it isn't one. Itanium C++ ABI names (GCC and Clang, also on MinGW); clang-cl's MSVC names
// come later.
std::string demangle(const std::string& name);

// current_exception_type returns the type of the exception that the calling thread is
// handling (in a catch block, or in std::terminate's handler), from the C++ ABI
// (abi::__cxa_current_exception_type), or null when there is none or the C++ library can't
// tell. It works without C++'s type information (-fno-rtti), for an exception of any type,
// as it was thrown (std::out_of_range, int).
const std::type_info* current_exception_type() noexcept;

// --- threads (Phase 08, steps 8.15 and 8.1) ---
//
// Every goroutine is an OS thread (ADR-007), and this is the only way the runtime starts one,
// keeps a pointer for it, or learns what its stack is. Linux has pthreads, Windows (Phase 14)
// CreateThread; the portable runtime (gocpp/goroutine.hpp, src/goroutine.cpp) never sees
// either. Blocking is not here: gp::waiter is a mutex and a condition variable of the C++
// library.

// thread_start starts a detached thread that calls fn(arg) on a stack of at least stack_size
// bytes (the OS may give more: it rounds up to its page size and has a minimum), and returns 0,
// or the OS's error code (EAGAIN when there are no more threads, ENOMEM; Windows: GetLastError)
// and then starts nothing. The thread has no handle: it ends when fn returns, and what fn
// needs to tell another thread it tells itself (a gp::waiter). On Linux the stack has a guard
// (a protected area of 256 KiB below it), so a frame that is probed page by page (every frame
// of the compiles of gocpp: -fstack-clash-protection) can't skip over it into another
// mapping; a frame of more than 256 KiB that is not probed can, and its first write may land
// in the stack of another thread without a fault. The thread also has an alternate signal stack for
// the overflow hook below, which is part of the thread, not of the stack size asked for. fn must
// not let an exception go: nothing above it catches one (the C++ library then calls std::terminate
// before it unwinds, which run_main's handler turns into Go's report).
int thread_start(void (*fn)(void*), void* arg, std::size_t stack_size) noexcept;

// tls_get and tls_set are the calling thread's slot: one pointer per thread, null before the
// first tls_set, which the portable runtime uses for the thread's goroutine record. They don't
// allocate and tls_get is async-signal-safe (the overflow hook may need it). A thread that
// ends doesn't free what its slot points to. (C++'s thread_local, in constinit form with a
// trivial type, is fine in portable code too; the slot is the part that a Windows thread layer
// also reads from its exception handler, where thread_local isn't safe to touch.)
void* tls_get() noexcept;
void tls_set(void* p) noexcept;

// stack_bounds returns the calling thread's stack as [lo, hi): the lowest address that a frame
// may use, and the end of the stack (the highest address plus one). It is known for the main
// thread (after start) and for threads that thread_start started, and returns false for any
// other thread (a thread of a C library, a std::thread). Then no hook runs on an overflow
// either. The end may be some KB above the first frame: the thread's own data (thread-local
// variables, the alternate stack) takes the top.
bool stack_bounds(uintptr_t& lo, uintptr_t& hi) noexcept;

// stack_fault describes a stack overflow, for the hook: gp_sp is the stack pointer of the code
// that faulted (the address of the fault when the CPU's registers are unknown), and gp_lo and
// gp_hi are the thread's stack, as stack_bounds returns it.
struct stack_fault {
    uintptr_t gp_sp;
    uintptr_t gp_lo;
    uintptr_t gp_hi;
};

// set_stack_overflow_hook sets the function that the OS layer calls when a thread that has
// stack bounds runs past the end of its stack (a fault in the guard area, or near the end of
// the main thread's stack), and returns the one before (null at first). It may be called at any
// time. The hook runs inside a signal handler on the thread's alternate stack (Linux; on Windows,
// in a vectored exception handler): it must only use async-signal-safe functions (write, exit_now,
// atomics, its own arguments), no allocation and no lock. It should not return: the runtime's
// hook prints Go's lines and calls exit_now(2). If it returns, or when there is no hook, the
// fault goes to the action that was there before, a crash. Faults that are not stack
// overflows always do. Not in a build with a sanitizer (AddressSanitizer reports a stack overflow
// itself, as "stack-overflow"; ThreadSanitizer doesn't, the program just crashes), nor with `ulimit
// -s unlimited` for the main thread.
using stack_overflow_hook = void (*)(const stack_fault&) noexcept;
stack_overflow_hook set_stack_overflow_hook(stack_overflow_hook hook) noexcept;

// exit_now ends the process at once with the exit code, and does nothing else: no flush of C
// stdio, no leak check, no atexit functions. It is async-signal-safe: for the stack overflow
// hook. Everywhere else use exit.
[[noreturn]] void exit_now(int code) noexcept;

// no_core_dump makes the process leave no core file if it dies of a signal (Linux:
// prctl(PR_SET_DUMPABLE, 0), which also stops a core_pattern that pipes to a handler such as
// systemd-coredump; ulimit -c 0 does not). The runtime's tests that crash on purpose call it:
// on a loaded machine the handler holds the dying process for minutes. Other systems: nothing.
void no_core_dump() noexcept;

} // namespace gp::os
