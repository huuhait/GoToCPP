// gocpp/fmtshim.hpp: gp::fmtshim, a stand-in for Go's fmt until the real one (Phase 05 needs
// interfaces, Phase 09 ports Go's fmt): Print, Println, Printf, Sprint, Sprintln, Sprintf.
//
// What prints as Go's fmt does (Go 1.25's fmt/print.go and format.go, mirrored):
//
//     bool, integers, floats, string    true, 42, and floats with the shortest digits that
//                                       read back as the same float, in exponent form when the
//                                       decimal exponent is < -4 or >= 6 (123456, 1e+06,
//                                       0.0001, 1e-05), and +Inf, -Inf, NaN; a float32 with
//                                       its own shortest digits (float32(0.1) is 0.1)
//     complex64, complex128             (1+2i): each part as a float of half the size, the
//                                       imaginary part always with its sign, (1.00-2.00i)
//                                       for %.2f; a width pads each part (fmtComplex)
//     []T, [N]T                         [1 2 3], nested [[1 2] [3]], nil and empty []; a verb
//                                       applies to each element. %s and %q of a slice or array
//                                       of bytes (elements of kind uint8) print the bytes as a
//                                       string
//     map[K]V                           map[a:1 b:2], the keys sorted with the rules of Go's
//                                       internal/fmtsort (below); nil map[]
//     *T                                &{1 2}, &[1 2], &map[a:1] at the top level, for a
//                                       pointer to a struct, array, slice or map; any other
//                                       pointer (also one inside a composite value) prints its
//                                       address, 0xc000012345; nil is <nil>. %d prints the
//                                       address in decimal
//     structs                           {1 2}; %+v {X:1 Y:2}; through the struct protocol below
//     named types                       as their underlying type, through the protocol below
//     a String() or Error() method      used for %v and %s (and %q, %x, %X) and by Print and
//                                       Println, not for %d; Error() first, as fmt checks
//                                       error before Stringer. A panic in the method prints
//                                       %!v(PANIC=String method: <value>), or <nil> for a nil
//                                       pointer receiver, as fmt's catchPanic does
//
// Printf knows the verbs %v %+v %d %s %q %t %f (Phase 03), %T (the type: Phase 05 lifts its
// gate), the flags - + space 0 and #, a width and a precision (%5d, %-8s, %.2f, %05d, %+.1f,
// %5.3v), and *, [n] argument indexes; the gate (internal/check, formatVerbs) lets only
// Phase 03's through. Go's other verbs (%x %c %e %g %p ...) come with Go's fmt in Phase 09:
// here they print the error form, as an unknown verb does. The # flag only changes %q
// (backquotes, strconv.CanBackquote); %#v is Go syntax in Go's fmt and plain %v here (Phase
// 09). The error forms are Go's: %!d(string=x), %!d(MISSING), %!(EXTRA int=2, string=x),
// %!(NOVERB), %!d(BADINDEX), %!(BADWIDTH), %!(BADPREC). Width and precision pad or cut each
// element of a composite value, as in Go ([    1     2] for %5d).
//
// Go's type names in the error forms (and %T): int64_t is Go's int and uint64_t its uint,
// unless the emitter says the Go type is int64, uint64 or uintptr (C++ can't tell them apart).
// It does that with a type tag at the call site (Phase 03 STATE, 2026-10-13):
//
//     fmt.Printf("%s", x), x is int64
//         gp::fmtshim::Printf("%s"_gs, gp::fmtshim::typed<gp::fmtshim::tag::int64>(x))
//     fmt.Println(s), s is []uintptr
//         gp::fmtshim::Println(gp::fmtshim::typed<tag::slice<tag::uintptr>>(s))
//     map[string]uint64, *[3]int64
//         tag::map<tag::none, tag::uint64>, tag::ptr<tag::array<tag::int64>>
//
// A tag mirrors the type: tag::slice<E>, tag::array<E>, tag::map<K, V>, tag::ptr<E>, the
// leaves tag::int64, tag::uint64, tag::uintptr, and tag::none for "the C++ type says it" (any
// part without int64, uint64 or uintptr). Only an argument whose type has one of these three
// in an unnamed part needs a tag: a struct field gets its tag in gp_fields, and a named type
// over such a type in gp_value_tag (below). typed() copies the value, so the emitter may keep
// it in a temporary like any other argument (evaluation order, internal/lower/order.go). An
// array or struct larger than 128 KiB comes as heap(gp::new_<A>(a)) (heap_value, below), a copy
// on the heap.
//
// The protocols that the emitter's types follow (steps 3.8, 3.9, 3.10; the names start with
// gp_, which no escaped Go name does, and Go methods are C++ members of their own names, so
// nothing clashes with a Go method or field):
//
//   - A struct: its Go type name and its fields in declaration order, all of them (also the
//     blank _ fields, which fmt prints), each with its Go name, whether it is exported and
//     whether it is embedded: gp::field(member, name, exported, embedded). An embedded field
//     has its type's name (inner for *pkg.inner). fmt can't call the String() and Error()
//     methods of a value reached through an unexported field, nor of anything inside it, with
//     one exception, as in Go's reflect: the exported fields of an embedded unexported field
//     are open again, with everything inside them (Go's flagEmbedRO; detail::access below).
//     A field of type int64, uint64 or uintptr (or one that holds them) gets a tag:
//
//         struct Point {
//             int64_t X = 0;
//             int64_t y = 0;
//             int64_t Z = 0;                         // Go: Z int64
//             inner inner_;                          // Go: embedded inner
//             static constexpr const char* gp_type_name = "main.Point";
//             static constexpr auto gp_fields = gp::fields(
//                 gp::field(&Point::X, "X", true, false), gp::field(&Point::y, "y", false, false),
//                 gp::field<gp::fmtshim::tag::int64>(&Point::Z, "Z", true, false),
//                 gp::field(&Point::inner_, "inner", false, true));
//         };
//
//   - A named type that isn't a struct (gp::newtype, step 3.9): gp_type_name and the
//     underlying value, const U& gp_value() const; and, if U needs one, the tag of U:
//     using gp_value_tag = gp::fmtshim::tag::slice<gp::fmtshim::tag::int64>;
//
//   - The method set (step 3.10): a type whose method set has String() string gets
//
//         gp::string gp_String() const { return String(); }
//
//     and one whose method set has it only through a pointer (a pointer receiver) gets
//
//         static gp::string gp_String_ptr(const gp::ptr<T>& p) { return T::String(p); }
//
//     The same for Error() string: gp_Error, gp_Error_ptr. The emitter decides from Go's
//     method sets (promoted methods of embedded fields too), so a field named String, a String
//     with another signature or a method of a C++ base class never counts. A value method is
//     in the pointer's method set too: a nil pointer then prints <nil>, as in Go (its call
//     panics, and fmt prints <nil> for a nil receiver). A member, not a free function found by
//     argument-dependent lookup: such a function would also match a value that converts to T.
//
// Map keys sort as internal/fmtsort.compare does: numbers by value with NaN first (-0 == +0),
// complex numbers by the real part, then the imaginary part (each so),
// strings by their bytes, false before true, pointers by address, arrays element by element,
// structs field by field, named types by their underlying values; the sort is stable over the
// map's order (several NaN keys print in that order, which Go randomizes). The keys and values
// are copied first, as Go does, so a String() method that changes the map can't break the
// printing. The arguments are taken by value, so they are the values at the call, as in Go
// (which copies them into interfaces). Below them, fmt reads through: a slice is read again for
// each element (reflect's Len and Index), so a String() method that changes a slice that it
// reaches through a pointer changes what prints, also here; while an element prints, the
// array that holds it is held (its owner copied), and so is a pointer's target, so nothing
// that is being printed can be freed.
//
// Each call formats everything first and then writes it with one write, and nothing is
// buffered, so the output order is Go's (Go's os.Stdout isn't buffered either).
#pragma once

#include <gocpp/array.hpp>
#include <gocpp/checks.hpp>
#include <gocpp/complex.hpp>
#include <gocpp/map.hpp>
#include <gocpp/panic.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/slice.hpp>
#include <gocpp/string.hpp>

#include <concepts>
#include <cstddef>
#include <cstdint>
#include <string>
#include <string_view>
#include <tuple>
#include <type_traits>
#include <utility>
#include <vector>

namespace gp::fmtshim {

namespace detail {
template <typename E>
struct slice_tag {};
template <typename E>
struct array_tag {};
template <typename K, typename V>
struct map_tag {};
template <typename E>
struct ptr_tag {};
} // namespace detail

// The type tags (above). Each is only a name: none holds data. (The composite ones are aliases,
// so that internal/check's size table, which finds gp::slice by "struct slice", doesn't find
// them.)
namespace tag {
struct none {};
struct int64 {};
struct uint64 {};
struct uintptr {};
template <typename E>
using slice = detail::slice_tag<E>;
template <typename E>
using array = detail::array_tag<E>;
template <typename K, typename V>
using map = detail::map_tag<K, V>;
template <typename E>
using ptr = detail::ptr_tag<E>;
} // namespace tag

// typed_value is a value with its type tag: what typed<Tag>(v) gives.
template <typename Tag, typename T>
struct typed_value {
    T gp_v;
};

template <typename Tag, typename T>
typed_value<Tag, T> typed(T v) {
    return typed_value<Tag, T>{std::move(v)};
}

// heap_value is an array or struct larger than 128 KiB that the emitter copied to the heap at
// the call, heap(gp::new_<A>(a)), because a copy by value would be on the C++ stack, which
// doesn't grow (D24). It prints as the value, not as a pointer.
template <typename T>
struct heap_value {
    gp::ptr<T> gp_p;
};

template <typename T>
heap_value<T> heap(gp::ptr<T> p) {
    return heap_value<T>{std::move(p)};
}

namespace detail {

template <typename>
inline constexpr bool always_false = false;

// type_fn appends a Go type name, for the error forms and %T.
using type_fn = void (*)(std::string&);

// printer is Go's pp with its fmt: the output and the flags of the directive being printed.
// The formatting functions below are fmt's methods (format.go: pad, fmtInteger, ...) and pp's
// (print.go: fmtInteger, fmtFloat, ..., badVerb), in runtime/src/fmt.cpp.
struct printer {
    explicit printer(std::string& out) noexcept : buf(out) {}

    std::string& buf;
    bool wid_present = false;
    bool prec_present = false;
    bool minus = false;
    bool plus = false;
    bool sharp = false;
    bool space = false;
    bool zero = false;
    bool plus_v = false; // %+v: struct field names, and plus is cleared
    int wid = 0;
    int prec = 0;
    bool erroring = false; // printing an error form: no methods are called

    void clear_flags() noexcept {
        wid_present = prec_present = minus = plus = sharp = space = zero = plus_v = false;
        wid = prec = 0;
    }
};

// format.go
void pad(printer& p, std::string_view s);
void fmt_s(printer& p, std::string_view s);
void fmt_0x64(printer& p, uint64_t v, bool leading0x);

// print.go: the verbs of each kind, with Go's error form for a verb that doesn't fit. type is
// the value's Go type, for that form.
void pp_bool(printer& p, bool v, char32_t verb, type_fn type);
void pp_int(printer& p, uint64_t v, bool is_signed, char32_t verb, type_fn type);
void pp_float(printer& p, double v, int size, char32_t verb, type_fn type);
// pp_complex is fmtComplex: size is 64 for a complex64, 128 for a complex128.
void pp_complex(printer& p, double re, double im, int size, char32_t verb, type_fn type);
void pp_string(printer& p, std::string_view v, char32_t verb, type_fn type);
void pp_bytes(printer& p, std::string_view v, char32_t verb); // %s and %q of bytes

// bad_verb_begin writes "%!verb(type=" and sets erroring; the caller then prints the value
// with %v and calls bad_verb_end, which writes ")".
void bad_verb_begin(printer& p, char32_t verb, type_fn type);
void bad_verb_end(printer& p);

// catch_panic writes what fmt's catchPanic does for a panic in a String or Error method.
void catch_panic(printer& p, bool nil_receiver, char32_t verb, const char* method,
                 std::string_view value);

// float_v and float_f append a float as %v and as %f do. bits is 32 for a float32, whose
// shortest digits differ: float32(0.1) prints as 0.1.
void float_v(std::string& out, double v, int bits);
void float_f(std::string& out, double v);

// append_float is strconv.AppendFloat(out, v, fmt, prec, bits) for fmt 'e', 'E', 'f', 'g' and
// 'G'; prec -1 is the shortest digits that read back as the same value.
void append_float(std::string& out, double v, char fmt, int prec, int bits);

// is_print is strconv.IsPrint.
bool is_print(char32_t r) noexcept;

// --- the types ---

template <typename T>
struct slice_of : std::false_type {};
template <typename E>
struct slice_of<gp::slice<E>> : std::true_type {
    using elem = E;
};

template <typename T>
struct array_of : std::false_type {};
template <typename E, int64_t N>
struct array_of<gp::array<E, N>> : std::true_type {
    using elem = E;
    static constexpr int64_t len = N;
};

template <typename T>
struct map_of : std::false_type {};
template <typename K, typename V>
struct map_of<gp::map<K, V>> : std::true_type {
    using key = K;
    using mapped = V;
};

template <typename T>
struct ptr_of : std::false_type {};
template <typename E>
struct ptr_of<gp::ptr<E>> : std::true_type {
    using elem = E;
};

// A struct of the emitter: it has gp_fields.
template <typename T>
concept struct_type = requires { T::gp_fields; };

// A named type over another type (gp::newtype): it has gp_value().
template <typename T>
concept named_type = !struct_type<T> && requires(const T& v) { v.gp_value(); };

template <named_type T>
using underlying_t = std::remove_cvref_t<decltype(std::declval<const T&>().gp_value())>;

// The tag of a named type's underlying value.
template <typename T>
struct value_tag {
    using type = tag::none;
};
template <typename T>
    requires requires { typename T::gp_value_tag; }
struct value_tag<T> {
    using type = typename T::gp_value_tag;
};

// base_t: the type under all the named types (Go's kind): a named type over a named type over
// uint8_t is uint8_t.
template <typename T>
struct base_of {
    using type = T;
};
template <named_type T>
struct base_of<T> {
    using type = typename base_of<underlying_t<T>>::type;
};
template <typename T>
using base_t = typename base_of<T>::type;

template <typename T>
const base_t<T>& base_value(const T& v) {
    if constexpr (named_type<T>) {
        return detail::base_value(v.gp_value());
    } else {
        return v;
    }
}

// The parts of a tag: tag::slice<E> has the element E, tag::map<K, V> the key and the value.
// The parts of tag::none are tag::none.
template <typename Tag>
struct tag_parts {
    using elem = tag::none;
    using key = tag::none;
    using value = tag::none;
};
template <typename E>
struct tag_parts<tag::slice<E>> {
    using elem = E;
};
template <typename E>
struct tag_parts<tag::array<E>> {
    using elem = E;
};
template <typename E>
struct tag_parts<tag::ptr<E>> {
    using elem = E;
};
template <typename K, typename V>
struct tag_parts<tag::map<K, V>> {
    using key = K;
    using value = V;
};

template <typename Tag, template <typename...> class Of>
inline constexpr bool tag_is = false;
template <typename... A, template <typename...> class Of>
inline constexpr bool tag_is<Of<A...>, Of> = true;

// check_tag: a tag fits its type.
template <typename Tag, typename T>
constexpr void check_tag() {
    if constexpr (std::is_same_v<Tag, tag::int64>) {
        static_assert(std::is_same_v<T, int64_t>, "fmtshim: tag::int64 is for an int64_t");
    } else if constexpr (std::is_same_v<Tag, tag::uint64> || std::is_same_v<Tag, tag::uintptr>) {
        static_assert(std::is_same_v<T, uint64_t>,
                      "fmtshim: tag::uint64 and tag::uintptr are for a uint64_t");
    } else if constexpr (!std::is_same_v<Tag, tag::none>) {
        static_assert((slice_of<T>::value && tag_is<Tag, slice_tag>) ||
                          (array_of<T>::value && tag_is<Tag, array_tag>) ||
                          (map_of<T>::value && tag_is<Tag, map_tag>) ||
                          (ptr_of<T>::value && tag_is<Tag, ptr_tag>),
                      "fmtshim: the tag doesn't fit the type");
    }
}

template <typename T>
constexpr const char* int_type_name() {
    if constexpr (std::is_signed_v<T>) {
        return sizeof(T) == 1   ? "int8"
               : sizeof(T) == 2 ? "int16"
               : sizeof(T) == 4 ? "int32"
                                : "int";
    } else {
        return sizeof(T) == 1   ? "uint8"
               : sizeof(T) == 2 ? "uint16"
               : sizeof(T) == 4 ? "uint32"
                                : "uint";
    }
}

// type_name appends the Go name of the type T with the tag Tag: what reflect's Type.String
// gives (int, []string, map[string]int, *main.Point).
template <typename Tag, typename T>
void type_name(std::string& out) {
    detail::check_tag<Tag, T>();
    using parts = tag_parts<Tag>;
    if constexpr (std::is_same_v<Tag, tag::int64>) {
        out += "int64";
    } else if constexpr (std::is_same_v<Tag, tag::uint64>) {
        out += "uint64";
    } else if constexpr (std::is_same_v<Tag, tag::uintptr>) {
        out += "uintptr";
    } else if constexpr (std::is_same_v<T, bool>) {
        out += "bool";
    } else if constexpr (go_int<T>) {
        out += detail::int_type_name<T>();
    } else if constexpr (std::is_same_v<T, float>) {
        out += "float32";
    } else if constexpr (std::is_same_v<T, double>) {
        out += "float64";
    } else if constexpr (std::is_same_v<T, gp::complex64>) {
        out += "complex64";
    } else if constexpr (std::is_same_v<T, gp::complex128>) {
        out += "complex128";
    } else if constexpr (std::is_same_v<T, gp::string>) {
        out += "string";
    } else if constexpr (slice_of<T>::value) {
        out += "[]";
        detail::type_name<typename parts::elem, typename slice_of<T>::elem>(out);
    } else if constexpr (array_of<T>::value) {
        out += '[';
        out += std::to_string(array_of<T>::len);
        out += ']';
        detail::type_name<typename parts::elem, typename array_of<T>::elem>(out);
    } else if constexpr (map_of<T>::value) {
        out += "map[";
        detail::type_name<typename parts::key, typename map_of<T>::key>(out);
        out += ']';
        detail::type_name<typename parts::value, typename map_of<T>::mapped>(out);
    } else if constexpr (ptr_of<T>::value) {
        out += '*';
        detail::type_name<typename parts::elem, typename ptr_of<T>::elem>(out);
    } else if constexpr (struct_type<T> || named_type<T>) {
        out += T::gp_type_name;
    } else {
        static_assert(always_false<T>, "fmtshim: a value of this type can't be printed yet");
    }
}

// --- methods ---

template <typename T>
concept error_hook = requires(const T& v) {
    { v.gp_Error() } -> std::same_as<gp::string>;
};
template <typename T>
concept string_hook = requires(const T& v) {
    { v.gp_String() } -> std::same_as<gp::string>;
};
template <typename T>
concept error_ptr_hook = requires(const gp::ptr<T>& p) {
    { T::gp_Error_ptr(p) } -> std::same_as<gp::string>;
};
template <typename T>
concept string_ptr_hook = requires(const gp::ptr<T>& p) {
    { T::gp_String_ptr(p) } -> std::same_as<gp::string>;
};

// methods<T>: whether T's method set has Error() string and String() string. A pointer's
// method set has its element's value methods too.
template <typename T>
struct methods {
    static constexpr bool error = error_hook<T>;
    static constexpr bool string = string_hook<T>;
};
template <typename E>
struct methods<gp::ptr<E>> {
    static constexpr bool error = error_ptr_hook<E> || error_hook<E>;
    static constexpr bool string = string_ptr_hook<E> || string_hook<E>;
};

// call_method calls the Error (is_error) or String method of v and prints its result with the
// verb, as fmt's handleMethods does.
template <bool is_error, typename T>
void call_method(printer& p, const T& v, char32_t verb) {
    const char* name = is_error ? "Error" : "String";
    bool nil = false;
    if constexpr (ptr_of<T>::value) {
        using E = typename ptr_of<T>::elem;
        nil = v == nullptr;
        constexpr bool by_ptr = is_error ? error_ptr_hook<E> : string_ptr_hook<E>;
        if (nil && !by_ptr) {
            // A value method through a nil pointer: in Go, the call panics, and fmt prints
            // <nil> for a nil receiver.
            detail::catch_panic(p, true, verb, name, {});
            return;
        }
    }
    gp::string s;
    try {
        if constexpr (ptr_of<T>::value) {
            using E = typename ptr_of<T>::elem;
            if constexpr (is_error && error_ptr_hook<E>) {
                s = E::gp_Error_ptr(v);
            } else if constexpr (is_error) {
                s = (*v).gp_Error();
            } else if constexpr (string_ptr_hook<E>) {
                s = E::gp_String_ptr(v);
            } else {
                s = (*v).gp_String();
            }
        } else if constexpr (is_error) {
            s = v.gp_Error();
        } else {
            s = v.gp_String();
        }
    } catch (const gp::panic_error& e) {
        detail::catch_panic(p, nil, verb, name, e.gp_text.gp_view());
        return;
    }
    detail::pp_string(p, s.gp_view(), verb, &detail::type_name<tag::none, gp::string>);
}

// handle_methods is fmt's handleMethods: for %v %s %q %x %X, a value with an Error or String
// method prints what the method returns. It reports whether it printed v.
template <typename T>
bool handle_methods(printer& p, const T& v, char32_t verb) {
    if constexpr (!methods<T>::error && !methods<T>::string) {
        return false;
    } else {
        if (p.erroring ||
            (verb != 'v' && verb != 's' && verb != 'x' && verb != 'X' && verb != 'q')) {
            return false;
        }
        detail::call_method<methods<T>::error>(p, v, verb);
        return true;
    }
}

// --- printing ---

// access is how a value was reached, Go's read-only flags of reflect: a value reached through
// an unexported field can't call its methods (CanInterface is false). The flag of an embedded
// unexported field (flagEmbedRO) holds for that field only: the exported fields inside it are
// open again. Any other read-only flag (flagStickyRO) holds for everything below: the fields,
// elements, keys and values. A pointer's target keeps the flag of the pointer.
enum class access : uint8_t { open, embed_ro, sticky_ro };

// below: the access of an element, key or value of a value reached with ro.
constexpr access below(access ro) noexcept {
    return ro == access::open ? access::open : access::sticky_ro;
}

// field_access: the access of a field, as reflect's Value.Field gives it.
constexpr access field_access(access ro, bool exported, bool embedded) noexcept {
    if (ro == access::sticky_ro) {
        return access::sticky_ro;
    }
    if (!exported) {
        return embedded ? access::embed_ro : access::sticky_ro;
    }
    return access::open;
}

template <typename Tag, typename T>
void print_value(printer& p, const T& v, char32_t verb, int depth, access ro);

template <typename Tag, typename T>
void print_kind(printer& p, const T& v, char32_t verb, int depth, access ro, type_fn type);

// runs_no_code<T>: printing a T can't call a method (only numbers, bools and strings, also
// named ones without methods), so nothing can change while it prints.
template <typename T>
constexpr bool runs_no_code() {
    if constexpr (std::is_same_v<T, bool> || go_int<T> || go_float<T> || go_complex<T> ||
                  std::is_same_v<T, gp::string>) {
        return true;
    } else if constexpr (named_type<T>) {
        return !methods<T>::error && !methods<T>::string && runs_no_code<underlying_t<T>>();
    } else {
        return false;
    }
}

// print_bytes prints %s and %q of the n elements at data, of kind uint8.
template <typename E>
void print_bytes(printer& p, const E* data, int64_t n, char32_t verb) {
    if constexpr (std::is_same_v<E, uint8_t>) {
        detail::pp_bytes(p,
                         n == 0 ? std::string_view()
                                : std::string_view(reinterpret_cast<const char*>(data),
                                                   static_cast<std::size_t>(n)),
                         verb);
    } else {
        std::string bytes;
        for (int64_t i = 0; i < n; i++) {
            bytes += static_cast<char>(detail::base_value(data[i]));
        }
        detail::pp_bytes(p, bytes, verb);
    }
}

// print_slice prints a slice as printValue does: like reflect's Len and Index, it reads the
// slice again for each element, so a method that changes the slice (through a pointer: the
// slice is a field of a struct that a pointer points to) changes what prints, as in Go. While
// an element prints, its array is held, so such a method can't free it.
template <typename Tag, typename E>
void print_slice(printer& p, const gp::slice<E>& v, char32_t verb, int depth, access ro) {
    if constexpr (std::is_same_v<base_t<E>, uint8_t>) {
        if (verb == 's' || verb == 'q') {
            detail::print_bytes(p, v.gp_data(), v.gp_len(), verb);
            return;
        }
    }
    p.buf += '[';
    for (int64_t i = 0; i < v.gp_len(); i++) {
        if (i > 0) {
            p.buf += ' ';
        }
        if constexpr (runs_no_code<E>()) {
            detail::print_value<Tag>(p, v.gp_data()[i], verb, depth + 1, detail::below(ro));
        } else {
            gp::slice<E> keep = v;
            detail::print_value<Tag>(p, keep.gp_data()[i], verb, depth + 1, detail::below(ro));
        }
    }
    p.buf += ']';
}

// print_array prints an array; it is inside the value being printed, which the level above
// holds (an argument is a copy, a map's entries are copied, a slice's array is held).
template <typename Tag, typename E, int64_t N>
void print_array(printer& p, const gp::array<E, N>& v, char32_t verb, int depth, access ro) {
    const E* data = nullptr;
    if constexpr (N > 0) {
        data = v.gp_elems.data();
    }
    if constexpr (std::is_same_v<base_t<E>, uint8_t>) {
        if (verb == 's' || verb == 'q') {
            detail::print_bytes(p, data, N, verb);
            return;
        }
    }
    p.buf += '[';
    for (int64_t i = 0; i < N; i++) {
        if (i > 0) {
            p.buf += ' ';
        }
        detail::print_value<Tag>(p, data[i], verb, depth + 1, detail::below(ro));
    }
    p.buf += ']';
}

// compare is internal/fmtsort's compare for map keys: -1, 0 or +1.
template <typename T>
int compare(const T& a, const T& b) {
    if constexpr (std::is_same_v<T, bool>) {
        return a == b ? 0 : a ? 1 : -1;
    } else if constexpr (go_int<T>) {
        return a < b ? -1 : a > b ? 1 : 0;
    } else if constexpr (go_float<T>) {
        // cmp.Compare: a NaN is less than any other number and equal to a NaN; -0 == +0.
        bool an = a != a, bn = b != b;
        if (an || bn) {
            return an && bn ? 0 : an ? -1 : 1;
        }
        return a < b ? -1 : a > b ? 1 : 0;
    } else if constexpr (go_complex<T>) {
        // The real parts, then the imaginary parts, each like a float. A named complex type
        // comes here through its value (named_type, below), never through the < that
        // gp::newtype declares for every number type: complex numbers have no order.
        if (int c = detail::compare(a.gp_re, b.gp_re); c != 0) {
            return c;
        }
        return detail::compare(a.gp_im, b.gp_im);
    } else if constexpr (std::is_same_v<T, gp::string>) {
        auto c = a <=> b;
        return c < 0 ? -1 : c > 0 ? 1 : 0;
    } else if constexpr (ptr_of<T>::value) {
        auto x = reinterpret_cast<std::uintptr_t>(a.gp_get());
        auto y = reinterpret_cast<std::uintptr_t>(b.gp_get());
        return x < y ? -1 : x > y ? 1 : 0;
    } else if constexpr (array_of<T>::value) {
        if constexpr (array_of<T>::len > 0) {
            for (std::size_t i = 0; i < static_cast<std::size_t>(array_of<T>::len); i++) {
                if (int c = detail::compare(a.gp_elems[i], b.gp_elems[i]); c != 0) {
                    return c;
                }
            }
        }
        return 0;
    } else if constexpr (struct_type<T>) {
        int c = 0;
        std::apply(
            [&](const auto&... f) {
                ((c = c != 0 ? c : detail::compare(a.*(f.gp_member), b.*(f.gp_member))), ...);
            },
            T::gp_fields);
        return c;
    } else if constexpr (named_type<T>) {
        return detail::compare(a.gp_value(), b.gp_value());
    } else {
        static_assert(always_false<T>, "fmtshim: a map key of this type can't be printed yet");
    }
}

// stable_sort sorts the indexes by less, keeping equal ones in their order: a merge sort
// (std::stable_sort is in <algorithm>, which would add its macros to every generated file).
template <typename Less>
void stable_sort(std::vector<std::size_t>& idx, Less less) {
    std::size_t n = idx.size();
    std::vector<std::size_t> tmp(n);
    for (std::size_t width = 1; width < n; width *= 2) {
        for (std::size_t lo = 0; lo < n; lo += 2 * width) {
            std::size_t mid = lo + width < n ? lo + width : n;
            std::size_t hi = lo + 2 * width < n ? lo + 2 * width : n;
            std::size_t i = lo, j = mid, k = lo;
            while (i < mid && j < hi) {
                tmp[k++] = less(idx[j], idx[i]) ? idx[j++] : idx[i++];
            }
            while (i < mid) {
                tmp[k++] = idx[i++];
            }
            while (j < hi) {
                tmp[k++] = idx[j++];
            }
        }
        idx.swap(tmp);
    }
}

template <typename Tag, typename K, typename V>
void print_map(printer& p, const gp::map<K, V>& m, char32_t verb, int depth, access ro) {
    using parts = tag_parts<Tag>;
    p.buf += "map[";
    // A copy of the entries, as fmtsort.Sort makes one.
    std::vector<std::pair<K, V>> entries;
    if (const auto* impl = m.gp_get(); impl != nullptr) {
        entries.reserve(impl->gp_t.size());
        for (const auto& e : impl->gp_t) {
            entries.emplace_back(e.first.gp_k, e.second);
        }
    }
    std::vector<std::size_t> order(entries.size());
    for (std::size_t i = 0; i < order.size(); i++) {
        order[i] = i;
    }
    detail::stable_sort(order, [&](std::size_t a, std::size_t b) {
        return detail::compare(entries[a].first, entries[b].first) < 0;
    });
    for (std::size_t i = 0; i < order.size(); i++) {
        if (i > 0) {
            p.buf += ' ';
        }
        const std::pair<K, V>& e = entries[order[i]];
        detail::print_value<typename parts::key>(p, e.first, verb, depth + 1, detail::below(ro));
        p.buf += ':';
        detail::print_value<typename parts::value>(p, e.second, verb, depth + 1, detail::below(ro));
    }
    p.buf += ']';
}

template <typename T>
void print_struct(printer& p, const T& v, char32_t verb, int depth, access ro) {
    p.buf += '{';
    bool first = true;
    std::apply(
        [&](const auto&... f) {
            [[maybe_unused]] auto one = [&](const auto& field) {
                using F = std::remove_cvref_t<decltype(field)>;
                if (!first) {
                    p.buf += ' ';
                }
                first = false;
                if (p.plus_v) {
                    p.buf += field.gp_name;
                    p.buf += ':';
                }
                detail::print_value<typename F::gp_tag>(
                    p, v.*(field.gp_member), verb, depth + 1,
                    detail::field_access(ro, field.gp_exported, field.gp_embedded));
            };
            (one(f), ...);
        },
        T::gp_fields);
    p.buf += '}';
}

// A pointer prints &{...} at the top level when it points to one of these.
template <typename T>
inline constexpr bool composite =
    slice_of<T>::value || array_of<T>::value || map_of<T>::value || struct_type<T>;

template <typename Tag, typename E>
void print_pointer(printer& p, const gp::ptr<E>& v, char32_t verb, int depth, access ro,
                   type_fn type) {
    if constexpr (composite<base_t<E>>) {
        if (depth == 0 && v != nullptr) {
            gp::ptr<E> keep = v; // the pointee stays alive while it prints
            p.buf += '&';
            detail::print_value<typename tag_parts<Tag>::elem>(p, *keep, verb, depth + 1, ro);
            return;
        }
    }
    auto u = static_cast<uint64_t>(reinterpret_cast<std::uintptr_t>(v.gp_get()));
    switch (verb) {
    case 'v':
        if (u == 0) {
            detail::pad(p, "<nil>");
        } else {
            detail::fmt_0x64(p, u, !p.sharp);
        }
        break;
    case 'd':
        detail::pp_int(p, u, false, verb, type);
        break;
    default:
        detail::bad_verb_begin(p, verb, type);
        detail::print_kind<Tag>(p, v, 'v', 0, ro, type);
        detail::bad_verb_end(p);
    }
}

// print_kind prints v by its kind, without its methods (printValue's switch). type is the Go
// type of the value, which a named type keeps for its underlying value.
template <typename Tag, typename T>
void print_kind(printer& p, const T& v, char32_t verb, int depth, access ro, type_fn type) {
    if constexpr (std::is_same_v<T, bool>) {
        detail::pp_bool(p, v, verb, type);
    } else if constexpr (go_int<T> && std::is_signed_v<T>) {
        detail::pp_int(p, static_cast<uint64_t>(static_cast<int64_t>(v)), true, verb, type);
    } else if constexpr (go_int<T>) {
        detail::pp_int(p, static_cast<uint64_t>(v), false, verb, type);
    } else if constexpr (std::is_same_v<T, float>) {
        detail::pp_float(p, static_cast<double>(v), 32, verb, type);
    } else if constexpr (std::is_same_v<T, double>) {
        detail::pp_float(p, v, 64, verb, type);
    } else if constexpr (go_complex<T>) {
        detail::pp_complex(p, static_cast<double>(v.gp_re), static_cast<double>(v.gp_im),
                           static_cast<int>(sizeof(T) * 8), verb, type);
    } else if constexpr (std::is_same_v<T, gp::string>) {
        detail::pp_string(p, v.gp_view(), verb, type);
    } else if constexpr (slice_of<T>::value) {
        detail::print_slice<typename tag_parts<Tag>::elem>(p, v, verb, depth, ro);
    } else if constexpr (array_of<T>::value) {
        detail::print_array<typename tag_parts<Tag>::elem>(p, v, verb, depth, ro);
    } else if constexpr (map_of<T>::value) {
        detail::print_map<Tag>(p, v, verb, depth, ro);
    } else if constexpr (ptr_of<T>::value) {
        detail::print_pointer<Tag>(p, v, verb, depth, ro, type);
    } else if constexpr (struct_type<T>) {
        detail::print_struct(p, v, verb, depth, ro);
    } else if constexpr (named_type<T>) {
        detail::print_kind<typename value_tag<T>::type>(p, v.gp_value(), verb, depth, ro, type);
    } else {
        static_assert(always_false<T>, "fmtshim: a value of this type can't be printed yet");
    }
}

// print_value is printValue: below the top level (depth > 0), a value's methods print it,
// unless it was reached through an unexported field (ro is not open).
template <typename Tag, typename T>
void print_value(printer& p, const T& v, char32_t verb, int depth, access ro) {
    if (depth > 0 && ro == access::open && detail::handle_methods(p, v, verb)) {
        return;
    }
    detail::print_kind<Tag>(p, v, verb, depth, ro, &detail::type_name<Tag, T>);
}

// print_arg is printArg: an argument of a Print call.
template <typename Tag, typename T>
void print_arg(printer& p, const void* arg, char32_t verb) {
    const T& v = *static_cast<const T*>(arg);
    type_fn type = &detail::type_name<Tag, T>;
    if (verb == 'T') {
        std::string name;
        type(name);
        detail::fmt_s(p, name);
        return;
    }
    if (verb == 'w') {
        // %w is for fmt.Errorf (Phase 05); Sprintf and Printf print the error form.
        detail::bad_verb_begin(p, verb, type);
        detail::print_kind<Tag>(p, v, 'v', 0, access::open, type);
        detail::bad_verb_end(p);
        return;
    }
    if (detail::handle_methods(p, v, verb)) {
        return;
    }
    detail::print_kind<Tag>(p, v, verb, 0, access::open, type);
}

// arg is one argument of a Print call, with its type erased, so that the parsing of the format
// and the spacing are not templates (they are in runtime/src/fmt.cpp).
struct arg {
    const void* v;
    void (*print)(printer&, const void*, char32_t verb);
    type_fn type;
    bool is_string;   // of kind string: Print puts no space next to it
    int8_t int_kind;  // 1: a signed integer, 2: an unsigned one, 0: neither (for * widths)
    uint64_t int_val; // the integer's bits
};

template <typename Tag, typename T>
arg make_tagged_arg(const T& v) {
    using B = base_t<T>;
    arg a{&v,
          &detail::print_arg<Tag, T>,
          &detail::type_name<Tag, T>,
          std::is_same_v<B, gp::string>,
          0,
          0};
    if constexpr (go_int<B>) {
        const B& b = detail::base_value(v);
        a.int_kind = std::is_signed_v<B> ? 1 : 2;
        a.int_val = std::is_signed_v<B> ? static_cast<uint64_t>(static_cast<int64_t>(b))
                                        : static_cast<uint64_t>(b);
    }
    return a;
}

template <typename T>
arg make_arg(const T& v) {
    return detail::make_tagged_arg<tag::none>(v);
}

template <typename Tag, typename T>
arg make_arg(const typed_value<Tag, T>& v) {
    return detail::make_tagged_arg<Tag>(v.gp_v);
}

template <typename T>
arg make_arg(const heap_value<T>& v) {
    return detail::make_tagged_arg<tag::none>(*v.gp_p);
}

template <typename Tag, typename T>
arg make_arg(const typed_value<Tag, heap_value<T>>& v) {
    return detail::make_tagged_arg<Tag>(*v.gp_v.gp_p);
}

// The formatting, as Go's fmt.Sprint, Sprintln and Sprintf do it. They append to out.
void sprint(std::string& out, const arg* args, std::size_t n);
void sprintln(std::string& out, const arg* args, std::size_t n);
void sprintf(std::string& out, std::string_view format, const arg* args, std::size_t n);

// write_stdout writes the bytes to standard output with one write.
void write_stdout(std::string_view s);

// field_info is one field of a struct's gp_fields (gp::field).
template <typename C, typename M, typename Tag>
struct field_info {
    using gp_tag = Tag;
    M C::* gp_member;
    const char* gp_name;
    bool gp_exported;
    bool gp_embedded;
};

} // namespace detail

// Sprint is fmt.Sprint: it adds a space between two arguments when neither is a string.
template <typename... Ts>
gp::string Sprint(Ts... args) {
    std::string out;
    if constexpr (sizeof...(Ts) > 0) {
        const detail::arg a[] = {detail::make_arg(args)...};
        detail::sprint(out, a, sizeof...(Ts));
    }
    return gp::string::gp_copy(out);
}

// Sprintln is fmt.Sprintln: a space between all arguments, and a newline at the end.
template <typename... Ts>
gp::string Sprintln(Ts... args) {
    std::string out;
    if constexpr (sizeof...(Ts) > 0) {
        const detail::arg a[] = {detail::make_arg(args)...};
        detail::sprintln(out, a, sizeof...(Ts));
    } else {
        detail::sprintln(out, nullptr, 0);
    }
    return gp::string::gp_copy(out);
}

// Sprintf is fmt.Sprintf.
template <typename... Ts>
gp::string Sprintf(gp::string format, Ts... args) {
    std::string out;
    if constexpr (sizeof...(Ts) > 0) {
        const detail::arg a[] = {detail::make_arg(args)...};
        detail::sprintf(out, format.gp_view(), a, sizeof...(Ts));
    } else {
        detail::sprintf(out, format.gp_view(), nullptr, 0);
    }
    return gp::string::gp_copy(out);
}

// Print is fmt.Print: it adds a space between two arguments when neither is a string.
template <typename... Ts>
void Print(Ts... args) {
    std::string out;
    if constexpr (sizeof...(Ts) > 0) {
        const detail::arg a[] = {detail::make_arg(args)...};
        detail::sprint(out, a, sizeof...(Ts));
    }
    detail::write_stdout(out);
}

// Println is fmt.Println: a space between all arguments, and a newline at the end.
template <typename... Ts>
void Println(Ts... args) {
    std::string out;
    if constexpr (sizeof...(Ts) > 0) {
        const detail::arg a[] = {detail::make_arg(args)...};
        detail::sprintln(out, a, sizeof...(Ts));
    } else {
        detail::sprintln(out, nullptr, 0);
    }
    detail::write_stdout(out);
}

// Printf is fmt.Printf.
template <typename... Ts>
void Printf(gp::string format, Ts... args) {
    std::string out;
    if constexpr (sizeof...(Ts) > 0) {
        const detail::arg a[] = {detail::make_arg(args)...};
        detail::sprintf(out, format.gp_view(), a, sizeof...(Ts));
    } else {
        detail::sprintf(out, format.gp_view(), nullptr, 0);
    }
    detail::write_stdout(out);
}

} // namespace gp::fmtshim

namespace gp {

// field and fields build a struct's gp_fields (the protocol in gocpp/fmtshim.hpp): each field
// with its member, its Go name, whether it is exported and whether it is embedded; Tag is its
// fmtshim type tag.
template <typename Tag = fmtshim::tag::none, typename C, typename M>
constexpr fmtshim::detail::field_info<C, M, Tag> field(M C::* member, const char* name,
                                                       bool exported, bool embedded) noexcept {
    return {member, name, exported, embedded};
}

template <typename... F>
constexpr std::tuple<F...> fields(F... f) noexcept {
    return std::tuple<F...>(f...);
}

} // namespace gp
