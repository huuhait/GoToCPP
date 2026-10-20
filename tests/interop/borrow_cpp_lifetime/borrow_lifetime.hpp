#pragma once
#include <functional>
#include <utility>

namespace lifetime {
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

struct Default {
    Default() { fire(); }
};

struct Destroy {
    ~Destroy() { fire(); }
};

struct Copy {
    Copy() = default;
    Copy(const Copy&) { fire(); }
    Copy& operator=(const Copy&) {
        fire();
        return *this;
    }
};
} // namespace lifetime
