#pragma once
#include <cstdint>
#include <functional>
#include <utility>

namespace arguments {
inline std::function<void()> hook;

inline void set(std::function<void()> f) {
    hook = std::move(f);
}

inline void fire() {
    auto f = std::move(hook);
    hook = {};
    if (f) {
        f();
    }
}

struct Copy {
    Copy() = default;
    Copy(const Copy&) { fire(); }
    Copy& operator=(const Copy&) {
        fire();
        return *this;
    }
};

struct Convert {
    operator std::int64_t() const {
        fire();
        return 1;
    }
};
} // namespace arguments
