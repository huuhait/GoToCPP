// C++ code that keeps the exception it sees with std::current_exception, lets it go on, and
// throws it again later.
#pragma once
#include <cstdio>
#include <exception>
#include <functional>

namespace xp {
inline std::exception_ptr saved;
inline int call_save(const std::function<int()>& f) {
    try {
        return f();
    } catch (...) {
        saved = std::current_exception();
        throw;
    }
}
// swallow calls f and drops any exception (as C++ code may).
inline int swallow(const std::function<int()>& f) {
    try {
        return f();
    } catch (...) {
        std::printf("swallowed\n");
        return -1;
    }
}
inline int rethrow_saved() {
    std::rethrow_exception(saved);
}
} // namespace xp
