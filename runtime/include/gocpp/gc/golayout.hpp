// gocpp/gc/golayout.hpp: Go's layout of a GC-engine type, and the helpers that make a C++
// struct laid out exactly as gc lays out the Go struct it stands for (phase-16 README §2.2;
// job 16.1e).
//
// The GC engine's values have Go's sizes and offsets, because Go code reads them through
// unsafe and because reflect and Go's runtime depend on them. C++ almost gives that for free:
// both place fields in order at the next multiple of their alignment and round the size up to
// the struct's alignment. The differences are the zero-size types, and this file carries them:
//
//   - struct{} is 0 bytes in Go and 1 in C++. A zero-size *field* is a
//     [[no_unique_address]] member of a distinct empty type, gp::gc::zero_field<I, Align>: the
//     distinct types are what lets several of them share an address (C++ gives two members of
//     the same type distinct addresses). Align is the Go type's alignment, so [0]int64 is
//     zero_field<I, 8> and struct{} is zero_field<I>.
//   - Two zero-size fields have the same offset in Go; in C++ a [[no_unique_address]] member's
//     offset is not constrained (both g++ and clang++ put it at offset 0). It has no data, so
//     its offset is not observable in a Go program (Go allows two distinct zero-size variables
//     to have the same address, and unsafe.Offsetof is a constant that go/types computes with
//     Go's numbers, not with C++'s). The emitter checks the offsets of the fields that occupy
//     bytes (GP_GC_FIELD_AT) and the struct's size and alignment (GP_GC_LAYOUT).
//   - A struct whose last field is zero-size gets one byte of padding in Go when its size is
//     above zero, so that a pointer to that field stays inside the object. The emitter adds it
//     with gp::gc::padding<1>.
//   - Go aligns a zero-size field like its type, which can leave a gap before it ([0]int64
//     after an int32 starts at offset 8). The emitter adds that gap as gp::gc::padding<N>.
//
// What the emitter writes for a Go struct (internal/cppgen/gc.go, step 16.1f; the test
// runtime/test/gc_layout_test.cpp is generated from real Go's numbers and is the example):
//
//     struct Point {
//         int64_t X;
//         int64_t Y;
//         static constexpr auto gp_fields = gp::gc::fields(
//             gp::gc::field(&Point::X, "X"), gp::gc::field(&Point::Y, "Y"));
//     };
//     GP_GC_LAYOUT(Point, 16, 8);
//     GP_GC_FIELD_AT(Point, X, 0);
//     GP_GC_FIELD_AT(Point, Y, 8);
//
// and for `type T struct { A int32; Z [0]int64; B int32 }` (Go: A at 0, Z at 8, B at 8, size
// 16):
//
//     struct T {
//         int32_t A;
//         gp::gc::padding<4> gp_pad0;
//         [[no_unique_address]] gp::gc::zero_field<0, 8> Z;
//         int32_t B;
//         static constexpr auto gp_fields = gp::gc::fields(
//             gp::gc::field(&T::A, "A"), gp::gc::field(&T::Z, "Z"), gp::gc::field(&T::B, "B"));
//     };
//     GP_GC_LAYOUT(T, 16, 8);
//     GP_GC_FIELD_AT(T, A, 0);
//     GP_GC_FIELD_AT(T, B, 8);
//
// The gp_fields tuple is what lets the runtime know the Go layout without asking the C++
// compiler for field offsets: go_layout<S>() lays the tuple's types out with gc's rules, which
// gives the element stride of []S and [N]S and whether S holds a pointer. It is also what
// equality and (step 16.8) reflect read: each field carries its member pointer.
//
// Like every runtime type, nothing here is a member of a user's type: they are free templates
// of gp::gc.
#pragma once

#include <cstddef>
#include <cstdint>
#include <tuple>
#include <type_traits>
#include <utility>

namespace gp::gc {

template <typename T>
class slice;
template <typename T, std::int64_t N>
struct array;
class string;
class iface;
class eface;
template <typename K, typename V>
class map;
template <typename T>
class chan;
template <typename Sig>
class func;

// go_layout_t is Go's size, alignment and pointer-ness of a type, as gc gives it on 64-bit
// targets. ptrs is gc's "not noscan": the type holds a pointer somewhere, so an allocation of
// it is scanned and its size class can carry gc's malloc header.
struct go_layout_t {
    std::uint64_t size = 0;
    std::uint64_t align = 1;
    bool ptrs = false;
};

// zero_field<I, Align, Comparable> is the C++ member of a Go field whose type is zero-size:
// struct{}, [0]T, a named empty struct, sync.noCopy. I makes each one a distinct type, so
// several of them can share an address ([[no_unique_address]]); Align is the Go type's
// alignment, which decides where Go puts the field (and any gap before it); Comparable is
// Go's comparability of the field's type, because the type itself is gone ([0]func() makes
// log/slog.Value uncomparable, while struct{} and [0]*T are comparable). It has no data:
// gp_fields records its Go type as {0, Align, false}.
template <std::size_t I, std::size_t Align = 1, bool Comparable = true>
struct alignas(Align) zero_field {};

// padding<N> is N bytes of explicit padding: the gap before a field whose Go offset is above
// the running offset (a zero-size field with alignment), and Go's one byte after a trailing
// zero-size field of a struct whose size is above zero. It is never a field of gp_fields.
template <std::size_t N>
struct padding {
    unsigned char gp_bytes[N];
};

// field<M C::*, name> is one Go field of a struct: its C++ member and its Go name. The name is
// for reflect and error messages (step 16.8); the runtime's layout only reads gp_type.
template <typename C, typename M>
struct field_info {
    using gp_type = M;
    M C::* gp_member;
    const char* gp_name;
};

template <typename C, typename M>
constexpr field_info<C, M> field(M C::* member, const char* name = "") noexcept {
    return {member, name};
}

template <typename... F>
constexpr std::tuple<F...> fields(F... fs) noexcept {
    return std::tuple<F...>(fs...);
}

namespace detail {

// The GC engine's runtime classes with a fixed Go layout. A function only for overload
// resolution: a class derived from one of them (a named type) converts to its base.
constexpr go_layout_t go_known(const string*) noexcept {
    return {16, 8, true};
}
template <typename T>
constexpr go_layout_t go_known(const slice<T>*) noexcept {
    return {24, 8, true};
}
template <typename K, typename V>
constexpr go_layout_t go_known(const map<K, V>*) noexcept {
    return {8, 8, true};
}
template <typename T>
constexpr go_layout_t go_known(const chan<T>*) noexcept {
    return {8, 8, true};
}
template <typename Sig>
constexpr go_layout_t go_known(const func<Sig>*) noexcept {
    return {8, 8, true};
}
constexpr go_layout_t go_known(const iface*) noexcept {
    return {16, 8, true};
}
constexpr go_layout_t go_known(const eface*) noexcept {
    return {16, 8, true};
}
template <std::size_t I, std::size_t A, bool C>
constexpr go_layout_t go_known(const zero_field<I, A, C>*) noexcept {
    return {0, A, false};
}

template <typename T>
constexpr go_layout_t go_layout() noexcept;

// Go's layout of [N]T: N times the element's size, with its alignment and pointer-ness; N == 0
// takes no space but is aligned as a T.
template <typename T, std::int64_t N>
constexpr go_layout_t go_known(const array<T, N>*) noexcept {
    go_layout_t e = detail::go_layout<T>();
    if constexpr (N == 0) {
        return {0, e.align, false};
    } else {
        return {e.size * static_cast<std::uint64_t>(N), e.align, e.ptrs};
    }
}

template <typename T>
concept go_known_class = requires(const T* p) { detail::go_known(p); };

// gc's struct layout (go/types gcSizes): each field at the next multiple of its alignment; the
// size rounded up to the largest alignment; a zero-size last field of a struct whose offset is
// above zero gets a byte, so that a pointer to it stays inside the object.
template <typename S, std::size_t... I>
constexpr go_layout_t go_struct_layout(std::index_sequence<I...>) noexcept {
    std::uint64_t off = 0, last_off = 0, last_size = 0, align = 1;
    bool ptrs = false;
    [[maybe_unused]] auto f = [&](go_layout_t l) {
        off = (off + l.align - 1) / l.align * l.align;
        last_off = off;
        last_size = l.size;
        off += l.size;
        if (l.align > align) {
            align = l.align;
        }
        ptrs = ptrs || l.ptrs;
    };
    (f(detail::go_layout<
         typename std::tuple_element_t<I, std::remove_cvref_t<decltype(S::gp_fields)>>::gp_type>()),
     ...);
    if constexpr (sizeof...(I) == 0) {
        return {0, 1, false};
    } else {
        std::uint64_t size = last_off + (last_off > 0 && last_size == 0 ? 1 : last_size);
        return {(size + align - 1) / align * align, align, ptrs};
    }
}

template <typename T>
concept go_struct_class = requires { T::gp_fields; };

// A struct whose real Go layout differs from its fields (a stub that leaves fields out) says
// so: the emitter writes `static constexpr gp::gc::go_layout_t gp_go_layout = {size, align,
// ptrs};`. The real version is step 16.8; the spike's stubs need it.
template <typename T>
concept go_declared_layout = requires { T::gp_go_layout; };

// A named type over a number or a bool has gp_value(): the layout of the value.
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
    } else if constexpr (go_struct_class<T>) {
        return detail::go_struct_layout<T>(
            std::make_index_sequence<
                std::tuple_size_v<std::remove_cvref_t<decltype(T::gp_fields)>>>{});
    } else if constexpr (go_named_value<T>) {
        return detail::go_layout<
            std::remove_cvref_t<decltype(std::declval<const T&>().gp_value())>>();
    } else if constexpr (std::is_empty_v<T>) {
        // An empty class stands for a Go zero-size type. (A class from C++ interop never
        // reaches the GC engine.)
        return {0, alignof(T), false};
    } else {
        // A class the emitter did not describe: conservatively scanned, its C++ size. Every Go
        // type has one of the layouts above.
        return {sizeof(T), alignof(T), true};
    }
}

} // namespace detail

// go_layout<T>() is Go's layout of the type that the C++ type T stands for. T must be
// complete.
template <typename T>
constexpr go_layout_t go_layout() noexcept {
    return detail::go_layout<T>();
}

} // namespace gp::gc

// GP_GC_LAYOUT(S, size, align): Go's size and alignment of S (from go_layout, which reads
// gp_fields or gp_go_layout), and the C++ struct's alignment and (for a Go size above zero)
// its size. A Go struct of size zero is 1 byte in C++; that is the one allowed difference.
#define GP_GC_LAYOUT(S, go_size, go_align)                                                         \
    static_assert(gp::gc::go_layout<S>().size == (go_size), #S ": Go's size");                     \
    static_assert(gp::gc::go_layout<S>().align == (go_align), #S ": Go's alignment");              \
    static_assert(alignof(S) == (go_align), #S ": C++ alignment");                                 \
    static_assert((go_size) == 0 ? true : sizeof(S) == (go_size), #S ": C++ size")

// GP_GC_FIELD_AT(S, member, offset): a field that occupies bytes is at Go's offset. A
// zero-size field has no C++ offset to check (see the top of this file); the emitter checks
// only its alignment, through the padding before it and the struct's size.
#define GP_GC_FIELD_AT(S, member, go_offset)                                                       \
    static_assert(offsetof(S, member) == (go_offset), #S "." #member " is not at Go's offset")
