// Helpers for cxx_recover_kinds.gp: exceptions of several kinds, RAII objects that count their
// destructors, and C++ templates that call back into Go.
#pragma once

#include <algorithm>
#include <exception>
#include <functional>
#include <stdexcept>
#include <string>
#include <vector>

namespace cxxrec {

struct AppError : std::exception {
    std::string msg;
    explicit AppError(std::string m) : msg(std::move(m)) {}
    const char* what() const noexcept override { return msg.c_str(); }
};

struct Plain {
    int code;
};

inline int destroyed = 0;

struct Guard {
    Guard() = default;
    Guard(const Guard&) = delete;
    ~Guard() { ++destroyed; }
};

inline void throw_app(const std::string& m) { throw AppError(m); }
inline void throw_logic(const std::string& m) { throw std::logic_error(m); }
inline void throw_cstr() { throw "a C string"; }
inline void throw_string() { throw std::string("a std::string"); }
inline void throw_int(int n) { throw n; }
inline void throw_plain(int n) { throw Plain{n}; }
inline void throw_nested() {
    try {
        throw std::runtime_error("inner");
    } catch (...) {
        std::throw_with_nested(std::runtime_error("outer"));
    }
}

// Throws after creating RAII objects: their destructors must run while the exception unwinds.
inline int guarded_throw(int depth) {
    Guard g;
    if (depth == 0) {
        throw AppError("deep");
    }
    return guarded_throw(depth - 1) + 1;
}

// Calls f for each element: a Go panic or a C++ exception thrown by f unwinds through this frame.
inline int for_each_sum(const std::vector<int>& v, const std::function<void(int)>& f) {
    Guard g;
    int n = 0;
    for (int x : v) {
        f(x);
        n += x;
    }
    return n;
}

template <typename F>
int apply_twice(F f, int x) {
    Guard g;
    return f(f(x));
}

} // namespace cxxrec
