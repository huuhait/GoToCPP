// gocpp/typeops.hpp: the "reflect-lite" operations of a type descriptor (Phase 05 step 5.11, ADR
// 027). A descriptor says WHAT a Go type is (its kind, its parts); the operations say how to READ
// a value of its C++ type without knowing the C++ type: the elements of a slice, the entries of
// a map, the target of a pointer. They are small functions that gp::detail::with_cxx<T>
// instantiates when a descriptor is made, from the C++ type T that the descriptor stands for, so
// the layouts of gp::slice, gp::map, gp::ptr and the rest stay private to the headers that
// define them. fmt (runtime/src/fmt.cpp) is the first user; Phase 09's reflect is the next.
//
// Every operation takes the address of the C++ object (a value of the descriptor's type). The
// ones that apply to a kind are set for that kind only; the others are null.
//
//     Slice, Array   gp_seq_data, gp_seq_len   the first element (the elements are contiguous
//                                              C++ objects of the element type) and the count
//     Slice          gp_hold                   the owner of the array, to keep it alive while
//                                              the elements print
//     Slice, Map, Pointer, Func   gp_nil       the value is nil
//     Map            gp_map_snap, gp_snap_len, gp_snap_at   a copy of the entries (keys and
//                                              values), and access to entry i of the copy
//     Pointer, UnsafePointer   gp_target, gp_owner   the address it points to and its owner
//     Func, Chan     gp_nil, gp_addr                   the address that fmt prints
//     Weak pointer   gp_word                   the address a weak pointer was made from
//     Foreign        gp_fclass, gp_go_name, gp_text   how a C/C++ value prints (below)
//
// A named type over one of them (a gp::newtype) is a class that derives from, or holds as its
// only member, the underlying class: its operations are the underlying type's, applied to the
// same address.
//
// Foreign (C/C++) types, kind Foreign: gp_fclass classifies the C++ type by its traits, so that
// fmt prints it as the Go type of the same kind and size (a C int as int32, docs/spec/interop.md
// "Printing C/C++ values"): a number, a bool, an enum, a raw pointer (the address), a gocpp
// pointer to a C++ type (cptr: the address), nullptr, std::string and std::string_view. gp_go_name
// is that Go type's name for %T and the error forms. A class type has gp_text only when a call of
// gp::cgo::fmt_arg registered its std::formatter or operator<< (gocpp/fmt.hpp).
#pragma once

#include <gocpp/array.hpp>
#include <gocpp/func.hpp>
#include <gocpp/map.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/slice.hpp>

#include <cstddef>
#include <cstdint>
#include <memory>
#include <string>
#include <string_view>
#include <type_traits>
#include <utility>
#include <vector>

namespace gp {

template <typename T>
class chan; // gocpp/chan.hpp (included after the descriptors)

// fclass classifies a C/C++ type (kind Foreign) for fmt.
enum class fclass : std::uint8_t {
    none,       // a class (or union, or anything else): gp_text if it has one, else its address
    sint,       // a signed integer of gp_size bytes
    uint_,      // an unsigned integer of gp_size bytes
    f32,        // float
    f64,        // double
    boolean,    // bool
    enum_s,     // an enum over a signed integer
    enum_u,     // an enum over an unsigned integer
    raw_ptr,    // T*, void*, a function pointer
    cptr,       // gp::cptr<T>
    cxx_string, // std::string
    cxx_string_view, // std::string_view
    null_ptr,        // std::nullptr_t
};

struct type_ops {
    // Slice and Array.
    const void* (*gp_seq_data)(const void*) = nullptr;
    std::int64_t (*gp_seq_len)(const void*) = nullptr;
    // Slice: the owner of the array.
    std::shared_ptr<void> (*gp_hold)(const void*) = nullptr;
    // Slice, Map, Pointer, Func: the value is nil.
    bool (*gp_nil)(const void*) = nullptr;
    // Map: a copy of the entries, as fmtsort.Sort makes one: gp_map_snap(m) is an opaque handle
    // (null for a nil map), gp_snap_len(h) the number of entries and gp_snap_at(h, i, &k, &v)
    // the addresses of entry i's key and value in the copy.
    std::shared_ptr<void> (*gp_map_snap)(const void*) = nullptr;
    std::int64_t (*gp_snap_len)(const void*) = nullptr;
    void (*gp_snap_at)(const void*, std::int64_t, const void**, const void**) = nullptr;
    // Pointer and UnsafePointer: the object is a gp::ptr<U>.
    const void* (*gp_target)(const void*) = nullptr;
    std::shared_ptr<void> (*gp_owner)(const void*) = nullptr;
    // Func (and Chan): an address that identifies the value.
    const void* (*gp_addr)(const void*) = nullptr;
    // A weak pointer (gp::weak_ptr<U>): the address it was made from, as a number.
    std::uintptr_t (*gp_word)(const void*) = nullptr;
    // Foreign.
    fclass gp_fclass = fclass::none;
    const char* gp_go_name = nullptr;
    void (*gp_text)(std::string&, const void*) = nullptr;
};

namespace detail {

// underlying_class<T>: the class under all the gp::newtype layers of T (T itself if it is no
// newtype).
template <typename T, typename = void>
struct underlying_class {
    using type = T;
};
template <typename T>
struct underlying_class<T, std::void_t<typename T::gp_underlying>> {
    using type = typename underlying_class<typename T::gp_underlying>::type;
};

template <typename E>
std::true_type is_weak_probe(const weak_ptr<E>*);
std::false_type is_weak_probe(...);

template <typename B>
struct ops_builder {
    static constexpr type_ops build() { return {}; }
};

template <typename E>
struct ops_builder<slice<E>> {
    static const slice<E>& as(const void* p) { return *static_cast<const slice<E>*>(p); }
    static const void* data(const void* p) { return as(p).gp_data(); }
    static std::int64_t len(const void* p) { return as(p).gp_len(); }
    static std::shared_ptr<void> hold(const void* p) { return as(p).gp_owner(); }
    static bool nil(const void* p) { return as(p) == nullptr; }
    static constexpr type_ops build() {
        type_ops o;
        o.gp_seq_data = &data;
        o.gp_seq_len = &len;
        o.gp_hold = &hold;
        o.gp_nil = &nil;
        return o;
    }
};

template <typename E, std::int64_t N>
struct ops_builder<array<E, N>> {
    static const void* data(const void* p) {
        if constexpr (N > 0) {
            return static_cast<const array<E, N>*>(p)->gp_elems.data();
        } else {
            return p;
        }
    }
    static std::int64_t len(const void*) { return N; }
    static constexpr type_ops build() {
        type_ops o;
        o.gp_seq_data = &data;
        o.gp_seq_len = &len;
        return o;
    }
};

template <typename K, typename V>
struct ops_builder<map<K, V>> {
    using entries = std::vector<std::pair<K, V>>;
    static const map<K, V>& as(const void* p) { return *static_cast<const map<K, V>*>(p); }
    static bool nil(const void* p) { return as(p) == nullptr; }
    static std::shared_ptr<void> snap(const void* p) {
        auto out = std::make_shared<entries>();
        if (const auto* impl = as(p).gp_get(); impl != nullptr) {
            out->reserve(impl->gp_t.size());
            for (const auto& e : impl->gp_t) {
                out->emplace_back(e.first.gp_k, e.second);
            }
        }
        return out;
    }
    static std::int64_t snap_len(const void* h) {
        return static_cast<std::int64_t>(static_cast<const entries*>(h)->size());
    }
    static void snap_at(const void* h, std::int64_t i, const void** k, const void** v) {
        const auto& e = (*static_cast<const entries*>(h))[static_cast<std::size_t>(i)];
        *k = &e.first;
        *v = &e.second;
    }
    static constexpr type_ops build() {
        type_ops o;
        o.gp_nil = &nil;
        if constexpr (std::is_copy_constructible_v<K> && std::is_copy_constructible_v<V>) {
            o.gp_map_snap = &snap;
            o.gp_snap_len = &snap_len;
            o.gp_snap_at = &snap_at;
        }
        return o;
    }
};

template <typename E>
struct ops_builder<ptr<E>> {
    static const ptr<E>& as(const void* p) { return *static_cast<const ptr<E>*>(p); }
    static const void* target(const void* p) { return as(p).gp_get(); }
    static std::shared_ptr<void> owner(const void* p) { return as(p).gp_shared(); }
    static bool nil(const void* p) { return as(p).gp_get() == nullptr; }
    static constexpr type_ops build() {
        type_ops o;
        o.gp_target = &target;
        o.gp_owner = &owner;
        o.gp_nil = &nil;
        return o;
    }
};

template <typename E>
struct ops_builder<cptr<E>> : ops_builder<ptr<E>> {};

template <typename S>
struct ops_builder<func<S>> {
    static const func<S>& as(const void* p) { return *static_cast<const func<S>*>(p); }
    static bool nil(const void* p) { return as(p) == nullptr; }
    static const void* addr(const void* p) {
        return reinterpret_cast<const void*>(reinterpret_cast<std::uintptr_t>(as(p).gp_code()));
    }
    static constexpr type_ops build() {
        type_ops o;
        o.gp_nil = &nil;
        o.gp_addr = &addr;
        return o;
    }
};

template <typename E>
struct ops_builder<chan<E>> {
    static const chan<E>& as(const void* p) { return *static_cast<const chan<E>*>(p); }
    static bool nil(const void* p) { return as(p) == nullptr; }
    static const void* addr(const void* p) {
        return reinterpret_cast<const void*>(as(p).gp_address());
    }
    static constexpr type_ops build() {
        type_ops o;
        o.gp_nil = &nil;
        o.gp_addr = &addr;
        return o;
    }
};

template <typename E>
struct ops_builder<weak_ptr<E>> {
    static std::uintptr_t word(const void* p) {
        return static_cast<const weak_ptr<E>*>(p)->gp_addr();
    }
    static constexpr type_ops build() {
        type_ops o;
        o.gp_word = &word;
        return o;
    }
};

// The parts of a runtime class under a named type: gp::detail::elem_t<Items> is E for `type Items
// []E`, `type A [3]E`, `type P *E` and `type M map[K]E`, and key_t<M> is K. The emitter names them
// in the complete descriptor of a named type with a C/C++ element (internal/lower/typefull.go).
template <typename B>
struct class_parts {};
template <typename E>
struct class_parts<slice<E>> {
    using elem = E;
};
template <typename E, std::int64_t N>
struct class_parts<array<E, N>> {
    using elem = E;
};
template <typename E>
struct class_parts<ptr<E>> {
    using elem = E;
};
template <typename E>
struct class_parts<cptr<E>> {
    using elem = E;
};
template <typename E>
struct class_parts<chan<E>> {
    using elem = E;
};
template <typename K, typename V>
struct class_parts<map<K, V>> {
    using key = K;
    using elem = V;
};
template <typename T>
using elem_t = typename class_parts<typename underlying_class<T>::type>::elem;
template <typename T>
using key_t = typename class_parts<typename underlying_class<T>::type>::key;

// ops_of<T>() is the operations of the type T (through its newtype layers).
template <typename T>
constexpr type_ops ops_of() noexcept {
    return ops_builder<typename underlying_class<T>::type>::build();
}

// A descriptor points at a static object; one per C++ type.
template <typename T>
inline constexpr type_ops ops_object = ops_of<T>();

// The Go name of a C/C++ number or bool for %T (sizeof says the width; the 64-bit int64_t and
// uint64_t are Go's int and uint, because C++ can't tell them from them; any other 8-byte
// integer is int64 or uint64).
template <typename T>
constexpr const char* foreign_go_name() noexcept {
    if constexpr (std::is_same_v<T, bool>) {
        return "bool";
    } else if constexpr (std::is_integral_v<T> && std::is_signed_v<T>) {
        return sizeof(T) == 1               ? "int8"
               : sizeof(T) == 2             ? "int16"
               : sizeof(T) == 4             ? "int32"
               : std::is_same_v<T, int64_t> ? "int"
                                            : "int64";
    } else if constexpr (std::is_integral_v<T>) {
        return sizeof(T) == 1                ? "uint8"
               : sizeof(T) == 2              ? "uint16"
               : sizeof(T) == 4              ? "uint32"
               : std::is_same_v<T, uint64_t> ? "uint"
                                             : "uint64";
    } else if constexpr (std::is_same_v<T, float>) {
        return "float32";
    } else if constexpr (std::is_same_v<T, double>) {
        return "float64";
    } else if constexpr (std::is_same_v<T, std::string>) {
        return "std::string";
    } else if constexpr (std::is_same_v<T, std::string_view>) {
        return "std::string_view";
    } else if constexpr (std::is_null_pointer_v<T>) {
        return "<nil>"; // as Go's %T of a nil interface
    } else {
        return nullptr;
    }
}

template <typename T>
struct is_cptr : std::false_type {};
template <typename E>
struct is_cptr<cptr<E>> : std::true_type {};

// foreign_ops_of<U>() classifies the C/C++ type U.
template <typename U>
type_ops foreign_ops_of() {
    type_ops o;
    if constexpr (std::is_same_v<U, bool>) {
        o.gp_fclass = fclass::boolean;
    } else if constexpr (std::is_integral_v<U>) {
        o.gp_fclass = std::is_signed_v<U> ? fclass::sint : fclass::uint_;
    } else if constexpr (std::is_same_v<U, float>) {
        o.gp_fclass = fclass::f32;
    } else if constexpr (std::is_same_v<U, double>) {
        o.gp_fclass = fclass::f64;
    } else if constexpr (std::is_enum_v<U>) {
        o.gp_fclass = std::is_signed_v<std::underlying_type_t<U>> ? fclass::enum_s : fclass::enum_u;
    } else if constexpr (std::is_pointer_v<U>) {
        o.gp_fclass = fclass::raw_ptr;
    } else if constexpr (is_cptr<U>::value) {
        o = ops_of<U>();
        o.gp_fclass = fclass::cptr;
    } else if constexpr (std::is_same_v<U, std::string>) {
        o.gp_fclass = fclass::cxx_string;
    } else if constexpr (std::is_same_v<U, std::string_view>) {
        o.gp_fclass = fclass::cxx_string_view;
    } else if constexpr (std::is_null_pointer_v<U>) {
        o.gp_fclass = fclass::null_ptr;
    }
    o.gp_go_name = foreign_go_name<U>();
    return o;
}

} // namespace detail

} // namespace gp
