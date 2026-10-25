// The Windows part of the stack trace of a panic (gocpp/os.hpp): the return addresses
// (RtlCaptureStackBackTrace), the program's own PE file (the COFF symbol table and the DWARF
// sections that MinGW writes into it, read through GetModuleFileNameW) and C++ demangling.
// runtime/src/trace.cpp does the rest in standard C++, so a Windows trace reads like a Linux one.
//
// MinGW's linker keeps the symbol table of a program that is not stripped, with the functions
// (type 0x20) and their addresses but not their sizes, and the DWARF sections under names that
// are longer than a PE section name can hold (they are "/4", "/19": offsets in the string table
// after the symbols). addr2line of the dev shell reads the same file; the runtime has no tool,
// so it reads them itself. A program linked with -s has neither: the trace prints addresses.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include <gocpp/trace.hpp>
#include "windows.hpp"

#include <algorithm>
#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <memory>
#include <string>
#include <vector>

#include <cxxabi.h>
#include <windows.h>

namespace gp::os {

#if defined(__x86_64__) || defined(_M_X64)

namespace {

// walk unwinds the calling thread's stack with the unwind tables of the PE file (RtlVirtualUnwind,
// as RtlCaptureStackBackTrace does, which takes only about 300 frames in all with its chunks of
// skip and count). mode 0 records the return addresses of the whole stack, innermost first, up to
// max; mode 1 (stack_until) stops after the frame that holds the address bound (the frame whose
// CFA, the stack pointer after it has returned, is above bound: the frames inside it have a CFA at
// or below its stack pointer); mode 2 (stack_above) records only the frames outside that frame.
// The first address is in walk itself, as the first address of the Linux walk is in the function
// that calls the unwinder. *reached tells whether the frame of bound was seen (modes 1 and 2).
__attribute__((noinline)) int walk(uintptr_t* pcs, int max, uintptr_t bound, int mode,
                                   bool* reached) noexcept {
    *reached = false;
    if (max <= 0) {
        return 0;
    }
    CONTEXT ctx;
    RtlCaptureContext(&ctx);
    int n = 0;
    for (std::size_t depth = 0; depth < (std::size_t(1) << 22); depth++) {
        auto pc = static_cast<uintptr_t>(ctx.Rip);
        if (pc == 0) {
            break;
        }
        if (mode != 2 || *reached) {
            pcs[n++] = pc;
        }
        DWORD64 image = 0;
        PRUNTIME_FUNCTION fe = RtlLookupFunctionEntry(ctx.Rip, &image, nullptr);
        DWORD64 before = ctx.Rsp;
        if (fe == nullptr) {
            // A leaf function without unwind data: the return address is on top of the stack.
            ctx.Rip = *reinterpret_cast<DWORD64*>(ctx.Rsp);
            ctx.Rsp += 8;
        } else {
            PVOID handler_data = nullptr;
            DWORD64 establisher = 0;
            RtlVirtualUnwind(UNW_FLAG_NHANDLER, image, ctx.Rip, fe, &ctx, &handler_data,
                             &establisher, nullptr);
        }
        if (ctx.Rsp <= before) {
            break; // no progress
        }
        if (mode != 0 && !*reached && bound < ctx.Rsp) {
            *reached = true;
            if (mode == 1) {
                break;
            }
        }
        if (n == max) {
            break;
        }
    }
    return n;
}

} // namespace

int stack(uintptr_t* pcs, int max) noexcept {
    bool reached;
    return walk(pcs, max, 0, 0, &reached);
}

int stack_until(uintptr_t* pcs, int max, const void* bound, bool* found) noexcept {
    int n = walk(pcs, max, reinterpret_cast<uintptr_t>(bound), 1, found);
    return n;
}

int stack_above(uintptr_t* pcs, int max, const void* bound) noexcept {
    bool reached;
    int n = walk(pcs, max, reinterpret_cast<uintptr_t>(bound), 2, &reached);
    return reached || max <= 0 ? n : -1;
}

#else

int stack(uintptr_t* pcs, int max) noexcept {
    if (max <= 0) {
        return 0;
    }
    // RtlCaptureStackBackTrace takes fewer than 63 frames a call on old Windows versions:
    // chunks, each skipping the frames that the ones before took.
    constexpr int chunk = 62;
    void* buf[chunk];
    int n = 0;
    while (n < max) {
        int want = max - n < chunk ? max - n : chunk;
        int got =
            RtlCaptureStackBackTrace(static_cast<DWORD>(n), static_cast<DWORD>(want), buf, nullptr);
        for (int i = 0; i < got; i++) {
            pcs[n + i] = reinterpret_cast<uintptr_t>(buf[i]);
        }
        n += got;
        if (got < want) {
            break;
        }
    }
    return n;
}

int stack_until(uintptr_t* pcs, int max, const void*, bool* found) noexcept {
    // (No unwinder loop for this CPU: the whole stack, at once.)
    *found = false;
    return stack(pcs, max);
}

int stack_above(uintptr_t*, int, const void*) noexcept {
    return -1;
}

#endif

#ifdef GOCPP_RUNTIME_TEST
int walk_check(const void*) noexcept {
    return 0; // (the walks are the system's: there is no fast walk to check)
}
#endif

namespace {

// A file handle that closes itself.
struct handle_closer {
    void operator()(HANDLE h) const noexcept {
        if (h != nullptr && h != INVALID_HANDLE_VALUE) {
            CloseHandle(h);
        }
    }
};
using file_handle = std::unique_ptr<void, handle_closer>;

// read_at reads n bytes at offset off of the file, whose size is size, into out, and reports
// whether it could. A header that points past the end of the file is refused before any
// allocation.
bool read_at(HANDLE f, uint64_t size, uint64_t off, uint64_t n, std::string& out) {
    if (off > size || n > size - off) {
        return false;
    }
    out.resize(static_cast<std::size_t>(n));
    std::size_t done = 0;
    while (done < out.size()) {
        OVERLAPPED ov{};
        uint64_t at = off + done;
        ov.Offset = static_cast<DWORD>(at);
        ov.OffsetHigh = static_cast<DWORD>(at >> 32);
        DWORD want =
            static_cast<DWORD>(std::min<std::size_t>(out.size() - done, std::size_t(1) << 28));
        DWORD got = 0;
        if (!ReadFile(f, out.data() + done, want, &got, &ov) || got == 0) {
            return false;
        }
        done += got;
    }
    return true;
}

template <typename T>
T load(const std::string& s, std::size_t off) {
    T v{};
    if (off <= s.size() && sizeof(T) <= s.size() - off) {
        std::memcpy(&v, s.data() + off, sizeof(T));
    }
    return v;
}

// The path of the program's file (the module of the process), in UTF-16.
std::wstring exe_path() {
    std::wstring path(512, L'\0');
    for (;;) {
        DWORD n = GetModuleFileNameW(nullptr, path.data(), static_cast<DWORD>(path.size()));
        if (n == 0) {
            return {};
        }
        if (n < path.size()) {
            path.resize(n);
            return path;
        }
        path.resize(path.size() * 2);
    }
}

} // namespace

bool read_exe(detail::exe_image& img) {
    img = detail::exe_image{};
    std::wstring path = exe_path();
    if (path.empty()) {
        return false;
    }
    file_handle f(CreateFileW(path.c_str(), GENERIC_READ,
                              FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE, nullptr,
                              OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, nullptr));
    if (f.get() == INVALID_HANDLE_VALUE) {
        f.release();
        return false;
    }
    LARGE_INTEGER sz{};
    if (!GetFileSizeEx(f.get(), &sz) || sz.QuadPart < 0) {
        return false;
    }
    auto size = static_cast<uint64_t>(sz.QuadPart);
    std::string head;
    if (!read_at(f.get(), size, 0, 64, head) || load<uint16_t>(head, 0) != 0x5A4D) { // "MZ"
        return false;
    }
    uint32_t pe = load<uint32_t>(head, 0x3C);
    // PE\0\0, the COFF header (20 bytes) and the optional header up to the image base.
    if (!read_at(f.get(), size, pe, 24 + 32, head) || load<uint32_t>(head, 0) != 0x00004550) {
        return false;
    }
    uint16_t machine = load<uint16_t>(head, 4);
    uint16_t nsec = load<uint16_t>(head, 6);
    uint32_t symptr = load<uint32_t>(head, 12);
    uint32_t nsym = load<uint32_t>(head, 16);
    uint16_t optsize = load<uint16_t>(head, 20);
    if (machine != 0x8664 || load<uint16_t>(head, 24) != 0x20B || optsize < 32) { // x64, PE32+
        return false;
    }
    uint64_t image_base = load<uint64_t>(head, 24 + 24);
    img.bias =
        reinterpret_cast<uintptr_t>(GetModuleHandleW(nullptr)) - static_cast<uintptr_t>(image_base);

    std::string secs;
    if (!read_at(f.get(), size, uint64_t(pe) + 24 + optsize, uint64_t(nsec) * 40, secs)) {
        return false;
    }
    // The string table follows the symbols: its first four bytes are its size.
    std::string strtab;
    if (symptr != 0 && nsym != 0) {
        uint64_t strat = uint64_t(symptr) + uint64_t(nsym) * 18;
        std::string len;
        if (read_at(f.get(), size, strat, 4, len)) {
            uint32_t n = load<uint32_t>(len, 0);
            if (n < 4 || !read_at(f.get(), size, strat, n, strtab)) {
                strtab.clear();
            }
        }
    }
    struct section {
        uint32_t va, vsize, raw, rawsize, flags;
    };
    std::vector<section> sections;
    for (uint16_t i = 0; i < nsec; i++) {
        std::size_t o = std::size_t(i) * 40;
        section sec{load<uint32_t>(secs, o + 12), load<uint32_t>(secs, o + 8),
                    load<uint32_t>(secs, o + 20), load<uint32_t>(secs, o + 16),
                    load<uint32_t>(secs, o + 36)};
        sections.push_back(sec);
        // The name: up to 8 bytes, or "/N" for the offset N (decimal) in the string table.
        std::string name(secs.data() + o, strnlen(secs.data() + o, 8));
        if (name.size() >= 2 && name[0] == '/') {
            std::size_t off = std::strtoul(name.c_str() + 1, nullptr, 10);
            name = off < strtab.size() ? std::string(strtab.c_str() + off) : std::string();
        }
        if ((sec.flags & (0x00000020 | 0x20000000)) != 0) { // contains code, or executable
            uintptr_t lo = static_cast<uintptr_t>(image_base) + sec.va + img.bias;
            img.code.emplace_back(lo, lo + std::max(sec.vsize, 1u));
        }
        std::string* dst = img.dwarf_section(name);
        if (dst != nullptr && sec.rawsize != 0) {
            // The raw data is padded to the file alignment; the size in memory is the section's.
            uint32_t n = sec.vsize != 0 && sec.vsize < sec.rawsize ? sec.vsize : sec.rawsize;
            if (!read_at(f.get(), size, sec.raw, n, *dst)) {
                dst->clear();
            }
        }
    }

    // The symbol table: 18 bytes each, with auxiliary records after some. The functions of the
    // program are the symbols of type 0x20 that are in a section.
    if (symptr != 0 && nsym != 0) {
        std::string syms;
        if (read_at(f.get(), size, symptr, uint64_t(nsym) * 18, syms)) {
            for (uint32_t i = 0; i < nsym;) {
                std::size_t o = std::size_t(i) * 18;
                auto aux = static_cast<uint32_t>(static_cast<unsigned char>(syms[o + 17]));
                uint32_t value = load<uint32_t>(syms, o + 8);
                auto secnum = load<int16_t>(syms, o + 12);
                uint16_t type = load<uint16_t>(syms, o + 14);
                auto cls = static_cast<unsigned char>(syms[o + 16]);
                if (type == 0x20 && secnum > 0 &&
                    static_cast<std::size_t>(secnum) <= sections.size() && (cls == 2 || cls == 3)) {
                    std::string name;
                    if (load<uint32_t>(syms, o) == 0) { // the name is in the string table
                        uint32_t off = load<uint32_t>(syms, o + 4);
                        if (off < strtab.size()) {
                            name = strtab.c_str() + off;
                        }
                    } else {
                        name.assign(syms.data() + o, strnlen(syms.data() + o, 8));
                    }
                    if (!name.empty()) {
                        img.funcs.push_back({static_cast<uintptr_t>(image_base) +
                                                 sections[static_cast<std::size_t>(secnum) - 1].va +
                                                 value,
                                             0, std::move(name)});
                    }
                }
                i += 1 + aux;
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
