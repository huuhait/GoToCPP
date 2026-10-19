// Shared by the files of the Windows OS layer (os.cpp, thread.cpp, fault.cpp, trace.cpp); not a
// header of the runtime.
#pragma once

#include <gocpp/os.hpp>

// The Windows layer is x86-64 only: the fault code reads the registers of the x64 CONTEXT (Rip,
// Rsp) and unwinds with its tables, and the stack guarantee is sized for it. 32-bit x86 and ARM64
// Windows are not targets (ADR-021, ADR-053).
#if defined(_WIN32) && !defined(__x86_64__) && !defined(_M_X64)
#error "the gocpp Windows OS layer supports x86-64 only"
#endif

// The Windows version that the OS layer needs: Windows 10 (GetCurrentThreadStackLimits needs
// Windows 8, SetThreadStackGuarantee Vista). The build passes -D_WIN32_WINNT=0x0A00 too.
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#ifndef NOMINMAX
#define NOMINMAX
#endif
#ifndef _WIN32_WINNT
#define _WIN32_WINNT 0x0A00
#endif

#include <cstddef>
#include <string>
#include <string_view>
#include <vector>

namespace gp::os {

// start_threads is what start does for the thread layer (thread.cpp): the main thread's stack
// bounds, and the handler for a stack overflow.
void start_threads() noexcept;

// start_faults is what start does for the faults of the CPU that are not stack overflows
// (fault.cpp): the filter for the exceptions that nothing handles (Ctrl-C keeps the OS default).
void start_faults() noexcept;

// to_utf8 converts n UTF-16 code units to UTF-8 (a lone surrogate becomes U+FFFD, as in Go).
std::string to_utf8(const wchar_t* w, std::size_t n);

// to_wide converts UTF-8 to UTF-16 (bytes that are not UTF-8 become U+FFFD).
std::wstring to_wide(std::string_view s);

// split_command_line splits a Windows command line (UTF-8) into arguments with the rules of Go's
// os package (os/exec_windows.go: the rules of the C runtime since 2008, plus `""` inside quotes
// as one quote, and the same rule for the program name). It is plain C++ (no Windows header),
// so it is tested on every OS.
std::vector<std::string> split_command_line(std::string_view cmd);

// utf8_tail holds the start of a UTF-8 sequence that a write ended in the middle of (0 to 3
// bytes), until the next write of the same handle completes it.
struct utf8_tail {
    unsigned char bytes[4] = {};
    std::size_t n = 0;
};

// utf8_to_utf16 converts the UTF-8 text data[0..n) (after the bytes held in tail) to UTF-16 for a
// console and calls emit(ctx, units, count) for each piece, in order (pieces of at most 2048
// units); it stops at the first emit that returns false and returns false. Like Go's console
// writer (internal/poll, writeConsole): a start of a valid sequence at the end is kept in tail,
// and a byte that cannot continue a sequence is U+FFFD by itself (one for each byte, DecodeRune's
// way). It allocates nothing and uses plain C++ only, so it is tested on every OS.
bool utf8_to_utf16(utf8_tail& tail, const char* data, std::size_t n,
                   bool (*emit)(void* ctx, const char16_t* units, std::size_t count) noexcept,
                   void* ctx) noexcept;

} // namespace gp::os
