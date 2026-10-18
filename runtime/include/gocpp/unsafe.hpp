// gocpp/unsafe.hpp: Go's package unsafe (Phase 04 §2.7, step 4.10; divergence D16).
//
// unsafe.Pointer is gp::ptr<void>: an owning pointer, as in Go, where the garbage collector sees
// it. A pointer made from a gocpp pointer shares that object's owner, so the object lives as
// long as the unsafe.Pointer does, and so do the results of unsafe.SliceData, unsafe.Slice,
// unsafe.String and unsafe.StringData, which share the owner of the slice, pointer or string they
// come from. This is what ported Go code relies on: strings.Builder.String is
// unsafe.String(unsafe.SliceData(b.buf), len(b.buf)), and a view that owned nothing would dangle
// when the builder goes away.
//
// Only memory that comes from C owns nothing: a C pointer converted to unsafe.Pointer (or given
// to unsafe.Slice or unsafe.String) shares gp::foreign_owner(), a count that is never freed. It
// is an owner only because a non-nil gp::ptr must have one (ptr.hpp: the invariant); it keeps
// nothing alive, and for a string it also says "do not read the byte after the bytes" (a string
// without an owner promises that a NUL follows). C++ lifetime rules apply to such memory.
//
// What generated code writes (Phase 04 step 4.10):
//
//     unsafe.Pointer(p)   (p *T)      gp::unsafe_pointer(p)         shares p's owner
//     (*T)(p)  (p unsafe.Pointer)     gp::unsafe_cast<T>(p)         shares p's owner
//     uintptr(p)                      gp::unsafe_addr(p)
//     unsafe.Pointer(u) (u uintptr)   gp::unsafe_from_addr(u)       owns nothing (foreign_owner)
//     unsafe.Add(p, n)                gp::unsafe_add(p, n)
//     unsafe.SliceData(s)             gp::unsafe_slice_data(s)      shares the array of s
//     unsafe.Slice(p, n)              gp::unsafe_slice(p, n)        p a gp::ptr<T> (shares its
//                                                                   owner) or a raw T* (C memory)
//     unsafe.StringData(s)            gp::unsafe_string_data(s)     shares the bytes of s
//     unsafe.String(p, n)             gp::unsafe_string(p, n)       p a gp::ptr<uint8_t> or a raw
//                                                                   char pointer (C memory)
//     unsafe.Sizeof(cv), Alignof(cv)  gp::unsafe_sizeof<decltype(cv)>(), gp::unsafe_alignof<..>()
//     unsafe.Offsetof(cv.f)           offsetof(T, f)                (a C/C++ value or a type with
//                                                                   a C/C++ part; the others are
//                                                                   Go constants)
//
// and, for conversions of C/C++ values (gp::cgo::as, interop.hpp):
//
//     unsafe.Pointer(cp)              gp::cgo::as<gp::ptr<void>>(cp)    cp: any raw pointer,
//                                                                       also a C function
//     (*C.T)(p)  (p unsafe.Pointer)   gp::cgo::as<T*>(p)
//
// The panics are Go's: "runtime error: unsafe.Slice: len out of range", "...: ptr is nil and len
// is not zero", and the same for unsafe.String. A conversion (*T)(p) to a type with an
// alignment above 1 checks the address and ends the program with Go's checkptr message ("fatal
// error: checkptr: misaligned pointer conversion") when it is not aligned: Go reads such a
// pointer without a word on amd64 (its checkptr mode, -race, reports it), but here the read
// would be undefined behavior (-fsanitize=alignment, and vectorized loops that assume the
// alignment); -nochecks turns the check off.
//
// Like every runtime type, nothing here has plain-named members: these are free functions.
#pragma once

#include <gocpp/checks.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/slice.hpp>
#include <gocpp/string.hpp>

#include <bit>
#include <cstddef>
#include <cstdint>
#include <memory>
#include <type_traits>
#include <utility>

namespace gp {

template <typename T>
ptr<void> unsafe_pointer(const ptr<T>& p) noexcept;

namespace detail {

// unsafe_panic ends the program with a panic of Go's unsafe checks (runtime/unsafe.go):
// "runtime error: unsafe.Slice: len out of range" and the three like it (rt.cpp's own panics
// have the same identity). In runtime/src/unsafe.cpp.
enum class unsafe_what { slice_len, slice_nil, string_len, string_nil };
[[noreturn]] void unsafe_panic(unsafe_what what);

// misaligned ends the program with Go's checkptr fatal error (in runtime/src/unsafe.cpp).
[[noreturn]] void misaligned() noexcept;

// unsafe_length is the length n of unsafe.Slice(p, n) or unsafe.String(p, n) for elements of
// elem bytes, with the checks of runtime.unsafeslice and unsafestring: a negative length (an
// unsigned one too big for int64 counts as negative), a nil p with a length above 0, and a
// length whose bytes would run past the end of the address space.
template <typename N>
int64_t unsafe_length(N n, std::size_t elem, const void* p, bool is_string) {
    auto len = static_cast<int64_t>(n);
    unsafe_what len_error = is_string ? unsafe_what::string_len : unsafe_what::slice_len;
    unsafe_what nil_error = is_string ? unsafe_what::string_nil : unsafe_what::slice_nil;
    if (len < 0) [[unlikely]] {
        unsafe_panic(len_error);
    }
    auto un = static_cast<std::uint64_t>(len);
    bool overflow = elem != 0 && un > UINT64_MAX / elem;
    std::uint64_t mem = overflow ? 0 : un * elem;
    // mem > -uintptr(p): the end of the bytes wraps around the address space. For a nil p,
    // that is any length above 0.
    if (overflow || mem > std::uint64_t{0} - reinterpret_cast<std::uintptr_t>(p)) [[unlikely]] {
        unsafe_panic(p == nullptr ? nil_error : len_error);
    }
    return len;
}

// A raw pointer to a character, for unsafe.String over C memory: char*, unsigned char*,
// signed char*, const or not.
template <typename P>
concept raw_char_pointer =
    std::is_pointer_v<P> &&
    (std::is_same_v<std::remove_cv_t<std::remove_pointer_t<P>>, char> ||
     std::is_same_v<std::remove_cv_t<std::remove_pointer_t<P>>, signed char> ||
     std::is_same_v<std::remove_cv_t<std::remove_pointer_t<P>>, unsigned char>);

// An address of C memory as an unsafe.Pointer: nil for null, else a pointer that shares the
// foreign owner.
inline ptr<void> foreign_pointer(void* p) noexcept {
    return raw_owned(p); // the owner of a C struct that gocpp made (box_keep), else none
}

// check_aligned is the checkptr test of (*T)(p): p's address is a multiple of T's alignment.
template <typename T>
inline void check_aligned([[maybe_unused]] const void* p) noexcept {
#ifndef GOCPP_NOCHECKS
    if constexpr (alignof(T) > 1) {
        if (reinterpret_cast<std::uintptr_t>(p) % alignof(T) != 0) [[unlikely]] {
            misaligned();
        }
    }
#endif
}

// A gocpp pointer, gp::ptr<T>, or a named type over one (gp::newtype derives from it).
template <typename T>
void accept_gocpp_pointer(const ptr<T>&);
template <typename X>
concept gocpp_pointer = requires(const X& x) { detail::accept_gocpp_pointer(x); };

// object_ptr_elem<T> is E when T is exactly gp::ptr<E>.
template <typename T>
struct object_ptr_elem {};
template <typename E>
struct object_ptr_elem<ptr<E>> {
    using type = E;
};

// gocpp_object_pointer<T>: T is exactly gp::ptr<E> for an object type E (not void).
template <typename T>
concept gocpp_object_pointer = requires { typename object_ptr_elem<T>::type; } &&
                               !std::is_void_v<typename object_ptr_elem<T>::type>;

// to_unsafe_pointer(x) is unsafe.Pointer(x) for x a C/C++ value: a raw pointer to an object, to
// void or to a function (a C function: C.fn; bit_cast, since a cast between a function and an
// object pointer is only conditionally supported, and g++ with -Werror=conditionally-supported
// rejects it), a gocpp pointer, nullptr, or a number the size of a pointer (an address, as a
// uintptr).
template <typename X>
ptr<void> to_unsafe_pointer(X&& x) noexcept {
    using F = std::remove_cvref_t<std::decay_t<X>>;
    if constexpr (std::is_same_v<F, std::nullptr_t>) {
        return ptr<void>();
    } else if constexpr (gocpp_pointer<F>) {
        return gp::unsafe_pointer(x);
    } else if constexpr (std::is_pointer_v<F> && std::is_function_v<std::remove_pointer_t<F>>) {
        static_assert(sizeof(F) == sizeof(void*), "a function pointer is not the size of void*");
        return foreign_pointer(std::bit_cast<void*>(static_cast<F>(x)));
    } else if constexpr (std::is_pointer_v<F>) {
        return foreign_pointer(
            const_cast<void*>(static_cast<const volatile void*>(static_cast<F>(x))));
    } else if constexpr (std::is_integral_v<F> && sizeof(F) == sizeof(void*)) {
        return foreign_pointer(reinterpret_cast<void*>(static_cast<std::uintptr_t>(x)));
    } else {
        static_assert(std::is_pointer_v<F>,
                      "gocpp: unsafe.Pointer(x) needs a pointer, a function or a uintptr");
        return ptr<void>();
    }
}

// from_unsafe_pointer<T>(p) is (T)(p) for an unsafe.Pointer p and a C pointer type T (a raw
// pointer to an object, void or a function): the address, with the alignment check of
// (*T)(p) for an object type.
template <typename T>
T from_unsafe_pointer(const ptr<void>& p) noexcept {
    static_assert(std::is_pointer_v<T>, "gocpp: only unsafe.Pointer converts to a C pointer");
    using E = std::remove_pointer_t<T>;
    if constexpr (std::is_function_v<E>) {
        static_assert(sizeof(T) == sizeof(void*), "a function pointer is not the size of void*");
        return std::bit_cast<T>(p.gp_get());
    } else if constexpr (std::is_void_v<E>) {
        return p.gp_get();
    } else {
        static_assert(!std::is_void_v<E>);
        check_aligned<std::remove_cv_t<E>>(p.gp_get());
        return static_cast<T>(p.gp_get());
    }
}

} // namespace detail

// unsafe_pointer(p) is unsafe.Pointer(p) for a gocpp pointer p (also of a named pointer type):
// the same address, sharing p's owner. nil stays nil.
template <typename T>
ptr<void> unsafe_pointer(const ptr<T>& p) noexcept {
    if constexpr (std::is_void_v<T>) {
        return p;
    } else {
        if (p.gp_get() == nullptr) {
            return ptr<void>();
        }
        return ptr<void>(p, static_cast<void*>(p.gp_get()));
    }
}
template <typename T>
ptr<void> unsafe_pointer(ptr<T>&& p) noexcept {
    if constexpr (std::is_void_v<T>) {
        return std::move(p);
    } else {
        if (p.gp_get() == nullptr) {
            return ptr<void>();
        }
        void* addr = static_cast<void*>(p.gp_get());
        return ptr<void>(std::move(p), addr);
    }
}

// unsafe_cast<T>(p) is (*T)(p) for an unsafe.Pointer p: the same address as a *T, sharing p's
// owner. nil stays nil. A misaligned address ends the program (the checkptr rule, above).
template <typename T>
ptr<T> unsafe_cast(const ptr<void>& p) noexcept {
    static_assert(!std::is_void_v<T>, "unsafe_cast: use unsafe_pointer for unsafe.Pointer");
    if (p.gp_get() == nullptr) {
        return ptr<T>();
    }
    detail::check_aligned<T>(p.gp_get());
    return ptr<T>(p, static_cast<T*>(p.gp_get()));
}

// unsafe_addr(p) is uintptr(p): the address as a number (it keeps nothing alive, as in Go).
inline std::uintptr_t unsafe_addr(const ptr<void>& p) noexcept {
    return reinterpret_cast<std::uintptr_t>(p.gp_get());
}

// unsafe_from_addr(u) is unsafe.Pointer(u) for a uintptr u: the address of memory that gocpp
// doesn't know the owner of, so the pointer owns nothing (foreign_owner), as a pointer from C.
// The valid Go patterns work: unsafe.Pointer(uintptr(p) + off) points into the object that p
// keeps alive.
inline ptr<void> unsafe_from_addr(std::uintptr_t u) noexcept {
    return detail::foreign_pointer(reinterpret_cast<void*>(u));
}

// unsafe_add(p, n) is unsafe.Add(p, n): p moved by n bytes (n of any integer type, negative
// too), sharing p's owner. The sum is made as a number, since a pointer past its object is
// undefined in C++. A nil p with n == 0 is nil; otherwise the result is a pointer made from an
// address (a nil p moved by n).
template <go_int N>
ptr<void> unsafe_add(const ptr<void>& p, N n) noexcept {
    auto addr = reinterpret_cast<std::uintptr_t>(p.gp_get()) + static_cast<std::uintptr_t>(n);
    if (p.gp_get() == nullptr) {
        return detail::foreign_pointer(reinterpret_cast<void*>(addr));
    }
    return ptr<void>(p, reinterpret_cast<void*>(addr));
}

// unsafe_slice_data(s) is unsafe.SliceData(s): a pointer to the first element of the slice's
// array (of s[:1] when the capacity is above 0), sharing the array's owner; nil for a nil slice.
// For an empty slice that isn't nil, Go says only "a non-nil pointer"; here it is the data
// pointer of the slice.
template <typename T>
ptr<T> unsafe_slice_data(const slice<T>& s) noexcept {
    if (s.gp_data() == nullptr) {
        return ptr<T>();
    }
    if (s.gp_owner() == nullptr) {
        return ptr<T>(static_owner(), s.gp_data()); // the static empty block
    }
    return ptr<T>(s.gp_owner(), s.gp_data());
}

// unsafe_slice(p, n) is unsafe.Slice(p, n) for a gocpp pointer p: a slice of n elements
// starting at *p, with capacity n, over the array that p points into; it shares p's owner. A nil
// p with n == 0 gives a nil slice. Panics as Go's (above).
template <typename T, go_int N>
slice<T> unsafe_slice(const ptr<T>& p, N n) {
    int64_t len = detail::unsafe_length(n, sizeof(T), p.gp_get(), false);
    if (p.gp_get() == nullptr) {
        return slice<T>();
    }
    return slice<T>(p.gp_shared(), p.gp_get(), len, len);
}
template <typename T, go_int N>
slice<T> unsafe_slice(ptr<T>&& p, N n) {
    int64_t len = detail::unsafe_length(n, sizeof(T), p.gp_get(), false);
    if (p.gp_get() == nullptr) {
        return slice<T>();
    }
    T* data = p.gp_get();
    return slice<T>(std::move(p).gp_take(), data, len, len);
}

// unsafe_slice(p, n) for a raw pointer p of C memory (a C array, a C function's result): the
// slice shares the foreign owner, so it keeps nothing alive. A const element type loses its
// const, as in cgo (Go has no const).
template <typename P, go_int N>
    requires std::is_pointer_v<P>
auto unsafe_slice(P p, N n) {
    using Pointee = std::remove_cv_t<std::remove_pointer_t<P>>;
    // (after the error below, only the one error: a slice of void would add others)
    using E =
        std::conditional_t<std::is_void_v<Pointee> || std::is_function_v<Pointee>, char, Pointee>;
    static_assert(!std::is_void_v<Pointee> && !std::is_function_v<Pointee>,
                  "gocpp: unsafe.Slice needs a pointer to an element type, not void*: convert it, "
                  "(*C.char)(p)");
    int64_t len = detail::unsafe_length(n, sizeof(E), p, false);
    if (p == nullptr) {
        return slice<E>();
    }
    return slice<E>(foreign_owner(), const_cast<E*>(reinterpret_cast<const E*>(p)), len, len);
}

// unsafe_string_data(s) is unsafe.StringData(s): a pointer to the string's bytes, sharing the
// string's owner. It must not be used to write (Go's rule too: a literal is in read-only
// memory). For the empty string it is nil, which Go allows.
inline ptr<uint8_t> unsafe_string_data(const string& s) noexcept {
    if (len(s) == 0) {
        return ptr<uint8_t>();
    }
    return ptr<uint8_t>(s.gp_owner(), reinterpret_cast<uint8_t*>(const_cast<char*>(s.gp_data())));
}

// unsafe_string(p, n) is unsafe.String(p, n) for a gocpp pointer p to a byte: a string of the n
// bytes at p, without a copy, sharing p's owner. n == 0 gives "". Panics as Go's (above).
template <go_int N>
string unsafe_string(const ptr<uint8_t>& p, N n) {
    int64_t len = detail::unsafe_length(n, 1, p.gp_get(), true);
    if (len == 0) {
        return string();
    }
    return string(p.gp_shared(), reinterpret_cast<const char*>(p.gp_get()), len);
}
template <go_int N>
string unsafe_string(ptr<uint8_t>&& p, N n) {
    int64_t len = detail::unsafe_length(n, 1, p.gp_get(), true);
    if (len == 0) {
        return string();
    }
    const char* data = reinterpret_cast<const char*>(p.gp_get());
    return string(std::move(p).gp_take(), data, len);
}

// unsafe_string(p, n) for a raw char pointer p of C memory: the bytes are not copied, and the
// string shares the foreign owner, which is not null: a string without an owner promises that
// a NUL byte follows its bytes (string.hpp: gp_c_terminated), which C memory doesn't.
template <detail::raw_char_pointer P, go_int N>
string unsafe_string(P p, N n) {
    int64_t len = detail::unsafe_length(n, 1, p, true);
    if (len == 0) {
        return string();
    }
    return string(foreign_owner(), reinterpret_cast<const char*>(p), len);
}

// unsafe_sizeof<decltype(x)>() is unsafe.Sizeof(x) of a C/C++ value or of a type with a C/C++
// part, as a value (check wraps the call in gp_cgoh.UnsafeSize; the C++ compiler's number, not a
// Go constant); unsafe_alignof is unsafe.Alignof(x). unsafe.Offsetof(x.f) is C's offsetof(T, f).
template <typename T>
constexpr std::uintptr_t unsafe_sizeof() noexcept {
    return sizeof(std::remove_cvref_t<T>);
}
template <typename T>
constexpr std::uintptr_t unsafe_alignof() noexcept {
    return alignof(std::remove_cvref_t<T>);
}

} // namespace gp
