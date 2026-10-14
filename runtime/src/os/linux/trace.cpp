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
        std::string* dst = name == ".debug_line"       ? &img.debug_line
                           : name == ".debug_line_str" ? &img.debug_line_str
                           : name == ".debug_str"      ? &img.debug_str
                                                       : nullptr;
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

} // namespace gp::os
