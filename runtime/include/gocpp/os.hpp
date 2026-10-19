// gocpp/os.hpp: the OS layer, the only way the runtime reaches the operating system.
//
// These are only declarations, in standard C++. Each operating system has its own
// implementation in runtime/src/os/<os>/ (Linux, and Windows: ADR-053), and only those
// files may include OS headers such as <unistd.h> or <windows.h>. Everything else in the
// runtime uses standard C++. The thread layer (threads with a stack size, thread-local storage and
// the stack-overflow hook; Phase 08, steps 8.15 and 8.1) is at the end of this file.
#pragma once

#include <cstddef>
#include <cstdint>
#include <functional>
#include <string>
#include <string_view>
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
// (the stack bounds, and on Linux the SIGSEGV handler on an alternate stack, on Windows the
// exception handlers; see "threads" and "faults" below). Windows has no SIGPIPE: there a write
// to a broken pipe is write_result::failed, as in Go on Windows.
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
// Linux they are argv; on Windows they come from the wide command line (UTF-16 converted to
// UTF-8, split with Go's rules for it; argv, in the ANSI code page, is not used).
std::vector<std::string> arguments(int argc, char** argv);

// process_arguments returns the arguments of the running process when nobody passed them to the
// program (a library that runs in a program that is not gocpp's: the arguments of the host
// program; Linux: /proc/self/cmdline), the program name first, or an empty list when the OS
// can't tell.
std::vector<std::string> process_arguments();

// environment returns the process environment as "KEY=value" strings, in the OS's order (on
// Windows, from the wide environment block, as UTF-8; like Go, it includes the entries that
// start with '=', the current folders of the drives: "=C:=C:\\dir").
std::vector<std::string> environment();

// --- the stack trace of a panic (gocpp/trace.hpp) ---

// stack writes the return addresses of the calling thread's stack into pcs, innermost first
// (the first one is in the function that called stack), at most max of them, and returns how
// many it wrote (0 when it can't). It runs at every panic, so it only collects addresses:
// read_exe and the DWARF line table turn them into names and lines later, and only for a
// panic that ends the program. On Linux it is glibc's backtrace(); on Windows
// RtlCaptureStackBackTrace. Not async-signal-safe: a panic is a C++ exception, not a signal.
int stack(uintptr_t* pcs, int max) noexcept;

// stack_until is stack for the part of the stack that a panic needs at once (panic_error): the
// frames from the caller up to the frame that holds the address bound (the address of a local
// variable of a function that is still running), which is the last one recorded. It records at
// most max frames. *found is false when the stack ended or max was reached before that frame (the
// frames are then what stack gives). A system that can't stop early (Windows on a CPU other than
// x86-64, today) gives the whole stack and false.
int stack_until(uintptr_t* pcs, int max, const void* bound, bool* found) noexcept;

// stack_above records the frames outside the frame that holds the address bound (the frame that
// stack_until stopped at) from the calling thread's stack, in the way of stack: its caller's
// return address first, at most max of them. It returns -1 when that frame is not on the calling
// thread's stack (and on a system whose stack_until never finds it).
int stack_above(uintptr_t* pcs, int max, const void* bound) noexcept;

// read_exe reads the running program's file into img, and reports whether it could. It reads
// the file at once (a few hundred KB for a small program), so it is for the end of a program
// only. On Linux the file is /proc/self/exe, an ELF file; on Windows it is the module's PE
// file (GetModuleFileNameW): its COFF symbol table and the DWARF sections that MinGW writes
// into it (a program linked without -g or with -s has no lines or names).
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
// keeps a pointer for it, or learns what its stack is. Linux has pthreads, Windows
// CreateThread (ADR-053); the portable runtime (gocpp/goroutine.hpp, src/goroutine.cpp) never sees
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

// exit_thread ends the calling thread the way C code can end it, with pthread_exit: glibc unwinds
// the thread's stack with a forced unwind, a C++ exception that no handler may swallow (the
// frames' destructors run, a catch (...) must throw it again). No Go code runs. The runtime
// never calls it; the tests do, to check that a function with defers lets the forced unwind through
// (step 5.10, test/defer_test.cpp). Linux only: Windows has no forced unwind, so there it ends the
// thread at once (ExitThread: no frame's destructor runs).
[[noreturn]] void exit_thread(); // (not noexcept: the forced unwind passes through it)

// tls_get and tls_set are the calling thread's slot: one pointer per thread, null before the
// first tls_set, which the portable runtime uses for the thread's goroutine record. They don't
// allocate and tls_get is async-signal-safe (the overflow hook may need it). A thread that
// ends doesn't free what its slot points to. (C++'s thread_local, in constinit form with a
// trivial type, is fine in portable code too; the slot is the part that a Windows thread layer
// also reads from its exception handler, where thread_local isn't safe to touch.)
void* tls_get() noexcept;
void tls_set(void* p) noexcept;

// tls_on_thread_exit makes the calling thread call fn(arg) when it ends (Linux: a destructor of a
// pthread key, which runs for a thread that returns from its function or calls pthread_exit, in
// any thread, also one that C created). Only the last call of a thread counts. The portable
// runtime uses it to free the goroutine record of a thread that no go statement started. fn runs
// on the ending thread, on Linux after the C++ thread-local destructors (on Windows it is a
// fiber-local storage callback, which Windows runs at the start of the thread's end, before the C++
// thread-local destructors that run at DLL_THREAD_DETACH: there no order is promised, and a
// destructor that uses the goroutine record gets a new one), and must not block. A thread that is
// killed with the process doesn't run it. tls_on_thread_exit_stop forgets all registrations and
// makes later calls do nothing: a shared library calls it before it is unloaded, because the
// address of fn is in it. May fail silently (no key available): the record is then never freed.
void tls_on_thread_exit(void (*fn)(void*), void* arg) noexcept;
void tls_on_thread_exit_stop() noexcept;

// library_ref takes one more reference to the shared library that holds the code at addr, and
// returns a handle for library_unref; null when addr is in the main program (nothing to unload) or
// the OS can't tell. While the reference is held, the host's dlclose of the library only gives
// back the host's own reference: the library stays mapped, its destructors do not run.
void* library_ref(const void* addr) noexcept;
void library_unref(void* handle) noexcept;

// stack_bounds returns the calling thread's stack as [lo, hi): the lowest address that a frame
// may use, and the end of the stack (the highest address plus one). It is known for the main
// thread (after start) and for threads that thread_start started, and returns false for any
// other thread (a thread of a C library, a std::thread). Then no hook runs on an overflow
// either (Windows: the same rule, kept in a flag of the thread, although the OS knows the stack of
// every thread). The end may be some KB above the first frame: the thread's own data (thread-local
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

// cpu_count returns the number of CPUs the process may run on (Linux: sched_getaffinity, so it
// respects taskset and cpusets, which std::thread::hardware_concurrency does not), at least 1. A
// goroutine that waits spins only when this is more than 1.
int cpu_count() noexcept;

// cpu_limit returns the CPU limit of the control group that the process runs in, as a number of
// CPUs (the quota divided by the period; 2.5 is two and a half CPUs), or 0 when there is none: no
// control group, no limit ("max"), or a file that can't be read or understood. It is what Go
// 1.25's runtime reads to lower its default GOMAXPROCS in a container (Linux: cgroup v2's
// cpu.max, or v1's cpu.cfs_quota_us and cpu.cfs_period_us, found through /proc/self/cgroup and
// /proc/self/mountinfo like Go does). Other systems: 0. It reads files, so it is for the start of
// a program, not a hot path.
double cpu_limit() noexcept;

// cgroup_cpu_limit is the part of cpu_limit that doesn't touch the machine, for tests: the
// contents of /proc/self/cgroup and /proc/self/mountinfo, and read_file, which gives the contents
// of the control group's limit files by path (false when it can't).
double cgroup_cpu_limit(
    std::string_view proc_cgroup, std::string_view mountinfo,
    const std::function<bool(const std::string& path, std::string& out)>& read_file) noexcept;

// mem_stats is what the allocator says about the heap, in bytes, for runtime.ReadMemStats:
// gp_in_use the bytes of the allocations that are live, gp_sys the bytes that the allocator got
// from the system, gp_idle the bytes that it holds free. All zero when the system can't tell.
struct mem_stats {
    uint64_t gp_in_use = 0;
    uint64_t gp_sys = 0;
    uint64_t gp_idle = 0;
};
mem_stats mem_info() noexcept;

// trim_memory gives the allocator's free memory back to the system when it can (Linux: glibc's
// malloc_trim), for runtime/debug.FreeOSMemory.
void trim_memory() noexcept;

// exit_now ends the process at once with the exit code, and does nothing else: no flush of C
// stdio, no leak check, no atexit functions. It is async-signal-safe: for the stack overflow
// hook. Everywhere else use exit.
[[noreturn]] void exit_now(int code) noexcept;

// no_core_dump makes the process leave no core file if it dies of a signal (Linux:
// prctl(PR_SET_DUMPABLE, 0), which also stops a core_pattern that pipes to a handler such as
// systemd-coredump; ulimit -c 0 does not). The runtime's tests that crash on purpose call it:
// on a loaded machine the handler holds the dying process for minutes. Windows: no error box and
// no Windows Error Reporting (SetErrorMode).
void no_core_dump() noexcept;

#if defined(_WIN32)
// --- faults (Windows) ---
//
// On Linux a fault that is not a stack overflow is a plain crash (SIGSEGV, exit code 139). On
// Windows the OS layer turns the exceptions of the CPU that nothing handles (an access violation
// from C code or from a build without checks, an integer division by zero, an illegal
// instruction) into Go's report, through a hook like the stack overflow's. A C++ exception, and
// anything that a frame-based handler of the program's own (a __try block) takes, is not touched:
// the OS layer only sees what reaches the unhandled-exception filter. So a __try/__except block
// still catches a CPU fault, but C's signal(SIGSEGV) handler does not (the hook ends the process
// first), and neither a __try block nor signal() catches a stack overflow (see
// set_stack_overflow_hook: that one is a vectored handler, which runs before frame-based ones).

// A fault_info describes one: gp_code is the exception code (0xc0000005, access violation),
// gp_access and gp_addr the first two words of the exception record (for an access violation: 0
// read, 1 write, 8 execute, and the address), gp_pc the instruction that faulted, and gp_pcs the
// return addresses of the faulting stack, innermost first, gp_pcs[0] being gp_pc, gp_npcs of them
// (found by unwinding the exception's context, so it also works for the code that crashed).
struct fault_info {
    uint32_t gp_code;
    uint64_t gp_access;
    uint64_t gp_addr;
    uintptr_t gp_pc;
    const uintptr_t* gp_pcs;
    int gp_npcs;
};

// set_fault_hook sets the function that gets a fault_info, on the stack of the faulting thread
// (it may allocate and print; it is not a signal handler), and returns the one before (null at
// first). It should not return: the runtime's hook prints Go's lines and ends the process with
// code 2. If it returns, or there is none, the fault goes to the filter that was there before
// (the C runtime's, then Windows' own: the crash, exit code 0xc0000005 and so on). Only the
// exceptions listed above come here.
using fault_hook = void (*)(const fault_info&) noexcept;
fault_hook set_fault_hook(fault_hook hook) noexcept;
#endif

} // namespace gp::os
