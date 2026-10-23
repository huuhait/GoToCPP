// The Windows OS layer (gocpp/os.hpp): the console and pipes for standard output and error, the
// process's arguments and environment as UTF-8, and the end of the process. The thread layer is
// thread.cpp, the exceptions fault.cpp, the stack trace trace.cpp, the command line's rules
// cmdline.cpp. This folder is the only place in the runtime that may include OS headers.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include "windows.hpp"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <functional>
#include <new>
#include <string>
#include <string_view>
#include <vector>

#include <windows.h>

#include <fcntl.h>
#include <io.h>
#include <psapi.h>
#include <shellapi.h>

// The main thread's stack: 1 GiB, Go's limit of a goroutine's stack, which the Linux layer gives
// its main thread too (it raises RLIMIT_STACK); the runtime's goroutines get 8 MiB (goroutine.hpp).
// A Windows program has 1 MiB by default (MinGW's linker: 2 MiB). It is a reservation, committed
// as the stack grows. The size is in the PE header, so it
// is the linker's: gocpp's driver passes -Wl,--stack,1073741824 (the way for every linker), and
// this directive does the same for GNU ld when someone links the runtime without the driver (GNU ld
// reads the number after a space; LLVM's lld takes only the command line's flag, and a
// directive it doesn't know could be an error, so clang's builds leave it out).
#if defined(__GNUC__) && !defined(__clang__)
asm(".section .drectve\n\t.ascii \" -stack 1073741824\"\n\t.text\n");
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

write_result write_empty(int) noexcept {
    return write_result::ok;
}

const char* write_error() noexcept {
    return "input/output error";
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

const char* read_error() noexcept {
    return "input/output error";
}

std::int64_t read(int fd, char* data, std::size_t n) noexcept {
    HANDLE h = handle_of(fd);
    if (h == nullptr || h == INVALID_HANDLE_VALUE) {
        return -1;
    }
    DWORD want = n > (std::size_t(1) << 30) ? DWORD(1) << 30 : static_cast<DWORD>(n);
    DWORD done = 0;
    if (!ReadFile(h, data, want, &done, nullptr)) {
        // A pipe whose writer has gone reports ERROR_BROKEN_PIPE: the end of the input, as in Go.
        return GetLastError() == ERROR_BROKEN_PIPE ? 0 : -1;
    }
    return static_cast<std::int64_t>(done);
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

namespace {

// timeBeginPeriod and timeEndPeriod of winmm.dll, found when first needed. The DLL is loaded by
// name from the system folder and not linked: a program built with any link line (the driver's,
// a CMake project that uses the runtime, the runtime's own tests) has the functions.
using period_fn = UINT(WINAPI*)(UINT);

period_fn find_period_fn(HMODULE winmm, const char* name) {
    if (winmm == nullptr) {
        return nullptr;
    }
    return reinterpret_cast<period_fn>(reinterpret_cast<void*>(GetProcAddress(winmm, name)));
}

} // namespace

// The system timer ticks every 15.6 ms unless a process asks for less; a wait with a timeout ends
// at the first tick after its time. Two ticks of margin cover that rounding with room to spare.
std::int64_t timer_margin_ns() noexcept {
    return 32'000'000;
}

void fine_timer(bool on) noexcept {
    // The caller serializes the calls (src/timed_wait.hpp), so the statics need no lock.
    static bool loaded = false;
    static period_fn begin_period = nullptr;
    static period_fn end_period = nullptr;
    if (!loaded) {
        loaded = true;
        HMODULE winmm = LoadLibraryExW(L"winmm.dll", nullptr, LOAD_LIBRARY_SEARCH_SYSTEM32);
        begin_period = find_period_fn(winmm, "timeBeginPeriod");
        end_period = find_period_fn(winmm, "timeEndPeriod");
    }
    period_fn fn = on ? begin_period : end_period;
    if (fn != nullptr) {
        fn(1);
    }
}

// A slot of a precise wait: a high-resolution waitable timer for the time, a manual-reset event
// that wait_slot_wake sets, and a flag for the spin at the end of the wait. The lock keeps a late
// wake-up from using the handles while wait_slot_close closes them.
struct wait_slot {
    SRWLOCK lock = SRWLOCK_INIT;
    HANDLE timer = nullptr;
    HANDLE wake = nullptr;
    std::atomic<bool> woken{false};
};

namespace {

// CREATE_WAITABLE_TIMER_HIGH_RESOLUTION (Windows 10 version 1803): older systems do not know the
// flag and fail the call, so there is nothing to look up at run time.
constexpr DWORD high_resolution_timer = 0x00000002;

HANDLE new_timer() noexcept {
    return CreateWaitableTimerExW(nullptr, nullptr, high_resolution_timer, TIMER_ALL_ACCESS);
}

// milliseconds is ns as a timeout of the Win32 waits: rounded up, and never INFINITE.
DWORD milliseconds(std::int64_t ns) noexcept {
    std::int64_t ms = ns / 1'000'000 + (ns % 1'000'000 != 0 ? 1 : 0);
    return ms > 0xFFFFFFFEll ? 0xFFFFFFFEu : static_cast<DWORD>(ms);
}

// wake_hook is the test hook between the two steps of wait_slot_wake.
std::atomic<testing::wake_pause_hook> wake_hook{nullptr};

// fallback_forced is the test switch of force_fallback.
std::atomic<bool> fallback_forced{false};

} // namespace

bool precise_timers() noexcept {
    static const bool available = [] {
        HANDLE t = new_timer();
        if (t == nullptr) {
            return false;
        }
        CloseHandle(t);
        return true;
    }();
    return available && !fallback_forced.load(std::memory_order_acquire);
}

bool wait_slot_open(wait_slot* s) noexcept {
    AcquireSRWLockExclusive(&s->lock);
    if (s->timer == nullptr) {
        s->timer = new_timer();
    }
    if (s->wake == nullptr) {
        s->wake = CreateEventW(nullptr, TRUE, FALSE, nullptr);
    }
    bool ok = s->timer != nullptr && s->wake != nullptr;
    ReleaseSRWLockExclusive(&s->lock);
    if (!ok) {
        wait_slot_close(s);
    }
    return ok;
}

void wait_slot_close(wait_slot* s) noexcept {
    AcquireSRWLockExclusive(&s->lock);
    if (s->timer != nullptr) {
        CloseHandle(s->timer);
        s->timer = nullptr;
    }
    if (s->wake != nullptr) {
        CloseHandle(s->wake);
        s->wake = nullptr;
    }
    ReleaseSRWLockExclusive(&s->lock);
}

wait_slot* wait_slot_new() noexcept {
    auto* s = new (std::nothrow) wait_slot;
    if (s == nullptr) {
        return nullptr;
    }
    if (!wait_slot_open(s)) {
        delete s; // (nothing else has the pointer yet)
        return nullptr;
    }
    return s;
}

void wait_slot_arm(wait_slot* s) noexcept {
    // Both are cleared every time: a wake-up that came after the last wait ended (a late waker,
    // or the second step of one) must not end this one. Its second step may still come after
    // this call; then it ends the wait once, early, and the next arm clears it.
    s->woken.store(false);
    ResetEvent(s->wake);
}

void wait_slot_wake(wait_slot* s) noexcept {
    AcquireSRWLockShared(&s->lock);
    if (s->wake != nullptr) {
        SetEvent(s->wake);
    }
    if (auto hook = wake_hook.load(std::memory_order_acquire)) {
        hook();
    }
    s->woken.store(true);
    ReleaseSRWLockShared(&s->lock);
}

void wait_slot_wait(wait_slot* s, std::int64_t timer_ns, std::int64_t spin_ns) noexcept {
    using clock = std::chrono::steady_clock;
    auto end = clock::now() + std::chrono::nanoseconds(timer_ns + spin_ns);

    if (timer_ns > 0) {
        // The timer is relative (negative, in units of 100 ns). A SetWaitableTimerEx that is
        // already armed is re-armed and its signal cleared, so a timer left from an earlier wait
        // cannot end this one.
        LARGE_INTEGER due;
        due.QuadPart = -(timer_ns / 100);
        if (due.QuadPart == 0) {
            due.QuadPart = -1;
        }
        if (!SetWaitableTimerEx(s->timer, &due, 0, nullptr, nullptr, nullptr, 0)) {
            // No timer: the event alone with a timeout of whole milliseconds (the old accuracy;
            // the caller's loop looks at the clock).
            WaitForSingleObject(s->wake, milliseconds(timer_ns + spin_ns));
            return;
        }
        HANDLE handles[2] = {s->wake, s->timer};
        DWORD got = WaitForMultipleObjects(2, handles, FALSE, INFINITE);
        if (got == WAIT_OBJECT_0) {
            // Woken: the timer is still armed, and would fire into nobody.
            CancelWaitableTimer(s->timer);
            return;
        }
        if (got != WAIT_OBJECT_0 + 1) {
            Sleep(1); // a failed wait must not become a busy loop of the caller
            return;
        }
    }

    // The end of the wait is a spin: the timer is only as exact as the system's interrupt, and
    // this part is what makes the wait end on time.
    while (clock::now() < end) {
        if (s->woken.load(std::memory_order_acquire)) {
            return;
        }
        YieldProcessor();
    }
}

namespace {
std::atomic<unsigned long> barrier_init_count{0};
std::atomic<bool> barrier_init_failing{false};
} // namespace

bool process_barrier_init() noexcept {
    barrier_init_count.fetch_add(1, std::memory_order_relaxed);
    return !barrier_init_failing.load(std::memory_order_relaxed);
}

namespace {
std::atomic<unsigned long> barrier_count{0};
std::atomic<testing::barrier_hook> before_barrier{nullptr};
std::atomic<bool> barrier_failing{false};
} // namespace

bool process_barrier() noexcept {
    if (auto hook = before_barrier.load(std::memory_order_acquire)) {
        hook();
    }
    if (barrier_failing.load(std::memory_order_relaxed)) {
        return false;
    }
    FlushProcessWriteBuffers(); // (no result: it does not fail)
    barrier_count.fetch_add(1, std::memory_order_release);
    return true;
}

namespace testing {

unsigned long barrier_calls() noexcept {
    return barrier_count.load(std::memory_order_acquire);
}

barrier_hook set_barrier_hook(barrier_hook hook) noexcept {
    return before_barrier.exchange(hook, std::memory_order_acq_rel);
}

unsigned long barrier_init_calls() noexcept {
    return barrier_init_count.load(std::memory_order_relaxed);
}

void set_barrier_failing(bool on) noexcept {
    barrier_failing.store(on, std::memory_order_relaxed);
}

void set_barrier_init_failing(bool on) noexcept {
    barrier_init_failing.store(on, std::memory_order_relaxed);
}

int stdin_pipe(bool) noexcept {
    return -1;
}
bool stdin_directory() noexcept {
    return false;
}
bool stdin_closed() noexcept {
    return false;
}
void restore_stdin() noexcept {}
void fd_write(int, const char*, std::size_t) noexcept {}
void fd_close(int) noexcept {}
bool interrupt_in(int) noexcept {
    return false;
}
bool map_readonly_page(uintptr_t) noexcept {
    return false;
}

void force_fallback(bool on) noexcept {
    fallback_forced.store(on, std::memory_order_release);
}

wake_pause_hook set_wake_pause_hook(wake_pause_hook hook) noexcept {
    return gp::os::wake_hook.exchange(hook, std::memory_order_acq_rel);
}

std::int64_t cpu_time_ns() noexcept {
    // GetProcessTimes counts in system ticks (15.6 ms), far too coarse for the 30 microseconds of
    // CPU that a precise wait costs. QueryProcessCycleTime counts the cycles of the process's
    // threads; the cycles per nanosecond are measured once, on this thread, against the
    // performance counter (an estimate, good to a few percent).
    static const double cycles_per_ns = [] {
        double best = 0;
        for (int attempt = 0; attempt < 2; attempt++) {
            ULONG64 c0 = 0;
            ULONG64 c1 = 0;
            LARGE_INTEGER f, t0, t1;
            QueryPerformanceFrequency(&f);
            QueryThreadCycleTime(GetCurrentThread(), &c0);
            QueryPerformanceCounter(&t0);
            do {
                QueryPerformanceCounter(&t1);
            } while ((t1.QuadPart - t0.QuadPart) * 1000 < f.QuadPart * 10); // 10 ms
            QueryThreadCycleTime(GetCurrentThread(), &c1);
            double ns = static_cast<double>(t1.QuadPart - t0.QuadPart) * 1e9 /
                        static_cast<double>(f.QuadPart);
            best = std::max(best, static_cast<double>(c1 - c0) / ns);
        }
        return best;
    }();
    ULONG64 cycles = 0;
    if (cycles_per_ns <= 0 || !QueryProcessCycleTime(GetCurrentProcess(), &cycles)) {
        return 0;
    }
    return static_cast<std::int64_t>(static_cast<double>(cycles) / cycles_per_ns);
}

std::int64_t open_handles() noexcept {
    DWORD n = 0;
    if (!GetProcessHandleCount(GetCurrentProcess(), &n)) {
        return 0;
    }
    return n;
}

} // namespace testing

} // namespace gp::os
