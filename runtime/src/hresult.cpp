// The error of a failed HRESULT (gocpp/hresult.hpp, ADR-137): runtime.HResultError, described like
// a generated Go type (gocpp/type.hpp) so that `err.(interface{ HResult() int32 })`, errors.As and
// fmt find its methods by name and signature.
#include <gocpp/rt.hpp>

#include <cstdint>
#include <cstdio>
#include <string>

namespace gp {

namespace {

// The HRESULTs whose names are well known: the generic COM ones, and the DXGI and Direct3D 11
// ones that a Direct3D program meets. Sorted by code (read as unsigned), searched in order.
struct known_code {
    uint32_t code;
    const char* name;
};

constexpr known_code known_codes[] = {
    {0x80004001, "E_NOTIMPL"},
    {0x80004002, "E_NOINTERFACE"},
    {0x80004003, "E_POINTER"},
    {0x80004004, "E_ABORT"},
    {0x80004005, "E_FAIL"},
    {0x8000FFFF, "E_UNEXPECTED"},
    {0x80010108, "RPC_E_DISCONNECTED"},
    {0x80040154, "REGDB_E_CLASSNOTREG"},
    {0x800401F0, "CO_E_NOTINITIALIZED"},
    {0x80070005, "E_ACCESSDENIED"},
    {0x80070006, "E_HANDLE"},
    {0x8007000E, "E_OUTOFMEMORY"},
    {0x80070057, "E_INVALIDARG"},
    {0x887A0001, "DXGI_ERROR_INVALID_CALL"},
    {0x887A0002, "DXGI_ERROR_NOT_FOUND"},
    {0x887A0004, "DXGI_ERROR_UNSUPPORTED"},
    {0x887A0005, "DXGI_ERROR_DEVICE_REMOVED"},
    {0x887A0006, "DXGI_ERROR_DEVICE_HUNG"},
    {0x887A0007, "DXGI_ERROR_DEVICE_RESET"},
    {0x887A000A, "DXGI_ERROR_WAS_STILL_DRAWING"},
    {0x887A0020, "DXGI_ERROR_DRIVER_INTERNAL_ERROR"},
    {0x887A0022, "DXGI_ERROR_NOT_CURRENTLY_AVAILABLE"},
    {0x887A0026, "DXGI_ERROR_ACCESS_LOST"},
    {0x887C0001, "D3D11_ERROR_TOO_MANY_UNIQUE_STATE_OBJECTS"},
    {0x887C0002, "D3D11_ERROR_FILE_NOT_FOUND"},
};

// The facility of an HRESULT: 7 is FACILITY_WIN32, which HRESULT_FROM_WIN32 uses.
constexpr uint32_t facility_win32 = 7;

gp::string thunk_error(const iface& self) {
    return hresult_text(unbox<HResultError>(self).gp_code);
}

int32_t thunk_hresult(const iface& self) {
    return unbox<HResultError>(self).gp_code;
}

const type_desc* td_hresult_error() {
    static const method methods[] = {
        method_of<gp::string(), &thunk_error>("Error", nullptr, "func() string"),
        method_of<int32_t(), &thunk_hresult>("HResult", nullptr, "func() int32"),
    };
    static const type_desc td = named<HResultError>("runtime.HResultError", "runtime",
                                                    shape_struct(nullptr, 0), methods_of(methods));
    return &td;
}

} // namespace

gp::string hresult_text(int32_t code) {
    uint32_t bits = static_cast<uint32_t>(code);
    char digits[16];
    std::snprintf(digits, sizeof digits, "%08X", static_cast<unsigned>(bits));
    std::string text = std::string("HRESULT 0x") + digits;
    for (const known_code& k : known_codes) {
        if (k.code == bits) {
            return gp::string::gp_copy(text + " (" + k.name + ")");
        }
    }
    // HRESULT_FROM_WIN32(n): the severity bit, facility 7 and the Win32 error in the low 16 bits.
    if (((bits >> 16) & 0x1FFF) == facility_win32 && (bits >> 31) != 0) {
        text += " (Win32 error " + std::to_string(bits & 0xFFFF) + ")";
    }
    return gp::string::gp_copy(text);
}

namespace detail {

gp::error hresult_failure(int32_t code) {
    return to_iface<gp::error>(HResultError{code}, td_hresult_error());
}

} // namespace detail

} // namespace gp
