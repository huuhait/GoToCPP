// The Windows part of the stack trace of a panic (gocpp/os.hpp), a first piece of the Windows
// OS layer, which Phase 14 completes (the rest of gocpp/os.hpp is not here yet, so a Windows
// program doesn't link before then). The addresses come from CaptureStackBackTrace and the
// names from the C++ ABI's demangler, as on Linux; read_exe can't read the program's PE file
// yet, so the trace has no frames. Phase 14: read the symbol table and the DWARF sections that
// MinGW writes into the PE file (ADR-021), from GetModuleFileNameW and the module's base.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include <gocpp/trace.hpp>

#include <cstdlib>
#include <memory>

#include <cxxabi.h>
#include <windows.h>

namespace gp::os {

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

bool read_exe(detail::exe_image& img) {
    img = detail::exe_image{};
    return false; // Phase 14
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
