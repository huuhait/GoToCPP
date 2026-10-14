// gocpp/ptr.hpp: Go's pointers. gp::ptr<T> is *T, gp::new_<T> is new(T) and &T{...}, and
// gp::weak_ptr<T> is weak.Pointer[T] (Go 1.24's package weak).
//
// One ownership system for everything (ADR-004): every heap block is owned by a
// std::shared_ptr, so a gp::ptr<T> is a std::shared_ptr<T> with Go's nil checks (16 bytes). A
// block is freed when the last pointer to it goes away. A pointer *into* a block (&p.f, &s[i],
// &x.arr[i] for a boxed x) shares the block's owner through std::shared_ptr's aliasing
// constructor, so it keeps the whole block alive: no pointer can dangle. A pointer to a
// package-level variable (&global) shares a static owner that is never freed (static_ptr).
//
// The invariant: every pointer to an object shares the one block that owns the object, and a
// non-nil pointer always has an owner. Pointers into package-level variables (also, from step
// 3.5, slices of global arrays) use static_owner(), never an empty owner: an empty owner would
// make weak pointers to it expire at once. The aliasing constructors check it (gp_check_owner).
//
// What generated code writes (Phase 03 §2.2):
//
//     new(T)              gp::new_<T>()                    the zero value
//     &T{...}             gp::new_<T>(T{...})
//     &x  (x is boxed)    x                                the box is the pointer
//     &p.f                gp::ptr<F>(p, &p->f)             aliasing: shares p's owner
//     &s[i]               gp::ptr<T>(s.gp_owner(), &s[i])  shares the slice's array
//     &global, &global.f  gp::static_ptr(global), gp::static_ptr(global.f)
//     *p, p.f             *p, p->f                         a nil p panics like Go
//     p == nil            p == nullptr
//     (*Q)(p)             gp::ptr_cast<Q>(p)               only for Go's legal conversions
//     weak.Make(p)        gp::weak_make(p)
//     w.Value()           gp::weak_value(w)                nil once the target is freed
//
// Dereferencing a nil pointer (*p, p->f, &p->f) panics with Go's message, "runtime error:
// invalid memory address or nil pointer dereference" (gp::panic_nil). Unrecovered, it prints
// Go's signal line too and exits with code 2, like a Go program.
//
// Cycles: reference counting never frees a cycle (A points to B, B points to A); that is the
// price of having no garbage collector (D1). A program breaks a cycle with a weak pointer.
//
// Weak pointers and memory: gp::new_ puts the value and its count block in one allocation
// (std::make_shared). When the last strong pointer goes, the value is destroyed at once, but
// that allocation (sizeof(T) plus the count) is freed only when the last weak pointer to it
// goes too. That matters for a cache of weak pointers to big values (D1); Phase 15 could
// allocate large values apart from their counts.
//
// Long chains: freeing a list of a million nodes frees each node from the destructor of the one
// before it. ~ptr goes through detail::release, which stops that recursion at a depth of
// release_max_depth and finishes the rest in a loop, so freeing never runs out of stack.
//
// Threads: the reference counts are std::shared_ptr's, which are atomic, so goroutines on other
// threads (Phase 08) can copy and drop pointers to the same block at the same time. One
// gp::ptr *variable* that one thread writes while another reads it is a data race, as in Go,
// and here it can corrupt the count (D10; -race finds it).
//
// Cost (counted in the -O2 code of g++ 15 and clang++ 21 on x86-64):
//
//     copy, aliasing from an owner that is copied (an lvalue),
//     static_ptr, weak_make, weak_value               one atomic increment
//     dropping a pointer                              one atomic decrement, plus two plain
//                                                     thread-local counter updates and a
//                                                     compare (detail::release); a nil pointer
//                                                     only a compare. On ELF (Linux) the
//                                                     thread-local state is one %fs-relative
//                                                     access; MinGW-GCC uses emulated TLS,
//                                                     one __emutls_get_address call per drop
//                                                     (Phase 14 measures it)
//     move, aliasing from a moved owner, *p, p->f,
//     ==, a nil check                                 no atomic operation
//     the owner check of the aliasing constructors    one plain load (none with -nochecks)
//     gp::new_                                        one allocation; the count is set up
//                                                     without an atomic operation
//
// (libstdc++ even skips the lock prefix while the process has only one thread.)
//
// Like every runtime type, these types have only operators and gp_-prefixed members, and
// everything else is a free function (Phase 03 §2.1), so a Go method or field with any other
// name (get, reset, Value) can never clash with a runtime member. ptr and weak_ptr accept an
// incomplete T (type Node struct{ next *Node }), like std::shared_ptr; new_ and ptr_cast need
// a complete one.
#pragma once

#include <gocpp/checks.hpp>

// Only standard headers that the runtime included before: memory also gives std::move,
// std::forward and std::hash.
//
// A new standard header adds its macros to every generated file, and the emitter must keep
// Go names off them (internal/cppgen/cxx_macros.txt, checked by TestMacroList).
#include <cstddef>
#include <cstdint>
#include <memory>
#include <type_traits>

namespace gp {

namespace detail {

// --- release: dropping an owner without deep recursion ---
//
// Dropping the last pointer to an object destroys it, which drops the pointers inside it, and
// so on: a linked list of n nodes nests n destructors. release counts that nesting on each
// thread. Below release_max_depth it drops the owner at once; deeper, it moves the owner (no
// atomic operation) onto the thread's pending list, and the outermost release drops the
// pending owners one by one, in a loop. Every runtime type that holds an owner must release it
// through gp::detail::release: ~ptr now, and the slices and maps of steps 3.5 and 3.7, closure
// cells, func values and interfaces later (any of them can hold the rest of a chain).
//
// The limit is small because a goroutine is a thread (ADR-007), whose stack may be small
// (Phase 08, Windows). Measured with ulimit -s on Linux x86-64 (g++ 15, clang++ 21), the
// smallest main-thread stack on which freeing a 200,000-node list and a tree 200,000 deep on
// one side works, against 24-32 KB for the same program with ten nodes:
//
//                    g++      clang++
//     -O0            96 KB    96 KB
//     -O2            24 KB    32 KB
//     -O0 + ASan    128 KB   192 KB
//
// With the old limit of 1000 it took about 0.5 MB at -O0 and 1 MB with ASan.
inline constexpr int release_max_depth = 128;

// The thread's release state: the nesting depth, and the owners that wait to be dropped (a
// stack). One plain thread-local struct with constant initialization, so reading it needs no
// initialization check, and one access finds both (with MinGW's emulated TLS, one call).
// Nothing is pending when the outermost release returns.
struct release_node {
    std::shared_ptr<const void> gp_owner;
    release_node* gp_next;
};
struct release_state {
    int gp_depth;
    release_node* gp_pending;
};
inline constinit thread_local release_state release_tls{0, nullptr};

// release_defer puts the owner on the pending list; release_drain drops the pending owners.
// In runtime/src/rt.cpp.
void release_defer(std::shared_ptr<const void>&& owner) noexcept;
void release_drain() noexcept;

// release(owner): owner.reset(), without deep recursion. owner is empty afterwards.
template <typename T>
inline void release(std::shared_ptr<T>& owner) noexcept {
    release_state& st = release_tls;
    if (st.gp_depth >= release_max_depth) [[unlikely]] {
        release_defer(std::shared_ptr<const void>(std::move(owner)));
        return;
    }
    ++st.gp_depth;
    owner.reset();
    if (--st.gp_depth == 0 && st.gp_pending != nullptr) [[unlikely]] {
        release_drain();
    }
}

// bad_owner reports a broken invariant: a non-nil pointer without an owner. It is a bug in the
// runtime or in the generated code, so it ends the program like a Go runtime fatal error:
// "fatal error: ..." and exit code 2.
[[noreturn]] void bad_owner() noexcept;

} // namespace detail

// ptr<T> is Go's *T for a gocpp type T. The zero value is nil.
template <typename T>
class ptr {
public:
    // nil.
    constexpr ptr() noexcept = default;
    constexpr ptr(std::nullptr_t) noexcept {}

    // A pointer that owns what the shared_ptr owns, for the runtime's own code (gp::new_,
    // slices, maps). Generated code uses gp::new_ and the aliasing constructors instead.
    explicit ptr(const std::shared_ptr<T>& sp) noexcept : gp_sp_(sp) {}
    explicit ptr(std::shared_ptr<T>&& sp) noexcept : gp_sp_(std::move(sp)) {}

    // The aliasing constructors: a pointer to p (an object inside the block that owner owns)
    // that shares owner's block, so the whole block lives as long as this pointer. &p.f is
    // gp::ptr<F>(p, &p->f), and &s[i] is gp::ptr<T>(s.gp_owner(), &s[i]). From an owner that
    // is moved in, they cost no atomic operation; std::move(p) is only a cast, so in
    // gp::ptr<F>(std::move(p), &p->f) the argument &p->f still sees p.
    template <typename U>
    ptr(const ptr<U>& owner, T* p) noexcept : gp_sp_(owner.gp_shared(), p) {
        gp_check_owner();
    }
    template <typename U>
    ptr(ptr<U>&& owner, T* p) noexcept : gp_sp_(std::move(owner).gp_take(), p) {
        gp_check_owner();
    }
    template <typename U>
    ptr(const std::shared_ptr<U>& owner, T* p) noexcept : gp_sp_(owner, p) {
        gp_check_owner();
    }
    template <typename U>
    ptr(std::shared_ptr<U>&& owner, T* p) noexcept : gp_sp_(std::move(owner), p) {
        gp_check_owner();
    }

    // Copies share the block (one atomic increment); moves leave nil behind (no atomic
    // operation). There is no implicit conversion to another pointer type, to a raw pointer
    // or to bool: Go has none either (ptr_cast is the explicit conversion).
    ptr(const ptr&) noexcept = default;
    ptr(ptr&&) noexcept = default;
    ptr& operator=(const ptr&) noexcept = default;
    ptr& operator=(ptr&&) noexcept = default;

    // Dropping a pointer goes through detail::release, so that freeing a long chain doesn't
    // recurse deeply. (An assignment drops the old value directly: the pointers inside that
    // value go through here.)
    ~ptr() {
        if (gp_sp_.get() != nullptr) {
            detail::release(gp_sp_);
        }
    }

    // *p and p->f: the pointee, or Go's nil pointer panic. Not for gp::ptr<void>
    // (unsafe.Pointer, Phase 04), which can't be dereferenced.
    std::add_lvalue_reference_t<T> operator*() const
        requires(!std::is_void_v<T>)
    {
        return *gp_checked();
    }
    T* operator->() const
        requires(!std::is_void_v<T>)
    {
        return gp_checked();
    }

    // p == q compares the addresses, as Go does: two pointers to the same object (or both nil)
    // are equal. Only pointers of the same type compare, as in Go. p == nullptr is p == nil.
    friend bool operator==(const ptr& a, const ptr& b) noexcept {
        return a.gp_sp_.get() == b.gp_sp_.get();
    }
    friend bool operator==(const ptr& a, std::nullptr_t) noexcept {
        return a.gp_sp_.get() == nullptr;
    }

    // gp_get: the address, without a nil check (null for nil). For hashing, printing and the
    // runtime's own code.
    T* gp_get() const noexcept { return gp_sp_.get(); }

    // gp_shared: the shared_ptr, for the runtime's own code (weak pointers, interfaces).
    const std::shared_ptr<T>& gp_shared() const noexcept { return gp_sp_; }

    // gp_take: the shared_ptr, moved out; this pointer is nil afterwards.
    std::shared_ptr<T> gp_take() && noexcept { return std::move(gp_sp_); }

private:
    // gp_checked: the address, or Go's nil pointer panic.
    T* gp_checked() const {
        T* p = gp_sp_.get();
        if (p == nullptr) [[unlikely]] {
            panic_nil();
        }
        return p;
    }

    // gp_check_owner: the invariant above. An aliasing shared_ptr whose owner is empty has a
    // use count of 0. It is on unless GOCPP_NOCHECKS is defined, gocpp's own switch, which the
    // -nochecks build (step 3.14) defines for every file of the program; not NDEBUG, which a
    // user's file or a library may define for itself (one header compiled two ways in one
    // program would break the one-definition rule).
    void gp_check_owner() const noexcept {
#ifndef GOCPP_NOCHECKS
        if (gp_sp_.get() != nullptr && gp_sp_.use_count() == 0) [[unlikely]] {
            detail::bad_owner();
        }
#endif
    }

    std::shared_ptr<T> gp_sp_;
};

static_assert(sizeof(ptr<int>) == 16, "gp::ptr is 16 bytes, like std::shared_ptr (D15)");

// new_<T>(): Go's new(T), a pointer to a new zero T. new_<T>(v): &T{...}, a pointer to a new
// T made from v, and also a boxed variable (Phase 03 §2.2: the box of a variable whose address
// is taken). One allocation holds the value and its reference count.
template <typename T, typename... Args>
ptr<T> new_(Args&&... args) {
    return ptr<T>(std::make_shared<T>(std::forward<Args>(args)...));
}

// static_owner is the owner of everything that is never freed: the package-level variables.
// Its count never reaches zero (it is made once and never destroyed), and it frees nothing, so
// a weak pointer to a global never expires. All globals share this one count, so every copy of
// a pointer to a global changes it, from every thread (Phase 15 may give each package its own).
inline const std::shared_ptr<void>& static_owner() noexcept {
    // On the heap and never deleted, so no static destructor can drop it at exit. LeakSanitizer
    // doesn't report it: this static variable still points at it.
    static const std::shared_ptr<void>* const owner =
        new std::shared_ptr<void>(nullptr, [](void*) {});
    return *owner;
}

// static_ptr(x): &x for an object that is never freed: a package-level variable, or a part of
// one (&global.f, &globalArr[i] after the index check). The pointer shares the static owner.
template <typename T>
ptr<T> static_ptr(T& x) noexcept {
    return ptr<T>(static_owner(), std::addressof(x));
}

// ptr_cast<U>(p): Go's conversion (*U)(p) between pointer types whose base types have
// identical underlying types (type Q P; (*Q)(p)). The emitter calls it only for conversions
// that Go allows, where U and T have the same layout. The result shares p's owner; nil stays
// nil. Using a T object through a U* is undefined behavior in ISO C++ (strict aliasing): it
// works because the driver compiles with -fno-strict-aliasing.
template <typename U, typename T>
ptr<U> ptr_cast(const ptr<T>& p) noexcept {
    static_assert(sizeof(U) == sizeof(T) && alignof(U) == alignof(T),
                  "ptr_cast: Go converts pointers only between types with the same layout");
    return ptr<U>(p, reinterpret_cast<U*>(p.gp_get()));
}

// weak_ptr<T> is Go's weak.Pointer[T]: a pointer that doesn't keep its target alive. The zero
// value is Go's zero weak.Pointer, whose Value is nil. weak.Pointer is generic, but it is a
// compiler intrinsic in Phase 03 (the generics come in Phase 07): weak.Make and Value become
// gp::weak_make and gp::weak_value. 24 bytes (Go's is 8): gocppSizes must say so (step 3.17).
template <typename T>
class weak_ptr {
public:
    constexpr weak_ptr() noexcept = default;

    // As in Go, two weak pointers are equal when the pointers they were made from were equal,
    // also after the target is freed. The owner is compared too: a weak pointer keeps its
    // owner's count block alive, so a new object made after the old one was freed (even at
    // the same address) has another block, and weak pointers to the two compare unequal, as
    // in Go.
    friend bool operator==(const weak_ptr& a, const weak_ptr& b) noexcept {
        return a.gp_addr_ == b.gp_addr_ && !a.gp_w_.owner_before(b.gp_w_) &&
               !b.gp_w_.owner_before(a.gp_w_);
    }

    // gp_addr: the address it was made from (0 for the zero value), as a number: the target may
    // be gone. For hashing.
    std::uintptr_t gp_addr() const noexcept { return gp_addr_; }

private:
    weak_ptr(const std::shared_ptr<T>& sp, T* addr) noexcept
        : gp_w_(sp), gp_addr_(reinterpret_cast<std::uintptr_t>(addr)) {}

    template <typename U>
    friend weak_ptr<U> weak_make(const ptr<U>& p) noexcept;
    template <typename U>
    friend ptr<U> weak_value(const weak_ptr<U>& w) noexcept;

    std::weak_ptr<T> gp_w_;
    std::uintptr_t gp_addr_ = 0;
};

static_assert(sizeof(weak_ptr<int>) == 24,
              "gp::weak_ptr is 24 bytes: std::weak_ptr and the address (D15, step 3.17)");

// weak_make(p): Go's weak.Make(p). weak_make(nil) is the zero weak pointer.
template <typename T>
weak_ptr<T> weak_make(const ptr<T>& p) noexcept {
    return weak_ptr<T>(p.gp_shared(), p.gp_get());
}

// weak_value(w): Go's w.Value(): the pointer w was made from while its target lives, nil after
// the last pointer to the target's block went away (and nil for the zero weak pointer).
template <typename T>
ptr<T> weak_value(const weak_ptr<T>& w) noexcept {
    return ptr<T>(w.gp_w_.lock());
}

} // namespace gp

// Pointers and weak pointers are comparable in Go, so they can be map keys (Phase 03 step
// 3.7): they hash by address, the same thing == compares (the address itself, as libstdc++'s
// std::hash<T*> does).
template <typename T>
struct std::hash<gp::ptr<T>> {
    std::size_t operator()(const gp::ptr<T>& p) const noexcept {
        return static_cast<std::size_t>(reinterpret_cast<std::uintptr_t>(p.gp_get()));
    }
};

template <typename T>
struct std::hash<gp::weak_ptr<T>> {
    std::size_t operator()(const gp::weak_ptr<T>& w) const noexcept {
        return static_cast<std::size_t>(w.gp_addr());
    }
};
