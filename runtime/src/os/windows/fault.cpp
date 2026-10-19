// The exceptions of the CPU that nothing handles, on Windows (gocpp/os.hpp, "faults"): an
// access violation (a nil or wild pointer in C code, or in a build without checks), an integer
// division by zero, an illegal instruction. They reach the unhandled-exception filter, which
// finds the faulting stack by unwinding the exception's own context and gives the facts to the
// runtime's hook, which prints Go's report on the faulting thread's own stack. (The stack
// overflow is thread.cpp's, because it needs the handler to run before the stack is dealt with.)
//
// The filter only sees what no frame-based handler took: a C++ exception (0xE06D7363) and a __try
// block of C code see the exception first, so they keep working for CPU faults too. What the
// program cannot do is catch a CPU fault that nobody else handled with the C runtime's
// signal(SIGSEGV) (that is built on the top-level filter, which this one replaces: it is called
// only when the hook returns), nor a stack overflow, which is handled in a vectored handler
// (thread.cpp) and so before any __try block.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include "windows.hpp"

#include <atomic>
#include <cstdint>

#include <windows.h>

namespace gp::os {

namespace {

constinit std::atomic<fault_hook> fault_hook_fn{nullptr};
constinit std::atomic<LPTOP_LEVEL_EXCEPTION_FILTER> previous_filter{nullptr};

// The thread that is reporting a fault, 0 when none: a fault inside the hook (the report itself
// crashed) ends the process at once instead of calling the hook again.
constinit std::atomic<DWORD> reporting_thread{0};

bool is_cpu_fault(DWORD code) noexcept {
    switch (code) {
    case EXCEPTION_ACCESS_VIOLATION:
    case EXCEPTION_IN_PAGE_ERROR:
    case EXCEPTION_INT_DIVIDE_BY_ZERO:
    case EXCEPTION_INT_OVERFLOW:
    case EXCEPTION_ILLEGAL_INSTRUCTION:
    case EXCEPTION_PRIV_INSTRUCTION:
    case EXCEPTION_ARRAY_BOUNDS_EXCEEDED:
    case EXCEPTION_DATATYPE_MISALIGNMENT:
        return true;
    }
    return false;
}

// unwind_context writes the return addresses of the stack that the context ctx describes, the
// instruction pointer first, into pcs (at most max), with the unwind tables (RtlVirtualUnwind), and
// returns how many. It stops at a frame without tables that isn't a leaf, or at the end.
int unwind_context(const CONTEXT& ctx_in, uintptr_t* pcs, int max) noexcept {
    CONTEXT ctx = ctx_in;
    int n = 0;
    while (n < max && ctx.Rip != 0) {
        pcs[n++] = static_cast<uintptr_t>(ctx.Rip);
        DWORD64 base = 0;
        PRUNTIME_FUNCTION fn = RtlLookupFunctionEntry(ctx.Rip, &base, nullptr);
        if (fn == nullptr) {
            // A leaf function: the return address is at the stack pointer.
            if (ctx.Rsp == 0 || n > 1) {
                break; // only the first frame may be a leaf
            }
            ctx.Rip = *reinterpret_cast<DWORD64*>(ctx.Rsp);
            ctx.Rsp += 8;
            continue;
        }
        PVOID handler_data = nullptr;
        DWORD64 frame = 0;
        RtlVirtualUnwind(UNW_FLAG_NHANDLER, base, ctx.Rip, fn, &ctx, &handler_data, &frame,
                         nullptr);
    }
    return n;
}

LONG WINAPI on_unhandled(EXCEPTION_POINTERS* info) {
    const EXCEPTION_RECORD& rec = *info->ExceptionRecord;
    fault_hook h = fault_hook_fn.load(std::memory_order_acquire);
    if (h != nullptr && is_cpu_fault(rec.ExceptionCode)) {
        DWORD me = GetCurrentThreadId();
        DWORD none = 0;
        if (!reporting_thread.compare_exchange_strong(none, me) && none == me) {
            exit_now(2); // the report crashed
        }
        // (Another thread that is reporting: this one goes on into the hook, which lets one
        // report only: claim_death.)
        uintptr_t pcs[64];
        int n = unwind_context(*info->ContextRecord, pcs, 64);
        fault_info f{};
        f.gp_code = rec.ExceptionCode;
        f.gp_pc = reinterpret_cast<uintptr_t>(rec.ExceptionAddress);
        if (rec.NumberParameters >= 1) {
            f.gp_access = rec.ExceptionInformation[0];
        }
        if (rec.NumberParameters >= 2) {
            f.gp_addr = rec.ExceptionInformation[1];
        }
        f.gp_pcs = pcs;
        f.gp_npcs = n;
        h(f); // does not return
        reporting_thread.store(0);
    }
    if (LPTOP_LEVEL_EXCEPTION_FILTER prev = previous_filter.load(); prev != nullptr) {
        return prev(info);
    }
    return EXCEPTION_CONTINUE_SEARCH;
}

constinit std::atomic<bool> installed{false};

} // namespace

void start_faults() noexcept {
    if (!installed.exchange(true)) {
        previous_filter.store(SetUnhandledExceptionFilter(on_unhandled));
    }
}

fault_hook set_fault_hook(fault_hook hook) noexcept {
    return fault_hook_fn.exchange(hook, std::memory_order_acq_rel);
}

} // namespace gp::os
