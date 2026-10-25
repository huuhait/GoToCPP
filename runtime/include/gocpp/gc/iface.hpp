// gocpp/gc/iface.hpp: Go's interfaces and their type descriptors in the GC engine (phase-16
// README §2.2 and §2.8; job 16.1e).
//
// An interface value is two words: for a non-empty interface the itab and the data pointer
// (16 bytes), for the empty interface the type descriptor and the data pointer (16 bytes). The
// data word either holds the value itself (Go's "direct interface": a pointer, a map, a channel
// or a func) or points at a copy of the value in the GC heap. A nil interface has both words
// null; a typed nil pointer in an interface has a type and a null data word, so it is not nil,
// as in Go.
//
// The type descriptor here is a placeholder for step 16.2's decision. Go's is
// internal/abi.Type (size, PtrBytes, Hash, TFlag, Align, FieldAlign, Kind, Equal, GCData,
// Str, PtrToThis, and the kind-specific parts). This one carries only what the spike's
// interfaces need:
//
//     gp_size, gp_align, gp_ptrs    Go's layout (golayout.hpp)
//     gp_direct                     Go's isDirectIface: the data word is the value
//     gp_comparable                 Go's Equal != nil: slices, maps and funcs are not
//     gp_name                       Go's type name, for "comparing uncomparable type ..."
//     gp_equal                      Go's equality function for an indirect value
//
// The ADR of 16.2 decides whether the GC engine uses abi.Type's layout (so Go's reflect is
// translated) or its own, and with it: the offsets of these fields and the kind-specific parts,
// how NameOff/TypeOff/TextOff resolve (module data, typelinks, unnamed types), GCData (the
// pointer mask, which step 16.21 may use for a precise heap), and the hash function that maps
// need. Step 16.8 writes one descriptor per Go type, with the name and the equality function of
// the real type; the ones here are the runtime's template for them, and type_name() is
// approximate ("[]T" for every slice).
//
// Directness follows Go for pointers, maps, channels and funcs. Go also makes a single-element
// struct or array of a direct type direct; this placeholder boxes it (the value is the same,
// only unsafe's view of the interface words differs). 16.2/16.8 decide that with abi.Type.
//
// Type identity is approximate in the same way: type_of<int64_t>() is shared by Go's int and
// int64 (and by named basic types the emitter maps onto int64_t), so to_eface(1) ==
// to_eface(int64(1)) is true here and false in Go, and type_name<int64_t>() is "int". Nothing
// in the layout is wrong, but a comparison of two such interface values is. The emitter must
// keep distinct C++ types (or pass the descriptor explicitly) where identity matters until
// 16.2/16.8 generate one descriptor per Go type.
//
// Equality is Go's: two interface values are equal when their types are identical and their
// values are equal; comparing two interface values whose dynamic type is a slice, map or func
// panics with "runtime error: comparing uncomparable type <name>". A value stored in an
// interface is boxed in the GC heap (alloc for a value that holds a pointer, alloc_atomic for
// one that does not); no ARC type is involved.
#pragma once

#include <gocpp/gc/checks.hpp>
#include <gocpp/gc/chan.hpp>
#include <gocpp/gc/func.hpp>
#include <gocpp/gc/golayout.hpp>
#include <gocpp/gc/heap.hpp>
#include <gocpp/gc/map.hpp>
#include <gocpp/gc/slice.hpp>
#include <gocpp/gc/string.hpp>

#include <cstddef>
#include <cstdint>
#include <cstring>
#include <tuple>
#include <type_traits>
#include <utility>

namespace gp::gc {

// type_desc is the placeholder type descriptor (see the top of this file).
struct type_desc {
    std::uint64_t gp_size = 0;
    std::uint64_t gp_align = 1;
    bool gp_ptrs = false;
    bool gp_direct = false;
    bool gp_comparable = false;
    const char* gp_name = "";
    bool (*gp_equal)(const void*, const void*) = nullptr;
};

// itab is the placeholder method table of a non-empty interface: the dynamic type. Go's
// abi.ITab also holds the interface type, the hash and the method pointers; step 16.2/16.8 add
// them (one canonical itab per interface type and dynamic type, built at run time and cached,
// as gc's getitab does).
struct itab {
    const type_desc* gp_type = nullptr;
};

// eface is Go's empty interface (any): the type and the data word, 16 bytes.
class eface {
public:
    // nil: both words null.
    constexpr eface() noexcept = default;
    constexpr eface(const type_desc* type, void* data) noexcept : gp_type_(type), gp_data_(data) {}

    const type_desc* gp_type() const noexcept { return gp_type_; }
    void* gp_data() const noexcept { return gp_data_; }
    bool gp_nil() const noexcept { return gp_type_ == nullptr; }

private:
    const type_desc* gp_type_ = nullptr;
    void* gp_data_ = nullptr;
};

// iface is Go's non-empty interface: the itab and the data word, 16 bytes.
class iface {
public:
    // nil: both words null.
    constexpr iface() noexcept = default;
    constexpr iface(itab* tab, void* data) noexcept : gp_itab_(tab), gp_data_(data) {}

    itab* gp_itab() const noexcept { return gp_itab_; }
    void* gp_data() const noexcept { return gp_data_; }
    bool gp_nil() const noexcept { return gp_itab_ == nullptr; }

private:
    itab* gp_itab_ = nullptr;
    void* gp_data_ = nullptr;
};

static_assert(sizeof(eface) == 16, "gp::gc::eface must be Go's 16-byte interface");
static_assert(alignof(eface) == 8, "gp::gc::eface must be 8-byte aligned");
static_assert(sizeof(iface) == 16, "gp::gc::iface must be Go's 16-byte interface");
static_assert(alignof(iface) == 8, "gp::gc::iface must be 8-byte aligned");

namespace detail {

// The shape traits that the descriptor, equality and boxing use.
template <typename T>
struct is_slice_t : std::false_type {};
template <typename T>
struct is_slice_t<slice<T>> : std::true_type {};

template <typename T>
struct is_array_t : std::false_type {};
template <typename T, std::int64_t N>
struct is_array_t<array<T, N>> : std::true_type {
    using elem = T;
    static constexpr std::int64_t len = N;
};

template <typename T>
struct is_map_t : std::false_type {};
template <typename K, typename V>
struct is_map_t<map<K, V>> : std::true_type {};

template <typename T>
struct is_chan_t : std::false_type {};
template <typename T>
struct is_chan_t<chan<T>> : std::true_type {};

template <typename T>
struct is_func_t : std::false_type {};
template <typename Sig>
struct is_func_t<func<Sig>> : std::true_type {};

// is_zero_field_t: a zero-size field carries Go's comparability of its type (zero_field).
template <typename T>
struct is_zero_field_t : std::false_type {};
template <std::size_t I, std::size_t A, bool C>
struct is_zero_field_t<zero_field<I, A, C>> : std::true_type {
    static constexpr bool comparable = C;
};

// is_direct<T>: Go's isDirectIface, for the shapes this placeholder knows. The data word holds
// the value itself for a pointer, a map, a channel and a func.
template <typename T>
inline constexpr bool is_direct_v = false;
template <typename T>
inline constexpr bool is_direct_v<T*> = true;
template <typename K, typename V>
inline constexpr bool is_direct_v<map<K, V>> = true;
template <typename T>
inline constexpr bool is_direct_v<chan<T>> = true;
template <typename Sig>
inline constexpr bool is_direct_v<func<Sig>> = true;

template <typename T>
constexpr bool comparable_of() noexcept;

template <typename S, std::size_t... I>
constexpr bool struct_comparable(std::index_sequence<I...>) noexcept {
    return (comparable_of<typename std::tuple_element_t<
                I, std::remove_cvref_t<decltype(S::gp_fields)>>::gp_type>() &&
            ...);
}

template <typename T>
constexpr bool comparable_of() noexcept {
    if constexpr (std::is_arithmetic_v<T> || std::is_enum_v<T> || std::is_pointer_v<T> ||
                  std::is_null_pointer_v<T>) {
        return true;
    } else if constexpr (std::is_same_v<T, string> || std::is_same_v<T, eface> ||
                         std::is_same_v<T, iface>) {
        return true;
    } else if constexpr (is_chan_t<T>::value) {
        return true;
    } else if constexpr (is_slice_t<T>::value || is_map_t<T>::value || is_func_t<T>::value) {
        return false;
    } else if constexpr (is_array_t<T>::value) {
        return comparable_of<typename is_array_t<T>::elem>();
    } else if constexpr (go_struct_class<T>) {
        return struct_comparable<T>(
            std::make_index_sequence<
                std::tuple_size_v<std::remove_cvref_t<decltype(T::gp_fields)>>>{});
    } else if constexpr (is_zero_field_t<T>::value) {
        return is_zero_field_t<T>::comparable;
    } else if constexpr (std::is_empty_v<T>) {
        return true; // an empty Go struct
    } else {
        return requires(const T& a, const T& b) { a == b; };
    }
}

template <typename T>
inline constexpr bool is_comparable_v = comparable_of<T>();

// The Go name of a type, for messages. Approximate until step 16.8 writes the real names.
template <typename T>
struct type_name_of {
    static constexpr const char* value = "T";
};
template <>
struct type_name_of<bool> {
    static constexpr const char* value = "bool";
};
template <>
struct type_name_of<std::int8_t> {
    static constexpr const char* value = "int8";
};
template <>
struct type_name_of<std::int16_t> {
    static constexpr const char* value = "int16";
};
template <>
struct type_name_of<std::int32_t> {
    static constexpr const char* value = "int32";
};
template <>
struct type_name_of<std::int64_t> {
    static constexpr const char* value = "int";
};
template <>
struct type_name_of<std::uint8_t> {
    static constexpr const char* value = "uint8";
};
template <>
struct type_name_of<std::uint16_t> {
    static constexpr const char* value = "uint16";
};
template <>
struct type_name_of<std::uint32_t> {
    static constexpr const char* value = "uint32";
};
template <>
struct type_name_of<std::uint64_t> {
    static constexpr const char* value = "uint64";
};
template <>
struct type_name_of<float> {
    static constexpr const char* value = "float32";
};
template <>
struct type_name_of<double> {
    static constexpr const char* value = "float64";
};
template <>
struct type_name_of<string> {
    static constexpr const char* value = "string";
};
template <typename T>
struct type_name_of<slice<T>> {
    static constexpr const char* value = "[]T";
};
template <typename K, typename V>
struct type_name_of<map<K, V>> {
    static constexpr const char* value = "map[K]V";
};
template <typename T>
struct type_name_of<chan<T>> {
    static constexpr const char* value = "chan T";
};
template <typename Sig>
struct type_name_of<func<Sig>> {
    static constexpr const char* value = "func";
};
template <>
struct type_name_of<eface> {
    static constexpr const char* value = "interface {}";
};
template <>
struct type_name_of<iface> {
    static constexpr const char* value = "interface";
};

template <typename T>
constexpr const char* type_name() noexcept {
    return type_name_of<T>::value;
}

// direct_bits: the data word of a direct interface value.
template <typename T>
void* direct_bits(T* const& p) noexcept {
    return const_cast<void*>(static_cast<const void*>(p));
}
template <typename K, typename V>
void* direct_bits(const map<K, V>& m) noexcept {
    return m.gp_table();
}
template <typename T>
void* direct_bits(const chan<T>& c) noexcept {
    return c.gp_core();
}
template <typename Sig>
void* direct_bits(const func<Sig>& f) noexcept {
    return f.gp_closure();
}

} // namespace detail

// eface_equal, iface_equal: Go's equality of two interface values (runtime/alg.go's efaceeq and
// ifaceeq). A nil interface equals only a nil interface of the same kind. Comparing two values
// of an uncomparable dynamic type panics with Go's message.
bool eface_equal(const eface& a, const eface& b);
bool iface_equal(const iface& a, const iface& b);

inline bool operator==(const eface& a, const eface& b) {
    return eface_equal(a, b);
}
inline bool operator!=(const eface& a, const eface& b) {
    return !eface_equal(a, b);
}
inline bool operator==(const iface& a, const iface& b) {
    return iface_equal(a, b);
}
inline bool operator!=(const iface& a, const iface& b) {
    return !iface_equal(a, b);
}

namespace detail {

template <typename T>
bool value_equal(const T& a, const T& b);

template <typename S, std::size_t I>
bool field_equal(const S& a, const S& b) {
    const auto& f = std::get<I>(S::gp_fields);
    return value_equal(a.*(f.gp_member), b.*(f.gp_member));
}

template <typename S, std::size_t... I>
bool struct_equal(const S& a, const S& b, std::index_sequence<I...>) {
    return (field_equal<S, I>(a, b) && ...);
}

// value_equal: Go's == for a value of type T. A type that Go does not compare is a compile
// error (the descriptor marks it uncomparable first).
template <typename T>
bool value_equal(const T& a, const T& b) {
    if constexpr (std::is_arithmetic_v<T> || std::is_enum_v<T> || std::is_pointer_v<T>) {
        return a == b;
    } else if constexpr (std::is_same_v<T, string>) {
        return a == b;
    } else if constexpr (std::is_same_v<T, eface>) {
        return eface_equal(a, b);
    } else if constexpr (std::is_same_v<T, iface>) {
        return iface_equal(a, b);
    } else if constexpr (is_chan_t<T>::value) {
        return a == b;
    } else if constexpr (is_array_t<T>::value) {
        for (std::int64_t i = 0; i < is_array_t<T>::len; i++) {
            if (!value_equal(a[i], b[i])) {
                return false;
            }
        }
        return true;
    } else if constexpr (go_struct_class<T>) {
        return struct_equal(a, b,
                            std::make_index_sequence<
                                std::tuple_size_v<std::remove_cvref_t<decltype(T::gp_fields)>>>{});
    } else if constexpr (is_zero_field_t<T>::value) {
        static_assert(is_zero_field_t<T>::comparable,
                      "this zero-size field's Go type is not comparable");
        return true;
    } else if constexpr (std::is_empty_v<T>) {
        return true; // an empty Go struct
    } else {
        static_assert(is_comparable_v<T>, "this GC-engine type is not comparable");
        return a == b;
    }
}

// equal_fn is the function pointer in a type descriptor: Go's == for the boxed value.
template <typename T>
bool equal_fn(const void* a, const void* b) {
    return value_equal<T>(*static_cast<const T*>(a), *static_cast<const T*>(b));
}

// make_desc builds the placeholder descriptor of T.
template <typename T>
constexpr type_desc make_desc() noexcept {
    type_desc d;
    d.gp_size = go_layout<T>().size;
    d.gp_align = go_layout<T>().align;
    d.gp_ptrs = go_layout<T>().ptrs;
    d.gp_direct = is_direct_v<T>;
    d.gp_comparable = is_comparable_v<T>;
    d.gp_name = type_name<T>();
    if constexpr (!is_direct_v<T> && is_comparable_v<T>) {
        d.gp_equal = &equal_fn<T>;
    } else {
        d.gp_equal = nullptr;
    }
    return d;
}

} // namespace detail

// type_of<T>() is the descriptor of T: one constant per type in the program (an inline
// constexpr variable, so no dynamic initializer). Step 16.8's descriptors carry Go's names and
// equality functions; this one is enough for the runtime's own interfaces.
template <typename T>
inline constexpr type_desc type_of_v = detail::make_desc<T>();

template <typename T>
constexpr const type_desc* type_of() noexcept {
    return &type_of_v<T>;
}

// to_eface(v): the empty interface value that holds v. A value is copied into the GC heap; a
// pointer, map, channel or func goes into the data word itself. An interface value keeps its
// dynamic type and data (no double boxing), as in Go.
template <typename T>
eface to_eface(const T& v) {
    if constexpr (std::is_null_pointer_v<T>) {
        return eface();
    } else if constexpr (std::is_same_v<T, eface>) {
        return v;
    } else if constexpr (std::is_same_v<T, iface>) {
        return eface(v.gp_itab() == nullptr ? nullptr : v.gp_itab()->gp_type, v.gp_data());
    } else if constexpr (detail::is_direct_v<T>) {
        return eface(type_of<T>(), detail::direct_bits(v));
    } else {
        static_assert(sizeof(T) >= go_layout<T>().size,
                      "a GC-engine type's C++ size must cover Go's (golayout.hpp)");
        constexpr std::uint64_t size = go_layout<T>().size;
        void* p;
        if constexpr (size == 0) {
            p = detail::empty_data<void>();
        } else {
            p = go_layout<T>().ptrs ? process_heap().alloc(size)
                                    : process_heap().alloc_atomic(size);
            std::memcpy(p, static_cast<const void*>(&v), static_cast<std::size_t>(size));
        }
        return eface(type_of<T>(), p);
    }
}

// to_iface(tab, v): the non-empty interface value with the itab tab that holds v. The boxing is
// to_eface's; tab is the canonical itab of (the interface type, T) (step 16.8 builds them).
template <typename T>
iface to_iface(itab* tab, const T& v) {
    if constexpr (std::is_same_v<T, iface>) {
        return v.gp_nil() ? iface() : iface(tab, v.gp_data());
    } else if constexpr (std::is_same_v<T, eface>) {
        // A nil any converts to a nil non-empty interface, as in Go.
        return v.gp_nil() ? iface() : iface(tab, v.gp_data());
    } else if constexpr (detail::is_direct_v<T>) {
        return iface(tab, detail::direct_bits(v));
    } else {
        static_assert(sizeof(T) >= go_layout<T>().size,
                      "a GC-engine type's C++ size must cover Go's (golayout.hpp)");
        constexpr std::uint64_t size = go_layout<T>().size;
        void* p;
        if constexpr (size == 0) {
            p = detail::empty_data<void>();
        } else {
            p = go_layout<T>().ptrs ? process_heap().alloc(size)
                                    : process_heap().alloc_atomic(size);
            std::memcpy(p, static_cast<const void*>(&v), static_cast<std::size_t>(size));
        }
        return iface(tab, p);
    }
}

// make_itab: a placeholder itab for the type. Step 16.8 makes itabs canonical and generated.
inline itab* make_itab(const type_desc* type) {
    itab* t = new_<itab>();
    t->gp_type = type;
    return t;
}

// eface_equal and iface_equal: Go's equality. A value of an uncomparable dynamic type (a
// slice, map or func) panics with "runtime error: comparing uncomparable type <name>".
inline bool eface_equal(const eface& a, const eface& b) {
    const type_desc* ta = a.gp_type();
    const type_desc* tb = b.gp_type();
    if (ta == nullptr || tb == nullptr) {
        return ta == tb;
    }
    if (ta != tb) {
        return false;
    }
    if (!ta->gp_comparable) [[unlikely]] {
        panic_uncomparable(ta->gp_name);
    }
    if (ta->gp_direct) {
        return a.gp_data() == b.gp_data();
    }
    return ta->gp_equal(a.gp_data(), b.gp_data());
}

inline bool iface_equal(const iface& a, const iface& b) {
    const type_desc* ta = a.gp_itab() == nullptr ? nullptr : a.gp_itab()->gp_type;
    const type_desc* tb = b.gp_itab() == nullptr ? nullptr : b.gp_itab()->gp_type;
    if (ta == nullptr || tb == nullptr) {
        return ta == tb;
    }
    if (ta != tb) {
        return false;
    }
    if (!ta->gp_comparable) [[unlikely]] {
        panic_uncomparable(ta->gp_name);
    }
    if (ta->gp_direct) {
        return a.gp_data() == b.gp_data();
    }
    return ta->gp_equal(a.gp_data(), b.gp_data());
}

// equal(a, b): Go's == for a value, for the emitter. It is a compile error for a type that Go
// does not compare.
template <typename T>
bool equal(const T& a, const T& b) {
    return detail::value_equal(a, b);
}

} // namespace gp::gc
