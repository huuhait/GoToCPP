// The native functions of internal/godebug and runtime/cgo (gocpp/shims.hpp, Phase 09 step 9.2).
// Standard C++ only.
#include <gocpp/rt.hpp>

#include <atomic>
#include <cstdlib>
#include <mutex>
#include <string>
#include <string_view>
#include <unordered_map>

namespace gp::godebug {
namespace {

// The value of $GODEBUG as Go's runtime keeps it (godebugEnv): read from the environment at
// program start, then the value that package syscall sets. Reading it never calls getenv, which
// another thread's setenv could change under it (a data race in C).
struct env_state {
    std::mutex mu;
    std::string value;
    std::atomic<std::uint64_t> generation{0};
};

env_state& state() {
    static env_state* const s = [] {
        auto* e = new env_state; // never destroyed: a goroutine may read it while the process exits
        const char* v = std::getenv("GODEBUG");
        e->value = v == nullptr ? "" : v;
        return e;
    }();
    return *s;
}

[[maybe_unused]] const bool read_at_start = (state(), true);

} // namespace

gp::string env() {
    env_state& s = state();
    std::lock_guard<std::mutex> lock(s.mu);
    return gp::string::gp_copy(s.value);
}

std::uint64_t generation() noexcept {
    return state().generation.load(std::memory_order_acquire);
}

void env_changed(std::string_view value) {
    env_state& s = state();
    {
        std::lock_guard<std::mutex> lock(s.mu);
        s.value.assign(value);
    }
    s.generation.fetch_add(1, std::memory_order_acq_rel);
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
