// gocpp/hresult.hpp: the result of a C/C++ function that returns an HRESULT, as a Go error
// (typed interop, ADR-137).
//
// A call of a function or method whose declared result type is the typedef HRESULT (the COM and
// Win32 convention: a signed 32-bit code whose sign bit is the failure bit) is lowered as
// gp::hresult_error(call): the nil error for every code that SUCCEEDED() accepts (S_OK, S_FALSE
// and the other positive success codes such as DXGI_STATUS_OCCLUDED), else an error value that
// keeps the code. The success path allocates nothing and is one comparison: the nil error is a
// null interface.
//
// The failure's Go type is runtime.HResultError (like runtime.CppException, a type of the
// runtime: a program cannot name it, it asks for the code through an interface):
//
//     Error() string   "HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)", or "HRESULT 0x80041234"
//                      for a code the runtime does not know the name of
//     HResult() int32  the code
//
// so `errors.As(err, &h)` with `var h interface{ HResult() int32 }` finds it, and two errors of
// the same code are equal (`err == err2`, errors.Is).
#pragma once

#include <gocpp/error.hpp>
#include <gocpp/hash.hpp>
#include <gocpp/string.hpp>

#include <cstdint>
#include <type_traits>

namespace gp {

// The value of a failed HRESULT; its descriptor is runtime.HResultError (src/hresult.cpp).
struct HResultError {
    int32_t gp_code;
    friend bool operator==(const HResultError&, const HResultError&) = default;
    // (the error is a map key, map[error]int: the hash of the code, as the struct of one field
    // would)
    std::size_t gp_hash() const { return gp::hash_fields(gp_code); }
};

namespace detail {

// hresult_failure(code) is the error value of the failed HRESULT code (code < 0).
gp::error hresult_failure(int32_t code);

} // namespace detail

// hresult_text(code) is the text of Error(): "HRESULT 0x" and the code in eight hexadecimal digits,
// then the name of the code in parentheses when the runtime knows one (E_FAIL, E_INVALIDARG,
// DXGI_ERROR_DEVICE_REMOVED, ...) or, for the HRESULTs that wrap a Win32 error (facility 7), "Win32
// error N".
gp::string hresult_text(int32_t code);

// hresult_error(hr) is the Go error of the HRESULT hr: nil when hr >= 0 (SUCCEEDED), else a
// runtime.HResultError. A 64-bit hr (LONG is 32 bits on Windows, but a program's own typedef
// may not be) is read as its low 32 bits, the HRESULT.
template <typename T>
    requires std::is_integral_v<T>
inline gp::error hresult_error(T hr) {
    int32_t code = static_cast<int32_t>(hr);
    if (code >= 0) {
        return gp::error();
    }
    return detail::hresult_failure(code);
}

} // namespace gp
