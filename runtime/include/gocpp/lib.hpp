// gocpp/lib.hpp: gocpp code that other programs call (Phase 06 step 6.10): the glue of the
// `//export` functions of the build modes c-archive and c-shared, and the start of the runtime
// when no gp::run_main ran.
//
// A library has no main(): the runtime starts lazily, at the first call of an exported function,
// on whatever thread makes it (the way Go's c-archive does), not in a load-time constructor whose
// order against C++ static initialization is unreliable. gp::lib::enter does it, once, thread-safe:
// it runs the package initialization in Go's order (the `gp_init_all` of the program) and later
// calls find it done. The calling thread, which no go statement started, gets a goroutine record
// (gp::this_goroutine) that is freed when the thread ends.
//
// Every exported function is an extern "C" function that the driver writes (a few lines, see
// docs/spec/build.md):
//
//     extern "C" GOCPP_EXPORT GoInt32 Add(GoInt32 a, GoInt32 b) {
//         return gp::lib::guard(&gp_init_all, [&]() -> GoInt32 {
//             return gp::lib::out<GoInt32>(gpkg_main::Add(gp::lib::in<&gpkg_main::Add, 0>(a),
//                                                         gp::lib::in<&gpkg_main::Add, 1>(b)));
//         });
//     }
//
// guard calls enter, runs the lambda and makes sure that nothing unwinds into the C caller: a
// panic (or any C++ exception) that leaves the lambda ends the program like an unrecovered Go
// panic, "panic: <value>", the goroutine's trace, exit code 2, before a C frame is touched (the
// handlers are in guard itself, so no terminate handler of the host is involved). A `recover()`
// inside the exported function works as everywhere.
//
// The values cross the boundary as copies (cgo's type rules, docs/spec/build.md "Libraries"):
//   - numbers, bool (as GoUint8), pointers (`unsafe.Pointer` as void*, `*C.char` as char*, a
//     pointer to a number as a C pointer to it) are converted, nothing is copied;
//   - a `string` argument is a GoString {p, n}: the function gets a copy (the caller's memory
//     may go the moment the call returns); a `[]T` argument is a GoSlice {data, len, cap}: the
//     function gets a copy of the len elements (a function that must write into the caller's
//     memory takes a pointer and a length);
//   - a `string` or slice result is a copy made with malloc, which the C caller frees with free();
//     a string's copy has a NUL after its last byte (not counted in n);
//   - a pointer result must point to memory that outlives the call: C memory or a package
//     variable. A pointer into memory that only gocpp's reference count keeps would dangle in C:
//     it is a fatal error that says so;
//   - several results are returned as a struct `<name>_return { r0, r1, ... }`.
//
// Like every runtime type, nothing here has plain-named members besides what the generated glue
// uses, which are free functions and templates.
#pragma once

#include <gocpp/rt.hpp>

#include <cstddef>
#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <memory>
#include <string_view>
#include <tuple>
#include <type_traits>
#include <utility>

namespace gp::lib {

// enter starts the runtime of a library, once for the whole process and thread-safe, and makes
// the calling thread a goroutine: it runs init (the program's package initialization, the
// generated `gp_init_all`, or null) and returns when it has finished, on every thread. A call
// that comes from inside init (an exported function that a package's init function reaches
// through C) goes through at once. It does not install signal handlers or a terminate handler
// (those are the host program's). When init panics, the program ends as for an unrecovered panic.
void enter(void (*init)()) noexcept;

// entered reports whether enter has finished its first call.
bool entered() noexcept;

// shutdown is what runs when a shared library is unloaded (dlclose) or the process exits: it ends
// the idle goroutine threads and waits for them (their code is in the library), and forgets the
// goroutine records of the threads that called in. enter registers it with atexit, which the C
// library runs at dlclose too. Exposed for tests.
//
// A shared library with a running goroutine (running or blocked: runtime.NumGoroutine counts
// it) is never unmapped, because the goroutine's thread runs code of the library: the first go
// statement takes a reference to the library, so the host's dlclose gives back only the host's
// reference, nothing runs and nothing is printed, and the library stays loaded (like Go's c-shared
// libraries, which can't be unloaded at all). The reference is given back when an exported function
// returns and no goroutine runs; then dlclose unloads. At the end of the process, shutdown does not
// wait for goroutines (they die with the process).
void shutdown() noexcept;

namespace detail {

// leave is what a call into the library does last: it gives back the reference that a running
// goroutine took (lib.cpp, pin) once no goroutine runs any more.
// before is running_goroutines() when the call began: a call that began with none and ends with
// some waits a moment for those that are just finishing (the receiver of a channel is awake before
// the sender's thread has left the goroutine).
void leave(int64_t before) noexcept;
int64_t goroutines_now() noexcept;

[[noreturn]] void fail_panic(const gp::panic_error& e) noexcept;
[[noreturn]] void fail_goexit() noexcept;
[[noreturn]] void fail_exception() noexcept;
[[noreturn]] void fail_owned_pointer() noexcept;
[[noreturn]] void fail_memory() noexcept;

// The C layouts of the header: a GoString has members p and n, a GoSlice data, len and cap.
template <typename S>
concept c_string = requires(S s) {
    { s.p } -> std::convertible_to<const char*>;
    s.n;
};

template <typename S>
concept c_slice = requires(S s) {
    s.data;
    s.len;
    s.cap;
};

template <typename T>
struct slice_elem {};
template <typename E>
struct slice_elem<gp::slice<E>> {
    using type = E;
};
template <typename T>
concept gp_slice = requires { typename slice_elem<T>::type; };

template <typename T>
concept newtype_of = requires { typename T::gp_underlying; };

// The I'th parameter type of the function Fn (a pointer to a function).
template <auto Fn, std::size_t I>
struct param;
template <typename R, typename... P, R (*Fn)(P...), std::size_t I>
struct param<Fn, I> {
    using type = std::tuple_element_t<I, std::tuple<P...>>;
};

// A copy of the n bytes at p in malloc memory with extra bytes after it (not initialized), or
// null when n and extra are 0.
inline void* malloc_copy(const void* p, std::size_t n, std::size_t extra) noexcept {
    if (n + extra == 0) {
        return nullptr;
    }
    void* m = std::malloc(n + extra);
    if (m == nullptr) {
        fail_memory();
    }
    if (n != 0) {
        std::memcpy(m, p, n);
    }
    return m;
}

// slice_in<S>(data, len): a copy of the len elements at data as the slice S; nil for a null
// pointer.
template <typename S, typename D>
S slice_in(D* data, int64_t len) {
    using E = typename slice_elem<S>::type;
    if (data == nullptr || len < 0) {
        return S();
    }
    static_assert(std::is_trivially_copyable_v<E> || std::is_same_v<E, bool>,
                  "gocpp: a slice that crosses the C boundary has plain elements");
    S s = gp::make_slice<E>(len, len);
    if constexpr (std::is_same_v<E, bool>) {
        for (int64_t i = 0; i < len; i++) {
            s[i] = static_cast<const unsigned char*>(static_cast<const void*>(data))[i] != 0;
        }
    } else if (len > 0) {
        std::memcpy(static_cast<void*>(s.gp_data()), static_cast<const void*>(data),
                    static_cast<std::size_t>(len) * sizeof(E));
    }
    return s;
}

} // namespace detail

// from_c<P>(c) is the C value c as a value of the gocpp parameter type P.
template <typename P, typename C>
P from_c(const C& c) {
    using V = std::remove_cvref_t<P>;
    if constexpr (std::is_same_v<V, gp::string>) {
        static_assert(detail::c_string<C>, "gocpp: a string parameter is a GoString");
        return gp::string::gp_copy(
            std::string_view(c.p, c.n > 0 ? static_cast<std::size_t>(c.n) : 0));
    } else if constexpr (detail::gp_slice<V>) {
        static_assert(detail::c_slice<C>, "gocpp: a slice parameter is a GoSlice");
        return detail::slice_in<V>(static_cast<const char*>(c.data), static_cast<int64_t>(c.len));
    } else if constexpr (std::is_same_v<V, bool>) {
        return c != 0;
    } else if constexpr (detail::newtype_of<V>) {
        return V(from_c<typename V::gp_underlying>(c)); // a named type over a number or a string
    } else if constexpr (std::is_same_v<V, gp::ptr<void>> && std::is_pointer_v<C>) {
        return gp::detail::to_unsafe_pointer(c);
    } else if constexpr (gp::detail::cptr_like<V> && std::is_pointer_v<C>) {
        using E = typename gp::detail::cptr_elem<V>::type;
        return V(const_cast<E*>(reinterpret_cast<const E*>(c)));
    } else if constexpr (gp::detail::gp_pointer<V> && std::is_pointer_v<C>) {
        using E = std::remove_cv_t<std::remove_pointer_t<decltype(std::declval<V>().gp_get())>>;
        return V(
            gp::raw_owned(reinterpret_cast<E*>(const_cast<void*>(static_cast<const void*>(c)))));
    } else if constexpr (std::is_arithmetic_v<V> || std::is_enum_v<V>) {
        return static_cast<V>(c);
    } else {
        return V(c);
    }
}

// in<Fn, I>(c) is the C argument c as the I'th parameter of the function Fn of the program.
template <auto Fn, std::size_t I, typename C>
auto in(const C& c) {
    return from_c<typename detail::param<Fn, I>::type>(c);
}

// out<CT>(r) is the result r of a gocpp function as the C type CT (a number, a pointer, a
// GoString or a GoSlice of the header).
template <typename CT, typename R>
CT out(R&& r) {
    using V = std::remove_cvref_t<R>;
    if constexpr (std::is_same_v<V, gp::string>) {
        static_assert(detail::c_string<CT>, "gocpp: a string result is a GoString");
        std::string_view v = r.gp_view();
        CT o{};
        o.p = static_cast<const char*>(detail::malloc_copy(v.data(), v.size(), 1));
        if (o.p != nullptr) {
            const_cast<char*>(o.p)[v.size()] = '\0';
        }
        o.n = static_cast<decltype(o.n)>(v.size());
        return o;
    } else if constexpr (detail::gp_slice<V>) {
        static_assert(detail::c_slice<CT>, "gocpp: a slice result is a GoSlice");
        using E = typename detail::slice_elem<V>::type;
        static_assert(std::is_trivially_copyable_v<E>,
                      "gocpp: a slice that crosses the C boundary has plain elements");
        CT o{};
        auto n = static_cast<std::size_t>(r.gp_len());
        o.data = detail::malloc_copy(r.gp_data(), n * sizeof(E), 0);
        o.len = static_cast<decltype(o.len)>(n);
        o.cap = o.len;
        return o;
    } else if constexpr (std::is_pointer_v<CT> && gp::detail::gp_pointer<V>) {
        // A gocpp pointer whose object only the reference count keeps would dangle in C when
        // the count drops: only pointers to C memory or to package variables can be handed out.
        std::shared_ptr<const void> o = gp::cgo::detail::owner_of(r);
        if (o != nullptr) {
            detail::fail_owned_pointer();
        }
        return reinterpret_cast<CT>(const_cast<void*>(static_cast<const void*>(r.gp_get())));
    } else if constexpr (std::is_pointer_v<CT> && std::is_pointer_v<V>) {
        return reinterpret_cast<CT>(const_cast<void*>(static_cast<const void*>(r)));
    } else if constexpr (detail::newtype_of<V>) {
        return out<CT>(r.gp_value());
    } else if constexpr (std::is_arithmetic_v<CT> || std::is_enum_v<CT>) {
        return static_cast<CT>(r);
    } else {
        return CT(std::forward<R>(r));
    }
}

// guard(init, f) is what an exported function does around its body f (a callable without
// arguments): it starts the runtime (enter), runs f and returns its result. A panic, a
// runtime.Goexit or any other C++ exception that leaves f ends the program (the functions above
// that never return); nothing unwinds into the caller.
template <typename F>
auto guard(void (*init)(), F&& f) noexcept -> decltype(f()) {
    enter(init);
    struct leaver {
        int64_t before = detail::goroutines_now();
        ~leaver() { detail::leave(before); }
    } on_return;
    try {
        return f();
    } catch (const gp::panic_error& e) {
        detail::fail_panic(e);
    } catch (const gp::goexit_signal&) {
        detail::fail_goexit();
    } catch (...) {
        detail::fail_exception();
    }
}

} // namespace gp::lib
