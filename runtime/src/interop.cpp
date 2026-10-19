// The parts of the interop helpers (gocpp/interop.hpp) that are not templates: cgo's
// C.CString, C.GoString, C.GoStringN, C.GoBytes and C.free, each a mirror of Go 1.25's code
// (cmd/cgo/out.go: _Cfunc_CString and _cgoPREFIX_Cfunc_CString; runtime/string.go: gostring,
// gostringn, gobytes). Standard C++ only.
#include <gocpp/rt.hpp>
#include <gocpp/wstr.hpp>

#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <string_view>

namespace gp {

namespace {

// The identity of a runtime error (panic.hpp), as rt.cpp gives its own.
constexpr panic_identity runtime_id{panic_identity::gp_runtime, 0, 0};

} // namespace

namespace detail {

char* c_string(const string& s) {
    // _cgoPREFIX_Cfunc_CString: malloc(len + 1), the bytes, a NUL. (Go's check that len + 1
    // doesn't overflow can't fail here: a length is at most max_alloc.)
    auto n = static_cast<std::size_t>(len(s));
    auto* p = static_cast<char*>(std::malloc(n + 1));
    if (p == nullptr) {
        fatal("runtime: C malloc failed");
    }
    if (n != 0) {
        std::memcpy(p, s.gp_data(), n);
    }
    p[n] = '\0';
    return p;
}

string go_string(const char* p) {
    // gostring: findnull gives 0 for a null p, and an empty result is "".
    if (p == nullptr) {
        return string();
    }
    return string::gp_copy(std::string_view(p));
}

string go_string_n(const char* p, int64_t n) {
    // gostringn: 0 gives "" without reading p. Otherwise rawstring(n) allocates n bytes, and
    // mallocgc of a negative n (a huge uintptr) or of more than maxAlloc is the fatal error
    // "out of memory". Then memmove reads p: a null p is a nil dereference.
    if (n == 0) {
        return string();
    }
    if (n < 0 || n > max_alloc) {
        fatal("out of memory");
    }
#ifndef GOCPP_NOCHECKS
    if (p == nullptr) {
        panic_nil(); // a nil check: off with -nochecks (checks.hpp), like *p
    }
#endif
    return string::gp_copy(std::string_view(p, static_cast<std::size_t>(n)));
}

bool wide_n_ok(int64_t n, [[maybe_unused]] const void* p) {
    if (n == 0) {
        return false;
    }
    if (n < 0 || n > max_alloc / 2) {
        fatal("out of memory");
    }
#ifndef GOCPP_NOCHECKS
    if (p == nullptr) {
        panic_nil();
    }
#endif
    return true;
}

slice<uint8_t> go_bytes(const void* p, int64_t n) {
    // gobytes: 0 gives make([]byte, 0), not nil; a negative n or one above maxAlloc panics;
    // then memmove reads p.
    if (n == 0) {
        return make_slice<uint8_t>(0);
    }
    if (n < 0 || n > max_alloc) {
        throw panic_error{"runtime error: gobytes: length out of range"_gs, runtime_id};
    }
#ifndef GOCPP_NOCHECKS
    if (p == nullptr) {
        panic_nil(); // a nil check: off with -nochecks (checks.hpp), like *p
    }
#endif
    slice<uint8_t> b = make_slice<uint8_t>(n);
    std::memcpy(b.gp_data(), p, static_cast<std::size_t>(n));
    return b;
}

} // namespace detail

namespace cgo {

void free(void* p) noexcept {
    std::free(p);
}

} // namespace cgo

} // namespace gp
