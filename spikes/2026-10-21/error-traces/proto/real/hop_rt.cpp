// Throwaway prototype of the register of ADR-124 (second version): no user code in hop, the identity
// is compared as it is, the register is trivial constinit storage, the two code addresses of the hop (one
// inside the function that has the `!`, and where that function returns to) are kept for the link check of Trace (not done here: the benchmark has no reader).
#include <gocpp/rt.hpp>
#include <cstdint>
#include <memory>
#include <new>

namespace gp::etrace {
struct site {
    const char* file;
    const char* func;
    int line;
};

constexpr int kFirst = 16;
constexpr int kRing = 16;

// Trivial type: constinit thread-local storage needs no guard and no destructor. The weak reference
// lives in raw storage and is released by clear() (goroutine end) and the thread-exit hook.
struct reg_t {
    alignas(std::weak_ptr<void>) unsigned char owner[sizeof(std::weak_ptr<void>)];
    bool has_owner;
    const void* data;
    const type_desc* type;
    std::uintptr_t last_frame;
    std::uint32_t total;
    const site* sites[kFirst + kRing];
    const void* rets[kFirst + kRing];   // return address of the function that has the `!`
    const void* heres[kFirst + kRing];  // an address inside that function (hop's own return address)
};

static constinit thread_local reg_t reg{};

static std::weak_ptr<void>& weak(reg_t& r) {
    return *std::launder(reinterpret_cast<std::weak_ptr<void>*>(r.owner));
}

static bool same(reg_t& r, const gp::error& e) {
    const std::shared_ptr<void>& d = e.gp_data();
    return r.has_owner && d != nullptr && r.data == d.get() && r.type == e.gp_type() &&
           !weak(r).owner_before(d) && !d.owner_before(weak(r));
}

static void set_identity(reg_t& r, const gp::error& e) {
    if (!r.has_owner) {
        new (r.owner) std::weak_ptr<void>(e.gp_data());
        r.has_owner = true;
    } else {
        weak(r) = e.gp_data();
    }
    r.data = e.gp_data().get();
    r.type = e.gp_type();
}

[[gnu::noinline, gnu::cold]] void hop(const site* s, gp::error e, const void* ret) noexcept {
    reg_t& r = reg;
    auto frame = reinterpret_cast<std::uintptr_t>(__builtin_frame_address(0));
    // Continue only for the same error object, from a frame that is older than the last hop's
    // (a caller: the stack grows down); else restart.
    bool same_error = r.total != 0 && same(r, e);
    bool cont = same_error && frame > r.last_frame;
    if (!cont) {
        r.total = 0;
    }
    std::uint32_t n = r.total++;
    std::uint32_t slot = n < kFirst ? n : kFirst + (n - kFirst) % kRing;
    r.sites[slot] = s;
    r.rets[slot] = ret;
    r.heres[slot] = __builtin_return_address(0);
    r.last_frame = frame;
    if (!same_error) {
        set_identity(r, e);
    }
}

// A wrapper that gocpp makes of the error in flight takes over its identity (O(1), no user code).
void rekey(const gp::error& old_err, const gp::error& wrapper) noexcept {
    reg_t& r = reg;
    if (r.total != 0 && same(r, old_err)) {
        set_identity(r, wrapper);
    }
}

void clear() noexcept {
    reg_t& r = reg;
    r.total = 0;
    if (r.has_owner) {
        weak(r).reset();
    }
}
} // namespace gp::etrace
