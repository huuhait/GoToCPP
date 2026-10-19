// gocpp/golayout.hpp: Go's layout of the Go type that a C++ type stands for: its size, its
// alignment and whether it holds a pointer, as gc lays it out on 64-bit targets. The sizes of
// the C++ types differ from Go's (a gp::string is 32 bytes, Go's string 16; a gp::ptr 16, Go's
// 8: divergence D15), but the capacity that append gives a slice depends on Go's own: gc rounds
// the new array up to a malloc size class, and to one that has an 8-byte header when the
// elements hold pointers (runtime/msize.go, roundupsize). gp::detail::grow_cap (slice.hpp) is
// given what go_layout<T>() says.
//
//     numbers, bools, enums           their C++ size; no pointer
//     string                          16 bytes, a pointer
//     *T, unsafe.Pointer, weak.Pointer, raw pointers   8 bytes, a pointer
//     []T                             24 bytes; map, chan, func: 8 bytes; interface: 16 bytes
//     [N]T                            N * size of T (N = 0: size 0, no pointer)
//     a standard struct that differs from Go's   gp_go_layout, written by the emitter
//                                     (internal/lower/golayouts.go: real Go's numbers)
//     a struct of the emitter         its fields (gp_fields) laid out as gc does, with gc's
//                                     one byte of padding after a zero-size last field
//     a named type (gp::newtype)      the layout of its underlying type
//     any other C/C++ class (a type from C or C++ only; every Go type has a layout above)
//                                     its C++ size; a pointer unless it is trivially copyable
//
// Everything is computed at compile time from the C++ type, so the emitter writes nothing for
// it. The element type must be complete. Like every runtime type, nothing here is a member of
// a user's type: they are free templates of gp::detail.
#pragma once

#include <gocpp/ptr.hpp>
#include <gocpp/string.hpp>

#include <cstddef>
#include <cstdint>
#include <tuple>
#include <type_traits>
#include <utility>

namespace gp {

template <typename T>
class slice;
template <typename K, typename V>
class map;
template <typename T>
class chan;
template <typename Sig>
class func;
template <typename T, int64_t N>
struct array;
class iface;

namespace detail {

struct go_layout_t {
    uint64_t size = 0;
    uint64_t align = 1;
    bool ptrs = false; // the type holds a pointer (gc's Pointers(): not "noscan")
};

// The gocpp classes with a fixed Go layout. A function only for overload resolution: a class
// derived from one of them (a named type, gp::cptr, an interface type) converts to its base.
constexpr go_layout_t go_known(const gp::string*) noexcept {
    return {16, 8, true};
}
template <typename E>
constexpr go_layout_t go_known(const ptr<E>*) noexcept {
    return {8, 8, true};
}
template <typename E>
constexpr go_layout_t go_known(const weak_ptr<E>*) noexcept {
    return {8, 8, true};
}
template <typename E>
constexpr go_layout_t go_known(const slice<E>*) noexcept {
    return {24, 8, true};
}
template <typename K, typename V>
constexpr go_layout_t go_known(const map<K, V>*) noexcept {
    return {8, 8, true};
}
template <typename E>
constexpr go_layout_t go_known(const chan<E>*) noexcept {
    return {8, 8, true};
}
template <typename Sig>
constexpr go_layout_t go_known(const func<Sig>*) noexcept {
    return {8, 8, true};
}
constexpr go_layout_t go_known(const iface*) noexcept {
    return {16, 8, true};
}

template <typename T>
concept go_known_class = requires(const T* p) { detail::go_known(p); };

template <typename T>
constexpr go_layout_t go_layout() noexcept;

template <typename E, int64_t N>
constexpr go_layout_t go_array_layout(const array<E, N>*) noexcept {
    go_layout_t e = detail::go_layout<E>();
    if constexpr (N == 0) {
        return {0, e.align, false}; // [0]T takes no space (gc), but is aligned as a T
    } else {
        return {e.size * static_cast<uint64_t>(N), e.align, e.ptrs};
    }
}

template <typename T>
concept go_array_class = requires(const T* p) { detail::go_array_layout(p); };

// gc's struct layout (go/types gcSizes): each field at the next multiple of its alignment; the
// size is rounded up to the largest alignment; a zero-size last field of a struct that has an
// offset above zero gets a byte, so that a pointer to it stays inside the object.
template <typename S, std::size_t... I>
constexpr go_layout_t go_struct_layout(std::index_sequence<I...>) noexcept {
    uint64_t off = 0, last_off = 0, last_size = 0, align = 1;
    bool ptrs = false;
    [[maybe_unused]] auto field = [&](go_layout_t f) {
        off = (off + f.align - 1) / f.align * f.align;
        last_off = off;
        last_size = f.size;
        off += f.size;
        if (f.align > align) {
            align = f.align;
        }
        ptrs = ptrs || f.ptrs;
    };
    (field(detail::go_layout<typename std::tuple_element_t<
               I, std::remove_cvref_t<decltype(S::gp_fields)>>::gp_type>()),
     ...);
    if (sizeof...(I) == 0) {
        return {0, 1, false};
    }
    uint64_t size = last_off + (last_off > 0 && last_size == 0 ? 1 : last_size);
    return {(size + align - 1) / align * align, align, ptrs};
}

template <typename T>
concept go_struct_class = requires { T::gp_fields; };

// A struct that real Go lays out differently from its fields (a standard type whose gocpp
// declaration has other fields: sync.Cond, time.Timer, a stub that leaves fields out) says so:
// the emitter writes gp_go_layout (internal/lower/golayouts.go).
template <typename T>
concept go_declared_layout = requires { T::gp_go_layout; };

// A named type over a number or a bool (gp::newtype) has gp_value(): the layout of the value.
template <typename T>
concept go_named_value = requires(const T& v) { v.gp_value(); };

template <typename T>
constexpr go_layout_t go_layout() noexcept {
    if constexpr (std::is_arithmetic_v<T> || std::is_enum_v<T>) {
        return {sizeof(T), alignof(T), false};
    } else if constexpr (std::is_pointer_v<T> || std::is_null_pointer_v<T>) {
        return {8, 8, true};
    } else if constexpr (go_declared_layout<T>) {
        return T::gp_go_layout;
    } else if constexpr (go_known_class<T>) {
        return detail::go_known(static_cast<const T*>(nullptr));
    } else if constexpr (go_array_class<T>) {
        return detail::go_array_layout(static_cast<const T*>(nullptr));
    } else if constexpr (go_struct_class<T>) {
        return detail::go_struct_layout<T>(
            std::make_index_sequence<
                std::tuple_size_v<std::remove_cvref_t<decltype(T::gp_fields)>>>{});
    } else if constexpr (go_named_value<T>) {
        return detail::go_layout<
            std::remove_cvref_t<decltype(std::declval<const T&>().gp_value())>>();
    } else if constexpr (std::is_class_v<T> && std::is_base_of_v<iface, T>) {
        return {16, 8, true};
    } else {
        // A C/C++ class (gp::complex, a C struct, std::string): its own size.
        return {sizeof(T), alignof(T), !std::is_trivially_copyable_v<T>};
    }
}

} // namespace detail

} // namespace gp
