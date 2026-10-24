// gocpp/fmtscan.hpp: the scanning half of Go's package fmt: Scan, Scanln, Scanf, Sscan, Sscanln,
// Sscanf, Fscan, Fscanln and Fscanf, with the interfaces fmt.ScanState and fmt.Scanner. The code
// (runtime/src/fmtscan.cpp) is a port of Go 1.25's fmt/scan.go, driven by type descriptors the
// way fmt.cpp is: the arguments are pointers, passed as operands with their descriptors, and
// the scanner stores through the pointer by the kind of the type it points to (Go's reflect
// fallback of scanOne).
//
// What generated code writes (internal/lower/fmtcall.go):
//
//     fmt.Sscan(s, &n, &name)     gp::fmt::Sscan(s, gp::fmt::as(d, p0), gp::fmt::as(d, p1));
//     fmt.Fscanf(r, "%d", &n)     gp::fmt::Fscanf(io_reader, "%d"_gs, gp::fmt::as(d, p));
//     fmt.Scan(&n)                gp::fmt::Scan(gp::fmt::as(d, p));      // reads standard input
//
// Fscan, Fscanln and Fscanf take an io.Reader interface value. The scanner reads runes through
// the reader's ReadRune and UnreadRune methods when it has them (io.RuneScanner), and a byte at a
// time through Read when it has not, as Go does, so one rune of look-ahead is lost between two
// calls on a reader without UnreadRune. Scan, Scanln and Scanf read os.Stdin: the emitter passes
// the program's os.Stdin when the package imports os, and the runtime reads standard input itself
// (one byte at a time, unbuffered, like Go's) when it does not.
//
// The errors are Go's texts. io.EOF and io.ErrUnexpectedEOF are the runtime's two values (package
// io's variables are set from io_eof and io_unexpected_eof, so that err == io.EOF holds).
#pragma once

#include <gocpp/fmt.hpp>
#include <gocpp/func.hpp>

namespace gp::fmt {

// ScanState is fmt.ScanState: what a Scan method gets. Its methods, sorted: Read, ReadRune,
// SkipSpace, Token, UnreadRune, Width.
struct ScanState : iface {
    using iface::iface;
    constexpr ScanState() noexcept = default;
    static const type_desc* gp_desc() noexcept;
    std::tuple<int64_t, gp::error> Read(gp::slice<uint8_t> buf) const;
    std::tuple<int32_t, int64_t, gp::error> ReadRune() const;
    void SkipSpace() const;
    std::tuple<gp::slice<uint8_t>, gp::error> Token(bool skipSpace,
                                                    gp::func<bool(int32_t)> f) const;
    gp::error UnreadRune() const;
    std::tuple<int64_t, bool> Width() const;
};

// Scanner is fmt.Scanner: a value with a Scan method.
struct Scanner : iface {
    using iface::iface;
    constexpr Scanner() noexcept = default;
    static const type_desc* gp_desc() noexcept;
    gp::error Scan(ScanState state, int32_t verb) const;
};

static_assert(sizeof(ScanState) == sizeof(iface) && sizeof(Scanner) == sizeof(iface),
              "the fmt interfaces have no members of their own");

namespace detail {
// The func types of the methods (imethod::gp_ftype; ft_write and ft_int_ok: gocpp/fmt.hpp).
inline const type_desc* ft_readrune() {
    static const type_desc* const d = td_sig({}, {&td_int32, &td_int, &td_error});
    return d;
}
inline const type_desc* ft_none() {
    static const type_desc* const d = td_sig({}, {});
    return d;
}
inline const type_desc* ft_token() {
    static const type_desc* const d = td_sig({&td_bool, td_sig({&td_int32}, {&td_bool})},
                                             {td_slice<std::uint8_t>(&td_uint8), &td_error});
    return d;
}
inline const type_desc* ft_error() {
    static const type_desc* const d = td_sig({}, {&td_error});
    return d;
}
inline const type_desc* ft_scan(); // func(fmt.ScanState, int32) error, after td_scanstate

inline constexpr imethod scanstate_methods[] = {
    imethod("Read", nullptr, "func([]uint8) (int, error)", nullptr, &ft_write),
    imethod("ReadRune", nullptr, "func() (int32, int, error)", nullptr, &ft_readrune),
    imethod("SkipSpace", nullptr, "func()", nullptr, &ft_none),
    imethod("Token", nullptr, "func(bool, func(int32) bool) ([]uint8, error)", nullptr, &ft_token),
    imethod("UnreadRune", nullptr, "func() error", nullptr, &ft_error),
    imethod("Width", nullptr, "func() (int, bool)", nullptr, &ft_int_ok),
};
inline constexpr imethod scanner_methods[] = {
    imethod("Scan", nullptr, "func(fmt.ScanState, int32) error", nullptr, &ft_scan)};
} // namespace detail

inline constexpr type_desc td_scanstate =
    named<ScanState>("fmt.ScanState", "fmt", shape_interface(detail::scanstate_methods));
inline constexpr type_desc td_scanner =
    named<Scanner>("fmt.Scanner", "fmt", shape_interface(detail::scanner_methods));

inline const type_desc* ScanState::gp_desc() noexcept {
    return &td_scanstate;
}
inline const type_desc* detail::ft_scan() {
    static const type_desc* const d = td_sig({&td_scanstate, &td_int32}, {&td_error});
    return d;
}
inline const type_desc* Scanner::gp_desc() noexcept {
    return &td_scanner;
}

inline std::tuple<int64_t, gp::error> ScanState::Read(gp::slice<uint8_t> buf) const {
    return icall<std::tuple<int64_t, gp::error>(gp::slice<uint8_t>)>(*this, 0, std::move(buf));
}
inline std::tuple<int32_t, int64_t, gp::error> ScanState::ReadRune() const {
    return icall<std::tuple<int32_t, int64_t, gp::error>()>(*this, 1);
}
inline void ScanState::SkipSpace() const {
    icall<void()>(*this, 2);
}
inline std::tuple<gp::slice<uint8_t>, gp::error> ScanState::Token(bool skipSpace,
                                                                  gp::func<bool(int32_t)> f) const {
    return icall<std::tuple<gp::slice<uint8_t>, gp::error>(bool, gp::func<bool(int32_t)>)>(
        *this, 3, skipSpace, std::move(f));
}
inline gp::error ScanState::UnreadRune() const {
    return icall<gp::error()>(*this, 4);
}
inline std::tuple<int64_t, bool> ScanState::Width() const {
    return icall<std::tuple<int64_t, bool>()>(*this, 5);
}
inline gp::error Scanner::Scan(ScanState state, int32_t verb) const {
    return icall<gp::error(ScanState, int32_t)>(*this, 0, std::move(state), verb);
}

// io_eof and io_unexpected_eof are the runtime's io.EOF and io.ErrUnexpectedEOF: package io's
// variables are initialized from them (stdlib/io/io.gp), so a comparison with either one holds
// for the errors that the scanner returns.
gp::error io_eof();
gp::error io_unexpected_eof();

namespace detail {

// The scanning (runtime/src/fmtscan.cpp). nl_is_space and nl_is_end are the modes of Scan
// (true, false), Scanln (false, true) and Scanf (false, false). A reader is an io.Reader
// interface value; a nil one panics at the first read.
std::tuple<int64_t, gp::error> scan_string(std::string_view str, bool nl_is_space, bool nl_is_end,
                                           const arg* a, std::size_t n, live_span live = {});
std::tuple<int64_t, gp::error> scanf_string(std::string_view str, std::string_view format,
                                            const arg* a, std::size_t n, live_span live = {});
std::tuple<int64_t, gp::error> scan_reader(const iface& r, bool nl_is_space, bool nl_is_end,
                                           const arg* a, std::size_t n, live_span live = {});
std::tuple<int64_t, gp::error> scanf_reader(const iface& r, std::string_view format, const arg* a,
                                            std::size_t n, live_span live = {});
std::tuple<int64_t, gp::error> scan_stdin(bool nl_is_space, bool nl_is_end, const arg* a,
                                          std::size_t n, live_span live = {});
std::tuple<int64_t, gp::error> scanf_stdin(std::string_view format, const arg* a, std::size_t n,
                                           live_span live = {});

} // namespace detail

// set_os_stdin is called by the init of package os (stdlib/os/gocpp_helpers.gp), like
// set_os_stdout (fmt.hpp), with &os.Stdin, the *os.file inside it, the flag that Close sets, and a
// function that turns a *os.File into the io.Reader (an any) to scan. Scan, Scanln and Scanf then
// read the file that os.Stdin holds when the program has assigned another file to os.Stdin or
// closed the original, as Go's do (also when they are used as func values or in a package that does
// not import os); else, and until the init has run, they read file descriptor 0.
// Registration starts no worker and does not invoke current; a later scanning thread calls it
// inline. That thread need not be the registering thread, so this is not a synchronous callback.
template <typename PVar, typename PFile, typename PFlag, typename F>
void set_os_stdin(const PVar& var, const PFile& file0, const PFlag& closed, F current) {
    using P = detail::pelem_t<PVar>;
    using File = detail::pelem_t<P>;
    detail::register_os_stdin(detail::make_os_std(var, file0, closed),
                              [current = std::move(current)](std::shared_ptr<void> snap) {
                                  return current(P(std::static_pointer_cast<File>(snap)));
                              });
}

// Sscan, Sscanln and Sscanf.
template <typename... Ts>
std::tuple<int64_t, gp::error> Sscan(gp::string str, Ts... args) {
    return detail::with_operands(
        [&](const detail::arg* a, std::size_t n, detail::live_span live) {
            return detail::scan_string(str.gp_view(), true, false, a, n, live);
        },
        args...);
}

template <typename... Ts>
std::tuple<int64_t, gp::error> Sscanln(gp::string str, Ts... args) {
    return detail::with_operands(
        [&](const detail::arg* a, std::size_t n, detail::live_span live) {
            return detail::scan_string(str.gp_view(), false, true, a, n, live);
        },
        args...);
}

template <typename... Ts>
std::tuple<int64_t, gp::error> Sscanf(gp::string str, gp::string format, Ts... args) {
    return detail::with_operands(
        [&](const detail::arg* a, std::size_t n, detail::live_span live) {
            return detail::scanf_string(str.gp_view(), format.gp_view(), a, n, live);
        },
        args...);
}

// Fscan, Fscanln and Fscanf.
template <typename R, typename... Ts>
std::tuple<int64_t, gp::error> Fscan(const R& r, Ts... args) {
    return detail::with_operands(
        [&](const detail::arg* a, std::size_t n, detail::live_span live) {
            return detail::scan_reader(r, true, false, a, n, live);
        },
        args...);
}

template <typename R, typename... Ts>
std::tuple<int64_t, gp::error> Fscanln(const R& r, Ts... args) {
    return detail::with_operands(
        [&](const detail::arg* a, std::size_t n, detail::live_span live) {
            return detail::scan_reader(r, false, true, a, n, live);
        },
        args...);
}

template <typename R, typename... Ts>
std::tuple<int64_t, gp::error> Fscanf(const R& r, gp::string format, Ts... args) {
    return detail::with_operands(
        [&](const detail::arg* a, std::size_t n, detail::live_span live) {
            return detail::scanf_reader(r, format.gp_view(), a, n, live);
        },
        args...);
}

// Scan, Scanln and Scanf read standard input.
template <typename... Ts>
std::tuple<int64_t, gp::error> Scan(Ts... args) {
    return detail::with_operands(
        [&](const detail::arg* a, std::size_t n, detail::live_span live) {
            return detail::scan_stdin(true, false, a, n, live);
        },
        args...);
}

template <typename... Ts>
std::tuple<int64_t, gp::error> Scanln(Ts... args) {
    return detail::with_operands(
        [&](const detail::arg* a, std::size_t n, detail::live_span live) {
            return detail::scan_stdin(false, true, a, n, live);
        },
        args...);
}

template <typename... Ts>
std::tuple<int64_t, gp::error> Scanf(gp::string format, Ts... args) {
    return detail::with_operands(
        [&](const detail::arg* a, std::size_t n, detail::live_span live) {
            return detail::scanf_stdin(format.gp_view(), a, n, live);
        },
        args...);
}

} // namespace gp::fmt
