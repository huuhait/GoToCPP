// The Linux part of the stack trace of a panic (gocpp/os.hpp): the return addresses
// (glibc's backtrace), the program's own ELF file (symbol table and DWARF sections, read from
// /proc/self/exe) and C++ demangling. runtime/src/trace.cpp does the rest in standard C++.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include <gocpp/trace.hpp>

#include <algorithm>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <memory>
#include <new>

#include <cxxabi.h>
#include <elf.h>
#include <execinfo.h>
#include <link.h>
#include <unwind.h>

#include <atomic>

// GOCPP_FAST_WALK (gocpp/os.hpp): the fast walk (below) is in this build.

namespace gp::os {

#if !GOCPP_FAST_WALK
int stack(uintptr_t* pcs, int max) noexcept {
    if (max <= 0) {
        return 0;
    }
    // backtrace fills an array of void*; the addresses are copied into pcs.
    constexpr int small = 128;
    void* local[small];
    std::unique_ptr<void*[]> big;
    void** buf = local;
    if (max > small) {
        // Under run_main's new handler (panic.hpp) a failed allocation is a fatal error
        // before nothrow gets a null pointer: the null check is for the trace of that fatal
        // error (out_of_memory removes the handler first) and for code without run_main.
        big.reset(new (std::nothrow) void*[static_cast<std::size_t>(max)]);
        if (!big) {
            return 0;
        }
        buf = big.get();
    }
    int n = ::backtrace(buf, max);
    for (int i = 0; i < n; i++) {
        pcs[i] = reinterpret_cast<uintptr_t>(buf[i]);
    }
    return n < 0 ? 0 : n;
}
#endif

namespace {

// The state of a walk of the stack with the C++ unwinder (libgcc's _Unwind_Backtrace, which glibc's
// backtrace uses too, without the cost of its first-call setup and with a way to stop).
struct unwind_walk {
    uintptr_t* pcs;
    int max;
    int n;
    // bound: the address of a local variable of a function X that is running. The unwinder gives
    // each frame as the return address in a function and the CFA of the frame it returns from
    // (the callee's): that is the lowest address of the function's own frame. So X's pc comes with
    // a CFA at or below bound, and the pc of X's caller with a CFA above it, as do all the frames
    // outside X; the frames inside X (X's callees) have lower CFAs still.
    // stop: the walk stops when it gets to a frame outside X (stack_until); else the frames inside
    // X and X itself are not recorded (stack_above).
    uintptr_t bound;
    bool stop;
    bool reached; // a frame outside X has been seen
    uintptr_t last_pc;
    uintptr_t last_cfa;
    uintptr_t* cfas = nullptr; // (walk_check) the CFA of each recorded frame
    int skip = 0;              // frames left out before the first recorded one (stack)
};

// record is what the walk does with each frame, the return address pc in a function and the CFA
// of the frame it returns from: _URC_NO_REASON to go on, else the walk ends.
_Unwind_Reason_Code record(unwind_walk* w, uintptr_t pc, uintptr_t cfa) {
    // As glibc's backtrace: no progress is the end of the stack.
    if (w->last_cfa != 0 && pc == w->last_pc && cfa == w->last_cfa) {
        return _URC_END_OF_STACK;
    }
    w->last_pc = pc;
    w->last_cfa = cfa;
    bool outside = cfa > w->bound;
    if (w->stop) {
        if (outside) {
            w->reached = true;
            return _URC_END_OF_STACK; // (the frame before was X's)
        }
    } else if (!outside) {
        return _URC_NO_REASON;
    } else {
        w->reached = true;
    }
    if (w->skip > 0) {
        w->skip--;
        return _URC_NO_REASON;
    }
    if (w->cfas != nullptr) {
        w->cfas[w->n] = cfa;
    }
    w->pcs[w->n++] = pc;
    return w->n == w->max ? _URC_END_OF_STACK : _URC_NO_REASON;
}

_Unwind_Reason_Code unwind_step(_Unwind_Context* ctx, void* data) {
    return record(static_cast<unwind_walk*>(data), static_cast<uintptr_t>(_Unwind_GetIP(ctx)),
                  static_cast<uintptr_t>(_Unwind_GetCFA(ctx)));
}

#if GOCPP_FAST_WALK

// --- the fast walk (x86-64, optimized builds) ---
//
// A walk with _Unwind_Backtrace costs about 50 ns a frame and 100 ns to start: for each frame the
// unwinder looks up its FDE and runs the CFI program of its function up to the return address.
// The fast walk computes the same frames, in the same order and with the same CFAs, from a rule
// per return address that it finds once in the same FDE and keeps: how the CFA of the frame
// follows from rsp or rbp, where the return address and the caller's rbp are saved. A step is
// then a lookup and two loads. Only these registers matter to the CFAs: a frame whose rule needs
// anything else (a CFA expression, a signal frame, a saved rsp, a rule for rbp other than "saved
// at an offset of the CFA") makes the fast walk give up, and walk() asks the unwinder instead,
// from the start, so the result is always the unwinder's. The rules of return addresses in the
// program itself are kept (its code is never unloaded); a frame in a shared library is stepped
// over with a rule found in its FDE each time.

struct dwarf_eh_bases {
    void* tbase;
    void* dbase;
    void* func;
};

extern "C" const void* _Unwind_Find_FDE(void* pc, dwarf_eh_bases* bases);

int main_object(dl_phdr_info* info, size_t, void* data); // (below)

// How to step from a frame to its caller. The frame's registers are its pc, its sp (the CFA of
// the frame it called) and rbp.
struct frame_rule {
    bool cfa_rbp = false; // CFA = rbp + cfa_off, else sp + cfa_off
    int64_t cfa_off = 0;
    int64_t ra_off = -8;    // the return address is at CFA + ra_off
    bool outermost = false; // the return address is undefined: the last frame
    bool rbp_saved = false; // the caller's rbp is at CFA + rbp_off, else it is this frame's rbp
    int64_t rbp_off = 0;
};

// A rule of a register column in a row of the CFI table.
struct col_rule {
    enum kind : uint8_t { same, offset, undefined, other };
    kind how = same;
    int64_t off = 0;
};

struct cfi_row {
    uint64_t cfa_reg = 7;
    int64_t cfa_off = 8;
    bool cfa_expr = false;
    col_rule rbp, rsp, ra;
};

constexpr uint64_t reg_rbp = 6, reg_rsp = 7, reg_ra = 16;

struct cfi_reader {
    const uint8_t* p;
    const uint8_t* end;
    bool bad = false;

    uint8_t u8() {
        if (p >= end) {
            bad = true;
            return 0;
        }
        return *p++;
    }
    uint64_t fixed(int n) {
        if (end - p < n) {
            bad = true;
            p = end;
            return 0;
        }
        uint64_t v = 0;
        std::memcpy(&v, p, static_cast<std::size_t>(n)); // little-endian
        p += n;
        return v;
    }
    uint64_t uleb() {
        uint64_t v = 0;
        int shift = 0;
        for (;;) {
            uint8_t b = u8();
            if (bad) {
                return 0;
            }
            if (shift < 64) {
                v |= uint64_t(b & 0x7f) << shift;
            }
            shift += 7;
            if ((b & 0x80) == 0) {
                return v;
            }
        }
    }
    int64_t sleb() {
        uint64_t v = 0;
        int shift = 0;
        uint8_t b;
        do {
            b = u8();
            if (bad) {
                return 0;
            }
            if (shift < 64) {
                v |= uint64_t(b & 0x7f) << shift;
            }
            shift += 7;
        } while ((b & 0x80) != 0);
        if (shift < 64 && (b & 0x40) != 0) {
            v |= ~uint64_t(0) << shift;
        }
        return static_cast<int64_t>(v);
    }
    // An encoded pointer (DW_EH_PE_*), as libgcc's read_encoded_value_with_base reads it. The
    // indirect bit is not followed (the walk needs these values only for DW_CFA_set_loc, and to
    // skip the personality routine's pointer).
    uint64_t encoded(uint8_t enc, const dwarf_eh_bases& b) {
        if (enc == 0xff) {
            return 0;
        }
        const uint8_t* at = p;
        uint64_t v;
        switch (enc & 0x0f) {
        case 0x00:
            v = fixed(8);
            break;
        case 0x01:
            v = uleb();
            break;
        case 0x02:
            v = fixed(2);
            break;
        case 0x03:
            v = fixed(4);
            break;
        case 0x04:
            v = fixed(8);
            break;
        case 0x09:
            v = static_cast<uint64_t>(sleb());
            break;
        case 0x0a:
            v = static_cast<uint64_t>(static_cast<int64_t>(static_cast<int16_t>(fixed(2))));
            break;
        case 0x0b:
            v = static_cast<uint64_t>(static_cast<int64_t>(static_cast<int32_t>(fixed(4))));
            break;
        case 0x0c:
            v = fixed(8);
            break;
        default:
            bad = true;
            return 0;
        }
        if (v == 0) {
            return 0;
        }
        switch (enc & 0x70) {
        case 0x00:
            break;
        case 0x10:
            v += reinterpret_cast<uintptr_t>(at);
            break;
        case 0x20:
            v += reinterpret_cast<uintptr_t>(b.tbase);
            break;
        case 0x30:
            v += reinterpret_cast<uintptr_t>(b.dbase);
            break;
        case 0x40:
            v += reinterpret_cast<uintptr_t>(b.func);
            break;
        default:
            bad = true;
            return 0;
        }
        return v;
    }
};

col_rule* column(cfi_row& r, uint64_t reg) {
    switch (reg) {
    case reg_rbp:
        return &r.rbp;
    case reg_rsp:
        return &r.rsp;
    case reg_ra:
        return &r.ra;
    default:
        return nullptr;
    }
}

// run executes the CFI instructions in rd on row while loc < target (libgcc's
// execute_cfa_program). init is the row after the CIE's instructions (DW_CFA_restore), or null
// while they run. It reports false for an instruction the fast walk doesn't know.
//
// rule_from_fde runs it twice, the CIE's instructions and then the FDE's, as libgcc's
// uw_frame_state_for does: that sets fs->pc to the function's start (context->bases.func) once,
// before the CIE's program, and the FDE's program goes on from where the CIE's left it; so loc is
// the start for the CIE's program and then what it left. libgcc's execute_cfa_program keeps the
// states of DW_CFA_remember_state in memory of its own call (alloca), linked through
// fs->regs.prev, which is no longer valid after the CIE's program: a state remembered in the CIE
// and restored in the FDE is not something that the unwinder supports. Here each program has its
// own empty stack of remembered states, and a restore_state with nothing remembered makes the walk
// give up, so such a frame is the unwinder's.
bool run(cfi_reader& rd, cfi_row& row, const cfi_row* init, uint64_t& loc, uint64_t target,
         uint64_t code_align, int64_t data_align, uint8_t fde_enc, const dwarf_eh_bases& b) {
    constexpr int depth = 8;
    cfi_row saved[depth];
    int nsaved = 0;
    auto set = [&row](uint64_t reg, col_rule::kind how, int64_t off) {
        if (col_rule* c = column(row, reg)) {
            c->how = how;
            c->off = off;
        }
    };
    auto restore = [&row, init](uint64_t reg) {
        col_rule* c = column(row, reg);
        if (c == nullptr) {
            return true;
        }
        if (init == nullptr) {
            return false; // DW_CFA_restore in a CIE
        }
        *c = *column(const_cast<cfi_row&>(*init), reg);
        return true;
    };
    while (rd.p < rd.end && loc < target) {
        uint8_t op = rd.u8();
        switch (op & 0xc0) {
        case 0x40:
            loc += (op & 0x3f) * code_align;
            continue;
        case 0x80:
            set(op & 0x3f, col_rule::offset, static_cast<int64_t>(rd.uleb()) * data_align);
            continue;
        case 0xc0:
            if (!restore(op & 0x3f)) {
                return false;
            }
            continue;
        default:
            break;
        }
        switch (op) {
        case 0x00: // nop
            break;
        case 0x01: // set_loc
            loc = rd.encoded(fde_enc, b);
            break;
        case 0x02:
            loc += rd.fixed(1) * code_align;
            break;
        case 0x03:
            loc += rd.fixed(2) * code_align;
            break;
        case 0x04:
            loc += rd.fixed(4) * code_align;
            break;
        case 0x05: { // offset_extended
            uint64_t reg = rd.uleb();
            set(reg, col_rule::offset, static_cast<int64_t>(rd.uleb()) * data_align);
            break;
        }
        case 0x06: // restore_extended
            if (!restore(rd.uleb())) {
                return false;
            }
            break;
        case 0x07:
            set(rd.uleb(), col_rule::undefined, 0);
            break;
        case 0x08:
            set(rd.uleb(), col_rule::same, 0);
            break;
        case 0x09: { // register
            uint64_t reg = rd.uleb();
            rd.uleb();
            set(reg, col_rule::other, 0);
            break;
        }
        case 0x0a: // remember_state (the CFA too, as libgcc and DWARF 4 do)
            if (nsaved == depth) {
                return false;
            }
            saved[nsaved++] = row;
            break;
        case 0x0b: // restore_state
            if (nsaved == 0) {
                return false;
            }
            row = saved[--nsaved];
            break;
        case 0x0c: // def_cfa
            row.cfa_reg = rd.uleb();
            row.cfa_off = static_cast<int64_t>(rd.uleb());
            row.cfa_expr = false;
            break;
        case 0x0d: // def_cfa_register
            row.cfa_reg = rd.uleb();
            row.cfa_expr = false;
            break;
        case 0x0e: // def_cfa_offset (the rule's kind stays, as in libgcc)
            row.cfa_off = static_cast<int64_t>(rd.uleb());
            break;
        case 0x0f: { // def_cfa_expression
            uint64_t n = rd.uleb();
            if (n > static_cast<uint64_t>(rd.end - rd.p)) {
                return false;
            }
            rd.p += n;
            row.cfa_expr = true;
            break;
        }
        case 0x10:   // expression
        case 0x16: { // val_expression
            uint64_t reg = rd.uleb();
            uint64_t n = rd.uleb();
            if (n > static_cast<uint64_t>(rd.end - rd.p)) {
                return false;
            }
            rd.p += n;
            set(reg, col_rule::other, 0);
            break;
        }
        case 0x11: { // offset_extended_sf
            uint64_t reg = rd.uleb();
            set(reg, col_rule::offset, rd.sleb() * data_align);
            break;
        }
        case 0x12: // def_cfa_sf
            row.cfa_reg = rd.uleb();
            row.cfa_off = rd.sleb() * data_align;
            row.cfa_expr = false;
            break;
        case 0x13: // def_cfa_offset_sf
            row.cfa_off = rd.sleb() * data_align;
            break;
        case 0x14: { // val_offset
            uint64_t reg = rd.uleb();
            rd.uleb();
            set(reg, col_rule::other, 0);
            break;
        }
        case 0x15: { // val_offset_sf
            uint64_t reg = rd.uleb();
            rd.sleb();
            set(reg, col_rule::other, 0);
            break;
        }
        case 0x2e: // GNU_args_size: for landing pads, not for the CFA
            rd.uleb();
            break;
        case 0x2f: { // GNU_negative_offset_extended
            uint64_t reg = rd.uleb();
            set(reg, col_rule::offset, -static_cast<int64_t>(rd.uleb()) * data_align);
            break;
        }
        default:
            return false;
        }
        if (rd.bad) {
            return false;
        }
    }
    return !rd.bad;
}

// rule_from_fde finds the rule of the return address ra in its FDE (fde, with the bases that
// _Unwind_Find_FDE gave), as libgcc's uw_frame_state_for and uw_update_context would use it.
bool rule_from_fde(const uint8_t* fde, const dwarf_eh_bases& b, uintptr_t ra, frame_rule& out) {
    uint32_t len, cie_off;
    std::memcpy(&len, fde, 4);
    if (len == 0 || len == 0xffffffffu) {
        return false;
    }
    std::memcpy(&cie_off, fde + 4, 4);
    const uint8_t* fde_end = fde + 4 + len;
    const uint8_t* cie = fde + 4 - cie_off;
    uint32_t cie_len;
    std::memcpy(&cie_len, cie, 4);
    if (cie_len == 0 || cie_len == 0xffffffffu) {
        return false;
    }
    cfi_reader c{cie + 8, cie + 4 + cie_len};
    uint8_t version = c.u8();
    const char* aug = reinterpret_cast<const char*>(c.p);
    std::size_t aug_len = strnlen(aug, static_cast<std::size_t>(c.end - c.p));
    c.p += aug_len + 1;
    if (version != 1 && version != 3) {
        return false;
    }
    if (aug_len >= 2 && aug[0] == 'e' && aug[1] == 'h') {
        return false; // (gcc 2's "eh": not on x86-64)
    }
    uint64_t code_align = c.uleb();
    int64_t data_align = c.sleb();
    uint64_t ra_reg = version == 1 ? c.u8() : c.uleb();
    if (c.bad || ra_reg != reg_ra) {
        return false;
    }
    uint8_t fde_enc = 0; // absptr
    bool saw_z = false;
    const uint8_t* insns = c.p;
    for (std::size_t i = 0; i < aug_len; i++) {
        switch (aug[i]) {
        case 'z': {
            if (i != 0) {
                return false;
            }
            saw_z = true;
            uint64_t n = c.uleb();
            if (c.bad || n > static_cast<uint64_t>(c.end - c.p)) {
                return false;
            }
            insns = c.p + n;
            break;
        }
        case 'R':
            fde_enc = c.u8();
            break;
        case 'L':
            c.u8();
            break;
        case 'P':
            c.encoded(static_cast<uint8_t>(c.u8() & 0x7f), b);
            break;
        default:
            return false; // 'S' (a signal frame) and the rest: for the unwinder
        }
        if (c.bad) {
            return false;
        }
    }
    if (!saw_z) {
        insns = c.p;
        if (aug_len != 0) {
            return false;
        }
    }
    auto func = reinterpret_cast<uintptr_t>(b.func);
    cfi_row row;
    row.ra.how = col_rule::undefined;
    cfi_reader ci{insns, cie + 4 + cie_len};
    uint64_t loc = func;
    if (!run(ci, row, nullptr, loc, ra, code_align, data_align, fde_enc, b)) {
        return false;
    }
    cfi_row init = row;
    // The FDE: length, CIE pointer, pc_begin and pc_range (fixed size), then the augmentation.
    int ptr_size;
    switch (fde_enc & 0x0f) {
    case 0x00:
    case 0x04:
    case 0x0c:
        ptr_size = 8;
        break;
    case 0x02:
    case 0x0a:
        ptr_size = 2;
        break;
    case 0x03:
    case 0x0b:
        ptr_size = 4;
        break;
    default:
        return false;
    }
    cfi_reader f{fde + 8 + 2 * ptr_size, fde_end};
    if (saw_z) {
        uint64_t n = f.uleb();
        if (f.bad || n > static_cast<uint64_t>(f.end - f.p)) {
            return false;
        }
        f.p += n;
    }
    if (!run(f, row, &init, loc, ra, code_align, data_align, fde_enc, b)) {
        return false;
    }
    if (row.cfa_expr || (row.cfa_reg != reg_rsp && row.cfa_reg != reg_rbp) ||
        row.rsp.how != col_rule::same) {
        return false;
    }
    out = frame_rule{};
    out.cfa_rbp = row.cfa_reg == reg_rbp;
    out.cfa_off = row.cfa_off;
    switch (row.ra.how) {
    case col_rule::undefined:
        out.outermost = true;
        break;
    case col_rule::offset:
        out.ra_off = row.ra.off;
        break;
    default:
        return false; // (the unwinder's "same value" of the return address: not on x86-64)
    }
    switch (row.rbp.how) {
    case col_rule::same:
    case col_rule::undefined: // (libgcc treats it as the same value)
        break;
    case col_rule::offset:
        out.rbp_saved = true;
        out.rbp_off = row.rbp.off;
        break;
    default:
        return false;
    }
    return true;
}

// The kept rules: an append-only open-addressed table of return addresses, read without a lock.
// A slot's key is set once (a compare-and-swap from 0), then its value; a reader that sees the
// key and not yet the value finds the rule in the FDE itself.
constexpr std::size_t rule_slots = 4096;
std::atomic<uintptr_t> rule_keys[rule_slots];
std::atomic<uint64_t> rule_vals[rule_slots];

std::size_t rule_hash(uintptr_t pc) {
    return static_cast<std::size_t>((pc * 0x9e3779b97f4a7c15ull) >> 52) & (rule_slots - 1);
}

// A rule in 64 bits: bit 0 set (a value), 1 cfa_rbp, 2 outermost, 3 rbp_saved, 8-15 ra_off,
// 16-31 rbp_off, 32-63 cfa_off. false when the offsets don't fit.
bool pack(const frame_rule& r, uint64_t& v) {
    if (r.cfa_off != static_cast<int32_t>(r.cfa_off) ||
        r.rbp_off != static_cast<int16_t>(r.rbp_off) || r.ra_off != static_cast<int8_t>(r.ra_off)) {
        return false;
    }
    v = 1 | (uint64_t(r.cfa_rbp) << 1) | (uint64_t(r.outermost) << 2) |
        (uint64_t(r.rbp_saved) << 3) | (uint64_t(static_cast<uint8_t>(r.ra_off)) << 8) |
        (uint64_t(static_cast<uint16_t>(r.rbp_off)) << 16) |
        (uint64_t(static_cast<uint32_t>(r.cfa_off)) << 32);
    return true;
}

frame_rule unpack(uint64_t v) {
    frame_rule r;
    r.cfa_rbp = (v >> 1) & 1;
    r.outermost = (v >> 2) & 1;
    r.rbp_saved = (v >> 3) & 1;
    r.ra_off = static_cast<int8_t>(v >> 8);
    r.rbp_off = static_cast<int16_t>(v >> 16);
    r.cfa_off = static_cast<int32_t>(v >> 32);
    return r;
}

// exe_code reports whether pc is in the code of the program itself.
bool exe_code(uintptr_t pc) {
    static const detail::exe_image* img = [] {
        auto* i = new detail::exe_image; // never freed: the walk may run at exit
        dl_iterate_phdr(main_object, i);
        return i;
    }();
    for (const auto& [lo, hi] : img->code) {
        if (pc >= lo && pc < hi) {
            return true;
        }
    }
    return false;
}

// rule_for finds the rule of the return address pc: 1 found, 0 no FDE, -1 a frame the fast walk
// can't step over.
int rule_for(uintptr_t pc, frame_rule& r) {
    std::size_t h = rule_hash(pc);
    for (std::size_t i = 0; i < 16; i++) {
        std::size_t k = (h + i) & (rule_slots - 1);
        uintptr_t key = rule_keys[k].load(std::memory_order_acquire);
        if (key == pc) {
            uint64_t v = rule_vals[k].load(std::memory_order_acquire);
            if (v != 0) {
                r = unpack(v);
                return 1;
            }
            break;
        }
        if (key == 0) {
            break;
        }
    }
    dwarf_eh_bases b{};
    const void* fde = _Unwind_Find_FDE(reinterpret_cast<void*>(pc - 1), &b);
    if (fde == nullptr) {
        return 0;
    }
    if (!rule_from_fde(static_cast<const uint8_t*>(fde), b, pc, r)) {
        return -1;
    }
    uint64_t v;
    if (!exe_code(pc) || !pack(r, v)) {
        return 1;
    }
    for (std::size_t i = 0; i < 16; i++) {
        std::size_t k = (h + i) & (rule_slots - 1);
        uintptr_t key = 0;
        if (rule_keys[k].compare_exchange_strong(key, pc, std::memory_order_acq_rel)) {
            rule_vals[k].store(v, std::memory_order_release);
            break;
        }
        if (key == pc) {
            break;
        }
    }
    return 1;
}

// fast_walk walks the stack as _Unwind_Backtrace(unwind_step) would, from the frame of its caller
// (walk); false when it met a frame that it can't step over (w is then half done). Not inlined:
// its own frame is the start, and has a frame pointer (__builtin_frame_address).
__attribute__((noinline)) bool fast_walk(unwind_walk& w) noexcept {
    auto* fp = static_cast<uintptr_t*>(__builtin_frame_address(0));
    // walk's registers at the call: the return address, the CFA of this frame, the saved rbp.
    auto pc = reinterpret_cast<uintptr_t>(__builtin_return_address(0));
    uintptr_t sp = reinterpret_cast<uintptr_t>(fp) + 16;
    uintptr_t rbp = fp[0];
    for (;;) {
        frame_rule r;
        int found = pc == 0 ? 0 : rule_for(pc, r);
        if (found < 0 || (found == 0 && pc != 0)) {
            // No FDE: libgcc may still step over the frame (x86_64_fallback_frame_state knows the
            // signal trampoline by its code) or end the stack there; the unwinder decides.
            return false;
        }
        if (record(&w, pc, sp) != _URC_NO_REASON || found == 0) {
            return true;
        }
        uintptr_t cfa = (r.cfa_rbp ? rbp : sp) + static_cast<uintptr_t>(r.cfa_off);
        if (cfa <= sp) {
            return false; // (no frame of a sane stack: for the unwinder)
        }
        if (r.outermost) {
            // The unwinder's last frame: "pc 0", with the CFA of the outermost frame (libgcc's
            // uw_update_context computes it, then sets the return address to 0).
            pc = 0;
            sp = cfa;
            continue;
        }
        uintptr_t next_pc, next_rbp = rbp;
        std::memcpy(&next_pc, reinterpret_cast<const void*>(cfa + static_cast<uintptr_t>(r.ra_off)),
                    8);
        if (r.rbp_saved) {
            std::memcpy(&next_rbp,
                        reinterpret_cast<const void*>(cfa + static_cast<uintptr_t>(r.rbp_off)), 8);
        }
        pc = next_pc;
        sp = cfa;
        rbp = next_rbp;
    }
}

#endif // GOCPP_FAST_WALK

// walk records frames as w says. Not inlined: the frame of this function is the first one that
// the unwinder shows, as glibc's backtrace shows the function that called it.
__attribute__((noinline)) void walk(unwind_walk& w) noexcept {
    if (w.max > 0) {
#if GOCPP_FAST_WALK
        int skip = w.skip;
        if (fast_walk(w)) {
            if (w.n > 0 && w.pcs[w.n - 1] == 0) {
                w.n--;
            }
            return;
        }
        // The fast walk met a frame it can't step over: the unwinder walks the stack again.
        w.n = 0;
        w.reached = false;
        w.last_pc = 0;
        w.last_cfa = 0;
        w.skip = skip;
#endif
        _Unwind_Backtrace(&unwind_step, &w);
        // The end of the stack is a frame whose address is 0: not a frame (glibc's backtrace
        // leaves it out too).
        if (w.n > 0 && w.pcs[w.n - 1] == 0) {
            w.n--;
        }
    }
}

} // namespace

int stack_until(uintptr_t* pcs, int max, const void* bound, bool* found) noexcept {
    unwind_walk w{pcs, max, 0, reinterpret_cast<uintptr_t>(bound), true, false, 0, 0};
    walk(w);
    *found = w.reached;
    return w.n;
}

int stack_above(uintptr_t* pcs, int max, const void* bound) noexcept {
    unwind_walk w{pcs, max, 0, reinterpret_cast<uintptr_t>(bound), false, false, 0, 0};
    walk(w);
    return w.reached || max <= 0 ? w.n : -1;
}

#if GOCPP_FAST_WALK
// Not inlined, as backtrace's version isn't: the frames are the same, this function's first.
__attribute__((noinline)) int stack(uintptr_t* pcs, int max) noexcept {
    // What glibc's backtrace gives (the unwinder's walk without backtrace's own frame, from the
    // function that called it), with the fast walk: walk's frame is left out, the first one is
    // this function's.
    unwind_walk w{pcs, max, 0, 0, false, false, 0, 0};
    w.skip = 1;
    walk(w);
    return w.n;
}
#endif

#ifdef GOCPP_RUNTIME_TEST
namespace {

// check_walk walks as walk() does, with the fast walk (fast) or the unwinder: from the same frame.
__attribute__((noinline)) bool check_walk(unwind_walk& w, bool fast) noexcept {
#if GOCPP_FAST_WALK
    if (fast) {
        bool ok = fast_walk(w);
        if (w.n > 0 && w.pcs[w.n - 1] == 0) {
            w.n--;
        }
        return ok;
    }
#else
    if (fast) {
        return false;
    }
#endif
    _Unwind_Backtrace(&unwind_step, &w);
    if (w.n > 0 && w.pcs[w.n - 1] == 0) {
        w.n--;
    }
    return true;
}

} // namespace

int walk_check(const void* bound) noexcept {
    constexpr int max = 512;
    for (bool stop : {true, false}) {
        uintptr_t pcs[2][max], cfas[2][max];
        unwind_walk w[2];
        for (int k = 0; k < 2; k++) {
            w[k] =
                unwind_walk{pcs[k], max, 0, reinterpret_cast<uintptr_t>(bound), stop, false, 0, 0};
            w[k].cfas = cfas[k];
        }
        if (!check_walk(w[0], true)) {
            return 0;
        }
        check_walk(w[1], false);
        // The first two frames are check_walk's and this function's, at two calls: the same CFAs,
        // other return addresses.
        if (w[0].n != w[1].n || w[0].reached != w[1].reached) {
            return -1;
        }
        for (int i = 0; i < w[0].n; i++) {
            if (cfas[0][i] != cfas[1][i] || (i > 1 && pcs[0][i] != pcs[1][i])) {
                return -1;
            }
        }
    }
    return 1;
}
#endif // GOCPP_RUNTIME_TEST

namespace {

// main_object finds the bias and the code ranges of the program: dl_iterate_phdr visits the
// program first.
int main_object(dl_phdr_info* info, size_t, void* data) {
    auto* img = static_cast<detail::exe_image*>(data);
    img->bias = info->dlpi_addr;
    for (int i = 0; i < info->dlpi_phnum; i++) {
        const ElfW(Phdr) & ph = info->dlpi_phdr[i];
        if (ph.p_type == PT_LOAD && (ph.p_flags & PF_X) != 0) {
            uintptr_t lo = info->dlpi_addr + ph.p_vaddr;
            img->code.emplace_back(lo, lo + ph.p_memsz);
        }
    }
    return 1; // stop after the program
}

struct file_closer {
    void operator()(std::FILE* f) const noexcept { std::fclose(f); }
};
using file_ptr = std::unique_ptr<std::FILE, file_closer>;

// read_at reads n bytes at offset off of f, whose size is size, into out, and reports whether
// it could. A header that points past the end of the file is refused before any allocation.
bool read_at(std::FILE* f, uint64_t size, uint64_t off, uint64_t n, std::string& out) {
    if (off > size || n > size - off) {
        return false;
    }
    out.resize(static_cast<std::size_t>(n));
    if (n == 0) {
        return true;
    }
    if (fseeko(f, static_cast<off_t>(off), SEEK_SET) != 0) {
        return false;
    }
    return std::fread(out.data(), 1, out.size(), f) == out.size();
}

} // namespace

bool read_exe(detail::exe_image& img) {
    img = detail::exe_image{};
    dl_iterate_phdr(main_object, &img);

    // /proc/self/exe is the program's file, also when it was moved or removed since it started.
    file_ptr f(std::fopen("/proc/self/exe", "rb"));
    if (!f) {
        return false;
    }
    if (fseeko(f.get(), 0, SEEK_END) != 0) {
        return false;
    }
    off_t end = ftello(f.get());
    if (end < 0) {
        return false;
    }
    auto size = static_cast<uint64_t>(end);
    std::string buf;
    if (!read_at(f.get(), size, 0, sizeof(Elf64_Ehdr), buf)) {
        return false;
    }
    Elf64_Ehdr eh;
    std::memcpy(&eh, buf.data(), sizeof eh);
    if (std::memcmp(eh.e_ident, ELFMAG, SELFMAG) != 0 || eh.e_ident[EI_CLASS] != ELFCLASS64 ||
        eh.e_ident[EI_DATA] != ELFDATA2LSB || eh.e_shentsize != sizeof(Elf64_Shdr) ||
        eh.e_shnum == 0 || eh.e_shstrndx >= eh.e_shnum) {
        return false; // only 64-bit little-endian ELF (x86-64, arm64) for now
    }
    if (!read_at(f.get(), size, eh.e_shoff, uint64_t(eh.e_shnum) * sizeof(Elf64_Shdr), buf)) {
        return false;
    }
    std::vector<Elf64_Shdr> sh(eh.e_shnum);
    std::memcpy(sh.data(), buf.data(), buf.size());
    std::string names;
    if (!read_at(f.get(), size, sh[eh.e_shstrndx].sh_offset, sh[eh.e_shstrndx].sh_size, names)) {
        return false;
    }
    auto name_of = [&](const Elf64_Shdr& s) -> std::string {
        if (s.sh_name >= names.size()) {
            return "";
        }
        return std::string(names.c_str() + s.sh_name);
    };

    const Elf64_Shdr* symtab = nullptr;
    for (const Elf64_Shdr& s : sh) {
        std::string name = name_of(s);
        if (s.sh_type == SHT_SYMTAB) {
            symtab = &s;
        }
        std::string* dst = img.dwarf_section(name);
        // A compressed section would need zlib or zstd: the trace goes on without lines.
        if (dst != nullptr && s.sh_type != SHT_NOBITS && (s.sh_flags & SHF_COMPRESSED) == 0) {
            if (!read_at(f.get(), size, s.sh_offset, s.sh_size, *dst)) {
                dst->clear();
            }
        }
    }

    // Only .symtab: a stripped program keeps .dynsym, which has only the exported symbols (the
    // C++ library's), so every frame would be misnamed or left out; without .symtab the trace
    // prints the addresses instead.
    const Elf64_Shdr* syms = symtab;
    if (syms != nullptr && syms->sh_link < sh.size() && syms->sh_entsize == sizeof(Elf64_Sym)) {
        std::string data, strs;
        if (read_at(f.get(), size, syms->sh_offset, syms->sh_size, data) &&
            read_at(f.get(), size, sh[syms->sh_link].sh_offset, sh[syms->sh_link].sh_size, strs)) {
            std::size_t n = data.size() / sizeof(Elf64_Sym);
            for (std::size_t i = 0; i < n; i++) {
                Elf64_Sym sym;
                std::memcpy(&sym, data.data() + i * sizeof sym, sizeof sym);
                int type = ELF64_ST_TYPE(sym.st_info);
                if ((type != STT_FUNC && type != STT_GNU_IFUNC) || sym.st_shndx == SHN_UNDEF ||
                    sym.st_value == 0 || sym.st_name >= strs.size()) {
                    continue;
                }
                img.funcs.push_back({static_cast<uintptr_t>(sym.st_value),
                                     static_cast<uintptr_t>(sym.st_size),
                                     std::string(strs.c_str() + sym.st_name)});
            }
        }
    }
    std::sort(
        img.funcs.begin(), img.funcs.end(),
        [](const detail::exe_symbol& a, const detail::exe_symbol& b) { return a.addr < b.addr; });
    return true;
}

std::string demangle(const std::string& name) {
    int status = 0;
    std::unique_ptr<char, decltype(&std::free)> s(
        abi::__cxa_demangle(name.c_str(), nullptr, nullptr, &status), &std::free);
    if (status != 0 || !s) {
        return "";
    }
    return std::string(s.get());
}

const std::type_info* current_exception_type() noexcept {
    return abi::__cxa_current_exception_type();
}

} // namespace gp::os
