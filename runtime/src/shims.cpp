// The native functions of internal/godebug and runtime/cgo (gocpp/shims.hpp, Phase 09 step 9.2).
// Standard C++ only.
#include <gocpp/rt.hpp>

#include <atomic>
#include <cstddef>
#include <mutex>
#include <string>
#include <string_view>
#include <unordered_map>

namespace gp::godebug {
namespace {

// The value of $GODEBUG as Go's runtime keeps it (godebugEnv): the one in the environment the
// program started with, then the value that package syscall sets. Reading it never calls getenv,
// which another thread's setenv could change under it (a data race in C).
struct env_state {
    std::mutex mu;
    std::string value;
    std::atomic<std::uint64_t> generation{0};
};

// at_start is GODEBUG in gp::envs(), the environment that run_main (or a library's first entry,
// load_process_info) copied before any Go code ran, "" when it is not there: the first entry
// with the name, as Go's gogetenv (runtime/env_posix.go), the name without case on Windows.
std::string at_start() {
    constexpr std::string_view key = "GODEBUG";
    for (const gp::string& kv : gp::envs()) {
        std::string_view s = kv.gp_view();
        if (s.size() <= key.size() || s[key.size()] != '=') {
            continue;
        }
        bool same = true;
        for (std::size_t i = 0; i < key.size(); i++) {
            char c = s[i];
#ifdef _WIN32
            if (c >= 'a' && c <= 'z') {
                c = static_cast<char>(c - 'a' + 'A');
            }
#endif
            if (c != key[i]) {
                same = false;
                break;
            }
        }
        if (same) {
            return std::string(s.substr(key.size() + 1));
        }
    }
    return "";
}

// state is made on the first use (a function-local static: no initializer at program start, which
// a library member must not have, internal/build TestRuntimeObjectsHaveNoStaticInit), from the
// start-up environment, so the first use may come at any time and from any thread.
env_state& state() {
    static env_state* const s = [] {
        auto* e = new env_state; // never destroyed: a goroutine may read it while the process exits
        e->value = at_start();
        return e;
    }();
    return *s;
}

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
