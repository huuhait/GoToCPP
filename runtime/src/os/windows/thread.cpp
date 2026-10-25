// The Windows thread layer (gocpp/os.hpp, "threads"): detached threads with a stack size, the
// thread's slot, the stack bounds, and the detection of a stack overflow on any thread that has
// bounds (a vectored exception handler). The portable runtime (src/goroutine.cpp, src/rt.cpp)
// builds the goroutines and Go's report on top of it. The choice of CreateThread over std::thread
// or winpthreads is ADR-053.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include "windows.hpp"

#include <atomic>
#include <cerrno>
#include <cstddef>
#include <cstdint>
#include <new>

#include <windows.h>

namespace gp::os {

// The index of the thread slot (tls_get), for a debugger: gocpp's gdb script reads the slot of the
// selected thread from the thread environment block (TlsSlots[index]) to find the goroutine of a
// thread, as it reads the thread_local tslot of the Linux layer (docs/guides/debugging.md). It is
// TLS_OUT_OF_INDEXES until the indexes are made.
DWORD debug_tls_slot = TLS_OUT_OF_INDEXES;

namespace {

constinit std::atomic<stack_overflow_hook> overflow_hook{nullptr};

// The two thread slots: the one of tls_get and tls_set, and a flag "this thread has stack bounds"
// (the main thread and the threads of thread_start; as on Linux no other thread has them, although
// Windows knows the stack of every thread). They are TLS indexes, not thread_local variables:
// the exception handler below reads the flag, and a thread_local of the MinGW compilers is
// emulated (a call that may allocate), not safe in an exception handler. The indexes are made by
// the first start_threads (run_main), or by the first use.
struct tls_indexes {
    DWORD slot;
    DWORD bounds;
    tls_indexes() : slot(TlsAlloc()), bounds(TlsAlloc()) { debug_tls_slot = slot; }
};

const tls_indexes& tls() noexcept {
    static const tls_indexes t;
    return t;
}

// The stack that Windows keeps free after the guard page, so that a handler can run after an
// overflow (SetThreadStackGuarantee). The default is 4 KiB (on x64); the hook needs a few KiB
// for its buffer, and the format code and Wine's handling more: 64 KiB.
constexpr ULONG stack_guarantee = 64 * 1024;

// Wine (its ntdll has wine_get_version) commits the whole stack of a thread and keeps one guard
// page at its end, and on a CPU with a large XSAVE area (AVX-512) the exception frame of an
// overflow does not fit in what is left below it: Wine ends the thread with "stack overflow"
// before any handler runs (ADR-053). A guard page of the runtime's own, wine_margin above the end
// of the stack, makes the overflow come while there is room: Wine raises STATUS_STACK_OVERFLOW
// there, as Windows does at its guard page, and on_exception reports it. (The stack is wine_margin
// shorter than it is; Windows itself is left as it is.)
constexpr ULONG_PTR wine_margin = 2 * stack_guarantee;

bool on_wine() noexcept {
    static const bool wine = [] {
        HMODULE ntdll = GetModuleHandleW(L"ntdll.dll");
        return ntdll != nullptr && GetProcAddress(ntdll, "wine_get_version") != nullptr;
    }();
    return wine;
}

void wine_guard() noexcept {
    ULONG_PTR lo = 0, hi = 0;
    GetCurrentThreadStackLimits(&lo, &hi);
    SYSTEM_INFO si;
    GetSystemInfo(&si);
    ULONG_PTR page = si.dwPageSize;
    ULONG_PTR at = (lo + wine_margin + page - 1) / page * page;
    if (lo == 0 || at + 4 * page >= hi) {
        return; // a stack too small for the margin: Wine's own guard
    }
    MEMORY_BASIC_INFORMATION m;
    if (VirtualQuery(reinterpret_cast<void*>(at), &m, sizeof m) == 0 || m.State != MEM_COMMIT ||
        (m.Protect & PAGE_GUARD) != 0) {
        return; // (Wine commits the stack: this one does not)
    }
    DWORD old = 0;
    VirtualProtect(reinterpret_cast<void*>(at), page, PAGE_READWRITE | PAGE_GUARD, &old);
}

void ensure_guarantee() noexcept {
    ULONG size = stack_guarantee;
    SetThreadStackGuarantee(&size);
    if (on_wine()) {
        wine_guard();
    }
}

// The smallest stack of a thread of thread_start: the guarantee must fit with room for the
// frames of a thread. Only address space: pages are committed as the thread uses them.
constexpr std::size_t min_stack = 256 * 1024;
// A reservation above this is refused (the OS has less address space than it, and the size
// would not be an honest request).
constexpr std::size_t max_stack = std::size_t(1) << 40;

LONG CALLBACK on_exception(EXCEPTION_POINTERS* info) {
    if (info->ExceptionRecord->ExceptionCode != EXCEPTION_STACK_OVERFLOW) {
        return EXCEPTION_CONTINUE_SEARCH;
    }
    stack_overflow_hook h = overflow_hook.load(std::memory_order_acquire);
    if (h == nullptr || TlsGetValue(tls().bounds) == nullptr) {
        return EXCEPTION_CONTINUE_SEARCH;
    }
    ULONG_PTR lo = 0, hi = 0;
    GetCurrentThreadStackLimits(&lo, &hi);
    auto sp = static_cast<uintptr_t>(info->ContextRecord->Rsp);
    h(stack_fault{sp, static_cast<uintptr_t>(lo), static_cast<uintptr_t>(hi)});
    return EXCEPTION_CONTINUE_SEARCH; // the hook did not end the process: the crash
}

constinit std::atomic<bool> handler_installed{false};

// The start of a thread of thread_start: what it runs.
struct start_info {
    void (*fn)(void*);
    void* arg;
};

DWORD WINAPI thread_main(LPVOID p) {
    auto* si = static_cast<start_info*>(p);
    void (*fn)(void*) = si->fn;
    void* arg = si->arg;
    delete si;
    ensure_guarantee();
    TlsSetValue(tls().bounds, reinterpret_cast<LPVOID>(1));
    fn(arg);
    TlsSetValue(tls().bounds, nullptr);
    return 0;
}

} // namespace

void start_threads() noexcept {
    // The calling thread is the main thread (run_main): it has bounds.
    TlsSetValue(tls().bounds, reinterpret_cast<LPVOID>(1));
    ensure_guarantee();
    if (!handler_installed.exchange(true)) {
        // First = 0: last, after the handlers of the program's own C code, which then see the
        // exception first (and may take it).
        AddVectoredExceptionHandler(0, on_exception);
    }
}

int thread_start(void (*fn)(void*), void* arg, std::size_t stack_size) noexcept {
    if (stack_size > max_stack) {
        return ENOMEM;
    }
    if (stack_size < min_stack) {
        stack_size = min_stack;
    }
    auto* si = new (std::nothrow) start_info{fn, arg};
    if (si == nullptr) {
        return ENOMEM;
    }
    // STACK_SIZE_PARAM_IS_A_RESERVATION: stack_size is address space; the pages are committed as
    // the thread uses them, as on Linux (a goroutine's 8 MiB costs what it touches).
    HANDLE h = CreateThread(nullptr, stack_size, thread_main, si, STACK_SIZE_PARAM_IS_A_RESERVATION,
                            nullptr);
    if (h == nullptr) {
        DWORD err = GetLastError();
        delete si;
        // The OS's error code, except where the runtime's message looks for the POSIX one.
        return err == ERROR_NOT_ENOUGH_MEMORY || err == ERROR_OUTOFMEMORY ? EAGAIN
                                                                          : static_cast<int>(err);
    }
    CloseHandle(h); // detached: nobody waits for it
    return 0;
}

[[noreturn]] void exit_thread() {
    // A forced unwind of the thread's stack (pthread_exit) has no Windows form: the runtime never
    // calls it, only a Linux test does.
    ExitThread(0);
}

void* tls_get() noexcept {
    return TlsGetValue(tls().slot);
}

void tls_set(void* p) noexcept {
    TlsSetValue(tls().slot, p);
}

namespace {

// The registration of tls_on_thread_exit: a fiber-local value (FLS) with a callback, which Windows
// calls when a thread ends, whoever started it (a library's thread too). The value of a thread is
// its own record {fn, arg}; the callback runs fn(arg) and frees the record.
struct exit_record {
    void (*fn)(void*);
    void* arg;
};

constinit std::atomic<DWORD> fls_index{FLS_OUT_OF_INDEXES};
constinit std::atomic<bool> fls_stopped{false};
// The state of the first use: 0 not tried, 1 being tried by a thread, 2 done (fls_index is valid or
// FLS_OUT_OF_INDEXES: no index was available).
constinit std::atomic<int> fls_state{0};

void WINAPI on_thread_exit(PVOID value) {
    auto* r = static_cast<exit_record*>(value);
    if (r == nullptr) {
        return;
    }
    if (!fls_stopped.load(std::memory_order_acquire)) {
        r->fn(r->arg);
    }
    delete r;
}

} // namespace

void tls_on_thread_exit(void (*fn)(void*), void* arg) noexcept {
    if (fls_stopped.load(std::memory_order_acquire)) {
        return;
    }
    int expected = 0;
    if (fls_state.compare_exchange_strong(expected, 1, std::memory_order_acq_rel)) {
        fls_index.store(FlsAlloc(on_thread_exit), std::memory_order_release);
        fls_state.store(2, std::memory_order_release);
    } else {
        // Another thread is allocating the index: wait for it (a few instructions), so that this
        // thread's record is not lost.
        while (fls_state.load(std::memory_order_acquire) != 2) {
            SwitchToThread();
        }
    }
    DWORD idx = fls_index.load(std::memory_order_acquire);
    if (idx == FLS_OUT_OF_INDEXES) {
        return; // no index: the record is never freed
    }
    // Only the last call of a thread counts: the thread's record is changed in place.
    if (auto* r = static_cast<exit_record*>(FlsGetValue(idx)); r != nullptr) {
        r->fn = fn;
        r->arg = arg;
        return;
    }
    auto* r = new (std::nothrow) exit_record{fn, arg};
    if (r != nullptr && !FlsSetValue(idx, r)) {
        delete r;
    }
}

void tls_on_thread_exit_stop() noexcept {
    fls_stopped.store(true, std::memory_order_release);
    // FlsFree runs the callback for the values that are left, which now only free them.
    DWORD idx = fls_index.exchange(FLS_OUT_OF_INDEXES, std::memory_order_acq_rel);
    if (idx != FLS_OUT_OF_INDEXES) {
        FlsFree(idx);
    }
}

bool stack_bounds(uintptr_t& lo, uintptr_t& hi) noexcept {
    if (TlsGetValue(tls().bounds) == nullptr) {
        return false;
    }
    ULONG_PTR l = 0, h = 0;
    GetCurrentThreadStackLimits(&l, &h);
    if (l == 0 || h <= l) {
        return false;
    }
    lo = static_cast<uintptr_t>(l);
    hi = static_cast<uintptr_t>(h);
    return true;
}

stack_overflow_hook set_stack_overflow_hook(stack_overflow_hook hook) noexcept {
    return overflow_hook.exchange(hook, std::memory_order_acq_rel);
}

} // namespace gp::os
