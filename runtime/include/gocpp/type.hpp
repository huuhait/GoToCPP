// gocpp/type.hpp: Go's type descriptors (Phase 05 §2.2, step 5.1). One gp::type_desc per Go
// type that flows into an interface: its Go name, kind and size, the hooks for ==, hash and
// the method tables. A descriptor is chosen by the GO type, never by the C++ type: Go's int
// and int64 are both int64_t, uint, uint64 and uintptr are all uint64_t, and chan T, <-chan T
// and chan<- T will all be gp::chan<T>; but they are different Go types, so
// any(int(1)).(int64) must fail and %T must print the right name. So there is one descriptor
// object per Go type, and pointer equality of two descriptors IS Go type identity.
//
// Where a descriptor comes from (docs/spec/cpp-mapping.md, "Interfaces"):
//
//   - Predeclared types: one constexpr object each in the runtime, gp::td_int, gp::td_int64,
//     gp::td_uint, gp::td_uint64, gp::td_uintptr, gp::td_string, ... (distinct objects even
//     where the C++ type is shared). byte and rune are uint8 and int32.
//   - Named types (type Rect struct{...}, type Celsius float64, type Shape interface{...}):
//     an object that the emitter writes in the package header, after all types (section 8 of
//     the header order, Phase 06), with the builder gp::named<T>, which takes the Go name, the
//     package path, the SHAPE of the underlying type and the method tables:
//
//         inline const gp::type_desc gp_td_Rect = gp::named<Rect>(
//             "main.Rect", "main", gp::shape_struct(gp_fields_Rect),
//             gp::methods_of(gp_vm_Rect),    // the method set of Rect: value methods
//             gp::methods_of(gp_pm_Rect));   // the method set of *Rect: all methods
//         inline const gp::type_desc gp_td_Celsius = gp::named<Celsius>(
//             "main.Celsius", "main", gp::shape_basic(gp::kind::Float64));
//         inline const gp::type_desc gp_td_Shape = gp::named<Shape>(
//             "main.Shape", "main", gp::shape_interface(gp_im_Shape));
//
//     (A descriptor whose parts are constants is constant-initialized: no guard, no code before
//     main. The method arrays are not: gp::method_of stores a thunk with reinterpret_cast,
//     which a constant expression can't have, so each array is a dynamically initialized
//     inline variable, with a guard. The descriptors hold only the arrays' ADDRESSES, so
//     &gp_td_Rect can be used anywhere, also in another descriptor's initializer; but code
//     that runs during static initialization and reads a method table or an itab must come
//     after the array's definition in the same header, as for any inline variable. For generic
//     types (Phase 07: variable templates, whose initialization is unordered) keep the arrays
//     out of static initializers: build them in a function-local static.)
//   - Composite types ([]int, [3]string, map[string]int, *main.Rect, chan<- int, func(int)
//     string, struct{ X int }, interface{ M() }) get descriptors built from the descriptors of
//     their parts and INTERNED: gp::td_slice<E>(elem), gp::td_array<E, N>(elem),
//     gp::td_map<K, V>(key, elem), gp::td_ptr<T>(elem), gp::td_chan<C>(dir, elem),
//     gp::td_func<F>({params}, {results}, variadic), gp::td_struct<S>({fields}),
//     gp::td_interface({methods}). The first template argument is the C++ type of the Go
//     type (gp::slice<E>, ...: the runtime reads its size, == and hash from it), the
//     arguments are Go descriptors. The same Go type always gives the same pointer, from any
//     thread. Each call looks the descriptor up (about a hash and a few loads, no lock), so a
//     site that runs often keeps it:
//
//         static const gp::type_desc* const gp_td = gp::td_slice<int64_t>(&gp::td_int);
//
//     A named type's pointer descriptor is interned like any other: gp::td_ptr<Rect>(&gp_td_Rect)
//     takes the method table of *Rect from gp_td_Rect.gp_pmethods (the second table above).
//   - C and C++ types (a std::string, a C int, a raw pointer, a class from a C++ header) that
//     are stored in an interface: gp::foreign_td<T>(), made from a template and named after
//     the C++ spelling ("std::string", "int", "char const*"). == and the hash use what T
//     supports. A C++ type that appears in a method signature that Go can't check
//     (go/types sees every C type as invalid) is written into the signature TEXT
//     instead (below), or as an opaque descriptor, gp::td_opaque("C.int").
//
// Method tables (gp::method) are sorted: first the exported names (gp_pkg null) by name,
// then the unexported ones by name and package path, byte order (Go's order, which also
// gives Go's text for an unnamed interface type). A method has its Go name, the import path
// of its package when the name is not exported (an unexported m of package a is another
// method than the m of package b, so another package's unexported method can never satisfy
// an interface), the signature as TEXT, and the thunk. The signature text is the identity of
// the method's type: the Go type of the method without its receiver, in a CANONICAL spelling
// that the emitter must produce, the same for every spelling of an identical signature
// (types.Identical): aliases unaliased, recursively; parameter and result names dropped; byte
// as uint8 and rune as int32; any as `interface {}`; every package as its FULL IMPORT PATH;
// function-local types made unique (two types named L in two functions are two types); the
// type arguments of an instantiated generic type spelled canonically too; a single space after
// each comma ("func(net/url.Values, ...string) (int, error)"); a C type as spelled in the
// source, "C.int". types.TypeString does NOT do this (it keeps names, byte, any and aliases,
// and prints two local types alike): a Go-side canonicaliser with a test for each rule is part
// of step 5.4, and until then the text is only as good as the emitter's. The same text must be
// in the interface and in the type that has the method (it is the one place where a mistake
// shows as a missing method at run time instead of a compile error). Two methods match when the
// name, the package and the text are equal, so `Get() C.int` never matches `Get() C.double`, which
// go/types alone would accept (it sees both as invalid). The thunk is the function that gp::icall
// calls through the itab (gocpp/iface.hpp); gp::method_of checks its C++ type at compile time.
//
// Two tables per named type: gp_methods is the method set of the type itself (a type T's
// value methods), gp_pmethods the method set of *T (all methods, value and pointer receiver),
// which td_ptr copies into the interned *T. For a type that is not named, or has no methods,
// both are empty. td_struct takes both tables too, for the promoted methods of an anonymous
// struct with an embedded field.
//
// What the descriptors don't have yet, and where it goes:
//   - Equality and hashing of INTERFACE values (kind Interface: gp_equal and gp_hash are
//     null), and the panics of an uncomparable or unhashable dynamic type: step 5.6.
//   - The format hook for fmt's %v, the foreign descriptors' printing, and Go's %T names of
//     C numbers: step 5.11. gp_ext is the slot for it and for Phase 09's reflect extensions
//     (the field offsets, accessors, the zero value).
//
// Every string and table that is passed in must live for the whole program (string literals,
// static arrays): a descriptor keeps the pointers; the arrays of an initializer_list are
// copied, and td_opaque copies its spelling.
//
// Threads (Phase 08): the interning table is an append-only open-addressed table that is
// read without a lock (acquire loads of immutable entries) and written under a mutex, which
// is how Go's itab table works too; a table that grows is replaced and the old one is kept,
// so a reader never sees a freed array. The descriptors are never freed.
//
// Like every runtime type, these types have only constructors and gp_-prefixed members.
#pragma once

#include <gocpp/array.hpp>
#include <gocpp/checks.hpp>
#include <gocpp/complex.hpp>
#include <gocpp/func.hpp>
#include <gocpp/hash.hpp>
#include <gocpp/map.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/slice.hpp>
#include <gocpp/string.hpp>

#include <cstddef>
#include <cstdint>
#include <initializer_list>
#include <string>
#include <type_traits>
#include <typeinfo>

namespace gp {

// kind is Go's reflect.Kind of a type, plus Foreign (a C/C++ type, foreign_td) and Opaque
// (the spelling of a C type in a signature, td_opaque, which is never a value's type).
// Capitalized like Go's, so that no C macro (Windows' `interface`) can be one of them.
enum class kind : std::uint8_t {
    Invalid,
    Bool,
    Int,
    Int8,
    Int16,
    Int32,
    Int64,
    Uint,
    Uint8,
    Uint16,
    Uint32,
    Uint64,
    Uintptr,
    Float32,
    Float64,
    Complex64,
    Complex128,
    Array,
    Chan,
    Func,
    Interface,
    Map,
    Pointer,
    Slice,
    String,
    Struct,
    UnsafePointer,
    Foreign,
    Opaque,
};

// chan_dir is a channel's direction, with reflect.ChanDir's numbers: <-chan T is recv,
// chan<- T is send, chan T is both.
enum class chan_dir : std::uint8_t { recv = 1, send = 2, both = 3 };

struct type_desc;

// fn_ptr is the type that a thunk is stored as (and cast back from, to its real type, by
// gp::icall).
using fn_ptr = void (*)();

// equal_fn and hash_fn are the hooks of a comparable type: they take the addresses of the
// values, which are objects of the C++ type of the descriptor.
using equal_fn = bool (*)(const void*, const void*);
using hash_fn = std::size_t (*)(const void*);

// field_desc is a field of a struct type. gp_pkg is the import path for an unexported name
// (null for an exported one); gp_tag is the struct tag (null for none).
struct field_desc {
    const char* gp_name;
    const char* gp_pkg;
    const type_desc* gp_type;
    const char* gp_tag;
    bool gp_embedded;

    constexpr field_desc(const char* name, const char* pkg, const type_desc* type,
                         const char* tag = nullptr, bool embedded = false) noexcept
        : gp_name(name), gp_pkg(pkg), gp_type(type), gp_tag(tag), gp_embedded(embedded) {}
};

// imethod is a method of an interface type: gp_pkgname is the package's NAME (not its path)
// for an unexported name, which an unnamed interface type's text shows ("interface { N();
// pkn.m() }"); null: the last element of gp_pkg.
struct imethod {
    const char* gp_name;
    const char* gp_pkg;
    const char* gp_sig;
    const char* gp_pkgname;

    constexpr imethod(const char* name, const char* pkg, const char* sig,
                      const char* pkgname = nullptr) noexcept
        : gp_name(name), gp_pkg(pkg), gp_sig(sig), gp_pkgname(pkgname) {}
};

// method is a method of a concrete type, with its thunk (gocpp/iface.hpp).
struct method {
    const char* gp_name;
    const char* gp_pkg;
    const char* gp_sig;
    fn_ptr gp_fn;

    constexpr method(const char* name, const char* pkg, const char* sig, fn_ptr fn) noexcept
        : gp_name(name), gp_pkg(pkg), gp_sig(sig), gp_fn(fn) {}
};

// method_table is a table of methods, for named<T>, td_struct and td_ptr.
struct method_table {
    const method* gp_data = nullptr;
    std::size_t gp_size = 0;
};

// methods_of(a) is the method_table of a static array of methods.
template <std::size_t N>
constexpr method_table methods_of(const method (&a)[N]) noexcept {
    return {a, N};
}

// type_desc is the descriptor of one Go type. Which fields are set depends on gp_kind:
//
//     Array             gp_elem, gp_len
//     Chan              gp_elem, gp_dir
//     Func              gp_in/gp_nin, gp_out/gp_nout, gp_variadic (the last parameter is
//                       then a slice type, and written "...T")
//     Interface         gp_imethods/gp_nimethods (the slot i of an itab is method i)
//     Map               gp_key, gp_elem
//     Pointer, Slice    gp_elem
//     Struct            gp_fields/gp_nfields (may be empty for a named struct)
//
// and, for every kind: the name as %T prints it, gp_size and gp_align of the C++ type (the
// size of a gp::ptr, 16, not Go's 8), the hooks (null when the type isn't comparable),
// and the method tables.
struct type_desc {
    const char* gp_name = "";     // main.Rect, []int, *main.Circle, map[string]int
    const char* gp_pkg = nullptr; // a named type's package path ("main"); null otherwise
    std::size_t gp_size = 0;
    std::size_t gp_align = 1;
    kind gp_kind = kind::Invalid;
    chan_dir gp_dir = chan_dir::both;
    bool gp_named = false; // declared with a name (type T ...), or predeclared
    bool gp_variadic = false;
    const type_desc* gp_elem = nullptr;
    const type_desc* gp_key = nullptr;
    std::int64_t gp_len = 0;
    const type_desc* const* gp_in = nullptr;
    std::size_t gp_nin = 0;
    const type_desc* const* gp_out = nullptr;
    std::size_t gp_nout = 0;
    const field_desc* gp_fields = nullptr;
    std::size_t gp_nfields = 0;
    const method* gp_methods = nullptr; // the method set of this type
    std::size_t gp_nmethods = 0;
    const method* gp_pmethods = nullptr; // a named type T: the method set of *T
    std::size_t gp_npmethods = 0;
    const imethod* gp_imethods = nullptr;
    std::size_t gp_nimethods = 0;
    equal_fn gp_equal = nullptr;  // null: not comparable
    hash_fn gp_hash = nullptr;    // null: not hashable
    const void* gp_ext = nullptr; // steps 5.11 (format) and Phase 09 (reflect)
};

namespace detail {

// iface_size and iface_align: the layout of an interface value (gocpp/iface.hpp asserts it).
inline constexpr std::size_t iface_size = 32;
inline constexpr std::size_t iface_align = 8;

// equatable<T>: a == b works. (Go's slices, maps and funcs are compared only with nil: their
// classes have no == for two values; go_incomparable below is the second guard.)
template <typename T>
concept equatable = requires(const T& a, const T& b) {
    { a == b } -> std::convertible_to<bool>;
};

// go_incomparable<T>: T is (or is a named type over) a slice, a map or a func: Go can't compare
// two of them, whatever C++ operators the class has.
template <typename E>
std::true_type incomparable_probe(const slice<E>*);
template <typename K, typename V>
std::true_type incomparable_probe(const map<K, V>*);
template <typename S>
std::true_type incomparable_probe(const func<S>*);
std::false_type incomparable_probe(...);
template <typename T>
inline constexpr bool go_incomparable =
    decltype(incomparable_probe(static_cast<T*>(nullptr)))::value;

// std_hashable<T>: std::hash<T> works (for C/C++ types, and for gp::ptr and gp::weak_ptr).
template <typename T>
concept std_hashable = requires(const T& v) {
    { std::hash<T>{}(v) } -> std::convertible_to<std::size_t>;
};

// hashable<T>: gp::hash_of(v) compiles: the cases of gocpp/hash.hpp's primary template and
// its specializations (it can't be asked of the template itself: its body is a static_assert).
template <typename T>
inline constexpr bool hashable =
    std::is_integral_v<T> || std::is_enum_v<T> || go_float<T> || has_gp_hash<T> ||
    complex_like<T> || std_hashable<T> || std::is_same_v<T, gp::string>;

// is_gp_ptr<T>: T is exactly gp::ptr<U> (a pointer value, which an interface holds by sharing
// its owner, gocpp/iface.hpp).
template <typename T>
inline constexpr bool is_gp_ptr = false;
template <typename U>
inline constexpr bool is_gp_ptr<gp::ptr<U>> = true;

// ptr_elem<gp::ptr<U>>::type is U.
template <typename T>
struct ptr_elem;
template <typename U>
struct ptr_elem<gp::ptr<U>> {
    using type = U;
};

template <typename T>
bool equal_of(const void* a, const void* b) {
    return static_cast<bool>(*static_cast<const T*>(a) == *static_cast<const T*>(b));
}

template <typename T>
std::size_t hash_value(const void* p) {
    return gp::hash_of(*static_cast<const T*>(p));
}

// The hooks of a gp::ptr<U> (a *T, unsafe.Pointer) take the TARGET addresses, because that is
// what an interface that holds a pointer stores (its data shares the pointer's owner and has
// the pointer's address: gocpp/iface.hpp), not a gp::ptr object: two pointers are equal when
// their targets are the same address (a nil pointer: null), and hash by it, as gp::hash does.
inline bool ptr_target_equal(const void* a, const void* b) {
    return a == b;
}

inline std::size_t ptr_target_hash(const void* p) {
    return hash_int(static_cast<uint64_t>(reinterpret_cast<std::uintptr_t>(p)));
}

// comparable_kind(k): a Go type of this kind can have an == hook (not an interface, whose
// == is step 5.6, nor a slice, a map or a func).
constexpr bool comparable_kind(kind k) noexcept {
    return k != kind::Interface && k != kind::Slice && k != kind::Map && k != kind::Func;
}

// equal_hook<T>(k) is the == hook of the C++ type T for a Go type of kind k: null for an
// interface, a slice, a map, a func and a type without ==. The hooks take what
// `x.gp_data().get()` is for an interface x that holds the value: the address of the box, which
// is an object of the C++ type T, except for a gp::ptr<U>, where it is the target address.
template <typename T>
constexpr equal_fn equal_hook(kind k) noexcept {
    if constexpr (is_gp_ptr<T>) {
        return &ptr_target_equal;
    } else if constexpr (equatable<T> && !go_incomparable<T>) {
        return comparable_kind(k) ? &equal_of<T> : nullptr;
    } else {
        return nullptr;
    }
}

// hash_hook<T>(k) is the hash hook: only for a type that also has the == hook.
template <typename T>
constexpr hash_fn hash_hook(kind k) noexcept {
    if constexpr (is_gp_ptr<T>) {
        return &ptr_target_hash;
    } else if constexpr (equatable<T> && !go_incomparable<T> && hashable<T>) {
        return comparable_kind(k) ? &hash_value<T> : nullptr;
    } else {
        return nullptr;
    }
}

// with_cxx<T>(d) completes the shape d with what the C++ type T says: its size and alignment
// and its hooks.
template <typename T>
constexpr type_desc with_cxx(type_desc d) noexcept {
    d.gp_size = sizeof(T);
    d.gp_align = alignof(T);
    d.gp_equal = equal_hook<T>(d.gp_kind);
    d.gp_hash = hash_hook<T>(d.gp_kind);
    return d;
}

} // namespace detail

// --- shapes: the structure of a type, for named<T> and the interners ---

constexpr type_desc shape_basic(kind k) noexcept {
    type_desc d;
    d.gp_kind = k;
    return d;
}

constexpr type_desc shape_array(std::int64_t len, const type_desc* elem) noexcept {
    type_desc d;
    d.gp_kind = kind::Array;
    d.gp_len = len;
    d.gp_elem = elem;
    return d;
}

constexpr type_desc shape_chan(chan_dir dir, const type_desc* elem) noexcept {
    type_desc d;
    d.gp_kind = kind::Chan;
    d.gp_dir = dir;
    d.gp_elem = elem;
    return d;
}

constexpr type_desc shape_func(const type_desc* const* in, std::size_t nin,
                               const type_desc* const* out, std::size_t nout,
                               bool variadic) noexcept {
    type_desc d;
    d.gp_kind = kind::Func;
    d.gp_in = in;
    d.gp_nin = nin;
    d.gp_out = out;
    d.gp_nout = nout;
    d.gp_variadic = variadic;
    return d;
}

constexpr type_desc shape_interface(const imethod* methods, std::size_t n) noexcept {
    type_desc d;
    d.gp_kind = kind::Interface;
    d.gp_imethods = methods;
    d.gp_nimethods = n;
    return d;
}
template <std::size_t N>
constexpr type_desc shape_interface(const imethod (&methods)[N]) noexcept {
    return shape_interface(methods, N);
}

constexpr type_desc shape_map(const type_desc* key, const type_desc* elem) noexcept {
    type_desc d;
    d.gp_kind = kind::Map;
    d.gp_key = key;
    d.gp_elem = elem;
    return d;
}

constexpr type_desc shape_pointer(const type_desc* elem) noexcept {
    type_desc d;
    d.gp_kind = kind::Pointer;
    d.gp_elem = elem;
    return d;
}

constexpr type_desc shape_slice(const type_desc* elem) noexcept {
    type_desc d;
    d.gp_kind = kind::Slice;
    d.gp_elem = elem;
    return d;
}

constexpr type_desc shape_struct(const field_desc* fields, std::size_t n) noexcept {
    type_desc d;
    d.gp_kind = kind::Struct;
    d.gp_fields = fields;
    d.gp_nfields = n;
    return d;
}
template <std::size_t N>
constexpr type_desc shape_struct(const field_desc (&fields)[N]) noexcept {
    return shape_struct(fields, N);
}

// named<T>(name, pkg, shape, vm, pm) is the descriptor of the named type T (also a
// predeclared one, with pkg null): shape is the structure of its underlying type, vm the
// method set of T and pm the method set of *T (above). T is the C++ type, which gives the
// size, the alignment and the hooks.
template <typename T>
constexpr type_desc named(const char* name, const char* pkg, type_desc shape, method_table vm = {},
                          method_table pm = {}) noexcept {
    type_desc d = detail::with_cxx<T>(shape);
    d.gp_name = name;
    d.gp_pkg = pkg;
    d.gp_named = true;
    d.gp_methods = vm.gp_data;
    d.gp_nmethods = vm.gp_size;
    d.gp_pmethods = pm.gp_data;
    d.gp_npmethods = pm.gp_size;
    return d;
}

namespace detail {
constexpr type_desc empty_interface_desc() noexcept {
    type_desc d = shape_interface(nullptr, 0);
    d.gp_name = "interface {}";
    d.gp_size = iface_size;
    d.gp_align = iface_align;
    return d;
}
} // namespace detail

// --- the predeclared types: one object each, distinct also where the C++ type is shared ---

inline constexpr type_desc td_bool = named<bool>("bool", nullptr, shape_basic(kind::Bool));
inline constexpr type_desc td_int = named<int64_t>("int", nullptr, shape_basic(kind::Int));
inline constexpr type_desc td_int8 = named<int8_t>("int8", nullptr, shape_basic(kind::Int8));
inline constexpr type_desc td_int16 = named<int16_t>("int16", nullptr, shape_basic(kind::Int16));
inline constexpr type_desc td_int32 = named<int32_t>("int32", nullptr, shape_basic(kind::Int32));
inline constexpr type_desc td_int64 = named<int64_t>("int64", nullptr, shape_basic(kind::Int64));
inline constexpr type_desc td_uint = named<uint64_t>("uint", nullptr, shape_basic(kind::Uint));
inline constexpr type_desc td_uint8 = named<uint8_t>("uint8", nullptr, shape_basic(kind::Uint8));
inline constexpr type_desc td_uint16 =
    named<uint16_t>("uint16", nullptr, shape_basic(kind::Uint16));
inline constexpr type_desc td_uint32 =
    named<uint32_t>("uint32", nullptr, shape_basic(kind::Uint32));
inline constexpr type_desc td_uint64 =
    named<uint64_t>("uint64", nullptr, shape_basic(kind::Uint64));
inline constexpr type_desc td_uintptr =
    named<uint64_t>("uintptr", nullptr, shape_basic(kind::Uintptr));
inline constexpr type_desc td_float32 =
    named<float>("float32", nullptr, shape_basic(kind::Float32));
inline constexpr type_desc td_float64 =
    named<double>("float64", nullptr, shape_basic(kind::Float64));
inline constexpr type_desc td_complex64 =
    named<gp::complex64>("complex64", nullptr, shape_basic(kind::Complex64));
inline constexpr type_desc td_complex128 =
    named<gp::complex128>("complex128", nullptr, shape_basic(kind::Complex128));
inline constexpr type_desc td_string =
    named<gp::string>("string", nullptr, shape_basic(kind::String));
inline constexpr type_desc td_unsafe_pointer =
    named<gp::ptr<void>>("unsafe.Pointer", nullptr, shape_basic(kind::UnsafePointer));

// td_any is interface {}, the type of every empty interface (gp::any, gocpp/iface.hpp).
// td_interface() of no methods gives it.
inline constexpr type_desc td_any = detail::empty_interface_desc();

// --- interning (runtime/src/type.cpp) ---

namespace detail {

// intern returns the one descriptor of the structure proto (the same pointer for every
// call with an equal structure, from any thread), making it from proto when it is new: the
// arrays are copied, and the name is composed from the parts' names. proto's gp_name is
// ignored, except for kind Opaque, where it is the spelling.
const type_desc* intern(const type_desc& proto);

// the number of interned descriptors (for tests).
std::size_t interned_count() noexcept;

// the C++ name of a type, demangled ("std::vector<long, std::allocator<long> >").
std::string cxx_name(const std::type_info& t);

template <typename T>
std::string cxx_name_of() {
#if defined(__cpp_rtti) || defined(__GXX_RTTI)
    return cxx_name(typeid(T));
#else
    return "?"; // the program's flags turned C++'s type information off (-fno-rtti)
#endif
}

} // namespace detail

// td_ptr<T>(elem) is the descriptor of *T (gp::ptr<T>), elem that of T. A named T's method
// set for *T is T's gp_pmethods.
template <typename T>
const type_desc* td_ptr(const type_desc* elem) {
    type_desc p = detail::with_cxx<gp::ptr<T>>(shape_pointer(elem));
    p.gp_methods = elem->gp_pmethods;
    p.gp_nmethods = elem->gp_npmethods;
    return detail::intern(p);
}

// td_slice<E>(elem) is the descriptor of []E (gp::slice<E>).
template <typename E>
const type_desc* td_slice(const type_desc* elem) {
    return detail::intern(detail::with_cxx<gp::slice<E>>(shape_slice(elem)));
}

// td_array<E, N>(elem) is the descriptor of [N]E (gp::array<E, N>).
template <typename E, std::int64_t N>
const type_desc* td_array(const type_desc* elem) {
    return detail::intern(detail::with_cxx<gp::array<E, N>>(shape_array(N, elem)));
}

// td_map<K, V>(key, elem) is the descriptor of map[K]V (gp::map<K, V>).
template <typename K, typename V>
const type_desc* td_map(const type_desc* key, const type_desc* elem) {
    return detail::intern(detail::with_cxx<gp::map<K, V>>(shape_map(key, elem)));
}

// td_chan<C>(dir, elem) is the descriptor of chan T, <-chan T or chan<- T: C is the C++ type
// (gp::chan<T> from Phase 08, the same for the three directions), dir tells them apart.
template <typename C>
const type_desc* td_chan(chan_dir dir, const type_desc* elem) {
    return detail::intern(detail::with_cxx<C>(shape_chan(dir, elem)));
}

// td_func<F>(params, results, variadic) is the descriptor of a func type: F is the C++ type
// (gp::func<R(A...)>). For a variadic func the last parameter is the slice type.
template <typename F>
const type_desc* td_func(std::initializer_list<const type_desc*> params,
                         std::initializer_list<const type_desc*> results, bool variadic = false) {
    return detail::intern(detail::with_cxx<F>(
        shape_func(params.begin(), params.size(), results.begin(), results.size(), variadic)));
}

// td_struct<S>(fields, vm, pm) is the descriptor of an anonymous struct type: S is its C++
// type (gp_anon_...), vm and pm the method sets of S and *S when it has embedded fields with
// methods.
template <typename S>
const type_desc* td_struct(std::initializer_list<field_desc> fields, method_table vm = {},
                           method_table pm = {}) {
    type_desc d = detail::with_cxx<S>(shape_struct(fields.begin(), fields.size()));
    d.gp_methods = vm.gp_data;
    d.gp_nmethods = vm.gp_size;
    d.gp_pmethods = pm.gp_data;
    d.gp_npmethods = pm.gp_size;
    return detail::intern(d);
}

// td_interface(methods, name) is the descriptor of an unnamed interface type, with its methods
// in the order above (it is a fatal error otherwise: the slots of an itab follow this order).
// No methods: td_any.
//
// The name is the text that Go's %T / reflect prints for the type, NOT what go/types prints: the
// SAME canonical form as the signature texts (above): the method set flattened (embedded
// interfaces expanded) and sorted, aliases unaliased, `[]uint8` for []byte, `interface {}` for
// any, no parameter names, package NAMES (not import paths) in qualified types, and Go's
// spacing: `interface { Close() error; Read([]uint8) (int, error) }`. Two identical types
// (types.Identical) have one name, whatever way they were written (`interface{ io.Reader;
// Close() error }` and `interface{ Close() error; Read(p []byte) (int, error) }`; `M(MyInt)`
// and `M(int)` for an alias). The Go-side canonicaliser of step 5.4 must produce both this
// name and the signature texts, and be tested against %T of a real Go program (a corpus of
// about 30 types is in the review notes of step 5.1-5.3).
//
// The emitter must ALWAYS pass the name, at every site. The first call creates the descriptor
// and its name (without a name, one is composed from the methods' signature texts, which has
// package paths where Go's text has names, so it is Go's text only for signatures without a
// package-qualified type); a later call that passes a different name is a fatal error, so the
// text of %T never depends on which site runs first. That includes a first call without a
// name followed by a call with one, whenever the composed name differs from the real one.
const type_desc* td_interface(std::initializer_list<imethod> methods, const char* name = nullptr);

// td_opaque(spelling) is the descriptor of a C type that Go can't see into, by its spelling
// in the source, "C.int": one descriptor per spelling. It stands for the type in a signature
// (td_func), and is never the type of a value.
const type_desc* td_opaque(const char* spelling);

namespace detail {

template <typename U>
struct foreign_holder {
    std::string gp_name;
    type_desc gp_desc;
    foreign_holder() : gp_name(cxx_name_of<U>()), gp_desc(with_cxx<U>(shape_basic(kind::Foreign))) {
        gp_desc.gp_name = gp_name.c_str();
    }
};

template <typename U>
const type_desc* foreign_td_of() {
    // On the heap and never deleted: no static destructor can drop it at exit, and
    // LeakSanitizer doesn't report it (this static variable still points at it).
    static const foreign_holder<U>* const h = new foreign_holder<U>;
    return &h->gp_desc;
}

} // namespace detail

// foreign_td<T>() is the descriptor of the C/C++ type T stored in an interface, made once
// from the template: kind Foreign, named after the C++ spelling, the size of T, == and the
// hash when T has them (operator== and std::hash, a raw pointer by its address).
// A different C++ type is a different descriptor, whatever the Go type of the value is.
template <typename T>
const type_desc* foreign_td() {
    return detail::foreign_td_of<std::remove_cv_t<std::remove_reference_t<T>>>();
}

// methods_sorted(table) and imethods_sorted(methods, n): the table is in the order above
// (the emitter writes them so; a type's itab is not built from an unsorted table).
bool methods_sorted(const method* methods, std::size_t n) noexcept;
bool imethods_sorted(const imethod* methods, std::size_t n) noexcept;

// The type of the thunk of a method whose signature is Sig, as C++ declares it without the
// receiver (double(), void(int64_t, gp::string)): R(const gp::iface&, A...). (gocpp/iface.hpp
// has the interface value.)
class iface;
namespace detail {
template <typename Sig>
struct thunk_of;
template <typename R, typename... A>
struct thunk_of<R(A...)> {
    using type = R(const iface&, A...);
};
} // namespace detail
template <typename Sig>
using thunk_t = typename detail::thunk_of<Sig>::type;

// thunk_matches<Sig, Fn>: Fn is the type of a function (or a pointer to one, decltype(&thunk))
// that is the thunk for the signature Sig. The emitter asserts it for every method entry
// (method_of) and at every static conversion to an interface: C types are invalid for
// go/types, so `Get() C.int` may stand for `Get() C.double` in the checker's eyes, and the
// call through the itab would then be a call with the wrong signature.
template <typename Sig, typename Fn>
inline constexpr bool thunk_matches = std::is_same_v<std::remove_pointer_t<Fn>, thunk_t<Sig>>;

// method_of<Sig>(name, pkg, sig, thunk) is the table entry of a method of the C++ signature
// Sig, which must be the type of its thunk. (Not constexpr: it stores the thunk as an fn_ptr.)
template <typename Sig, typename Fn>
method method_of(const char* name, const char* pkg, const char* sig, Fn* thunk) {
    static_assert(thunk_matches<Sig, Fn>,
                  "gp::method_of: the thunk's type is not R(const gp::iface&, A...) for the "
                  "method's signature R(A...) (a C/C++ type that Go can't tell apart?)");
    return method(name, pkg, sig, reinterpret_cast<fn_ptr>(thunk));
}

} // namespace gp
