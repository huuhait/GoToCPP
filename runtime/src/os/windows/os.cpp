// The Windows OS layer (gocpp/os.hpp): the console and pipes for standard output and error, the
// process's arguments and environment as UTF-8, and the end of the process. The thread layer is
// thread.cpp, the exceptions fault.cpp, the stack trace trace.cpp, the command line's rules
// cmdline.cpp. This folder is the only place in the runtime that may include OS headers.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include "windows.hpp"

#include <atomic>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <functional>
#include <string>
#include <string_view>
#include <vector>

#include <windows.h>

#include <fcntl.h>
#include <io.h>
#include <psapi.h>
#include <shellapi.h>

// The main thread's stack. Linux gives it 8 MiB (ulimit -s) and the runtime's goroutines get
// 8 MiB too (goroutine.hpp); a Windows program has 1 MiB by default (MinGW's linker: 2 MiB), which
// a recursion that Go and the Linux build run fine overflows. The size is in the PE header, so it
// is the linker's: gocpp's driver passes -Wl,--stack,8388608 (the way for every linker), and this
// directive does the same for GNU ld when someone links the runtime without the driver (GNU ld
// reads the number after a space; LLVM's lld takes only the command line's flag, and a
// directive it doesn't know could be an error, so clang's builds leave it out).
#if defined(__GNUC__) && !defined(__clang__)
asm(".section .drectve\n\t.ascii \" -stack 8388608\"\n\t.text\n");
#endif

namespace gp::os {

namespace {

// handle_of returns the Windows handle of the file descriptor fd: the standard handles of the
// process for 0, 1 and 2 (they are what the parent gave, whatever the C library did with its
// descriptors), the C library's handle for any other.
HANDLE handle_of(int fd) noexcept {
    switch (fd) {
    case 0:
        return GetStdHandle(STD_INPUT_HANDLE);
    case 1:
        return GetStdHandle(STD_OUTPUT_HANDLE);
    case 2:
        return GetStdHandle(STD_ERROR_HANDLE);
    }
    return reinterpret_cast<HANDLE>(_get_osfhandle(fd));
}

// The tails of stdout and stderr (a character that a write cut in the middle waits for the next
// write of the same handle; utf8.cpp) with a lock each.
struct console_tail {
    SRWLOCK lock = SRWLOCK_INIT;
    utf8_tail tail;
};
console_tail console_tails[2];

static_assert(sizeof(wchar_t) == sizeof(char16_t));

bool emit_console(void* handle, const char16_t* units, std::size_t n) noexcept {
    auto h = static_cast<HANDLE>(handle);
    const char16_t* p = units;
    while (n > 0) {
        DWORD done = 0;
        if (!WriteConsoleW(h, reinterpret_cast<const wchar_t*>(p), static_cast<DWORD>(n), &done,
                           nullptr) ||
            done == 0) {
            return false;
        }
        p += done;
        n -= done;
    }
    return true;
}

// write_console writes the UTF-8 text to a console as UTF-16 (WriteConsoleW), so that it shows
// right whatever the console's code page: like Go, which does the same (utf8.cpp has the rules).
// It does not allocate, because the stack overflow hook writes through it on a stack that has
// little left.
bool write_console(HANDLE h, console_tail& ct, const char* data, std::size_t n) noexcept {
    AcquireSRWLockExclusive(&ct.lock);
    bool ok = utf8_to_utf16(ct.tail, data, n, emit_console, h);
    ReleaseSRWLockExclusive(&ct.lock);
    return ok;
}

} // namespace

std::string to_utf8(const wchar_t* w, std::size_t n) {
    if (n == 0) {
        return {};
    }
    // WC_ERR_INVALID_CHARS is not used: a lone surrogate becomes U+FFFD, like Go's UTF-16
    // decoding.
    int size =
        WideCharToMultiByte(CP_UTF8, 0, w, static_cast<int>(n), nullptr, 0, nullptr, nullptr);
    if (size <= 0) {
        return {};
    }
    std::string s(static_cast<std::size_t>(size), '\0');
    WideCharToMultiByte(CP_UTF8, 0, w, static_cast<int>(n), s.data(), size, nullptr, nullptr);
    return s;
}

std::wstring to_wide(std::string_view s) {
    if (s.empty()) {
        return {};
    }
    int size = MultiByteToWideChar(CP_UTF8, 0, s.data(), static_cast<int>(s.size()), nullptr, 0);
    if (size <= 0) {
        return {};
    }
    std::wstring w(static_cast<std::size_t>(size), L'\0');
    MultiByteToWideChar(CP_UTF8, 0, s.data(), static_cast<int>(s.size()), w.data(), size);
    return w;
}

void start() noexcept {
    // The C library's text mode would turn "\n" into "\r\n" for C code that uses stdio;
    // gocpp's own output goes through write and never has it. (Go writes bytes as they are.)
    _setmode(_fileno(stdout), _O_BINARY);
    _setmode(_fileno(stderr), _O_BINARY);
    _setmode(_fileno(stdin), _O_BINARY);
    start_threads();
    start_faults();
}

void die_of_sigpipe() noexcept {
    // Windows has no SIGPIPE: write reports a broken pipe as failed, and nothing calls this.
    exit_now(2);
}

write_result write(int fd, const char* data, std::size_t n) noexcept {
    HANDLE h = handle_of(fd);
    if (h == nullptr || h == INVALID_HANDLE_VALUE) {
        return write_result::failed;
    }
    DWORD mode = 0;
    if (GetFileType(h) == FILE_TYPE_CHAR && GetConsoleMode(h, &mode)) {
        if (fd == 1 || fd == 2) {
            return write_console(h, console_tails[fd - 1], data, n) ? write_result::ok
                                                                    : write_result::failed;
        }
        return write_result::failed;
    }
    while (n > 0) {
        DWORD want = n > (std::size_t(1) << 30) ? DWORD(1) << 30 : static_cast<DWORD>(n);
        DWORD done = 0;
        if (!WriteFile(h, data, want, &done, nullptr) || done == 0) {
            // A broken pipe is ERROR_BROKEN_PIPE or ERROR_NO_DATA here too, but Go on Windows
            // doesn't die of it: the write is an error, which the caller sees as failed.
            return write_result::failed;
        }
        data += done;
        n -= done;
    }
    return write_result::ok;
}

void exit(int code) noexcept {
    // One thread ends the program (see the Linux layer): a second one that comes while the
    // first flushes waits, and the process ends with the first one's code.
    static std::atomic<bool> exiting{false};
    if (exiting.exchange(true, std::memory_order_acq_rel)) {
        Sleep(INFINITE);
    }
    std::fflush(nullptr);
    // _exit: no atexit functions and no static destructors, like Go's os.Exit.
    ::_exit(code);
}

void exit_now(int code) noexcept {
    // TerminateProcess ends the process at once, without running anything of the C library or
    // of the DLLs that are loaded, and from any thread (ExitProcess takes the loader lock).
    TerminateProcess(GetCurrentProcess(), static_cast<UINT>(code));
    for (;;) {
    }
}

std::vector<std::string> arguments(int, char**) {
    // The C library's argv is in the ANSI code page: the command line is read as UTF-16 and
    // split by Go's rules (cmdline.cpp). An empty command line (a parent that gave none) has the
    // program's file as its only argument, as in Go.
    const wchar_t* cmd = GetCommandLineW();
    std::string line = cmd != nullptr ? to_utf8(cmd, std::char_traits<wchar_t>::length(cmd)) : "";
    std::vector<std::string> args = split_command_line(line);
    if (args.empty()) {
        wchar_t path[MAX_PATH * 4];
        DWORD n =
            GetModuleFileNameW(nullptr, path, static_cast<DWORD>(sizeof path / sizeof path[0]));
        if (n > 0 && n < sizeof path / sizeof path[0]) {
            args.push_back(to_utf8(path, n));
        }
    }
    return args;
}

std::vector<std::string> process_arguments() {
    // The process's own command line is what arguments() reads anyway (argv is never used).
    return arguments(0, nullptr);
}

std::vector<std::string> environment() {
    std::vector<std::string> env;
    wchar_t* block = GetEnvironmentStringsW();
    if (block == nullptr) {
        return env;
    }
    // Every entry, also the ones that start with '=' ("=C:=C:\\dir", the drives' current
    // folders): Go's os.Environ has them too.
    for (const wchar_t* p = block; *p != L'\0';) {
        std::size_t len = std::char_traits<wchar_t>::length(p);
        env.push_back(to_utf8(p, len));
        p += len + 1;
    }
    FreeEnvironmentStringsW(block);
    return env;
}

int cpu_count() noexcept {
    DWORD_PTR proc = 0, sys = 0;
    if (GetProcessAffinityMask(GetCurrentProcess(), &proc, &sys) && proc != 0) {
        int n = 0;
        for (DWORD_PTR m = proc; m != 0; m &= m - 1) {
            n++;
        }
        return n;
    }
    SYSTEM_INFO si;
    GetSystemInfo(&si);
    return si.dwNumberOfProcessors > 0 ? static_cast<int>(si.dwNumberOfProcessors) : 1;
}

void no_core_dump() noexcept {
    // No "program has stopped working" box and no Windows Error Reporting for a crash.
    SetErrorMode(SEM_FAILCRITICALERRORS | SEM_NOGPFAULTERRORBOX | SEM_NOOPENFILEERRORBOX);
}

void* library_ref(const void* addr) noexcept {
    // GetModuleHandleEx without the flags that keep the count adds one reference to the DLL that
    // holds addr; the program's own module (an .exe) has nothing to unload.
    HMODULE mod = nullptr;
    if (!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS, static_cast<LPCWSTR>(addr),
                            &mod) ||
        mod == nullptr) {
        return nullptr;
    }
    if (mod == GetModuleHandleW(nullptr)) {
        FreeLibrary(mod);
        return nullptr;
    }
    return mod;
}

void library_unref(void* handle) noexcept {
    if (handle != nullptr) {
        FreeLibrary(static_cast<HMODULE>(handle));
    }
}

double cpu_limit() noexcept {
    // No control groups. (Job objects can limit CPU rate; Go on Windows does not read them either.)
    return 0;
}

double cgroup_cpu_limit(std::string_view, std::string_view,
                        const std::function<bool(const std::string&, std::string&)>&) noexcept {
    // The parser of Linux's control group files is Linux-only; the Windows version has no files
    // to read.
    return 0;
}

mem_stats mem_info() noexcept {
    // The system's view of the process (K32GetProcessMemoryInfo is in kernel32.dll, so no
    // psapi.dll is needed): the private bytes are what the allocator got, the working set what is
    // resident. Windows' heap does not say how much of it is live; this is an estimate.
    mem_stats m;
    PROCESS_MEMORY_COUNTERS_EX pmc{};
    pmc.cb = sizeof pmc;
    if (K32GetProcessMemoryInfo(GetCurrentProcess(),
                                reinterpret_cast<PROCESS_MEMORY_COUNTERS*>(&pmc), sizeof pmc)) {
        m.gp_sys = pmc.PrivateUsage;
        m.gp_in_use = pmc.WorkingSetSize < pmc.PrivateUsage ? pmc.WorkingSetSize : pmc.PrivateUsage;
        m.gp_idle = m.gp_sys - m.gp_in_use;
    }
    return m;
}

void trim_memory() noexcept {
    // Nothing to give back that Go's FreeOSMemory would reach: the heap returns pages itself.
}

} // namespace gp::os
