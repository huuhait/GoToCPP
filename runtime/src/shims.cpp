// The native functions of internal/godebug and runtime/cgo (gocpp/shims.hpp, Phase 09 step 9.2).
// Standard C++ only.
#include <gocpp/rt.hpp>

#include <cstdlib>
#include <mutex>
#include <unordered_map>

namespace gp::godebug {

gp::string env() {
    const char* v = std::getenv("GODEBUG");
    return v == nullptr ? gp::string() : gp::string::gp_copy(v);
}

} // namespace gp::godebug

namespace gp::cgo {
namespace {

// The table is a function-local static, never destroyed: a goroutine thread may still use a handle
// while the process exits.
struct handle_table {
    std::mutex mu;
    std::unordered_map<uintptr_t, gp::any> values;
    uintptr_t last = 0;
};

handle_table& table() {
    static auto* t = new handle_table;
    return *t;
}

[[noreturn]] void fail(const char* text) {
    gp::panic(gp::string::gp_copy(text));
}

} // namespace

uintptr_t handle_new(const gp::any& v) {
    handle_table& t = table();
    std::lock_guard<std::mutex> lock(t.mu);
    uintptr_t h = t.last + 1;
    if (h == 0) {
        fail("runtime/cgo: ran out of handle space");
    }
    t.last = h;
    t.values.emplace(h, v);
    return h;
}

gp::any handle_value(uintptr_t h) {
    handle_table& t = table();
    std::lock_guard<std::mutex> lock(t.mu);
    auto it = t.values.find(h);
    if (it == t.values.end()) {
        fail("runtime/cgo: misuse of an invalid Handle");
    }
    return it->second;
}

void handle_delete(uintptr_t h) {
    handle_table& t = table();
    gp::any old; // destroyed after the lock is released: a destructor may run Go code
    {
        std::lock_guard<std::mutex> lock(t.mu);
        auto it = t.values.find(h);
        if (it == t.values.end()) {
            fail("runtime/cgo: misuse of an invalid Handle");
        }
        old = std::move(it->second);
        t.values.erase(it);
    }
}

} // namespace gp::cgo
