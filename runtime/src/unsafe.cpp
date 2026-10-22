// The parts of package unsafe (gocpp/unsafe.hpp) that are not templates: the panics of
// unsafe.Slice and unsafe.String, and the checkptr fatal error. Standard C++ only.
#include <gocpp/rt.hpp>

namespace gp::detail {

void unsafe_panic(unsafe_what what) {
    // runtime/unsafe.go: panicunsafeslicelen, panicunsafeslicenilptr, panicunsafestringlen and
    // panicunsafestringnilptr panic with an errorString, which prints with "runtime error: ".
    constexpr panic_identity id{panic_identity::gp_runtime, 0, 0};
    switch (what) {
    case unsafe_what::slice_len:
        throw panic_error{"runtime error: unsafe.Slice: len out of range"_gs, id};
    case unsafe_what::slice_nil:
        throw panic_error{"runtime error: unsafe.Slice: ptr is nil and len is not zero"_gs, id};
    case unsafe_what::string_len:
        throw panic_error{"runtime error: unsafe.String: len out of range"_gs, id};
    case unsafe_what::string_nil:
        throw panic_error{"runtime error: unsafe.String: ptr is nil and len is not zero"_gs, id};
    }
    fatal("unsafe_panic: bad kind");
}

void arg_overflow(const char* name, const char* value, const char* ctype) {
    constexpr panic_identity id{panic_identity::gp_runtime, 0, 0};
    std::string text = "runtime error: the variable ";
    text += name;
    text += " holds ";
    text += value;
    text += ", which does not fit the C type ";
    text += ctype;
    text += " of the parameter it is passed to";
    throw panic_error{string::gp_copy(std::string_view(text)), id};
}

void misaligned() noexcept {
    // runtime/checkptr.go: checkptrAlignment throws this.
    fatal("checkptr: misaligned pointer conversion");
}

} // namespace gp::detail
