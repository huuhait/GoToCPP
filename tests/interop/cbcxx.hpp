// C++ callbacks for the tests of step 4.11: APIs that take a std::function, a function pointer
// or a template callable, and an object that keeps its handler for later.
#pragma once

#include <cstdio>
#include <functional>
#include <string>
#include <utility>

namespace cbx {

inline int apply(const std::function<int(int)>& f, int x) { return f(x) + 1; }
inline int apply_fn(int (*f)(int), int x) { return f(x); }
template <typename F>
int twice(F f, int x) {
    return f(f(x));
}

// Button keeps its handler: the closure must live as long as the button does.
struct Button {
    std::function<void(int)> handler;
    void on_click(std::function<void(int)> h) { handler = std::move(h); }
    void click(int x) {
        if (handler) {
            handler(x);
        }
    }
};

// Scope shows that a panic unwinds through C++ frames like through gocpp ones.
struct Scope {
    Scope() { std::printf("enter\n"); }
    ~Scope() { std::printf("leave\n"); }
};
inline int in_scope(const std::function<int()>& f) {
    Scope s;
    return f();
}

// Pick calls f with a name and returns what it returns.
inline std::string pick(const std::function<std::string(const std::string&)>& f) { return f("abc"); }

} // namespace cbx

#include <stdexcept>
namespace cbx {
// fail throws a C++ exception: called by a callback that C called.
inline int fail(int x) {
    throw std::runtime_error("no more rows " + std::to_string(x));
}
} // namespace cbx
