// C++ values for the tests of step 4.12 (cxx_print*.gp, cxx_exception*.gp,
// cpp_error_print.gp): types that print through an operator<< or a std::formatter, one that
// has neither, enums, raw pointers, C++ strings, and functions that throw.
#pragma once

#include <cstddef>
#include <format>
#include <new>
#include <ostream>
#include <stdexcept>
#include <string>
#include <string_view>
#include <vector>

struct Point {
    int x, y;
};
inline std::ostream& operator<<(std::ostream& os, const Point& p) {
    return os << "(" << p.x << ", " << p.y << ")";
}

// Money has only a std::formatter.
struct Money {
    long long cents;
};
template <>
struct std::formatter<Money> : std::formatter<long long> {
    auto format(const Money& m, std::format_context& ctx) const {
        return std::format_to(ctx.out(), "${}.{:02}", m.cents / 100, m.cents % 100);
    }
};

// Opaque has neither.
struct Opaque {
    int z;
};

enum Color { Red, Green = 5 };
enum class Level : unsigned char { Low = 1, High = 200 };

inline Point make_point(int x, int y) {
    return Point{x, y};
}
inline Money make_money(long long cents) {
    return Money{cents};
}
inline Opaque make_opaque() {
    return Opaque{1};
}
inline Color green() {
    return Green;
}
inline Level high() {
    return Level::High;
}
inline const char* hello() {
    return "hello";
}
inline const char* null_text() {
    return nullptr;
}
inline std::string name() {
    return "bob";
}
inline std::string_view view() {
    return "view";
}
inline long long big() {
    return 1LL << 40;
}
inline void noop() {}
using callback = void (*)();
inline callback get_callback() {
    return &noop;
}

inline long vec_at(const std::vector<long>& v, std::size_t i) {
    return v.at(i);
}
inline void throw_runtime(int n) {
    if (n > 0) {
        throw std::runtime_error("bad thing " + std::to_string(n));
    }
}
// arena_alloc is an allocator of a C++ library that throws std::bad_alloc itself when it has
// no room (new int[n] with a bad n would be the same with g++, which throws
// std::bad_array_new_length; clang++ asks operator new for SIZE_MAX bytes, which the runtime's
// new handler reports, or AddressSanitizer in a -asan build: runtime/test/cxxvalue_test.cpp).
inline void* arena_alloc(long long n) {
    static char arena[64];
    if (n < 0 || n > static_cast<long long>(sizeof arena)) {
        throw std::bad_alloc();
    }
    return arena;
}
inline void throw_int(int n) {
    if (n > 0) {
        throw n;
    }
}
