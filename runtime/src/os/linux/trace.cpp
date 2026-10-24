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

namespace gp::os {

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
};

_Unwind_Reason_Code unwind_step(_Unwind_Context* ctx, void* data) {
    auto* w = static_cast<unwind_walk*>(data);
    auto pc = static_cast<uintptr_t>(_Unwind_GetIP(ctx));
    auto cfa = static_cast<uintptr_t>(_Unwind_GetCFA(ctx));
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
    w->pcs[w->n++] = pc;
    return w->n == w->max ? _URC_END_OF_STACK : _URC_NO_REASON;
}

// walk records frames as w says. Not inlined: the frame of this function is the first one that
// the unwinder shows, as glibc's backtrace shows the function that called it.
__attribute__((noinline)) void walk(unwind_walk& w) noexcept {
    if (w.max > 0) {
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
