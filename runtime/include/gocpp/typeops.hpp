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
//     every kind     gp_trivial, gp_ctor, gp_dtor, gp_share   the lifetime of a C++ object of the
//     type
//                                              (reflect.New, Zero, MakeSlice: Phase 09 step 9.3)
//     Slice, Map, Pointer, Chan   gp_refl      the extra operations of reflect (slice_refl,
//     map_refl,
//                                              ptr_refl, chan_refl below): one pointer, so a type
//                                              pays only for the group of its own kind
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
#include <concepts>
#include <memory>
#include <new>
#include <string>
#include <string_view>
#include <type_traits>
#include <utility>
#include <vector>

namespace gp {

template <typename T>
class chan; // gocpp/chan.hpp (included after the descriptors)

namespace detail {
struct chan_core;
std::int64_t chan_len(const chan_core* c) noexcept; // gocpp/chan.hpp
std::int64_t chan_cap(const chan_core* c) noexcept;
} // namespace detail

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

// The reflect operations (Phase 09 step 9.3, runtime/src/reflect.cpp). They are typed (a template
// instantiates them for the element, key and value types) and reached through type_ops::gp_refl,
// which points at the group of the type's kind.
//
// slice_refl: gp_make(obj, owner, data, len, cap) assigns to the slice object `obj` the slice of
// that array view (E is the element type of the object); gp_cap reads the capacity.
struct slice_refl {
    void (*gp_make)(void* obj, const std::shared_ptr<void>& owner, void* data, std::int64_t len,
                    std::int64_t cap);
    std::int64_t (*gp_cap)(const void*);
};

// ptr_refl: gp_make(obj, owner, target) assigns to the pointer object `obj` a pointer to target
// that shares owner's block (the aliasing constructor).
struct ptr_refl {
    void (*gp_make)(void* obj, const std::shared_ptr<void>& owner, void* target);
};

// map_refl: the map as reflect needs it. gp_make assigns a new empty map to `obj`; gp_find is the
// address of the value of the key (null when it isn't there: a NaN is never), gp_set stores a copy
// of the value (a nil map panics as Go's store does), gp_erase and gp_clear are delete and clear.
struct map_refl {
    void (*gp_make)(void* obj);
    std::int64_t (*gp_len)(const void*);
    const void* (*gp_find)(const void*, const void* key);
    void (*gp_set)(const void*, const void* key, const void* val);
    void (*gp_erase)(const void*, const void* key);
    void (*gp_clear)(const void*);
};

// chan_refl: len(ch) and cap(ch) (a nil channel has 0 of both).
struct chan_refl {
    std::int64_t (*gp_len)(const void*);
    std::int64_t (*gp_cap)(const void*);
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
    // Map: gp_map_clone(m) is a new object of the map's own C++ type (a named map type too) that
    // holds a copy of the map (gp::map_clone), as the owner of what an interface holds
    // (maps.Clone).
    std::shared_ptr<void> (*gp_map_clone)(const void*) = nullptr;
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
    // A C++ class with operator<: gp_cmp(a, b) is -1, 0 or +1 by it, the order in which fmt
    // prints such map keys (fmtsort; a C++ string compares by its compare).
    int (*gp_cmp)(const void*, const void*) = nullptr;
    // The lifetime of a C++ object of the type (reflect: New, Zero, MakeSlice, Set). gp_trivial:
    // the zero value is all-zero bytes, a copy is memcpy and there is nothing to destroy (numbers,
    // bools, plain structs of them): gp_ctor and gp_dtor are null then. Otherwise gp_ctor
    // constructs the zero value in raw memory and gp_dtor destroys the object (null: nothing to
    // do); a null gp_ctor means the type can't be made without its C++ constructors (reflect says
    // so). gp_share: the type is exactly gp::ptr<U>: an interface that holds one shares the
    // pointer's owner as its data instead of boxing a copy (gocpp/iface.hpp).
    bool gp_trivial = false;
    bool gp_share = false;
    void (*gp_ctor)(void*) = nullptr;
    void (*gp_dtor)(void*) = nullptr;
    // Slice, Map, Pointer: the slice_refl, map_refl or ptr_refl of the type.
    const void* gp_refl = nullptr;
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
struct slice_refl_builder {
    static void make(void* obj, const std::shared_ptr<void>& owner, void* data, std::int64_t len,
                     std::int64_t cap) {
        *static_cast<slice<E>*>(obj) = slice<E>(owner, static_cast<E*>(data), len, cap);
    }
    static std::int64_t cap(const void* p) { return static_cast<const slice<E>*>(p)->gp_cap(); }
};
template <typename E>
inline constexpr slice_refl slice_refl_object{&slice_refl_builder<E>::make,
                                              &slice_refl_builder<E>::cap};

template <typename E>
struct ptr_refl_builder {
    static void make(void* obj, const std::shared_ptr<void>& owner, void* target) {
        *static_cast<ptr<E>*>(obj) = ptr<E>(owner, static_cast<E*>(target));
    }
};
template <typename E>
inline constexpr ptr_refl ptr_refl_object{&ptr_refl_builder<E>::make};

// map_reflable: the map's key and value support what map_refl does.
template <typename K, typename V>
concept map_reflable = std::is_copy_constructible_v<K> && std::is_copy_constructible_v<V> &&
                       std::is_move_assignable_v<V> && requires(const K& k) {
                           { gp::hash_of(k) } -> std::convertible_to<std::size_t>;
                       };

template <typename K, typename V>
struct map_refl_builder {
    static const map<K, V>& as(const void* p) { return *static_cast<const map<K, V>*>(p); }
    static void make(void* obj) { *static_cast<map<K, V>*>(obj) = gp::make_map<K, V>(); }
    static std::int64_t len(const void* m) { return gp::len(as(m)); }
    static const void* find(const void* m, const void* k) {
        auto* impl = as(m).gp_get();
        const K& key = *static_cast<const K*>(k);
        detail::check_empty_key(impl, key);
        if (impl == nullptr) {
            return nullptr;
        }
        auto* c = detail::map_find(*impl, key);
        return c == nullptr ? nullptr : static_cast<const void*>(&c->gp_v);
    }
    static void set(const void* m, const void* k, const void* v) {
        gp::set(as(m), *static_cast<const K*>(k), *static_cast<const V*>(v));
    }
    static void erase(const void* m, const void* k) { gp::erase(as(m), *static_cast<const K*>(k)); }
    static void clear(const void* m) { gp::clear(as(m)); }
};
template <typename K, typename V>
inline constexpr map_refl map_refl_object{
    &map_refl_builder<K, V>::make, &map_refl_builder<K, V>::len,   &map_refl_builder<K, V>::find,
    &map_refl_builder<K, V>::set,  &map_refl_builder<K, V>::erase, &map_refl_builder<K, V>::clear};

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
        o.gp_refl = &slice_refl_object<E>;
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
            out->reserve(impl->gp_size);
            impl->gp_each([&](const K& k, const V& v) { out->emplace_back(k, v); });
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
        if constexpr (map_reflable<K, V>) {
            o.gp_refl = &map_refl_object<K, V>;
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
        o.gp_refl = &ptr_refl_object<E>;
        return o;
    }
};

// cptr<E> is read like a gp::ptr<E>, but reflect does not make one (a C++ pointer type is opaque).
template <typename E>
struct ops_builder<cptr<E>> : ops_builder<ptr<E>> {
    static constexpr type_ops build() {
        type_ops o = ops_builder<ptr<E>>::build();
        o.gp_refl = nullptr;
        return o;
    }
};

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
    static std::int64_t length(const void* p) {
        const auto* c = as(p).gp_core();
        return c == nullptr ? 0 : detail::chan_len(c);
    }
    static std::int64_t capacity(const void* p) {
        const auto* c = as(p).gp_core();
        return c == nullptr ? 0 : detail::chan_cap(c);
    }
    static constexpr type_ops build() {
        type_ops o;
        o.gp_nil = &nil;
        o.gp_addr = &addr;
        o.gp_refl = &refl;
        return o;
    }
    static constexpr chan_refl refl{&length, &capacity};
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

// The lifetime operations (type_ops::gp_trivial and the rest). T is the C++ type of the descriptor
// itself (not its underlying class: a struct's own constructor and destructor run).
template <typename T>
void construct_zero(void* p) {
    ::new (p) T();
}
template <typename T>
void destroy_object(void* p) noexcept {
    static_cast<T*>(p)->~T();
}
template <typename T>
inline constexpr bool exact_ptr = false;
template <typename U>
inline constexpr bool exact_ptr<ptr<U>> = true;

template <typename T>
constexpr void set_lifetime(type_ops& o) noexcept {
    if constexpr (std::is_object_v<T> && !std::is_const_v<T>) {
        if constexpr (std::is_trivially_copyable_v<T> &&
                      std::is_trivially_default_constructible_v<T>) {
            o.gp_trivial = true;
        } else {
            if constexpr (std::is_default_constructible_v<T> && std::is_destructible_v<T>) {
                o.gp_ctor = &construct_zero<T>;
            }
            if constexpr (std::is_destructible_v<T> && !std::is_trivially_destructible_v<T>) {
                o.gp_dtor = &destroy_object<T>;
            }
        }
        o.gp_share = exact_ptr<T>;
    }
}

// map_clone_object<T, K, V>: for the map type T (a gp::map<K, V> or a named type over one) a new T
// that holds a copy of the map at p.
template <typename T, typename K, typename V>
std::shared_ptr<void> map_clone_object(const void* p) {
    const map<K, V>& src = *static_cast<const T*>(p);
    if constexpr (std::is_same_v<T, map<K, V>>) {
        return std::make_shared<map<K, V>>(gp::map_clone(src));
    } else {
        return std::make_shared<T>(T(gp::map_clone(src)));
    }
}

template <typename T, typename U>
constexpr void set_map_clone(type_ops&, const U*) noexcept {}
template <typename T, typename K, typename V>
constexpr void set_map_clone(type_ops& o, const map<K, V>*) noexcept {
    if constexpr (std::is_copy_constructible_v<K> && std::is_copy_constructible_v<V>) {
        o.gp_map_clone = &map_clone_object<T, K, V>;
    }
}

// ops_of<T>() is the operations of the type T (through its newtype layers).
template <typename T>
constexpr type_ops ops_of() noexcept {
    type_ops o = ops_builder<typename underlying_class<T>::type>::build();
    set_lifetime<T>(o);
    set_map_clone<T>(o, static_cast<typename underlying_class<T>::type*>(nullptr));
    return o;
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

// less_compare<U>(a, b) orders two U by U's operator< (type_ops::gp_cmp).
template <typename U>
int less_compare(const void* a, const void* b) {
    const U& x = *static_cast<const U*>(a);
    const U& y = *static_cast<const U*>(b);
    return x < y ? -1 : y < x ? 1 : 0;
}

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
    if constexpr (std::is_class_v<U> && requires(const U& a, const U& b) {
                      { a < b } -> std::convertible_to<bool>;
                  }) {
        o.gp_cmp = &less_compare<U>;
    }
    set_lifetime<U>(o);
    // A gp::map with a C/C++ key or value (map[int]std.string in an instance of maps.Clone, whose
    // any(m) is a foreign value): maps.Clone copies it as it copies a Go map.
    set_map_clone<U>(o, static_cast<U*>(nullptr));
    return o;
}

} // namespace detail

} // namespace gp
