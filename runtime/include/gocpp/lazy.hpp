// gocpp/lazy.hpp: gp::lazy<T>, the storage of a package variable whose type holds a C or C++
// class by value (Phase 06 step 6.4, ADR-093).
//
// Every other package variable is `constinit`: C++ runs no dynamic initializer of its own, so
// nothing can run after gp_var_init and wipe what it stored (a shared library, a c-archive
// whose host calls an exported function from its own static initializer). A C++ class may
// have a constructor that is not constexpr (std::ofstream, a class that counts its objects), and
// then constinit is an error. The emitter declares such a variable as
//
//     gp::lazy<Late> gp_lazy_l;                // constant-initialized: no object yet
//     constinit Late& l = gp_lazy_l.gp_ref();  // the variable's name, for every use
//     static const gp::lazy_boot gp_boot_l{gp_lazy_l};
//
// and gp_var_init starts with gp_lazy_l.gp_ensure(). The object is built by whichever comes
// first: gp_var_init (the first exported call of a library, or main) or the dynamic
// initializer gp_boot_l, which a program that never runs gp_var_init (a cpp-lib consumer that
// did not call init()) gets as before; the second finds it built and does nothing, so the
// early gp_var_init is not wiped. The object is value-initialized (T{}, Go's zero value) and
// never destroyed: Go runs no destructor of a package variable, and a thread still running at
// exit cannot meet a destroyed std::mutex. An executable pays nothing: the reference is a
// constant, and a use is a use of the object.
#pragma once

#include <new>
#include <type_traits>

namespace gp {

template <typename T>
class lazy {
public:
    constexpr lazy() noexcept : gp_dummy() {}
    lazy(const lazy&) = delete;
    lazy& operator=(const lazy&) = delete;
    // (No destructor work: the object is never destroyed. The empty body keeps T's union member
    // from deleting the destructor.)
    ~lazy() {}

    // The object, which gp_ensure builds. Naming the member of the union is a constant
    // expression (no value is read), so `constinit T& v = storage.gp_ref();` is
    // constant-initialized.
    constexpr T& gp_ref() noexcept { return gp_value; }

    // Builds the object (T{}) unless it exists.
    void gp_ensure() {
        if (!gp_made) {
            ::new (static_cast<void*>(&gp_value)) T{};
            gp_made = true;
        }
    }

    bool gp_exists() const noexcept { return gp_made; }

private:
    bool gp_made = false;
    union {
        char gp_dummy;
        T gp_value;
    };
};

// keep<T> is the storage of every package variable whose type has a destructor (and of a package's
// exported constant of a named type, for C++ code): `constinit gp::keep<T> gp_keep_v{init};`,
// initialized like the variable was (aggregate initialization of its one member gp_v), and never
// destroyed. The variable's name is an `inline constexpr T& v = gp_keep_v.gp_v;` of the package's
// header (an address constant: no pointer object that alias analysis would see escape). keep<T> is a
// union whose destructor does nothing (no member is destroyed) when T has a destructor, and a plain
// struct (a trivial destructor, so the compilers register nothing at exit) for any other T. A package
// variable of a trivially destructible Go type (numbers, complex numbers, bools, arrays and structs of
// them) is a plain constinit object and does not use keep. A variable that a host's exit() destroyed
// while the library's goroutines still ran crashed them (Go's c-archive runs no destructor either),
// and an executable ends without destructors anyway (gp::os::exit). (g++ at -O0 still registers the
// empty destructor of the union: it runs nothing.)
namespace detail {
template <typename T>
struct keep_plain {
    T gp_v;
};
template <typename T>
union keep_union {
    T gp_v;
    constexpr ~keep_union() {}
};
} // namespace detail

template <typename T>
using keep = std::conditional_t<std::is_trivially_destructible_v<T>, detail::keep_plain<T>,
                                detail::keep_union<T>>;

// lazy_boot{storage} builds the object when static initialization gets to it, if gp_var_init
// has not built it yet.
struct lazy_boot {
    template <typename T>
    explicit lazy_boot(lazy<T>& l) {
        l.gp_ensure();
    }
};

} // namespace gp
