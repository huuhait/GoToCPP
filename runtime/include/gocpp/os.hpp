// gocpp/os.hpp: the OS layer, the only way the runtime reaches the operating system.
//
// These are only declarations, in standard C++. Each operating system has its own
// implementation in runtime/src/os/<os>/ (Linux now, Windows in Phase 14), and only those
// files may include OS headers such as <unistd.h> or <windows.h>. Everything else in the
// runtime uses standard C++. Phase 08 adds the thread layer to this namespace.
#pragma once

#include <cstddef>
#include <cstdint>
#include <string>
#include <vector>

namespace gp::detail {
struct exe_image; // gocpp/trace.hpp
} // namespace gp::detail

namespace gp::os {

// The file descriptors of standard output and standard error.
inline constexpr int stdout_fd = 1;
inline constexpr int stderr_fd = 2;

// start prepares the process; run_main calls it first. Like Go's runtime, it makes a SIGPIPE
// harmless (a handler that does nothing, not "ignore", so programs that the process starts get
// the default back): a write to a broken pipe then fails with an error, and the caller decides
// (see write_result).
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

} // namespace gp::os
