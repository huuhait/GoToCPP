// The unit tests of gp::hresult_error (gocpp/hresult.hpp, ADR-137): which codes are errors, the
// text of the error, and the methods that a program finds through an interface. runtime/cxx_test.go
// compiles this file with the runtime's .cpp files, with g++ and clang++, at -O0, at -O2 and with
// ASan + UBSan, and runs it like test/unit.cpp. It prints "ok: N checks" and exits 0, or prints
// each failed check and exits 1.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <cstdint>
#include <string>

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

// `type HResulter interface{ HResult() int32 }`, as the emitter writes an interface.
const gp::type_desc* hresulter_desc();
struct HResulter : gp::iface {
    using gp::iface::iface;
    static const gp::type_desc* gp_desc() { return hresulter_desc(); }
    int32_t HResult() const { return gp::icall<int32_t()>(*this, 0); }
};
const gp::imethod gp_im_HResulter[] = {gp::imethod("HResult", nullptr, "func() int32")};
const gp::type_desc gp_td_HResulter =
    gp::named<HResulter>("main.HResulter", "main", gp::shape_interface(gp_im_HResulter));
const gp::type_desc* hresulter_desc() {
    return &gp_td_HResulter;
}

std::string text_of(const gp::error& e) {
    return std::string(e.Error().gp_view());
}

// SUCCEEDED(hr) is hr >= 0: S_OK, S_FALSE and the other success codes are the nil error.
void test_success_is_nil() {
    CHECK(gp::hresult_error(0L) == nullptr);
    CHECK(gp::hresult_error(1L) == nullptr);          // S_FALSE
    CHECK(gp::hresult_error(0x087A0001L) == nullptr); // DXGI_STATUS_OCCLUDED
    CHECK(gp::hresult_error(INT32_MAX) == nullptr);
    CHECK(gp::hresult_error(static_cast<int32_t>(0)) == nullptr);
}

// Every negative code is an error, whichever integer type holds it.
void test_failure_is_an_error() {
    CHECK(gp::hresult_error(-1L) != nullptr);
    CHECK(gp::hresult_error(INT32_MIN) != nullptr);
    CHECK(gp::hresult_error(static_cast<int32_t>(0x80004005u)) != nullptr);
    // A 64-bit LONG that holds the unsigned spelling of E_FAIL (a typedef long on Linux).
    CHECK(gp::hresult_error(0x80004005L) != nullptr);
    CHECK(gp::hresult_error(static_cast<unsigned long>(0x80004005UL)) != nullptr);
}

void test_text() {
    CHECK(text_of(gp::hresult_error(-1L)) == "HRESULT 0xFFFFFFFF");
    CHECK(text_of(gp::hresult_error(0x80004005L)) == "HRESULT 0x80004005 (E_FAIL)");
    CHECK(text_of(gp::hresult_error(0x887A0005L)) ==
          "HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)");
    // What D3D11CreateDeviceAndSwapChain gave on the real PC in a session without a desktop.
    CHECK(text_of(gp::hresult_error(0x887A0022L)) ==
          "HRESULT 0x887A0022 (DXGI_ERROR_NOT_CURRENTLY_AVAILABLE)");
    CHECK(text_of(gp::hresult_error(0x80041234L)) == "HRESULT 0x80041234");
    // HRESULT_FROM_WIN32(2): ERROR_FILE_NOT_FOUND
    CHECK(text_of(gp::hresult_error(0x80070002L)) == "HRESULT 0x80070002 (Win32 error 2)");
    // facility 7 without the failure bit is a success: no error at all
    CHECK(gp::hresult_error(0x00070002L) == nullptr);
}

// The code is found through an interface, and equal codes are equal errors.
void test_code_through_an_interface() {
    gp::error e = gp::hresult_error(0x887A0005L);
    CHECK(e.gp_type() != nullptr && std::string(e.gp_type()->gp_name) == "runtime.HResultError");
    HResulter h = gp::iface_cast<HResulter>(e);
    CHECK(h.HResult() == static_cast<int32_t>(0x887A0005u));
    CHECK(e == gp::hresult_error(0x887A0005L));
    CHECK(!(e == gp::hresult_error(0x80004005L)));
}

// The error is a map key (map[error]int): hashing it must not panic as an unhashable type, and
// equal codes hash alike.
void test_hash() {
    gp::error a = gp::hresult_error(0x887A0005L);
    gp::error b = gp::hresult_error(0x887A0005L);
    gp::error c = gp::hresult_error(0x80004005L);
    CHECK(a.gp_hash() == b.gp_hash());
    CHECK(a.gp_hash() != c.gp_hash());
}

} // namespace

int main() {
    test_success_is_nil();
    test_failure_is_an_error();
    test_text();
    test_code_through_an_interface();
    test_hash();
    if (failures != 0) {
        return 1;
    }
    std::string msg = "ok: " + std::to_string(checks) + " checks\n";
    gp::os::write(gp::os::stdout_fd, msg.data(), msg.size());
    return 0;
}
