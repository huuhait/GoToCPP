// The values of Go panics (gocpp/panic.hpp, step 5.10): the runtime.Error types that the runtime's
// own panics carry, *runtime.PanicNilError, *runtime.TypeAssertionError, runtime.CppException
// (a C++ exception that became a panic), the text that an unrecovered panic prints for a value,
// and the entry points panic(iface), panic(nil) and goexit.
//
// A runtime type is described like a generated one (gocpp/type.hpp): a descriptor named after
// its Go type, with the method table that Go's type has, so that a type assertion or a
// conversion to `error` or `runtime.Error` finds the methods by name and signature text, and
// fmt (which will read the same tables) prints the Error() text.
#include <gocpp/rt.hpp>

#include <atomic>
#include <cstdint>
#include <cstring>
#include <exception>
#include <string>
#include <utility>

namespace gp {

namespace {

// runtime.errorString, runtime.boundsError and runtime.plainError: Go's string types (and a
// struct, for boundsError), each with Error() and RuntimeError(). The text is the whole Error()
// (the "runtime error: " prefix is in it for the first two).
struct text_error {
    gp::string gp_s;
    friend bool operator==(const text_error& a, const text_error& b) { return a.gp_s == b.gp_s; }
};
struct error_string : text_error {};
struct bounds_error : text_error {};
struct plain_error : text_error {};

// *runtime.TypeAssertionError and *runtime.PanicNilError: pointers to structs.
struct type_assertion_error_t {
    gp::string gp_s;
    friend bool operator==(const type_assertion_error_t&, const type_assertion_error_t&) = default;
};
struct panic_nil_error_t {
    char gp_unused = 0;
    friend bool operator==(const panic_nil_error_t&, const panic_nil_error_t&) = default;
};

template <typename T>
gp::string thunk_error_text(const iface& self) {
    return unbox<T>(self).gp_s;
}
template <typename T>
gp::string thunk_error_text_ptr(const iface& self) {
    return unbox<gp::ptr<T>>(self)->gp_s;
}
gp::string thunk_panic_nil_error(const iface&) {
    return gp::string::gp_copy("panic called with nil argument");
}
void thunk_runtime_error(const iface&) {}

template <typename T>
const type_desc* td_text_error(const char* name, kind k) {
    static const method methods[] = {
        method_of<gp::string()>("Error", nullptr, "func() string", &thunk_error_text<T>),
        method_of<void()>("RuntimeError", nullptr, "func()", &thunk_runtime_error),
    };
    static const type_desc td = named<T>(
        name, "runtime", k == kind::String ? shape_basic(kind::String) : shape_struct(nullptr, 0),
        methods_of(methods));
    return &td;
}

const type_desc* td_error_string() {
    return td_text_error<error_string>("runtime.errorString", kind::String);
}
const type_desc* td_bounds_error() {
    return td_text_error<bounds_error>("runtime.boundsError", kind::Struct);
}
const type_desc* td_plain_error() {
    return td_text_error<plain_error>("runtime.plainError", kind::String);
}

// *runtime.TypeAssertionError.
const type_desc* td_type_assertion_error() {
    static const method pmethods[] = {
        method_of<gp::string()>("Error", nullptr, "func() string",
                                &thunk_error_text_ptr<type_assertion_error_t>),
        method_of<void()>("RuntimeError", nullptr, "func()", &thunk_runtime_error),
    };
    static const type_desc td =
        named<type_assertion_error_t>("runtime.TypeAssertionError", "runtime",
                                      shape_struct(nullptr, 0), {}, methods_of(pmethods));
    static const type_desc* const p = td_ptr<type_assertion_error_t>(&td);
    return p;
}

// *runtime.PanicNilError.
const type_desc* td_panic_nil_error() {
    static const method pmethods[] = {
        method_of<gp::string()>("Error", nullptr, "func() string", &thunk_panic_nil_error),
        method_of<void()>("RuntimeError", nullptr, "func()", &thunk_runtime_error),
    };
    static const type_desc td = named<panic_nil_error_t>(
        "runtime.PanicNilError", "runtime", shape_struct(nullptr, 0), {}, methods_of(pmethods));
    static const type_desc* const p = td_ptr<panic_nil_error_t>(&td);
    return p;
}

// runtime.CppException: a struct, with Error() (value receiver) only: an error, not a
// runtime.Error.
gp::string thunk_cpp_error(const iface& self) {
    return unbox<CppException>(self).gp_text;
}
const type_desc* td_cpp_exception() {
    static const method methods[] = {
        method_of<gp::string()>("Error", nullptr, "func() string", &thunk_cpp_error),
    };
    static const type_desc td = named<CppException>("runtime.CppException", "runtime",
                                                    shape_struct(nullptr, 0), methods_of(methods));
    return &td;
}

// The values of the package runtime's own types, once a program that imports the package has
// run its init (detail::set_panic_values): never freed, and read without a lock.
std::atomic<const gp::any*> registered_nil_error{nullptr};
std::atomic<const gp::func<gp::any(gp::string)>*> registered_assertion{nullptr};

} // namespace

namespace detail {

void set_panic_values(const gp::any& nil_error, gp::func<gp::any(gp::string)> assertion) {
    // Not freed, and the last call wins (a program has one runtime package).
    auto* f = new gp::func<gp::any(gp::string)>(std::move(assertion));
    auto* v = new gp::any(nil_error);
    registered_assertion.store(f, std::memory_order_release);
    registered_nil_error.store(v, std::memory_order_release);
}

gp::any runtime_error_value(rt_error kind, gp::string text) {
    switch (kind) {
    case rt_error::bounds:
        return to_iface<gp::any>(bounds_error{{std::move(text)}}, td_bounds_error());
    case rt_error::plain:
        return to_iface<gp::any>(plain_error{{std::move(text)}}, td_plain_error());
    default:
        return to_iface<gp::any>(error_string{{std::move(text)}}, td_error_string());
    }
}

gp::any runtime_error_value(const gp::string& text) {
    constexpr std::string_view prefix = "runtime error: ";
    return runtime_error_value(
        text.gp_view().starts_with(prefix) ? rt_error::error_string : rt_error::plain, text);
}

gp::any type_assertion_error(gp::string text) {
    if (const auto* make = registered_assertion.load(std::memory_order_acquire); make != nullptr) {
        return (*make)(std::move(text)); // the runtime package's *TypeAssertionError
    }
    return to_iface<gp::any>(
        gp::new_<type_assertion_error_t>(type_assertion_error_t{std::move(text)}),
        td_type_assertion_error());
}

void throw_type_assertion(gp::string text) {
    gp::any value = type_assertion_error(text);
    throw panic_error{std::move(value), std::move(text), false, {}};
}

gp::any cxx_exception_value(const std::exception_ptr& e) {
    std::string type;
    std::string what;
    bool has_what = false;
    cxx_exception_parts(e, type, what, has_what);
    CppException v;
    v.gp_text = gp::string::gp_copy(cxx_exception_text(e));
    v.gp_type = gp::string::gp_copy(type);
    v.gp_what = gp::string::gp_copy(what);
    return to_iface<gp::any>(std::move(v), td_cpp_exception());
}

bool string_method(const gp::iface& v, const char* name, gp::string& out) {
    const type_desc* t = v.gp_type();
    if (t == nullptr) {
        return false;
    }
    for (std::size_t i = 0; i < t->gp_nmethods; i++) {
        const method& m = t->gp_methods[i];
        if (m.gp_pkg == nullptr && std::strcmp(m.gp_name, name) == 0 &&
            std::strcmp(m.gp_sig, "func() string") == 0) {
            out = reinterpret_cast<thunk_t<gp::string()>*>(m.gp_fn)(v);
            return true;
        }
    }
    return false;
}

namespace {

// printpanicval's number and string forms, for a value of the basic kind k at p.
bool basic_text(std::string& out, kind k, const void* p) {
    switch (k) {
    case kind::Bool:
        append_print(out, *static_cast<const bool*>(p));
        return true;
    case kind::Int:
    case kind::Int64:
        append_print(out, *static_cast<const int64_t*>(p));
        return true;
    case kind::Int8:
        append_print(out, *static_cast<const int8_t*>(p));
        return true;
    case kind::Int16:
        append_print(out, *static_cast<const int16_t*>(p));
        return true;
    case kind::Int32:
        append_print(out, *static_cast<const int32_t*>(p));
        return true;
    case kind::Uint:
    case kind::Uint64:
    case kind::Uintptr:
        append_print(out, *static_cast<const uint64_t*>(p));
        return true;
    case kind::Uint8:
        append_print(out, *static_cast<const uint8_t*>(p));
        return true;
    case kind::Uint16:
        append_print(out, *static_cast<const uint16_t*>(p));
        return true;
    case kind::Uint32:
        append_print(out, *static_cast<const uint32_t*>(p));
        return true;
    case kind::Float32:
        append_print(out, *static_cast<const float*>(p));
        return true;
    case kind::Float64:
        append_print(out, *static_cast<const double*>(p));
        return true;
    case kind::Complex64:
        append_print(out, *static_cast<const complex64*>(p));
        return true;
    case kind::Complex128:
        append_print(out, *static_cast<const complex128*>(p));
        return true;
    case kind::String:
        out.append(static_cast<const gp::string*>(p)->gp_view());
        return true;
    default:
        return false;
    }
}

bool basic_kind(kind k) {
    return (k >= kind::Bool && k <= kind::Complex128) || k == kind::String;
}

// value_text is Go's printpanicval (after preprintpanics) for the value v.
std::string value_text(const gp::iface& v) {
    std::string out;
    const type_desc* t = v.gp_type();
    gp::string s;
    if (string_method(v, "Error", s) || string_method(v, "String", s)) {
        return std::string(s.gp_view());
    }
    const void* p = v.gp_data().get();
    if (p != nullptr && basic_kind(t->gp_kind)) {
        if (!t->gp_named || t->gp_pkg == nullptr) {
            // A predeclared type: the value as print writes it.
            basic_text(out, t->gp_kind, p);
            return out;
        }
        // printanycustomtype: main.MyInt(5), main.S("x").
        out += t->gp_name;
        out += t->gp_kind == kind::String ? "(\"" : "(";
        basic_text(out, t->gp_kind, p);
        out += t->gp_kind == kind::String ? "\")" : ")";
        return out;
    }
    // Any other type: its type and the address of the value (the pointer, for a pointer).
    out += '(';
    out += t->gp_name;
    out += ") ";
    print_hex(out, static_cast<uint64_t>(reinterpret_cast<uintptr_t>(p)));
    return out;
}

} // namespace

gp::string panic_text(const panic_error& e) {
    if (!e.gp_lazy) {
        return e.gp_text;
    }
    if (e.gp_value == nullptr) {
        return gp::string::gp_copy("nil");
    }
    try {
        return gp::string::gp_copy(value_text(e.gp_value));
    } catch (const panic_error& p) {
        // Go's preprintpanics recovers the panic of the method and ends the program with a fatal
        // error: "panic while printing panic value: " and the text when it is a string (a
        // plain string, not a named type), else ": type " and the name of its type.
        std::string msg = "panic while printing panic value";
        if (p.gp_value == nullptr) {
            msg += ": type <nil>";
        } else if (p.gp_value.gp_type() == &td_string) {
            msg += ": ";
            msg += unbox<gp::string>(p.gp_value).gp_view();
        } else {
            msg += ": type ";
            msg += p.gp_value.gp_type()->gp_name;
        }
        fatal(msg);
    } catch (...) {
        fatal("panic while printing panic value: type ?");
    }
}

[[noreturn]] void panic_value(const gp::iface& v) {
    if (v == nullptr) {
        panic(nullptr);
    }
    gp::any value(v);
    // The interface words: the same value panicked again is the same panic.
    throw panic_error{std::move(value), gp::string(), true, {panic_identity::gp_words, 0, 0}};
}

} // namespace detail

void panic(std::nullptr_t) {
    // Go 1.21: panic(nil) is a *runtime.PanicNilError.
    // (Go's new(PanicNilError) is a zero-size allocation: every one has the address
    // runtime.zerobase, so two panic(nil) are the same interface words, and a chain prints them
    // once.)
    static const gp::any* const one =
        new gp::any(to_iface<gp::any>(gp::new_<panic_nil_error_t>(), td_panic_nil_error()));
    const gp::any* registered = registered_nil_error.load(std::memory_order_acquire);
    gp::any value = registered != nullptr ? *registered : *one;
    throw panic_error{std::move(value), gp::string(), true, {panic_identity::gp_words, 0, 0}};
}

void goexit() {
    throw goexit_signal{};
}

} // namespace gp
