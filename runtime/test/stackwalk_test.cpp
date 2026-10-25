// The tests of the walks of a panic's stack (gocpp/os.hpp: stack_until, stack_above, stack): in an
// optimized build for x86-64 Linux they step over the frames with rules kept per return address
// (the fast walk of src/os/linux/trace.cpp), and must give the unwinder's frames, in its order and
// with its CFAs. The cases: frames of many sizes, frames that keep the CFA in rbp (alloca), stack
// realignment (alignas(64), force_align_arg_pointer with alloca: a DRAP frame), a variadic
// function, a noreturn call that leaves with longjmp, a callback from libc (qsort), threads (one,
// and 32 that fill the table of rules at once), a deep recursion, the same walk again (with the
// kept rules), the outermost frame, functions with hand-written CFI (the trampolines below: the
// _sf forms, advance_loc2/4, remember/restore state, a CIE that advances the location before the
// FDE goes on from there (raw .eh_frame), and the ones that the fast walk leaves to the
// unwinder: a CFA in rbx, rbp kept in a register, nine remembered states, a signal frame, a CFA
// expression, a val_offset rule, no FDE), a signal handler, and shared libraries that the program
// opens, with and without unwind tables (built by runtime/cxx_test.go). os::stack must give glibc's
// backtrace's frames, also across a frame that the fast walk leaves to the unwinder.
// runtime/cxx_test.go compiles it with the runtime's .cpp files, like test/unit.cpp, and runs it:
// it prints "ok: N checks" and exits 0, or prints each failed check and exits 1.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#ifndef __linux__
// The walks of this test are Linux's (the fast walk of src/os/linux/trace.cpp, glibc's backtrace,
// dlopen, signals): another system has nothing to check here, but the file still compiles
// (runtime/portable_test.go's TestWindowsSyntax compiles every test file for Windows).
int main() {
    static const char msg[] = "ok: 0 checks (the stack walks of this test are Linux's)\n";
    gp::os::write(gp::os::stdout_fd, msg, sizeof msg - 1);
    return 0;
}
#else
#include <alloca.h>
#include <dlfcn.h>
#include <execinfo.h>
#include <atomic>
#include <csetjmp>
#include <csignal>
#include <cstdarg>
#include <cstdint>
#include <cstdlib>
#include <string>
#include <thread>
#include <vector>

namespace {

int checks = 0;
int failures = 0;

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        std::string msg = std::string(file) + ":" + std::to_string(line) + ": " + what + "\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

// The fast walk is in optimized x86-64 Linux builds without a sanitizer (gocpp/os.hpp,
// GOCPP_FAST_WALK; the asan mode of the tests is -O0): there walk_check must say 1, elsewhere 0
// (the unwinder's walk).
constexpr int want = gp::os::fast_walk_built ? 1 : 0;

// fast is what walk_check says for a walk that the fast walk can do (1) or leaves to the unwinder
// (0), in this build.
constexpr int fast(int r) {
    return want == 1 ? r : 0;
}

// check runs walk_check for the bound and checks its answer; mixed means the walk crosses a frame
// that the fast walk may leave to the unwinder (then 0 is right too).
void check_at(const void* bound, int line, bool mixed = false) {
    int got = gp::os::walk_check(bound);
    bool ok = mixed ? got == want || got == 0 : got == want;
    report(ok, __FILE__, line, "walk_check gave " + std::to_string(got));
}

volatile int sink = 0;

// nest calls itself n times, with a frame of size bytes each, then checks the walks with the
// bound in the frame of the outermost call.
__attribute__((noinline)) void nest(int n, int size, const void* bound) {
    char* buf = static_cast<char*>(alloca(1));
    buf[0] = static_cast<char>(n);
    if (n == 0) {
        check_at(bound, __LINE__);
        check_at(bound, __LINE__); // (the kept rules)
        check_at(nullptr, __LINE__);
        check_at(reinterpret_cast<const void*>(~uintptr_t(0)), __LINE__);
        return;
    }
    if (size > 0) {
        // A frame with a variable size: its CFA is kept in rbp.
        char* big = static_cast<char*>(alloca(static_cast<std::size_t>(size)));
        big[0] = 1;
        big[size - 1] = 2;
        sink = sink + big[0] + big[size - 1];
    }
    nest(n - 1, size, bound);
    sink = sink + buf[0];
}

template <int N>
__attribute__((noinline)) void fixed_frame(int n, const void* bound) {
    volatile char frame[N];
    frame[0] = static_cast<char>(n);
    if (n == 0) {
        check_at(bound, __LINE__);
    } else {
        fixed_frame<N>(n - 1, bound);
    }
    sink = sink + frame[0];
}

const void* qsort_bound = nullptr;
bool qsort_checked = false;

int compare(const void* a, const void* b) {
    if (!qsort_checked) {
        qsort_checked = true;
        // qsort's frames are libc's: stepped over with their FDE's rules, not kept.
        check_at(qsort_bound, __LINE__);
    }
    int x = *static_cast<const int*>(a), y = *static_cast<const int*>(b);
    return x < y ? -1 : x > y;
}

__attribute__((noinline)) void through_qsort() {
    int bound = 0;
    qsort_bound = &bound;
    int v[] = {5, 3, 9, 1, 7};
    std::qsort(v, 5, sizeof v[0], compare);
    CHECK(qsort_checked);
    CHECK(v[0] == 1 && v[4] == 9);
    sink = sink + bound;
}

struct alignas(64) wide {
    char b[64];
};

__attribute__((noinline)) void realigned(int n) {
    // An over-aligned local: the frame realigns rsp.
    wide w{};
    w.b[0] = static_cast<char>(n);
    check_at(&w, __LINE__, true);
    sink = sink + w.b[0];
}

// stack_like_backtrace checks that os::stack gives glibc's backtrace's frames with the fast walk:
// its first one is in os::stack (backtrace's in the function that calls it), the others are the
// same. (Without the fast walk os::stack is backtrace, which ASan intercepts: a frame more.)
__attribute__((noinline)) void stack_like_backtrace(int n) {
    if (n > 0) {
        stack_like_backtrace(n - 1);
        sink = sink + 1;
        return;
    }
    constexpr int max = 400;
    uintptr_t got[max];
    void* bt[max];
    int ng = gp::os::stack(got, max);
    int nb = ::backtrace(bt, max);
    CHECK(ng > 2 && nb > 2);
    if (want == 1) {
        bool same = ng == nb + 1;
        for (int i = 1; same && i < nb; i++) {
            same = got[i + 1] == reinterpret_cast<uintptr_t>(bt[i]);
        }
        CHECK(same);
    }
    // A short buffer: the inner frames.
    int short_n = gp::os::stack(got, 3);
    CHECK(short_n == 3);
}

// --- functions with hand-written CFI. Each is void name(void (*fn)(void*), void* arg) and calls
// fn(arg). (x86-64 assembler: the fast walk is x86-64's.)
extern "C" {
void sw_t_sf(void (*)(void*), void*);
void sw_t_big(void (*)(void*), void*);
void sw_t_rbx(void (*)(void*), void*);
void sw_t_reg(void (*)(void*), void*);
void sw_t_state(void (*)(void*), void*);
void sw_t_deep(void (*)(void*), void*);
void sw_t_sig(void (*)(void*), void*);
void sw_t_expr(void (*)(void*), void*);
void sw_t_val(void (*)(void*), void*);
void sw_t_outer(void (*)(void*), void*);
void sw_t_nofde(void (*)(void*), void*);
void sw_t_cieadv(void (*)(void*), void*);
}

#if defined(__x86_64__) && defined(__linux__)
asm(R"(
	.pushsection .text
# def_cfa_offset_sf and offset_extended_sf (data_align -8)
	.globl sw_t_sf
	.type sw_t_sf,@function
sw_t_sf:
	.cfi_startproc
	push %rbp
	.cfi_escape 0x13, 0x7e
	.cfi_escape 0x11, 0x06, 0x02
	mov $0x1111, %rbp
	mov %rdi, %rax
	mov %rsi, %rdi
	call *%rax
	pop %rbp
	.cfi_def_cfa_offset 8
	.cfi_restore 6
	ret
	.cfi_endproc
	.size sw_t_sf, .-sw_t_sf

# an rbp frame with advance_loc2 and advance_loc4, GNU_args_size, restore_extended
	.globl sw_t_big
	.type sw_t_big,@function
sw_t_big:
	.cfi_startproc
	push %rbp
	.cfi_def_cfa_offset 16
	.cfi_offset 6, -16
	.fill 300,1,0x90
	mov %rsp, %rbp
	.cfi_def_cfa_register 6
	.fill 70000,1,0x90
	sub $32, %rsp
	.cfi_escape 0x2e, 0x20
	mov %rdi, %rax
	mov %rsi, %rdi
	call *%rax
	leave
	.cfi_def_cfa 7, 8
	.cfi_escape 0x06, 0x06
	ret
	.cfi_endproc
	.size sw_t_big, .-sw_t_big

# the CFA in rbx: the fast walk gives up
	.globl sw_t_rbx
	.type sw_t_rbx,@function
sw_t_rbx:
	.cfi_startproc
	push %rbx
	.cfi_def_cfa_offset 16
	.cfi_offset 3, -16
	mov %rsp, %rbx
	.cfi_def_cfa_register 3
	mov %rdi, %rax
	mov %rsi, %rdi
	call *%rax
	mov %rbx, %rsp
	.cfi_def_cfa_register 7
	pop %rbx
	.cfi_def_cfa_offset 8
	ret
	.cfi_endproc
	.size sw_t_rbx, .-sw_t_rbx

# rbp kept in rbx (DW_CFA_register), rbp changed: the fast walk gives up
	.globl sw_t_reg
	.type sw_t_reg,@function
sw_t_reg:
	.cfi_startproc
	push %rbx
	.cfi_def_cfa_offset 16
	.cfi_offset 3, -16
	mov %rbp, %rbx
	.cfi_register 6, 3
	mov $0x2222, %rbp
	mov %rdi, %rax
	mov %rsi, %rdi
	call *%rax
	mov %rbx, %rbp
	.cfi_same_value 6
	pop %rbx
	.cfi_def_cfa_offset 8
	ret
	.cfi_endproc
	.size sw_t_reg, .-sw_t_reg

# remember and restore state: an early return before the call
	.globl sw_t_state
	.type sw_t_state,@function
sw_t_state:
	.cfi_startproc
	push %rbp
	.cfi_def_cfa_offset 16
	.cfi_offset 6, -16
	mov %rsp, %rbp
	.cfi_def_cfa_register 6
	test %rdi, %rdi
	jnz 1f
	.cfi_remember_state
	pop %rbp
	.cfi_def_cfa 7, 8
	.cfi_restore 6
	ret
1:
	.cfi_restore_state
	mov %rdi, %rax
	mov %rsi, %rdi
	mov $0x3333, %rcx
	push %rcx
	push %rcx
	call *%rax
	leave
	.cfi_def_cfa 7, 8
	ret
	.cfi_endproc
	.size sw_t_state, .-sw_t_state

# nine nested remember_state, more than the fast walk keeps: it gives up
	.globl sw_t_deep
	.type sw_t_deep,@function
sw_t_deep:
	.cfi_startproc
	push %rbp
	.cfi_def_cfa_offset 16
	.cfi_offset 6, -16
	nop
	.cfi_remember_state
	nop
	.cfi_remember_state
	nop
	.cfi_remember_state
	nop
	.cfi_remember_state
	nop
	.cfi_remember_state
	nop
	.cfi_remember_state
	nop
	.cfi_remember_state
	nop
	.cfi_remember_state
	nop
	.cfi_remember_state
	nop
	.cfi_def_cfa_offset 16
	mov %rdi, %rax
	mov %rsi, %rdi
	call *%rax
	pop %rbp
	.cfi_def_cfa_offset 8
	ret
	.cfi_endproc
	.size sw_t_deep, .-sw_t_deep

# a signal frame's CIE ('S'): the fast walk gives up
	.globl sw_t_sig
	.type sw_t_sig,@function
sw_t_sig:
	.cfi_startproc
	.cfi_signal_frame
	push %rbp
	.cfi_def_cfa_offset 16
	.cfi_offset 6, -16
	mov %rdi, %rax
	mov %rsi, %rdi
	call *%rax
	pop %rbp
	.cfi_def_cfa_offset 8
	ret
	.cfi_endproc
	.size sw_t_sig, .-sw_t_sig

# a CFA expression (DW_CFA_def_cfa_expression: rsp + 16): the fast walk gives up
	.globl sw_t_expr
	.type sw_t_expr,@function
sw_t_expr:
	.cfi_startproc
	push %rbp
	.cfi_escape 0x0f, 0x02, 0x77, 0x10
	.cfi_offset 6, -16
	mov %rdi, %rax
	mov %rsi, %rdi
	call *%rax
	pop %rbp
	.cfi_def_cfa 7, 8
	ret
	.cfi_endproc
	.size sw_t_expr, .-sw_t_expr

# a val_offset rule for rsp: the fast walk gives up
	.globl sw_t_val
	.type sw_t_val,@function
sw_t_val:
	.cfi_startproc
	push %rbp
	.cfi_def_cfa_offset 16
	.cfi_offset 6, -16
	.cfi_escape 0x14, 0x07, 0x00
	mov %rdi, %rax
	mov %rsi, %rdi
	call *%rax
	pop %rbp
	.cfi_def_cfa_offset 8
	.cfi_restore 6
	ret
	.cfi_endproc
	.size sw_t_val, .-sw_t_val

# the outermost frame: the return address is undefined
	.globl sw_t_outer
	.type sw_t_outer,@function
sw_t_outer:
	.cfi_startproc
	push %rbp
	.cfi_def_cfa_offset 16
	.cfi_offset 6, -16
	.cfi_undefined 16
	mov %rdi, %rax
	mov %rsi, %rdi
	call *%rax
	pop %rbp
	.cfi_def_cfa_offset 8
	.cfi_same_value 16
	ret
	.cfi_endproc
	.size sw_t_outer, .-sw_t_outer

# no FDE at all (like code made at run time)
	.globl sw_t_nofde
	.type sw_t_nofde,@function
sw_t_nofde:
	push %rbp
	mov %rdi, %rax
	mov %rsi, %rdi
	call *%rax
	pop %rbp
	ret
	.size sw_t_nofde, .-sw_t_nofde

# sw_t_cieadv: its CFI is written by hand into .eh_frame below. Its CIE advances the location
# (advance_loc 1, then CFA = rsp+16 and rbp saved) and its FDE advances it again (advance_loc 8,
# to the return address, func+9) before a wrong def_cfa_offset 64. The unwinder goes on from the
# CIE's location, so at the return address the row is CFA = rsp+16; a walk that started the FDE's
# program at the function's start again would apply the 64.
	.globl sw_t_cieadv
	.type sw_t_cieadv,@function
sw_t_cieadv:
	push %rbp
	mov %rdi, %rax
	mov %rsi, %rdi
	call *%rax
	pop %rbp
	ret
.Lsw_cieadv_end:
	.size sw_t_cieadv, .-sw_t_cieadv
	.popsection

	.pushsection .eh_frame,"a",@progbits
.Lsw_cie:
	.long .Lsw_cie_end - .Lsw_cie_start
.Lsw_cie_start:
	.long 0
	.byte 1
	.string "zR"
	.uleb128 1
	.sleb128 -8
	.byte 16
	.uleb128 1
	.byte 0x1b
	.byte 0x0c, 0x07, 0x08
	.byte 0x90, 0x01
	.byte 0x41
	.byte 0x0e, 0x10
	.byte 0x86, 0x02
	.balign 8, 0
.Lsw_cie_end:
	.long .Lsw_fde_end - .Lsw_fde_start
.Lsw_fde_start:
	.long .Lsw_fde_start - .Lsw_cie
	.long sw_t_cieadv - .
	.long .Lsw_cieadv_end - sw_t_cieadv
	.uleb128 0
	.byte 0x48
	.byte 0x0e, 0x40
	.balign 8, 0
.Lsw_fde_end:
	.popsection
)");
#endif

const void* g_bound = nullptr;

struct cfi_case {
    int line;
    int fast_result; // walk_check where the fast walk is: 1 it steps over the frame, 0 it gives up
};

// os::stack must be glibc's backtrace (its first frame is in os::stack, backtrace's in its
// caller), with the fast walk also across a frame that it leaves to the unwinder.
__attribute__((noinline)) void check_stack_like_backtrace(int line) {
    constexpr int max = 400;
    uintptr_t got[max];
    void* bt[max];
    int ng = gp::os::stack(got, max);
    int nb = ::backtrace(bt, max);
    report(ng > 0 && nb > 0, __FILE__, line, "os::stack or backtrace gave no frames");
    if (want == 1) {
        bool same = ng == nb + 1;
        for (int i = 1; same && i < nb; i++) {
            same = got[i + 1] == reinterpret_cast<uintptr_t>(bt[i]);
        }
        report(same, __FILE__, line,
               "os::stack gave " + std::to_string(ng) + " frames, backtrace " + std::to_string(nb));
    }
}

__attribute__((noinline)) void cfi_cb(void* p) {
    auto* c = static_cast<cfi_case*>(p);
    int r = fast(c->fast_result);
    for (const void* b : {g_bound, static_cast<const void*>(nullptr)}) {
        int got = gp::os::walk_check(b);
        report(got == r, __FILE__, c->line,
               "walk_check gave " + std::to_string(got) + ", want " + std::to_string(r));
    }
    check_stack_like_backtrace(c->line);
}

using trampoline = void (*)(void (*)(void*), void*);

// via_alloca calls t from a frame whose CFA is in rbp.
__attribute__((noinline)) void via_alloca(trampoline t, void (*fn)(void*), void* c, int n) {
    char* b = static_cast<char*>(alloca(static_cast<std::size_t>(n)));
    b[0] = 1;
    t(fn, c);
    sink = sink + b[0];
}

void cfi_cases() {
#if defined(__x86_64__) && defined(__linux__)
    struct {
        trampoline t;
        cfi_case c;
    } cases[] = {
        {sw_t_sf, {__LINE__, 1}},    {sw_t_big, {__LINE__, 1}},   {sw_t_rbx, {__LINE__, 0}},
        {sw_t_reg, {__LINE__, 0}},   {sw_t_state, {__LINE__, 1}}, {sw_t_deep, {__LINE__, 0}},
        {sw_t_sig, {__LINE__, 0}},   {sw_t_expr, {__LINE__, 0}},  {sw_t_val, {__LINE__, 0}},
        {sw_t_outer, {__LINE__, 1}}, {sw_t_nofde, {__LINE__, 0}}, {sw_t_cieadv, {__LINE__, 1}},
    };
    for (int round = 0; round < 2; round++) { // the second time with the kept rules
        for (auto& k : cases) {
            k.t(cfi_cb, &k.c);
            via_alloca(k.t, cfi_cb, &k.c, 100 + k.c.line % 50);
        }
    }
#endif
}

// The outermost frame (sw_t_outer, return address undefined): a bound between the CFA of the
// callback's frame and the CFA of the outermost one.
__attribute__((noinline)) void outer_cb(void*) {
    auto fa = reinterpret_cast<uintptr_t>(__builtin_frame_address(0));
    check_at(reinterpret_cast<const void*>(fa + 16 + 8), __LINE__);
    check_at(nullptr, __LINE__);
}

// A signal handler: the walks go through the kernel's signal frame (libc's trampoline, a signal
// frame's CIE), which the fast walk leaves to the unwinder.
volatile std::sig_atomic_t handled = 0;
void on_usr1(int) {
    handled = 1;
    int got = gp::os::walk_check(nullptr);
    report(got == 0, __FILE__, __LINE__,
           "walk_check in a signal handler gave " + std::to_string(got));
    got = gp::os::walk_check(g_bound);
    report(got == 0, __FILE__, __LINE__,
           "walk_check in a signal handler gave " + std::to_string(got));
    check_stack_like_backtrace(__LINE__);
}

// A DRAP frame (the stack realigned with an argument pointer, and alloca) and an alignas(64) one:
// the compilers may describe them with a CFA expression (the walk then is the unwinder's).
struct alignas(32) v32 {
    double d[4];
};
__attribute__((noinline, force_align_arg_pointer)) void realign_drap(int n) {
    v32 x{};
    x.d[0] = n;
    char* b = static_cast<char*>(alloca(static_cast<std::size_t>(n)));
    b[0] = 2;
    check_at(&x, __LINE__, true);
    check_at(nullptr, __LINE__, true);
    check_stack_like_backtrace(__LINE__);
    sink = sink + b[0] + static_cast<int>(x.d[0]);
}

__attribute__((noinline)) void realign_64(int n) {
    alignas(64) volatile char big[256];
    big[0] = static_cast<char>(n);
    check_at(nullptr, __LINE__, true);
    sink = sink + big[0];
}

__attribute__((noinline)) void variadic(int n, ...) {
    va_list ap;
    va_start(ap, n);
    int s = 0;
    for (int i = 0; i < n; i++) {
        s += va_arg(ap, int);
    }
    va_end(ap);
    check_at(nullptr, __LINE__);
    sink = sink + s;
}

// A noreturn call at the end of a function (its return address may be past the function's end),
// which leaves with longjmp; and a walk after the longjmp.
std::jmp_buf jb;
[[noreturn]] __attribute__((noinline)) void no_return() {
    check_at(nullptr, __LINE__);
    std::longjmp(jb, 1);
}
__attribute__((noinline)) void ends_in_noreturn(int x) {
    sink = sink + x;
    no_return();
}

// Threads at once, each with many functions of its own (the instances of chain): they fill the
// table of rules together.
std::atomic<int> thread_differ{0};
std::atomic<int> thread_unwinder{0};
template <int N>
__attribute__((noinline)) void chain(int d) {
    volatile char f[N % 7 * 16 + 8];
    f[0] = static_cast<char>(d);
    if constexpr (N > 0) {
        chain<N - 1>(d);
    } else {
        int r = gp::os::walk_check(nullptr);
        if (r < 0) {
            thread_differ++;
        }
        if (r == 0) {
            thread_unwinder++;
        }
    }
    sink = sink + f[0];
}

void many_threads() {
    std::vector<std::thread> th;
    for (int k = 0; k < 32; k++) {
        th.emplace_back([k] {
            for (int r = 0; r < 200; r++) {
                switch ((k + r) % 4) {
                case 0:
                    chain<60>(r);
                    break;
                case 1:
                    chain<45>(r);
                    break;
                case 2:
                    chain<30>(r);
                    break;
                default:
                    chain<15>(r);
                    break;
                }
            }
        });
    }
    for (auto& t : th) {
        t.join();
    }
    CHECK(thread_differ.load() == 0);
    CHECK(thread_unwinder.load() == (want == 1 ? 0 : 32 * 200));
}

// Shared libraries that the program opens: lib_call(fn, arg) calls fn(arg); libgp_walk_cb.so has
// unwind tables (its frame is stepped over with the rules of its FDE, which are not kept), and
// libgp_walk_nofde.so has none (the walk is the unwinder's). runtime/cxx_test.go builds them next
// to this program, which runs in that folder.
void libraries() {
    struct {
        const char* path;
        cfi_case c;
    } libs[] = {{"./libgp_walk_cb.so", {__LINE__, 1}}, {"./libgp_walk_nofde.so", {__LINE__, 0}}};
    for (auto& l : libs) {
        void* h = dlopen(l.path, RTLD_NOW);
        report(h != nullptr, __FILE__, l.c.line, std::string("dlopen ") + l.path);
        if (h == nullptr) {
            continue;
        }
        auto f = reinterpret_cast<trampoline>(dlsym(h, "lib_call"));
        report(f != nullptr, __FILE__, l.c.line, std::string("dlsym lib_call in ") + l.path);
        if (f != nullptr) {
            f(cfi_cb, &l.c);
            via_alloca(f, cfi_cb, &l.c, 64);
            f(cfi_cb, &l.c);
        }
        dlclose(h);
    }
}

} // namespace

int main() {
    int bound = 0;
    nest(0, 0, &bound);
    nest(5, 0, &bound);
    nest(5, 100, &bound);
    nest(300, 0, &bound);
    nest(40, 5000, &bound);
    fixed_frame<16>(10, &bound);
    fixed_frame<4096>(10, &bound);
    fixed_frame<200000>(3, &bound);
    stack_like_backtrace(0);
    stack_like_backtrace(50);
    through_qsort();
    realigned(3);
    std::thread th([] {
        int tb = 0;
        nest(20, 64, &tb);
        fixed_frame<512>(5, &tb);
        stack_like_backtrace(10);
    });
    th.join();
    g_bound = &bound;
    cfi_cases();
#if defined(__x86_64__) && defined(__linux__)
    sw_t_outer(outer_cb, nullptr);
#endif
    std::signal(SIGUSR1, on_usr1);
    std::raise(SIGUSR1);
    CHECK(handled == 1);
    std::signal(SIGUSR1, SIG_DFL);
    realign_drap(40);
    realign_64(3);
    variadic(3, 1, 2, 3);
    if (setjmp(jb) == 0) {
        ends_in_noreturn(1);
    }
    check_at(nullptr, __LINE__);
    many_threads();
    libraries();
    sink = sink + bound;
    if (failures != 0) {
        std::string msg =
            std::to_string(failures) + " of " + std::to_string(checks) + " checks failed\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        return 1;
    }
    std::string msg = "ok: " + std::to_string(checks) + " checks\n";
    gp::os::write(gp::os::stdout_fd, msg.data(), msg.size());
    return 0;
}
#endif // __linux__
