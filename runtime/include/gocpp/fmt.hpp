// gocpp/fmt.hpp: Go's package fmt (Phase 05 step 5.11, ADR 028): Print, Println, Printf, Sprint,
// Sprintln, Sprintf, Fprint, Fprintln, Fprintf, Append, Appendln, Appendf and Errorf, with the
// interfaces fmt.Stringer, fmt.GoStringer, fmt.Formatter and fmt.State. The Scan functions,
// fmt.Scanner and fmt.ScanState are in gocpp/fmtscan.hpp, included at the end of this file.
//
// The formatting is a port of Go 1.25's fmt/print.go and fmt/format.go (and of strconv's float and
// quote formatting) in runtime/src/fmt.cpp, driven by TYPE DESCRIPTORS (gocpp/type.hpp), the way
// Go's is driven by reflect: an operand is a pair {descriptor, address of the value}, and the
// printer walks the value by the descriptor's kind, fields, elements and operations
// (gocpp/typeops.hpp). So one engine prints every value: statically typed ones and the ones in an
// `any`, error or Stringer. The emitter passes each operand with its descriptor; the cheap ones
// need none (below), so printing a number or a string costs no allocation.
//
// What generated code writes (internal/lower/fmtcall.go):
//
//     fmt.Println(n, s, ok)       gp::fmt::Println(n, s, ok);     // int, string, bool: plain
//     fmt.Println(p)              gp::fmt::Println(gp::fmt::as(&gp_td_Point, p));
//     fmt.Printf("%v", xs)        gp::fmt::Printf("%v"_gs, gp::fmt::as(gp_tdc_0, xs));
//     fmt.Println(err, x)         gp::fmt::Println(err, x);       // error, any: the dynamic type
//     fmt.Println(n64)            gp::fmt::Println(gp::fmt::as(&gp::td_int64, n64));
//     fmt.Fprintf(os.Stderr, ...) gp::fmt::Fprintf(gp::fmt::stderr_, ...);
//     fmt.Errorf("%w", err)       gp::fmt::Errorf("%w"_gs, err);
//
// A plain operand is a value whose C++ type says its Go type: bool, int8..int32, uint8..uint32,
// float32, float64, complex64, complex128, gp::string, an interface value (gp::any, gp::error,
// any interface struct: its dynamic type is used, nil prints <nil>), and nullptr (an untyped nil).
// C++ cannot tell Go's int from int64, nor uint from uint64 and uintptr (int64_t and uint64_t), so
// those are int and uint when plain, and gp::fmt::as(&gp::td_int64, n) says otherwise. Every other
// operand is gp::fmt::as(descriptor, value). The operands are taken by value, so they are the
// values at the call, as Go copies them into interfaces; an array or struct larger than 128 KiB is
// copied to the heap by the emitter (gp::fmt::heap, D24) and prints as the value.
//
// What prints (the rules are Go's; docs/spec/cpp-mapping.md "fmt" lists the few differences):
//   - the Go types and their pieces: numbers, strings, structs (%v {1 2}, %+v {A:1 B:2}, %#v
//     main.T{A:1, B:2}), slices, arrays, maps (sorted by key, internal/fmtsort), pointers (&{...}
//     at the top level, an address inside), interfaces, funcs (the address), unsafe.Pointer;
//   - methods: Format (fmt.Formatter, with a fmt.State), GoString for %#v, Error and String for
//     %v %s %x %X %q, not for a value reached through an unexported field; a panic in a method
//     prints %!v(PANIC=String method: ...), <nil> for a nil receiver;
//   - Errorf with %w: *fmt.wrapError, *fmt.wrapErrors, and errors.New's *errors.errorString;
//   - C/C++ values (docs/spec/interop.md "Printing C/C++ values"): numbers and bools as the Go
//     type of the same kind, enums as numbers, pointers as addresses, std::string as a string,
//     a class through its std::formatter or operator<< (registered by gp::cgo::fmt_arg at the
//     call, for the types in its operand); a class held in an any, whose text nobody registered,
//     prints as the address of its box.
//
// Each Print, Println and Printf call formats everything first and writes it with one write,
// after flushing C's stdio (gp::detail::flush_c_stdio), so the output order is Go's.
#pragma once

#include <gocpp/array.hpp>
#include <gocpp/checks.hpp>
#include <gocpp/complex.hpp>
#include <gocpp/error.hpp>
#include <gocpp/iface.hpp>
#include <gocpp/map.hpp>
#include <gocpp/panic.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/slice.hpp>
#include <gocpp/string.hpp>
#include <gocpp/type.hpp>

#include <concepts>
#include <cstddef>
#include <atomic>
#include <memory>
#include <mutex>
#include <cstdint>
#include <cstring>
#include <iosfwd>
#include <string>
#include <string_view>
#include <tuple>
#include <type_traits>
#include <typeinfo>
#include <utility>
#include <vector>

namespace gp::fmt {

// --- the interfaces of package fmt ---
//
// Interface structs like the emitter writes (gocpp/error.hpp is the model); the lowering maps the
// Go types fmt.Stringer, fmt.GoStringer, fmt.Formatter and fmt.State to them.

struct Stringer : iface {
    using iface::iface;
    constexpr Stringer() noexcept = default;
    static const type_desc* gp_desc() noexcept;
    gp::string String() const;
};

struct GoStringer : iface {
    using iface::iface;
    constexpr GoStringer() noexcept = default;
    static const type_desc* gp_desc() noexcept;
    gp::string GoString() const;
};

// State is fmt.State: what a Format method gets. Its methods, sorted: Flag, Precision, Width,
// Write.
struct State : iface {
    using iface::iface;
    constexpr State() noexcept = default;
    static const type_desc* gp_desc() noexcept;
    bool Flag(int64_t c) const;
    std::tuple<int64_t, bool> Precision() const;
    std::tuple<int64_t, bool> Width() const;
    std::tuple<int64_t, gp::error> Write(gp::slice<uint8_t> b) const;
};

struct Formatter : iface {
    using iface::iface;
    constexpr Formatter() noexcept = default;
    static const type_desc* gp_desc() noexcept;
    void Format(State f, int32_t verb) const;
};

static_assert(sizeof(Stringer) == sizeof(iface) && sizeof(State) == sizeof(iface),
              "the fmt interfaces have no members of their own");

namespace detail {
// The func types of the methods (imethod::gp_ftype; ft_string: gocpp/error.hpp).
inline const type_desc* ft_int_bool() {
    static const type_desc* const d = td_sig({&td_int}, {&td_bool});
    return d;
}
inline const type_desc* ft_int_ok() {
    static const type_desc* const d = td_sig({}, {&td_int, &td_bool});
    return d;
}
inline const type_desc* ft_write() {
    static const type_desc* const d =
        td_sig({td_slice<std::uint8_t>(&td_uint8)}, {&td_int, &td_error});
    return d;
}
inline const type_desc* ft_format(); // func(fmt.State, int32), after td_state

inline constexpr imethod stringer_methods[] = {
    imethod("String", nullptr, "func() string", nullptr, &gp::detail::ft_string)};
inline constexpr imethod gostringer_methods[] = {
    imethod("GoString", nullptr, "func() string", nullptr, &gp::detail::ft_string)};
inline constexpr imethod state_methods[] = {
    imethod("Flag", nullptr, "func(int) bool", nullptr, &ft_int_bool),
    imethod("Precision", nullptr, "func() (int, bool)", nullptr, &ft_int_ok),
    imethod("Width", nullptr, "func() (int, bool)", nullptr, &ft_int_ok),
    imethod("Write", nullptr, "func([]uint8) (int, error)", nullptr, &ft_write),
};
inline constexpr imethod formatter_methods[] = {
    imethod("Format", nullptr, "func(fmt.State, int32)", nullptr, &ft_format)};
} // namespace detail

inline constexpr type_desc td_stringer =
    named<Stringer>("fmt.Stringer", "fmt", shape_interface(detail::stringer_methods));
inline constexpr type_desc td_gostringer =
    named<GoStringer>("fmt.GoStringer", "fmt", shape_interface(detail::gostringer_methods));
inline constexpr type_desc td_state =
    named<State>("fmt.State", "fmt", shape_interface(detail::state_methods));
inline constexpr type_desc td_formatter =
    named<Formatter>("fmt.Formatter", "fmt", shape_interface(detail::formatter_methods));

inline const type_desc* Stringer::gp_desc() noexcept {
    return &td_stringer;
}
inline const type_desc* GoStringer::gp_desc() noexcept {
    return &td_gostringer;
}
inline const type_desc* State::gp_desc() noexcept {
    return &td_state;
}
inline const type_desc* detail::ft_format() {
    static const type_desc* const d = td_sig({&td_state, &td_int32}, {});
    return d;
}
inline const type_desc* Formatter::gp_desc() noexcept {
    return &td_formatter;
}

inline gp::string Stringer::String() const {
    return icall<gp::string()>(*this, 0);
}
inline gp::string GoStringer::GoString() const {
    return icall<gp::string()>(*this, 0);
}
inline bool State::Flag(int64_t c) const {
    return icall<bool(int64_t)>(*this, 0, c);
}
inline std::tuple<int64_t, bool> State::Precision() const {
    return icall<std::tuple<int64_t, bool>()>(*this, 1);
}
inline std::tuple<int64_t, bool> State::Width() const {
    return icall<std::tuple<int64_t, bool>()>(*this, 2);
}
inline std::tuple<int64_t, gp::error> State::Write(gp::slice<uint8_t> b) const {
    return icall<std::tuple<int64_t, gp::error>(gp::slice<uint8_t>)>(*this, 3, std::move(b));
}
inline void Formatter::Format(State f, int32_t verb) const {
    icall<void(State, int32_t)>(*this, 0, std::move(f), verb);
}

// --- operands ---

// described<T> is a value with the descriptor of its Go type: gp::fmt::as(d, v).
template <typename T>
struct described {
    const type_desc* gp_d;
    T gp_v;
};

template <typename T>
described<std::remove_cvref_t<T>> as(const type_desc* d, T&& v) {
    return {d, std::forward<T>(v)};
}

// heap_value is an array or struct larger than 128 KiB that the emitter copied to the heap at the
// call, heap(gp::new_<A>(a)), because a copy by value would be on the C++ stack, which doesn't
// grow (D24). It prints as the value, not as a pointer.
template <typename T>
struct heap_value {
    gp::ptr<T> gp_p;
};

template <typename T>
heap_value<T> heap(gp::ptr<T> p) {
    return heap_value<T>{std::move(p)};
}

namespace detail {

// access is how a value was reached, Go's read-only flags of reflect: a value reached through an
// unexported field can't call its methods (CanInterface is false). The flag of an embedded
// unexported field (flagEmbedRO) holds for that field only: the exported fields inside it are
// open again. Any other read-only flag (flagStickyRO) holds for everything below: the fields,
// elements, keys and values. A pointer's target keeps the flag of the pointer.
enum class access : uint8_t { open, embed_ro, sticky_ro };

// value is one operand or part of one: the descriptor of its Go type and the address of the C++
// object, except that for a pointer (kind Pointer or UnsafePointer) p is the address it points to,
// own its owner. own is also set for a value inside an interface, so the box stays alive while
// the value prints. A nil interface has no descriptor.
struct value {
    const type_desc* t = nullptr;
    const void* p = nullptr;
    std::shared_ptr<void> own;
};
using arg = value;

// live_span is the []any of a call `f(a...)`: its elements are the operands, and the printer reads
// them while it prints, as Go does, so a String method that stores into the slice changes the
// operands that come after it. Empty (data is null) when the call has no such slice.
struct live_span {
    const iface* data = nullptr;
};

// no_live is what with_operands passes for a call without a final a...: there is nothing to read
// again, and the plain functions below run (no live-operand code). It converts to an empty
// live_span for the callers that do not care.
struct no_live {
    constexpr operator live_span() const noexcept { return {}; }
};

// ptr_like<T>: T is a gp::ptr (a Go pointer or unsafe.Pointer).
template <typename T>
concept ptr_like = requires(const T& v) {
    v.gp_shared();
    v.gp_get();
};

// plain_known<T>: a plain operand of C++ type T has a descriptor (plain_desc).
template <typename T>
inline constexpr bool plain_known =
    std::is_same_v<T, bool> || std::is_same_v<T, int8_t> || std::is_same_v<T, int16_t> ||
    std::is_same_v<T, int32_t> || std::is_same_v<T, int64_t> || std::is_same_v<T, uint8_t> ||
    std::is_same_v<T, uint16_t> || std::is_same_v<T, uint32_t> || std::is_same_v<T, uint64_t> ||
    std::is_same_v<T, float> || std::is_same_v<T, double> || std::is_same_v<T, gp::complex64> ||
    std::is_same_v<T, gp::complex128> || std::is_same_v<T, gp::string>;

// plain_desc<T>() is the descriptor of a plain operand of C++ type T.
template <typename T>
constexpr const type_desc* plain_desc() noexcept {
    if constexpr (std::is_same_v<T, bool>) {
        return &td_bool;
    } else if constexpr (std::is_same_v<T, int8_t>) {
        return &td_int8;
    } else if constexpr (std::is_same_v<T, int16_t>) {
        return &td_int16;
    } else if constexpr (std::is_same_v<T, int32_t>) {
        return &td_int32;
    } else if constexpr (std::is_same_v<T, int64_t>) {
        return &td_int;
    } else if constexpr (std::is_same_v<T, uint8_t>) {
        return &td_uint8;
    } else if constexpr (std::is_same_v<T, uint16_t>) {
        return &td_uint16;
    } else if constexpr (std::is_same_v<T, uint32_t>) {
        return &td_uint32;
    } else if constexpr (std::is_same_v<T, uint64_t>) {
        return &td_uint;
    } else if constexpr (std::is_same_v<T, float>) {
        return &td_float32;
    } else if constexpr (std::is_same_v<T, double>) {
        return &td_float64;
    } else if constexpr (std::is_same_v<T, gp::complex64>) {
        return &td_complex64;
    } else if constexpr (std::is_same_v<T, gp::complex128>) {
        return &td_complex128;
    } else if constexpr (std::is_same_v<T, gp::string>) {
        return &td_string;
    } else {
        return nullptr;
    }
}

// from_iface is the operand of the interface value x: its dynamic value.
inline value from_iface(const iface& x) {
    value v;
    if (x.gp_type() == nullptr) {
        return v;
    }
    v.t = x.gp_type();
    v.p = x.gp_data().get();
    v.own = x.gp_data();
    if ((v.t->gp_kind == kind::Pointer || v.t->gp_kind == kind::UnsafePointer) && v.t->gp_named &&
        v.t != &td_unsafe_pointer && v.t->gp_ops != nullptr && v.t->gp_ops->gp_target != nullptr) {
        // A named pointer type (type P *T): the box holds the pointer, not its target (an unnamed
        // pointer shares the owner of its target as the box).
        v.p = v.t->gp_ops->gp_target(v.p);
        v.own = v.t->gp_ops->gp_owner(x.gp_data().get());
        if (v.own == nullptr) {
            v.own = x.gp_data();
        }
    }
    return v;
}

// of_object is the operand for the C++ object *p of the Go type t, with the pointer rule above.
inline value of_object(const type_desc* t, const void* p) {
    value v;
    v.t = t;
    if ((t->gp_kind == kind::Pointer || t->gp_kind == kind::UnsafePointer) &&
        t->gp_ops != nullptr && t->gp_ops->gp_target != nullptr) {
        v.p = t->gp_ops->gp_target(p);
        v.own = t->gp_ops->gp_owner(p);
    } else {
        v.p = p;
    }
    return v;
}

template <typename T>
value make_value(const type_desc* d, const T& v) {
    if constexpr (std::is_base_of_v<iface, T>) {
        return detail::from_iface(v);
    } else if constexpr (ptr_like<T>) {
        value r;
        r.t = d;
        if (d->gp_kind == kind::Pointer || d->gp_kind == kind::UnsafePointer) {
            r.p = v.gp_get();
            r.own = std::shared_ptr<void>(v.gp_shared());
        } else {
            r.p = &v; // a C++ pointer type (cptr) held as a foreign value
        }
        return r;
    } else {
        return value{d, &v, {}};
    }
}

} // namespace detail

// make_arg is the operand for an argument of a Print function.
template <typename T>
detail::arg make_arg(const T& v) {
    if constexpr (std::is_base_of_v<iface, T>) {
        return detail::from_iface(v);
    } else if constexpr (std::is_null_pointer_v<T>) {
        return detail::arg{};
    } else {
        static_assert(detail::plain_known<T>,
                      "gp::fmt: a value of this C++ type needs its descriptor: gp::fmt::as(d, v)");
        return detail::arg{detail::plain_desc<T>(), &v, {}};
    }
}

template <typename T>
detail::arg make_arg(const described<T>& v) {
    return detail::make_value(v.gp_d, v.gp_v);
}

template <typename T>
detail::arg make_arg(const described<heap_value<T>>& v) {
    return detail::make_value(v.gp_d, *v.gp_v.gp_p);
}

// spread(a) is the final argument `a...` of a call, a []any: one operand for each element.
struct spread_args {
    gp::slice<gp::any> gp_s;
};

inline spread_args spread(gp::slice<gp::any> s) {
    return {std::move(s)};
}

namespace detail {

template <typename T>
inline constexpr bool is_spread = std::is_same_v<T, spread_args>;

// add_operands appends the operand(s) of the argument x to v.
template <typename T>
void add_operands(std::vector<arg>& v, const T& x) {
    if constexpr (is_spread<T>) {
        int64_t n = x.gp_s.gp_len();
        v.reserve(v.size() + static_cast<std::size_t>(n));
        for (int64_t i = 0; i < n; i++) {
            v.push_back(from_iface(x.gp_s.gp_data()[i]));
        }
    } else {
        v.push_back(make_arg(x));
    }
}

// The formatting (runtime/src/fmt.cpp). They append to out.
void sprint(std::string& out, const arg* args, std::size_t n);
void sprintln(std::string& out, const arg* args, std::size_t n);
void sprintf(std::string& out, std::string_view format, const arg* args, std::size_t n);
// errorf formats like sprintf (with %w allowed) and returns the error of fmt.Errorf.
gp::error errorf(std::string_view format, const arg* args, std::size_t n);
// The same for a call with a final a... (live_span): the operands are the elements of that slice,
// read again as each one is printed.
void sprint_live(std::string& out, const arg* args, std::size_t n, live_span lv);
void sprintln_live(std::string& out, const arg* args, std::size_t n, live_span lv);
void sprintf_live(std::string& out, std::string_view format, const arg* args, std::size_t n,
                  live_span lv);
gp::error errorf_live(std::string_view format, const arg* args, std::size_t n, live_span lv);

// write_stdout and write_stderr write the bytes with one write, after gp::detail::flush_c_stdio.
// A broken pipe ends the program with SIGPIPE, as Go's os.Stdout does.
void write_stdout(std::string_view s);
// write_stdout_checked is write_stdout that reports a failed write: null, or the text of the error
// ("write /dev/stdout: bad file descriptor").
const char* write_stdout_checked(std::string_view s);
const char* empty_write_error();
void write_stderr(std::string_view s);
// write_state calls the Write method of f.
std::tuple<int64_t, gp::error> write_state(const State& f, std::string_view s);

// quoted appends the Go quoting of s (strconv.Quote).
void append_quote(std::string& out, std::string_view s);

// is_print is strconv.IsPrint.
bool is_print(char32_t r) noexcept;

// float_v and float_f append a float as %v and as %f do; bits is 32 for a float32 (whose shortest
// digits differ: float32(0.1) is 0.1).
void float_v(std::string& out, double v, int bits);
void float_f(std::string& out, double v);

// append_float is strconv.AppendFloat(out, v, fmt, prec, bits).
void append_float(std::string& out, double v, char fmt, int prec, int bits);

// cxx_type_name appends the C++ name of the type t, demangled ("std::vector<long,
// std::allocator<long> >", "char*").
void cxx_type_name(std::string& out, const std::type_info& t);

// stream_text appends what put writes to a std::ostream: put(os, v) is os << *v for v's type.
void stream_text(std::string& out, void (*put)(std::ostream&, const void*), const void* v);

// spread_data is the data of the slice of the final spread argument of args, or null.
template <typename T, typename... Ts>
const iface* spread_data(const T& x, const Ts&... rest) {
    if constexpr (is_spread<T>) {
        return x.gp_s.gp_data();
    } else if constexpr (sizeof...(Ts) > 0) {
        return spread_data(rest...);
    } else {
        return nullptr;
    }
}

// with_operands calls f(operands, count, live) with the operands of args: one for each argument,
// and for a final spread (`a...`) one for each element of the slice. live is a live_span for a call
// with such a slice, and no_live for the others.
template <typename F, typename... Ts>
auto with_operands(F&& f, const Ts&... args) {
    if constexpr ((is_spread<Ts> || ...)) {
        std::vector<arg> v;
        (add_operands(v, args), ...);
        return f(v.data(), v.size(), live_span{spread_data(args...)});
    } else if constexpr (sizeof...(Ts) > 0) {
        const arg a[] = {make_arg(args)...};
        return f(a, sizeof...(Ts), no_live{});
    } else {
        return f(static_cast<const arg*>(nullptr), std::size_t(0), no_live{});
    }
}

template <typename... Ts>
std::string run(void (*f)(std::string&, const arg*, std::size_t),
                void (*fl)(std::string&, const arg*, std::size_t, live_span), const Ts&... args) {
    std::string out;
    detail::with_operands(
        [&](const arg* a, std::size_t n, auto lv) {
            if constexpr (std::is_same_v<decltype(lv), no_live>) {
                f(out, a, n);
            } else {
                fl(out, a, n, lv);
            }
            return 0;
        },
        args...);
    return out;
}

template <typename... Ts>
std::string run_f(std::string_view format, const Ts&... args) {
    std::string out;
    detail::with_operands(
        [&](const arg* a, std::size_t n, auto lv) {
            if constexpr (std::is_same_v<decltype(lv), no_live>) {
                detail::sprintf(out, format, a, n);
            } else {
                detail::sprintf_live(out, format, a, n, lv);
            }
            return 0;
        },
        args...);
    return out;
}

inline std::tuple<int64_t, gp::error> written(const std::string& s) {
    return {static_cast<int64_t>(s.size()), gp::error()};
}

// --- Print, Println and Printf: os.Stdout ---
//
// They write to file descriptor 1 themselves, one write, unless the program changed os.Stdout (the
// variable, or the file in it) or closed the original: then they write through the *os.File that
// the variable holds. Bit 0 of a std_gate is set (and stays set) when the program may have changed
// it; the higher bits count the calls of the slow path that write directly.
//
// File.Close of the original os.Stdout must not run while a Print is between its check and its
// write (descriptor 1 could be reused by the next file, and the text would go there). The fast path
// announces itself in a slot of its own thread (a plain store: no atomic read-modify-write on every
// Print), and Close sets bit 0, then makes all threads pass a memory barrier
// (gp::os::process_barrier), then waits for the slots and the counter to drain: a Print either sees
// bit 0 and takes the slow path, or Close sees its slot. Where the system has no such barrier, bit
// 0 is set at the first Print or Close (ensure_process_barrier) and every Print takes the counted
// slow path. The calls that change the variable are the lowering's: an assignment to os.Stdout,
// os.Stdin or os.Stderr is gp::fmt::store_std, and File.Close of the original streams calls
// std_closing.
struct std_gate {
    std::atomic<uint32_t> word{0};
};
extern std_gate stdout_gate;
extern std_gate stdin_gate;

struct print_slot {
    std::atomic<uint32_t> inflight{0};
    print_slot* next = nullptr;
};
inline thread_local print_slot* tls_print_slot = nullptr;
print_slot* register_print_slot();
// ensure_process_barrier prepares the process-wide barrier (once); register_print_slot and
// defer_close call it. forget_process_barrier makes the next call prepare again (tests only).
void ensure_process_barrier();
void forget_process_barrier() noexcept;

// A Close of the original stream with prints (or scans) in flight does not close the descriptor:
// the last one out does (finish_close), as with Go's descriptor reference counts.
// pending_close.flag announces the close before its process-wide barrier starts, so a Print
// leaving during that barrier cannot miss it. The mutex protects the callback AND its readiness:
// a last user must not claim the close until defer_close has finished the barrier. Only these cold
// close paths lock it; the ordinary Print fast path still just tests flag.
// close is a pointer to the callback (null: none), owned by whoever takes it: an object with a
// destructor would register it at startup, and a runtime library member has no static initializer.
struct pending_close {
    std::atomic<bool> flag{false};
    std::mutex mu;
    gp::func<void()>* close = nullptr;
};
extern pending_close pending_stdout;
extern pending_close pending_stdin;
bool slots_drained() noexcept;
bool close_drained(bool is_stdout) noexcept;
void finish_close(bool is_stdout);

// The cold paths (runtime/src/fmtstd.cpp, fmt.cpp).
std::tuple<int64_t, gp::error> print_slow(const std::string& s);
// write_failed is the (0, error) of a Print whose write failed: "write /dev/stdout: <what>".
std::tuple<int64_t, gp::error> write_failed(const char* what, const std::string& s);

// print_out writes the text of Print, Println and Printf to os.Stdout.
inline std::tuple<int64_t, gp::error> print_out(const std::string& s) {
    print_slot* sl = tls_print_slot;
    if (sl == nullptr) [[unlikely]] {
        sl = register_print_slot();
    }
    sl->inflight.store(1, std::memory_order_relaxed);
    std::atomic_signal_fence(std::memory_order_seq_cst);
    if (stdout_gate.word.load(std::memory_order_relaxed) & 1) [[unlikely]] {
        sl->inflight.store(0, std::memory_order_relaxed);
        return print_slow(s);
    }
    const char* failed = write_stdout_checked(s);
    sl->inflight.store(0, std::memory_order_release);
    if (pending_stdout.flag.load(std::memory_order_relaxed)) [[unlikely]] {
        finish_close(true);
    }
    if (failed != nullptr) [[unlikely]] {
        return write_failed(failed, s);
    }
    return written(s);
}

} // namespace detail

// --- the functions ---

// Sprint is fmt.Sprint: it adds a space between two arguments when neither is a string.
template <typename... Ts>
gp::string Sprint(Ts... args) {
    return gp::string::gp_copy(detail::run(&detail::sprint, &detail::sprint_live, args...));
}

// Sprintln is fmt.Sprintln: a space between all arguments, and a newline at the end.
template <typename... Ts>
gp::string Sprintln(Ts... args) {
    return gp::string::gp_copy(detail::run(&detail::sprintln, &detail::sprintln_live, args...));
}

// Sprintf is fmt.Sprintf.
template <typename... Ts>
gp::string Sprintf(gp::string format, Ts... args) {
    return gp::string::gp_copy(detail::run_f(format.gp_view(), args...));
}

// Print is fmt.Print.
template <typename... Ts>
std::tuple<int64_t, gp::error> Print(Ts... args) {
    return detail::print_out(detail::run(&detail::sprint, &detail::sprint_live, args...));
}

// Println is fmt.Println.
template <typename... Ts>
std::tuple<int64_t, gp::error> Println(Ts... args) {
    return detail::print_out(detail::run(&detail::sprintln, &detail::sprintln_live, args...));
}

// Printf is fmt.Printf.
template <typename... Ts>
std::tuple<int64_t, gp::error> Printf(gp::string format, Ts... args) {
    return detail::print_out(detail::run_f(format.gp_view(), args...));
}

// The writers that Fprint, Fprintln and Fprintf take: os.Stdout and os.Stderr (the emitter writes
// gp::fmt::stdout_ and stderr_ for them), and a fmt.State, the argument of a Format method.
// (The other io.Writers come with package io, Phase 09.)
struct fd_writer {
    int gp_fd;
};
inline constexpr fd_writer stdout_{1};
inline constexpr fd_writer stderr_{2};

namespace detail {
inline std::tuple<int64_t, gp::error> write_to(const fd_writer& w, const std::string& s) {
    if (w.gp_fd == 2) {
        write_stderr(s);
    } else {
        write_stdout(s);
    }
    return written(s);
}
inline std::tuple<int64_t, gp::error> write_to(const State& w, const std::string& s) {
    return write_state(w, s);
}
// Any other writer: a value with a Write([]byte) (int, error) method, an io.Writer or the
// interface that the emitter made of a *os.File or another type that has one. Like Go's Fprint,
// it makes one Write call with the whole text and returns what Write returned.
template <typename W>
    requires(!std::is_same_v<W, State> &&
             requires(const W& w, gp::slice<uint8_t> b) {
                 { w.Write(b) } -> std::same_as<std::tuple<int64_t, gp::error>>;
             })
std::tuple<int64_t, gp::error> write_to(const W& w, const std::string& s) {
    auto b = gp::make_slice<uint8_t>(static_cast<int64_t>(s.size()));
    if (!s.empty()) {
        std::memcpy(b.gp_data(), s.data(), s.size());
    }
    return w.Write(std::move(b));
}
} // namespace detail

template <typename W, typename... Ts>
std::tuple<int64_t, gp::error> Fprint(const W& w, Ts... args) {
    return detail::write_to(w, detail::run(&detail::sprint, &detail::sprint_live, args...));
}

template <typename W, typename... Ts>
std::tuple<int64_t, gp::error> Fprintln(const W& w, Ts... args) {
    return detail::write_to(w, detail::run(&detail::sprintln, &detail::sprintln_live, args...));
}

template <typename W, typename... Ts>
std::tuple<int64_t, gp::error> Fprintf(const W& w, gp::string format, Ts... args) {
    return detail::write_to(w, detail::run_f(format.gp_view(), args...));
}

// Append, Appendln and Appendf: the text appended to b.
template <typename... Ts>
gp::slice<uint8_t> Append(gp::slice<uint8_t> b, Ts... args) {
    return gp::append_slice(std::move(b), gp::string::gp_copy(detail::run(
                                              &detail::sprint, &detail::sprint_live, args...)));
}

template <typename... Ts>
gp::slice<uint8_t> Appendln(gp::slice<uint8_t> b, Ts... args) {
    return gp::append_slice(std::move(b), gp::string::gp_copy(detail::run(
                                              &detail::sprintln, &detail::sprintln_live, args...)));
}

template <typename... Ts>
gp::slice<uint8_t> Appendf(gp::slice<uint8_t> b, gp::string format, Ts... args) {
    return gp::append_slice(std::move(b),
                            gp::string::gp_copy(detail::run_f(format.gp_view(), args...)));
}

// --- the standard streams of package os (stdlib/os/gocpp_helpers.gp) ---
//
// set_os_stdout is called by the init of package os with &os.Stdout, the *os.file that os.Stdout
// holds, the flag that File.Close sets for the original and a function that writes to a *os.File.
// The slow path of Print, Println and Printf (std_gate) takes an atomic snapshot of the variable
// and writes to the file in the snapshot, so a goroutine that swaps os.Stdout (store_std) while
// others print cannot free the file under them.
namespace detail {
// pelem_t<gp::ptr<X>> is X.
template <typename T>
struct pelem {};
template <typename X>
struct pelem<gp::ptr<X>> {
    using gp_type = X;
};
template <typename T>
using pelem_t = typename pelem<T>::gp_type;

// os_std describes os.Stdout or os.Stdin: the variable (a gp::ptr<File>), a function that takes an
// atomic snapshot of it, and what it held at the start.
struct os_std {
    const void* var = nullptr;
    std::shared_ptr<void> (*load)(const void*) = nullptr;
    const void* v0 = nullptr;         // the *os.File at the start
    const void* file0 = nullptr;      // the *os.file inside it (the first word of an os.File)
    const uint32_t* closed = nullptr; // the flag that File.Close sets for the original
    // replaced: the snapshot is another file (or nil), a File whose contents were replaced
    // (*os.Stdout = *w), or the original is closed.
    bool replaced(const std::shared_ptr<void>& snap) const noexcept {
        return snap.get() != v0 || *static_cast<const void* const*>(snap.get()) != file0 ||
               __atomic_load_n(closed, __ATOMIC_SEQ_CST) != 0;
    }
};
void register_os_stdout(
    const os_std& d,
    gp::func<std::tuple<int64_t, gp::error>(std::shared_ptr<void>, std::string_view)> write);
void register_os_stdin(const os_std& d, gp::func<gp::any(std::shared_ptr<void>)> current);
void check_os_file_layout(const void* v0, const void* file0);

// snapshot_loader<P>::load atomically copies the gp::ptr<File> object at v.
template <typename P>
std::shared_ptr<void> snapshot_of(const void* v) {
    using File = detail::pelem_t<P>;
    auto* sp = const_cast<std::shared_ptr<File>*>(&static_cast<const P*>(v)->gp_shared());
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wdeprecated-declarations"
    return std::atomic_load(sp);
#pragma GCC diagnostic pop
}

template <typename PVar, typename PFile, typename PFlag>
os_std make_os_std(const PVar& var, const PFile& file0, const PFlag& closed) {
    using P = detail::pelem_t<PVar>; // gp::ptr<File>
    os_std d;
    d.var = var.gp_get();
    d.load = &snapshot_of<P>;
    d.v0 = static_cast<const P*>(d.var)->gp_get();
    d.file0 = file0.gp_get();
    d.closed = reinterpret_cast<const uint32_t*>(closed.gp_get());
    check_os_file_layout(d.v0, d.file0);
    return d;
}
} // namespace detail

// Registration starts no worker and does not invoke write. The retained callback runs inline
// on a later printing thread, which can differ from or overlap the registering thread.
template <typename PVar, typename PFile, typename PFlag, typename W>
void set_os_stdout(const PVar& var, const PFile& file0, const PFlag& closed, W write) {
    using P = detail::pelem_t<PVar>;
    using File = detail::pelem_t<P>;
    detail::register_os_stdout(
        detail::make_os_std(var, file0, closed),
        [write = std::move(write)](std::shared_ptr<void> snap, std::string_view s) {
            auto b = gp::make_slice<uint8_t>(static_cast<int64_t>(s.size()));
            if (!s.empty()) {
                std::memcpy(b.gp_data(), s.data(), s.size());
            }
            return write(P(std::static_pointer_cast<File>(snap)), std::move(b));
        });
}

// store_std is the assignment os.Stdout = v (and os.Stdin, os.Stderr): an atomic store, and from
// then on fmt takes its slow paths. std_changed is the same for the other stores that the lowering
// sees
// (*os.Stdout = *w).
void std_changed() noexcept;
template <typename P, typename V>
void store_std(P& var, V&& v) {
    P tmp(std::forward<V>(v));
    auto& sp = const_cast<std::shared_ptr<detail::pelem_t<P>>&>(var.gp_shared());
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wdeprecated-declarations"
    std::atomic_store(&sp, tmp.gp_shared());
#pragma GCC diagnostic pop
    std_changed();
}
template <typename T>
void std_changed_after(T&&) noexcept {
    std_changed();
}

// std_closing is called by File.Close of the original os.Stdout (true) or os.Stdin (false), after
// the closed flag is set: from then on the slow paths run. defer_close is called when the file's
// descriptor is about to be closed (the close hook of package os, with the function that closes
// it): if a print or scan is in flight it keeps the function and returns true (the last one out
// closes, finish_close), else it returns false and the caller closes at once.
void std_closing(bool is_stdout) noexcept;
bool defer_close(bool is_stdout, gp::func<void()> close);

// Errorf is fmt.Errorf.
template <typename... Ts>
gp::error Errorf(gp::string format, Ts... args) {
    return detail::with_operands(
        [&](const detail::arg* a, std::size_t n, auto lv) {
            if constexpr (std::is_same_v<decltype(lv), detail::no_live>) {
                return detail::errorf(format.gp_view(), a, n);
            } else {
                return detail::errorf_live(format.gp_view(), a, n, lv);
            }
        },
        args...);
}

// FormatString is fmt.FormatString: the directive that a Format method was called for.
gp::string FormatString(State state, int32_t verb);

// --- C/C++ values (Phase 04 README §2.12, step 4.12; docs/spec/interop.md) ---

namespace detail {

template <typename>
inline constexpr bool always_false = false;

// A struct of the emitter that has a C/C++ part has gp_fields: gp::fields(gp::field(&S::m, ...)),
// which the check below walks.
template <typename T>
concept struct_type = requires { T::gp_fields; };

// A named type over another type (gp::newtype): it has gp_value().
template <typename T>
concept named_type = !struct_type<T> && requires(const T& v) { v.gp_value(); };

template <named_type T>
using underlying_t = std::remove_cvref_t<decltype(std::declval<const T&>().gp_value())>;

template <typename T>
struct slice_of : std::false_type {};
template <typename E>
struct slice_of<gp::slice<E>> : std::true_type {
    using elem = E;
};
template <typename T>
struct array_of : std::false_type {};
template <typename E, int64_t N>
struct array_of<gp::array<E, N>> : std::true_type {
    using elem = E;
    static constexpr int64_t len = N;
};
template <typename T>
struct map_of : std::false_type {};
template <typename K, typename V>
struct map_of<gp::map<K, V>> : std::true_type {
    using key = K;
    using mapped = V;
};
template <typename T>
struct ptr_of : std::false_type {};
template <typename E>
struct ptr_of<gp::ptr<E>> : std::true_type {
    using elem = E;
};
template <typename T>
struct cptr_of : std::false_type {};
template <typename E>
struct cptr_of<gp::cptr<E>> : std::true_type {
    using elem = E;
};
template <typename T>
struct weak_of : std::false_type {};
template <typename E>
struct weak_of<gp::weak_ptr<E>> : std::true_type {
    using elem = E;
};

template <typename T>
inline constexpr bool cxx_string =
    std::is_same_v<T, std::string> || std::is_same_v<T, std::string_view>;

// cxx_formatter<T>: whether T has a std::formatter<T, char> (gp_ok), and gp_append(out, v), which
// appends std::format("{}", v). gocpp/fmtcxx.hpp, which generated code includes after the
// program's C/C++ headers, specializes it when they brought in <format>: a type can have a
// std::formatter only then, and a program without it doesn't pay for <format>.
template <typename T>
struct cxx_formatter {
    static constexpr bool gp_ok = false;
};

// cxx_streamable: os << v works for a T v, through the operator<< that C++ finds for it: one of
// the C++ library, or one in T's namespace (the global one for a type there), which the program's
// headers may declare after this header (argument-dependent lookup at the point of
// instantiation).
template <typename T>
concept cxx_streamable = requires(std::ostream& os, const T& v) { os << v; };

// cxx_text_type: a C/C++ class (or union) type that prints as text: through its std::formatter
// when it has one, else through its operator<<.
template <typename T>
concept cxx_text_type =
    (std::is_class_v<T> || std::is_union_v<T>) && (cxx_formatter<T>::gp_ok || cxx_streamable<T>);

// cxx_text appends the text of the C/C++ value v (cxx_text_type).
template <typename T>
void cxx_text(std::string& out, const T& v) {
    if constexpr (cxx_formatter<T>::gp_ok) {
        cxx_formatter<T>::gp_append(out, v);
    } else {
        detail::stream_text(
            out, [](std::ostream& os, const void* x) { os << *static_cast<const T*>(x); }, &v);
    }
}

template <typename T>
void text_thunk(std::string& out, const void* v) {
    detail::cxx_text(out, *static_cast<const T*>(v));
}

// register_text<T>() gives the descriptor of the C++ class T its text (type_ops::gp_text).
template <typename T>
void register_text() {
    auto* ops = const_cast<type_ops*>(foreign_td<T>()->gp_ops);
    __atomic_store_n(&ops->gp_text, &detail::text_thunk<T>, __ATOMIC_RELEASE);
}

template <typename F, typename... Seen>
struct fields_visit;

template <typename T, typename... Seen>
constexpr bool printable();
template <typename T, typename... Seen>
void register_texts();

// field_info is one field of a struct's gp_fields (gp::field).
template <typename C, typename M>
struct field_info {
    using gp_type = M;
    M C::* gp_member;
    const char* gp_name;
    bool gp_exported;
    bool gp_embedded;
};

template <typename T, typename... Seen>
constexpr bool keys_ordered();
template <typename F, typename... Seen>
struct fields_ordered;
template <typename... F, typename... Seen>
struct fields_ordered<std::tuple<F...>, Seen...> {
    static constexpr bool value = (detail::keys_ordered<typename F::gp_type, Seen...>() && ...);
};

template <typename... F, typename... Seen>
struct fields_visit<std::tuple<F...>, Seen...> {
    static constexpr bool printable = (detail::printable<typename F::gp_type, Seen...>() && ...);
    static void register_all() { (detail::register_texts<typename F::gp_type, Seen...>(), ...); }
};

// key_ordered<K>: a C++ class key of a map that fmt prints has operator<, which orders the keys.
template <typename K>
concept key_ordered = requires(const K& a, const K& b) {
    { a < b } -> std::convertible_to<bool>;
};

// printable<T>(): whether fmt can print a T, all the way down: every C/C++ type in it is a
// number, a bool, an enum, a raw pointer, nullptr_t, a C++ string or a cxx_text_type (a C/C++
// type in a gocpp struct, slice, array, map or behind a gocpp pointer counts too). Seen are the
// gocpp structs and named types that are being checked further out, so a recursive type (type
// Node struct { kids []Node }) ends.
template <typename T, typename... Seen>
constexpr bool printable() {
    if constexpr ((std::is_same_v<T, Seen> || ...)) {
        return true;
    } else if constexpr (std::is_same_v<T, bool> || go_int<T> || go_float<T> || go_complex<T> ||
                         std::is_same_v<T, gp::string>) {
        return true;
    } else if constexpr (slice_of<T>::value) {
        return printable<typename slice_of<T>::elem, Seen...>();
    } else if constexpr (array_of<T>::value) {
        return printable<typename array_of<T>::elem, Seen...>();
    } else if constexpr (map_of<T>::value) {
        using K = typename map_of<T>::key;
        // (A C++ class key also needs operator<, which orders it: keys_ordered.)
        return printable<K, Seen...>() && printable<typename map_of<T>::mapped, Seen...>();
    } else if constexpr (std::is_same_v<T, gp::ptr<void>>) {
        return true; // unsafe.Pointer: an address
    } else if constexpr (ptr_of<T>::value) {
        return printable<typename ptr_of<T>::elem, Seen...>(); // &{...} prints the target
    } else if constexpr (weak_of<T>::value) {
        return true; // an address
    } else if constexpr (struct_type<T>) {
        return fields_visit<std::remove_cvref_t<decltype(T::gp_fields)>, T, Seen...>::printable;
    } else if constexpr (named_type<T>) {
        return printable<underlying_t<T>, T, Seen...>();
    } else if constexpr (std::is_base_of_v<gp::iface, T>) {
        return true;
    } else {
        return std::is_enum_v<T> || std::is_pointer_v<T> || cptr_of<T>::value ||
               std::is_null_pointer_v<T> || cxx_string<T> || cxx_text_type<T>;
    }
}

// keys_ordered<T>(): whether every map in a T has keys that fmt can sort (fmtsort, fmt.cpp:
// compare): Go types, C++ strings, and C++ classes with operator< (key_ordered). The walk of printable.
template <typename T, typename... Seen>
constexpr bool keys_ordered() {
    if constexpr ((std::is_same_v<T, Seen> || ...)) {
        return true;
    } else if constexpr (slice_of<T>::value) {
        return keys_ordered<typename slice_of<T>::elem, Seen...>();
    } else if constexpr (array_of<T>::value) {
        return keys_ordered<typename array_of<T>::elem, Seen...>();
    } else if constexpr (map_of<T>::value) {
        using K = typename map_of<T>::key;
        return (!cxx_text_type<K> || cxx_string<K> || key_ordered<K>) && keys_ordered<K, Seen...>() &&
               keys_ordered<typename map_of<T>::mapped, Seen...>();
    } else if constexpr (std::is_same_v<T, gp::ptr<void>>) {
        return true;
    } else if constexpr (ptr_of<T>::value) {
        return keys_ordered<typename ptr_of<T>::elem, Seen...>();
    } else if constexpr (struct_type<T>) {
        return fields_ordered<std::remove_cvref_t<decltype(T::gp_fields)>, T, Seen...>::value;
    } else if constexpr (named_type<T>) {
        return keys_ordered<underlying_t<T>, T, Seen...>();
    } else {
        return true;
    }
}

// printable_arg<T>: fmt can print an operand of the C++ type T (an operand with a descriptor,
// described<T>, counts as T).
template <typename T>
inline constexpr bool printable_arg = printable<T>();
template <typename T>
inline constexpr bool printable_arg<described<T>> = printable<T>();
template <typename T>
inline constexpr bool printable_arg<described<heap_value<T>>> = printable<T>();

// register_texts<T>() registers the text of every C++ class inside a T (the walk of printable).
template <typename T, typename... Seen>
void register_texts() {
    if constexpr ((std::is_same_v<T, Seen> || ...)) {
        return;
    } else if constexpr (slice_of<T>::value) {
        register_texts<typename slice_of<T>::elem, Seen...>();
    } else if constexpr (array_of<T>::value) {
        register_texts<typename array_of<T>::elem, Seen...>();
    } else if constexpr (map_of<T>::value) {
        register_texts<typename map_of<T>::key, Seen...>();
        register_texts<typename map_of<T>::mapped, Seen...>();
    } else if constexpr (ptr_of<T>::value) {
        register_texts<typename ptr_of<T>::elem, Seen...>();
    } else if constexpr (struct_type<T>) {
        fields_visit<std::remove_cvref_t<decltype(T::gp_fields)>, T, Seen...>::register_all();
    } else if constexpr (named_type<T>) {
        register_texts<underlying_t<T>, T, Seen...>();
    } else if constexpr (cxx_text_type<T>) {
        detail::register_text<T>();
    }
}

} // namespace detail

} // namespace gp::fmt

namespace gp {

// cxx_desc<T>() is the descriptor of a Go value that has the C++ type T, deduced from T, for the
// parts of a type that go/types has no type for (a C/C++ type in a slice, a map, a struct):
// a Go type that C++ identifies (a number, a string, gp::slice, gp::array, gp::map and gp::ptr of
// those, an interface struct) is described as Go does (an int64_t is int, an int32_t int32); any
// other type is a foreign C/C++ type (foreign_td<T>: a C int is int32, a class prints through
// its registered text). A named gocpp type inside is described as the foreign C++ type: its
// methods and its Go name are lost (the descriptor of a named type is the emitter's, not
// deducible from T).
template <typename T>
const type_desc* cxx_desc() {
    using U = std::remove_cv_t<std::remove_reference_t<T>>;
    if constexpr (std::is_base_of_v<iface, U>) {
        return U::gp_desc();
    } else if constexpr (fmt::detail::plain_known<U>) {
        return fmt::detail::plain_desc<U>();
    } else if constexpr (fmt::detail::slice_of<U>::value) {
        return td_slice<typename fmt::detail::slice_of<U>::elem>(
            cxx_desc<typename fmt::detail::slice_of<U>::elem>());
    } else if constexpr (fmt::detail::array_of<U>::value) {
        return td_array<typename fmt::detail::array_of<U>::elem, fmt::detail::array_of<U>::len>(
            cxx_desc<typename fmt::detail::array_of<U>::elem>());
    } else if constexpr (fmt::detail::map_of<U>::value) {
        return td_map<typename fmt::detail::map_of<U>::key,
                      typename fmt::detail::map_of<U>::mapped>(
            cxx_desc<typename fmt::detail::map_of<U>::key>(),
            cxx_desc<typename fmt::detail::map_of<U>::mapped>());
    } else if constexpr (std::is_same_v<U, gp::ptr<void>>) {
        return &td_unsafe_pointer;
    } else if constexpr (fmt::detail::ptr_of<U>::value) {
        return td_ptr<typename fmt::detail::ptr_of<U>::elem>(
            cxx_desc<typename fmt::detail::ptr_of<U>::elem>());
    } else {
        return foreign_td<U>();
    }
}

// field and fields build the gp_fields of a struct with a C/C++ part: each field with its member,
// its Go name, whether it is exported and whether it is embedded.
template <typename C, typename M>
constexpr fmt::detail::field_info<C, M> field(M C::* member, const char* name, bool exported,
                                              bool embedded) noexcept {
    return {member, name, exported, embedded};
}

template <typename... F>
constexpr std::tuple<F...> fields(F... f) noexcept {
    return std::tuple<F...>(f...);
}

} // namespace gp

#include <gocpp/fmtscan.hpp>
