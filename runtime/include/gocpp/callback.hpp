// gocpp/callback.hpp: gocpp functions that C calls (Phase 04 §2.8, step 4.11).
//
// A C function takes a callback as a function pointer, `int (*)(void*, int, char**, char**)`,
// and a gocpp func is a gp::func (an owner and a code pointer: 24 bytes, no address that C can
// call). What generated code writes where a func is an argument of a call of a C/C++ function,
// the value of a C/C++ variable or member, or an element of a C/C++ literal:
//
//     f  (a function of the package)      gp::cgo::fn<&gpkg_main::f>()
//     func(a C.int) C.int { ... }         gp::cgo::fn_lit<gp::func<int(int)>>(lambda)
//       (a literal without captures)
//     func(a C.int) C.int { ... n ... }   gp::cgo::fn_val(gp::func<int(int)>(lambda))
//       (a literal with captures), and any other func value: a variable, a field, a result
//
// Each is a gp::cgo::callback<Sig, F>, a callable that
//
//   - converts to a C function pointer, when it has no state to keep: a function of the
//     package (a static trampoline per function), or a func literal that captures nothing
//     (one per literal). The pointer is valid for the whole run of the program, so C may keep it
//     (a callback table in a struct, a handler it calls later, from another thread);
//   - is called like the func (a C++ template that takes a callable: std::sort,
//     std::find_if), and converts to std::function and to gp::func: the closure is shared
//     (a copy of a callback is cheap) and a panic unwinds through the C++ frames as it does
//     through gocpp ones, running the destructors and deferred calls on its way. These need no
//     trampoline;
//   - fails to compile to a function pointer when it holds a closure that captures variables,
//     or a func value that is only known at run time (`cb := printRow; lib.set(cb)`): a
//     function pointer is an address of code, and the closure's variables have no place to
//     live behind it. The messages below say what to write instead (pass the function by its
//     name, or pass the state in the user-data argument that C libraries give their callbacks).
//
// The trampoline of a function pointer is
//
//     static R call(B... b) noexcept { return convert(F{}(convert<A>(b)...)); }
//
// with B the C parameter types and A the gocpp parameter types. It is noexcept on purpose:
// **a panic (or any C++ exception) that leaves a callback does not unwind through the C frames
// below it**. The C++ ABI's search for a handler finds the noexcept function, calls
// std::terminate without unwinding anything, and the terminate handler that run_main sets
// (panic.hpp, on_terminate) ends the program like an unrecovered panic: "panic: <value>", the
// trace of the panic with the C frames in it, exit code 2. A deferred call and recover() inside
// the callback work as everywhere (they run before the panic reaches the trampoline); the
// deferred calls of the frames outside the C call do not run (they would, in Go: divergence
// D27). A C library never sees a half-unwound state: no lock stays held, no memory leaks, no
// state of the library is skipped.
//
// The arguments from C are converted to the gocpp parameter types: a `void*` is an
// unsafe.Pointer that owns nothing (gp::foreign_owner, except for the structs that gocpp made
// itself: raw_owned), a `T*` is a *C.T (gp::cptr, owns nothing), the other C types are
// themselves. The result goes back the same way: a pointer result must point to memory that
// outlives the callback, not to gocpp memory that the callback's last reference keeps (a
// gocpp-owned pointer is a fatal error: C would hold a dangling address). Variadic callbacks
// (`int (*)(const char*, ...)`) and callbacks with a different number of parameters than the
// function have are compile errors.
//
// Threads: a callback may run on any thread. A thread that no go statement started (a pthread
// of the C library) gets a goroutine record the first time it needs one (gp::this_goroutine),
// so panics, defers and recover work there too.
//
// User data (`void* arg` next to the callback) is plain unsafe.Pointer pass-through: a gocpp
// pointer converts to the `void*` (borrowed: C may use it only while the gocpp object lives,
// cgo's rule), and the callback gets it back as an unsafe.Pointer, which `(*T)(p)` converts.
// A closure goes through the same door: `unsafe.Pointer(&f)` for a func variable f, and
// `(*(*func(C.int))(p))(n)` in the callback. When C keeps the user data after the call
// returns, the owner must be kept by gocpp code (a package variable, a table); Phase 05's
// cgo.Handle will be that table: a handle is an integer that fits the `void*`, which
// `unsafe.Pointer(uintptr(h))` makes and the callback reads back, and nothing here has to
// change for it.
//
// Like every runtime type, nothing here has plain-named members besides operators.
#pragma once

#include <gocpp/func.hpp>
#include <gocpp/goroutine.hpp>
#include <gocpp/interop.hpp>
#include <gocpp/panic.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/unsafe.hpp>

#include <cstddef>
#include <type_traits>
#include <utility>

namespace gp::cgo {

namespace detail {

// static_fn<Fn> calls the function Fn of the program (a stateless callable, so that a callback
// of it is an empty object).
template <auto Fn>
struct static_fn {
    template <typename... X>
    decltype(auto) operator()(X&&... x) const {
        return Fn(std::forward<X>(x)...);
    }
};

// fnptr_parts<P>: P is a pointer to a function (not variadic).
template <typename P>
struct fnptr_parts {
    static constexpr bool is_fnptr = false;
    static constexpr bool variadic = false;
};
template <typename Rc, typename... B>
struct fnptr_parts<Rc (*)(B...)> {
    static constexpr bool is_fnptr = true;
    static constexpr bool variadic = false;
    using fn = Rc(B...);
};
template <typename Rc, typename... B>
struct fnptr_parts<Rc (*)(B..., ...)> {
    static constexpr bool is_fnptr = true;
    static constexpr bool variadic = true;
    using fn = void(); // never used: the conversion is a static_assert
};
template <typename Rc, typename... B>
struct fnptr_parts<Rc (*)(B...) noexcept> : fnptr_parts<Rc (*)(B...)> {};

// A callback whose target is a pointer to a function: the types it converts to.
template <typename P>
concept function_pointer = fnptr_parts<std::remove_cvref_t<P>>::is_fnptr;

// from_c<P>(b): the C argument b as the gocpp parameter type P.
template <typename P, typename B>
P from_c(B&& b) noexcept(std::is_nothrow_constructible_v<P, B>) {
    using V = std::remove_cvref_t<B>;
    if constexpr (std::is_same_v<std::remove_cvref_t<P>, gp::ptr<void>> && std::is_pointer_v<V>) {
        return gp::detail::to_unsafe_pointer(b); // unsafe.Pointer(p): a pointer from C
    } else if constexpr (gp::detail::cptr_like<std::remove_cvref_t<P>> && std::is_pointer_v<V>) {
        // *C.T of a T* (or a const T*: Go has no const)
        using E = typename gp::detail::cptr_elem<std::remove_cvref_t<P>>::type;
        using Pointee = std::remove_cv_t<std::remove_pointer_t<V>>;
        static_assert(!std::is_void_v<Pointee>,
                      "gocpp: the C parameter is a void*: declare this parameter as "
                      "unsafe.Pointer, and convert it with (*T)(p) in the callback");
        static_assert(std::is_same_v<Pointee, std::remove_cv_t<E>>,
                      "gocpp: this *C.T parameter is not the C parameter's pointer type: "
                      "declare it with the C type that the C function pointer type has");
        return std::remove_cvref_t<P>(const_cast<E*>(b));
    } else if constexpr (std::is_pointer_v<std::remove_cvref_t<P>> && std::is_pointer_v<V> &&
                         !std::is_function_v<std::remove_pointer_t<V>> &&
                         !std::is_same_v<std::remove_cvref_t<P>, V>) {
        return const_cast<std::remove_cvref_t<P>>(b); // **C.char of a const char**
    } else if constexpr (std::is_enum_v<std::remove_cvref_t<P>> || std::is_enum_v<V> ||
                         std::is_arithmetic_v<V>) {
        return static_cast<P>(std::forward<B>(b));
    } else {
        return std::forward<B>(b);
    }
}

// owned_by_args(o, b...): o is the owner of the object that one of the pointer arguments b of
// the callback points to (or into): that object is alive in the caller for the whole call, so the
// callback may hand the same pointer (or a pointer into it) back.
template <typename... B>
bool owned_by_args(const std::shared_ptr<const void>& o, const B&... b) {
    [[maybe_unused]] auto one = [&](const auto& x) {
        if constexpr (std::is_pointer_v<std::remove_cvref_t<decltype(x)>> &&
                      !std::is_function_v<
                          std::remove_pointer_t<std::remove_cvref_t<decltype(x)>>>) {
            std::shared_ptr<const void> a = gp::detail::box_owner(static_cast<const void*>(x));
            return a != nullptr && gp::cgo::detail::same_block(o, a);
        } else {
            return false;
        }
    };
    return (one(b) || ... || false);
}

// to_c<Rc>(r, b...): the result r of the gocpp function as the C result type Rc; b are the
// arguments that C passed.
template <typename Rc, typename R, typename... B>
Rc to_c(R&& r, const B&... b) {
    using V = std::remove_cvref_t<R>;
    if constexpr (std::is_pointer_v<Rc> && gp::detail::gp_pointer<V>) {
        // A gocpp pointer that owns something would dangle in C when the last reference to its
        // object (this one) goes: only pointers to memory that C owns (or a package
        // variable's) can be handed out.
        std::shared_ptr<const void> o = owner_of(r);
        if (o != nullptr && !owned_by_args(o, b...)) {
            gp::detail::fatal(
                "gocpp: a callback returned a pointer to gocpp memory to C: return a pointer "
                "from C (malloc), a package variable's, or the pointer that C passed");
        }
        return reinterpret_cast<Rc>(r.gp_get());
    } else if constexpr (std::is_pointer_v<Rc> && std::is_pointer_v<V> &&
                         !std::is_function_v<std::remove_pointer_t<Rc>>) {
        return const_cast<Rc>(r);
    } else if constexpr (std::is_arithmetic_v<Rc> || std::is_enum_v<Rc>) {
        return static_cast<Rc>(std::forward<R>(r));
    } else {
        return std::forward<R>(r);
    }
}

// The trampoline of the callback F, whose gocpp signature is R(A...), as the C function Rc(B...).
template <typename F, typename Gocpp, typename C>
struct tramp;
template <typename F, typename R, typename... A, typename Rc, typename... B>
struct tramp<F, R(A...), Rc(B...)> {
    static_assert(sizeof...(A) == sizeof...(B),
                  "gocpp: this func has a different number of parameters than the C function "
                  "pointer type it is passed as");
    // noexcept: a panic or C++ exception that leaves the callback ends the program (see the top
    // of the file): it must not unwind through the C frames.
    static Rc call(B... b) noexcept {
        static_assert(std::is_void_v<Rc> == std::is_void_v<R>,
                      "gocpp: the C function pointer type and this func disagree about a "
                      "result: one returns a value, the other returns nothing");
        if constexpr (std::is_void_v<Rc> || std::is_void_v<R>) {
            F{}(from_c<A>(static_cast<B&&>(b))...);
            if constexpr (!std::is_void_v<Rc>) {
                return Rc();
            }
        } else {
            return to_c<Rc>(F{}(from_c<A>(static_cast<B&&>(b))...), b...);
        }
    }
};

} // namespace detail

// callback<R(A...), F> is a gocpp func of the signature R(A...), held as the callable F (a
// function of the program, a lambda without captures, or a gp::func), that C can call when F
// has no state (the top of the file).
template <typename Sig, typename F>
class callback;

template <typename R, typename... A, typename F>
class callback<R(A...), F> {
public:
    constexpr callback() noexcept(std::is_nothrow_default_constructible_v<F>)
        requires std::is_default_constructible_v<F>
    = default;
    explicit callback(F f) : gp_f(std::move(f)) {}

    // User-provided copies make a callback non-trivially-copyable on purpose: C++ rejects passing
    // such an object through a C function's `...` (an empty class would go through as garbage and
    // crash the callee that reads a function pointer there). A callback is for a parameter that
    // has a type, and the variadic part has none.
    callback(const callback& o) : gp_f(o.gp_f) {}
    callback(callback&& o) noexcept(std::is_nothrow_move_constructible_v<F>)
        : gp_f(std::move(o.gp_f)) {}
    callback& operator=(const callback&) = default;
    callback& operator=(callback&&) = default;
    ~callback() {}

    // f(a...): the func is called as the gocpp func would be (a C++ template calls it).
    R operator()(A... a) const { return gp_f(static_cast<A&&>(a)...); }

    // A C function pointer: a static trampoline, for a func without state.
    template <typename P>
        requires detail::function_pointer<P>
    operator P() const noexcept {
        using parts = detail::fnptr_parts<std::remove_cvref_t<P>>;
        static_assert(!parts::variadic,
                      "gocpp: a callback can't be a variadic C function pointer: C's `...` has no "
                      "types for the arguments");
        static_assert(std::is_empty_v<F> && std::is_default_constructible_v<F>,
                      "gocpp: a func with captured variables, or a func value that is only known "
                      "at run time, can't be a C function pointer (it is an address of code, with "
                      "no place for the closure): pass a function of the package by its name, "
                      "or a func literal that captures nothing, and give the closure to the "
                      "callback through its user-data argument (unsafe.Pointer)");
        return &detail::tramp<F, R(A...), typename parts::fn>::call;
    }

private:
    // mutable: the lambda of a func literal is mutable (closure variables), a callback is const.
    [[no_unique_address]] mutable F gp_f{};
};

namespace detail {
template <typename T>
struct func_sig;
template <typename Sig>
struct func_sig<gp::func<Sig>> {
    using type = Sig;
};
} // namespace detail

// fn<&f>(): the callback of the function f of the program.
template <auto Fn>
auto fn() noexcept {
    gp::detail::note_foreign_entry(); // (see fn_val)
    using Sig = typename gp::detail::fn_sig<decltype(Fn)>::type;
    return callback<Sig, detail::static_fn<Fn>>();
}

// fn_lit<gp::func<Sig>>(lambda): the callback of a func literal that captures nothing.
template <typename FuncType, typename L>
auto fn_lit(L lambda) {
    gp::detail::note_foreign_entry(); // (see fn_val)
    return callback<typename detail::func_sig<FuncType>::type, L>(std::move(lambda));
}

// fn_val(f): the callback of a func value that is only known at run time (a variable, a field,
// a closure): usable as a C++ callable, std::function and gp::func, not as a function pointer.
template <typename Sig>
auto fn_val(const func<Sig>& f) {
    // The program gives a function of its own to C or C++ code, which may call it from a thread of
    // its own, at any time from now on: the deadlock detector can no longer know that nothing will
    // wake a goroutine, and stays quiet for good (gocpp/goroutine.hpp), as Go does in a cgo
    // program.
    gp::detail::note_foreign_entry();
    return callback<Sig, func<Sig>>(f);
}

} // namespace gp::cgo
