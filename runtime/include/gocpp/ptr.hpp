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
//     &f().f              gp::field_ptr(f(), &T::f)        the same, f() evaluated once
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
#include <atomic>
#include <cstddef>
#include <cstdint>
#include <memory>
#include <mutex>
#include <shared_mutex>
#include <type_traits>
#include <vector>
#include <algorithm>

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
    // operation). There is no implicit conversion to another gocpp pointer type: Go has none
    // either (ptr_cast is the explicit conversion). The one implicit conversion is to the raw
    // pointer, below.
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

    // A gocpp pointer converts implicitly to T*, for C and C++ functions that take one (Phase 04
    // §2.7): nil gives nullptr. The raw pointer is borrowed: it doesn't keep the object alive,
    // so C must not keep it after the last gocpp pointer to the object is gone (cgo's rule).
    // gp::ptr<void> (unsafe.Pointer) gives void*. It is implicit because in opaque mode the
    // emitter can't know whether a C++ parameter takes T* or a gp::ptr<T>
    // (std::vector<gp::ptr<Node>>::push_back): both work. The conversion never wins over the
    // operators above: p == q, p == nullptr and *p match them exactly (*p keeps its nil panic).
    //
    // The two deleted conversions keep the raw pointer's own operations away from a gocpp
    // pointer, which only C and C++ parameters should see:
    //   - to bool: Go has no pointer conditions. Without it, T* would convert on to bool, and
    //     if (p), !p, bool b = p and a call of f(bool) with p would compile; with it they choose
    //     the deleted function. It also makes p < q, p <= q, p <=> q, p + 1, p - q and +p
    //     ambiguous (the built-in operators of pointers and of integers both match).
    //   - to const volatile T*: with two conversions to object pointers, delete p (a double
    //     free) and p[i] (unchecked pointer arithmetic: past the box of a gp::ptr<gp::array>)
    //     don't compile either. A parameter of type T*, const T*, void* or const void* still
    //     takes p, as only T* converts to it.
    // A C++ overload set f(bool) + f(void*) is ambiguous for p; pass p.gp_get() there. A C
    // function's `...` doesn't convert p at all (it would get a copy of the gp::ptr): clang
    // rejects that, and g++ does with -Werror=conditionally-supported (build.GeneratedFlags),
    // and the emitter writes p.gp_get() there (Phase 04 steps 4.9 and 4.10).
    operator T*() const noexcept { return gp_sp_.get(); }
    operator bool() const = delete;
    operator const volatile T*() const = delete;

    // gp_get: the address, without a nil check (null for nil). For hashing, printing and the
    // runtime's own code.
    T* gp_get() const noexcept { return gp_sp_.get(); }

    // gp_shared: the shared_ptr, for the runtime's own code (weak pointers, interfaces).
    const std::shared_ptr<T>& gp_shared() const noexcept { return gp_sp_; }

    // gp_take: the shared_ptr, moved out; this pointer is nil afterwards.
    std::shared_ptr<T> gp_take() && noexcept { return std::move(gp_sp_); }

private:
    // gp_checked: the address, or Go's nil pointer panic. With GOCPP_NOCHECKS (checks.hpp),
    // only the address: a nil dereference is then undefined behavior.
    T* gp_checked() const {
        T* p = gp_sp_.get();
#ifndef GOCPP_NOCHECKS
        if (p == nullptr) [[unlikely]] {
            panic_nil();
        }
#endif
        return p;
    }

    // gp_check_owner: the invariant above. An aliasing shared_ptr whose owner is empty has a
    // use count of 0. It is on unless GOCPP_NOCHECKS is defined, gocpp's own switch, which the
    // -nochecks build defines for every file of the program; not NDEBUG, which a
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

// bptr<T> is a borrowed pointer: the parameter type of a function or method whose pointer
// parameter provably does not outlive the call (internal/lower borrow.go, ADR-086). It is a raw
// T* with Go's nil checks (8 bytes, trivially copyable, no reference count). It owns nothing:
// the caller keeps the object alive for the whole call. The compiler passes one only when the
// argument is a temporary (it lives to the end of the full-expression), a local variable that
// nothing can change during the call, a pointer into such an object, or when the callee can't
// release any pointer (no stores of pointers, no calls of unknown code). So *p, p->f, p == q
// and p == nil mean the same as for gp::ptr, and a nil dereference panics the same way.
//
// The conversions say who keeps the object alive:
//
//     bptr(p)               implicit, from a pointer that the caller keeps alive for the call
//                           (a temporary lives until the end of the full-expression)
//     lend(p)               the generated code's spelling for an lvalue that is known to be stable
//     bptr<T>(&x)           the address of an object that is alive for the call
//
// A bptr can't become a gp::ptr again (there is no owner to share): a function that stores,
// returns or captures its pointer takes a gp::ptr<T>.
template <typename T>
class bptr {
public:
    constexpr bptr() noexcept = default;
    constexpr bptr(std::nullptr_t) noexcept {}
    constexpr explicit bptr(T* raw) noexcept : gp_p_(raw) {}

    // From a pointer: for the call that the argument is in. C++ code calls a borrowing function
    // with its own gp::ptr (the caller keeps it alive, as for any C++ reference); the generated
    // code writes every borrow of a named pointer as gp::lend(p) or copies it first (ADR-086), so
    // that it is visible which pointers a call relies on.
    bptr(const ptr<T>& p) noexcept : gp_p_(p.gp_get()) {}

    constexpr bptr(const bptr&) noexcept = default;
    constexpr bptr& operator=(const bptr&) noexcept = default;

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

    friend bool operator==(const bptr& a, const bptr& b) noexcept { return a.gp_p_ == b.gp_p_; }
    friend bool operator==(const bptr& a, std::nullptr_t) noexcept { return a.gp_p_ == nullptr; }
    friend bool operator==(const bptr& a, const ptr<T>& b) noexcept {
        return a.gp_p_ == b.gp_get();
    }

    // The raw pointer, for C and C++ functions (borrowed, as gp::ptr's): see ptr::operator T*.
    operator T*() const noexcept { return gp_p_; }
    operator bool() const = delete;
    operator const volatile T*() const = delete;

    T* gp_get() const noexcept { return gp_p_; }

private:
    T* gp_checked() const {
#ifndef GOCPP_NOCHECKS
        if (gp_p_ == nullptr) [[unlikely]] {
            panic_nil();
        }
#endif
        return gp_p_;
    }

    T* gp_p_ = nullptr;
};

static_assert(sizeof(bptr<int>) == sizeof(int*), "gp::bptr is a raw pointer");
static_assert(std::is_trivially_copyable_v<bptr<int>>);

// lend(p): p as a borrowed pointer, for the call that the caller makes next. The caller must be
// sure that p's object stays alive until the call returns (see bptr).
template <typename T>
bptr<T> lend(const ptr<T>& p) noexcept {
    return bptr<T>(p.gp_get());
}
template <typename T>
bptr<T> lend(bptr<T> p) noexcept {
    return p;
}

namespace detail {

// owning_t<P>: the Go-level value collected for defer/go, before any frame or tuple is built.
// Strip references here, not just in the eventual storage: sibling argument copies can call
// back into Go before collection finishes. A borrowed gp::bptr<T> needs an owning gp::ptr<T>.
template <typename P>
struct owning {
    using type = P;
};
template <typename T>
struct owning<bptr<T>> {
    using type = ptr<T>;
};
template <typename P>
using owning_t = typename owning<std::remove_cvref_t<P>>::type;

} // namespace detail

namespace detail {

// The Go-level parameter type of a C++ parameter type: what a gp::func of the function's Go
// signature passes. A borrowed gp::bptr<T> is a gp::ptr<T> there, and a const reference X (ADR-086)
// is an X.
template <typename P>
struct go_param {
    using type = P;
};
template <typename T>
struct go_param<bptr<T>> {
    using type = ptr<T>;
};
template <typename X>
struct go_param<const X&> {
    using type = X;
};
template <typename P>
using go_param_t = typename go_param<P>::type;

template <typename P>
inline constexpr bool is_bptr = false;
template <typename T>
inline constexpr bool is_bptr<bptr<T>> = true;

// pass_param<P>(a): the argument that the function takes as P, made from its Go-level parameter a
// (a named parameter of the adapter): lent for a borrowed pointer, the parameter itself for a const
// reference (it lives for the call), moved otherwise.
template <typename P, typename A>
decltype(auto) pass_param(A& a) {
    if constexpr (is_bptr<P>) {
        return lend(a);
    } else if constexpr (std::is_lvalue_reference_v<P>) {
        return (a);
    } else {
        return static_cast<P&&>(a);
    }
}

template <typename Fn, Fn F>
struct adapt_for;
template <typename R, typename... P, R (*F)(P...)>
struct adapt_for<R (*)(P...), F> {
    static R call(go_param_t<P>... a) { return F(pass_param<P>(a)...); }
};

} // namespace detail

// adapt<&f>::call is the function that takes f's Go-level parameters (an owning gp::ptr<T> for a
// borrowed gp::bptr<T>, a value for a const reference X) and calls f: the entry of a gp::func made
// from a function that borrows or takes references (gp::func_of<&gp::adapt<&f>::call>()). One
// function for each f, so a func value of f has one code address wherever it is made.
template <auto F>
using adapt = detail::adapt_for<decltype(F), F>;

// borrow(&x): the address of an object that stays alive for the call (a field of the object a
// borrowed or lent pointer points to, a package variable, a boxed variable), as a borrowed
// pointer.
template <typename T>
bptr<T> borrow(T* p) noexcept {
    return bptr<T>(p);
}

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

// foreign_owner() is the owner of memory that gocpp doesn't own (pointers from C, an address
// made from a number): a count that is never freed and frees nothing. It is a separate one from
// static_owner() (package-level variables), so that C memory doesn't change that count. The
// pointer to it is made once and never deleted, as static_owner's.
//
// Unlike static_owner(), it stores a non-null pointer (the address of a static byte): a
// std::shared_ptr compares equal to nullptr when its stored pointer is null, however many
// owners it has, and gp::string tests its owner that way (gp_c_terminated: "no owner, so a NUL
// follows"). A string over C memory must not look like one that has no owner.
inline char foreign_anchor = 0;
inline const std::shared_ptr<void>& foreign_owner() noexcept {
    static const std::shared_ptr<void>* const owner =
        new std::shared_ptr<void>(static_cast<void*>(&foreign_anchor), [](void*) {});
    return *owner;
}

namespace detail {

// The registry of the C structs that gocpp owns (Phase 04 step 4.10): an object made by &T{...} of
// a C type (gp::cgo::box_keep) is registered under its address while it lives. A raw T* that C code
// or a member of a C struct holds is the address of such an object when the program stored a
// gocpp pointer in that member; reading the member back gives the raw address, and converting it
// to a gocpp pointer (a cptr) finds the owner here, so the pointer keeps the object alive again:
// head = head.next, return cur.next, next := cur.next. Memory the registry doesn't know is C's
// (foreign_owner). The table is split in shards by address, each with its own lock, and costs
// one atomic load while nothing is registered.
inline constexpr std::size_t box_shard_count = 128;
// A shard is an open-addressing hash table (linear probing) from the address of an object to its
// block: no allocation per object, and a probe or two per lookup. An object is never taken out
// when it dies: its entry expires (the weak pointer), a lookup of it finds nothing, the next object
// at the same address replaces it, and a rehash drops what has expired.
struct box_slot {
    const void* gp_key = nullptr; // nullptr: empty
    std::weak_ptr<void> gp_block;
};
struct box_shard {
    std::shared_mutex gp_mu;
    std::vector<box_slot> gp_slots;      // a power of two long, or empty
    std::size_t gp_used = 0;             // slots with a key (live or expired)
    std::atomic<std::size_t> gp_live{0}; // registered objects that have not died

    static std::size_t hash(const void* k) noexcept {
        return static_cast<std::size_t>(
            (reinterpret_cast<std::uintptr_t>(k) >> 4) * 0x9E3779B97F4A7C15ull >> 20);
    }
    static constexpr std::size_t npos = static_cast<std::size_t>(-1);

    // find returns the index of the slot with the key, or npos.
    std::size_t find(const void* k) const noexcept {
        if (gp_slots.empty()) {
            return npos;
        }
        const std::size_t mask = gp_slots.size() - 1;
        for (std::size_t i = hash(k) & mask;; i = (i + 1) & mask) {
            const void* sk = gp_slots[i].gp_key;
            if (sk == k) {
                return i;
            }
            if (sk == nullptr) {
                return npos;
            }
        }
    }
    void insert(const void* k, const std::shared_ptr<void>& sp) {
        std::size_t i = find(k);
        if (i != npos) {
            gp_slots[i].gp_block = sp; // the address is a new object's
            return;
        }
        if ((gp_used + 1) * 4 > gp_slots.size() * 3) {
            rehash();
        }
        const std::size_t mask = gp_slots.size() - 1;
        for (i = hash(k) & mask; gp_slots[i].gp_key != nullptr; i = (i + 1) & mask) {
        }
        gp_slots[i].gp_key = k;
        gp_slots[i].gp_block = sp;
        gp_used++;
    }

private:
    // rehash makes room: it drops the entries of dead objects when many of the slots are theirs
    // (gp_live counts the objects that haven't died, so no entry is touched to tell), and doubles
    // the table if what is left is more than a third of it.
    void rehash() {
        const bool purge = gp_used > 2 * gp_live.load(std::memory_order_relaxed) + 32;
        const std::size_t live = purge ? gp_live.load(std::memory_order_relaxed) : gp_used;
        std::size_t cap = std::max<std::size_t>(64, gp_slots.size());
        if (live * 3 + 3 > cap) {
            cap *= 2;
        }
        std::vector<box_slot> old(cap);
        old.swap(gp_slots);
        gp_used = 0;
        const std::size_t mask = cap - 1;
        for (box_slot& e : old) {
            if (e.gp_key != nullptr && !(purge && e.gp_block.expired())) {
                std::size_t i = hash(e.gp_key) & mask;
                while (gp_slots[i].gp_key != nullptr) {
                    i = (i + 1) & mask;
                }
                gp_slots[i].gp_key = e.gp_key;
                gp_slots[i].gp_block = std::move(e.gp_block);
                gp_used++;
            }
        }
    }
};
inline std::atomic<std::size_t> box_count{0};
inline box_shard& box_shard_of(const void* a) noexcept {
    static box_shard* const shards = new box_shard[box_shard_count];
    const auto u = reinterpret_cast<std::uintptr_t>(a);
    return shards[((u >> 4) ^ (u >> 11)) % box_shard_count];
}
// box_register records obj as the object of the block sp.
inline void box_register(const void* obj, const std::shared_ptr<void>& sp) {
    box_shard& s = box_shard_of(obj);
    {
        std::unique_lock<std::shared_mutex> lock(s.gp_mu);
        s.insert(obj, sp);
        s.gp_live.fetch_add(1, std::memory_order_relaxed);
    }
    box_count.store(1, std::memory_order_relaxed); // (not empty any more)
}
// box_died is the deleter's note that the registered object at obj is destroyed.
inline void box_died(const void* obj) noexcept {
    box_shard_of(obj).gp_live.fetch_sub(1, std::memory_order_relaxed);
}
// box_owner returns the block of the registered object at a, or an empty pointer (also when the
// object is dead).
inline std::shared_ptr<void> box_owner(const void* a) noexcept {
    if (a == nullptr || box_count.load(std::memory_order_relaxed) == 0) {
        return nullptr;
    }
    box_shard& s = box_shard_of(a);
    std::shared_lock<std::shared_mutex> lock(s.gp_mu);
    const std::size_t i = s.find(a);
    if (i == box_shard::npos) {
        return nullptr;
    }
    std::shared_ptr<void> sp = s.gp_slots[i].gp_block.lock();
    return sp.get() == a ? sp : nullptr;
}

} // namespace detail

// raw_owned(raw) is a gocpp pointer to the object at the raw address: it shares the block of a
// registered C struct that gocpp owns, else it owns nothing (foreign_owner), as every pointer that
// comes from C. Nil for null.
template <typename T>
ptr<T> raw_owned(T* raw) noexcept {
    if (raw == nullptr) {
        return ptr<T>();
    }
    if constexpr (std::is_object_v<T> || std::is_void_v<T>) {
        if (std::shared_ptr<void> o = detail::box_owner(static_cast<const void*>(raw))) {
            return ptr<T>(std::move(o), raw);
        }
    }
    return ptr<T>(foreign_owner(), raw);
}

// static_ptr(x): &x for an object that is never freed: a package-level variable, or a part of
// one (&global.f, &globalArr[i] after the index check). The pointer shares the static owner.
template <typename T>
ptr<T> static_ptr(T& x) noexcept {
    return ptr<T>(static_owner(), std::addressof(x));
}

// stack_ptr(x): &x for a variable on the C++ stack that the escape analysis (internal/lower
// escape.go, ADR-114) proved is never reached by a pointer that outlives it. The pointer owns
// nothing: copying and dropping it touch no reference count (the control block is empty), and
// it must not be used where an owner is needed (an aliasing pointer, a weak pointer, an
// interface), which the analysis never allows. A bug in the analysis would not be a leak but a
// dangling pointer, so the owner checks of the aliasing constructors still abort on one.
template <typename T>
ptr<T> stack_ptr(T& x) noexcept {
    return ptr<T>(std::shared_ptr<T>(std::shared_ptr<void>(), std::addressof(x)));
}

// local_ptr<T> is the variable `p := &T{...}` or `p := new(T)` when the object never escapes: a
// gp::ptr<T> that holds its object in itself, on the stack (gp_obj), so that the object is built
// with the variable and dies with it. It is made by local_new<T>(args...), which is T's
// constructor arguments (the literal, or none for the zero value).
template <typename T>
class local_ptr : public ptr<T> {
public:
    template <typename... Args>
    explicit local_ptr(Args&&... args) : gp_obj(std::forward<Args>(args)...) {
        static_cast<ptr<T>&>(*this) = stack_ptr(gp_obj);
    }
    local_ptr(const local_ptr&) = delete;
    // (without this one, the constructor above would take the other pointer as T's argument)
    local_ptr(local_ptr&&) = delete;
    local_ptr& operator=(const local_ptr&) = delete;

    T gp_obj;
};

template <typename T, typename... Args>
local_ptr<T> local_new(Args&&... args) {
    return local_ptr<T>(std::forward<Args>(args)...);
}

// borrow_tmp(T{...}): the address of a temporary that lives to the end of the full-expression:
// &T{...} passed straight to a borrowed parameter (escape.go).
template <typename T>
bptr<std::remove_reference_t<T>> borrow_tmp(T&& tmp) noexcept {
    return bptr<std::remove_reference_t<T>>(std::addressof(tmp));
}

// cptr<T> is a gocpp pointer to a C/C++ type T (Phase 04 step 4.10): what &T{...} of a C type,
// and &s.f and &s[i] of a gocpp struct or slice with C/C++ elements, give. It owns what it
// points into, like any gp::ptr<T>, and converts to the raw T* that C takes (borrowed). Unlike
// gp::ptr<T>, a raw T* converts to it too, and is assigned to it: a variable that starts as
// &T{...} then walks a list that C built, p = p.next, as Go's *T does. Such a pointer from C
// owns nothing (foreign_owner), as every pointer that comes from C.
//
// A gocpp variable, parameter, field, element or result declared with a C pointer type (*lib.T)
// is a cptr too, so a pointer that is returned, stored or appended keeps its object alive. The
// raw T* is what the members of C structs and C functions' results hold: a cptr converts to it
// (borrowed) and a raw T* converts to a cptr (owning nothing); gp::cgo::store_keep keeps the
// object of a cptr that is stored into a member (interop.hpp).
template <typename T>
class cptr : public ptr<T> {
public:
    constexpr cptr() noexcept = default;
    constexpr cptr(std::nullptr_t) noexcept {}
    cptr(const ptr<T>& p) noexcept : ptr<T>(p) {}
    cptr(ptr<T>&& p) noexcept : ptr<T>(std::move(p)) {}
    cptr(T* raw) noexcept : ptr<T>(raw_owned(raw)) {}
};

// new_init<T>(init): &T{...} for a struct larger than 128 KiB (step 3.8): a pointer to a new zero
// T, made in its block, which init(T&) then fills with the literal's fields, so the literal is
// never a temporary on the C++ stack (D24).
template <typename T, typename F>
ptr<T> new_init(F&& init) {
    ptr<T> p = new_<T>();
    std::forward<F>(init)(*p);
    return p;
}

// field_ptr(p, &T::f): &p.f for a pointer p that is not a variable (&f().f, &f()[i].f, step
// 3.8): a pointer to the field that shares p's owner, with p evaluated once. A nil p panics like
// Go. (For a variable p, generated code writes gp::ptr<F>(p, &p->f).)
template <typename T, typename F>
ptr<F> field_ptr(ptr<T> p, F T::* f) {
    F* q = &((*p).*f);
    return ptr<F>(std::move(p), q);
}

// field_ptr(p, &T::f) for a borrowed p: the borrowed pointer to the field (a nil p panics like Go).
// The order of evaluation uses it for the nil check of a promoted method's receiver (`*field_ptr`);
// a borrowed pointer owns nothing, so the result owns nothing either.
template <typename T, typename F>
bptr<F> field_ptr(bptr<T> p, F T::* f) {
    return bptr<F>(&((*p).*f));
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
struct std::hash<gp::cptr<T>> {
    std::size_t operator()(const gp::cptr<T>& p) const noexcept {
        return static_cast<std::size_t>(reinterpret_cast<std::uintptr_t>(p.gp_get()));
    }
};

template <typename T>
struct std::hash<gp::weak_ptr<T>> {
    std::size_t operator()(const gp::weak_ptr<T>& w) const noexcept {
        return static_cast<std::size_t>(w.gp_addr());
    }
};
