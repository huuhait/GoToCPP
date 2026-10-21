#include "support.hpp"

#include <atomic>
#include <barrier>
#include <cstdint>
#include <stdexcept>
#include <thread>
#include <utility>

using experiment_support::require;

namespace {

struct value {
    std::atomic<int>* destroyed;
    int first = 17;
    int second = 29;

    explicit value(std::atomic<int>& count) : destroyed(&count) {}
    ~value() { destroyed->fetch_add(1, std::memory_order_relaxed); }
};

struct throwing_value {
    explicit throwing_value(bool fail) {
        if (fail) {
            throw std::runtime_error("construction failed");
        }
    }
};

struct alignas(64) aligned_value {
    int member = 42;
};

template <typename Backend>
void copy_move_reset() {
    std::atomic<int> destroyed{0};
    auto original = Backend::template make<value>(destroyed);
    auto copied = original;
    require(original.use_count() == 2, "copy increments strong count");
    auto moved = std::move(copied);
    require(!copied && moved.get() == original.get(), "move transfers address and owner");
    auto* self = &moved;
    moved = *self;
    require(moved.use_count() == 2, "self-copy retains owner");
    moved = std::move(*self);
    require(moved && moved.use_count() == 2, "self-move retains owner");
    original.reset();
    require(destroyed == 0 && moved->second == 29, "copy outlives original");
    moved.reset();
    require(destroyed == 1, "last strong destroys exactly once");
}

template <typename Backend>
void aliased_member_lifetime() {
    std::atomic<int> destroyed{0};
    auto root = Backend::template make<value>(destroyed);
    auto alias = Backend::template alias<int>(root, &root->second);
    root.reset();
    require(destroyed == 0 && *alias == 29, "member alias keeps allocation alive");
    auto copy = alias;
    alias.reset();
    require(*copy == 29, "aliased address survives copy");
    copy.reset();
    require(destroyed == 1, "member alias destroys owning value, not member");
}

template <typename Backend>
void weak_alias_lifetime() {
    std::atomic<int> destroyed{0};
    auto root = Backend::template make<value>(destroyed);
    auto alias = Backend::template alias<int>(root, &root->second);
    typename Backend::template weak_type<int> weak(alias);
    auto copied = weak;
    auto moved = std::move(copied);
    require(copied.expired(), "moved weak is empty");
    auto* self = &moved;
    moved = *self;
    moved = std::move(*self);
    root.reset();
    alias.reset();
    require(destroyed == 1, "weak does not retain value");
    require(weak.expired() && !weak.lock(), "weak cannot resurrect value");
    require(moved.expired() && !moved.lock(), "copied/moved weak expires");
    moved.reset();
    weak.reset();
}

template <typename Backend>
void live_weak_lock() {
    std::atomic<int> destroyed{0};
    auto root = Backend::template make<value>(destroyed);
    auto alias = Backend::template alias<int>(root, &root->second);
    typename Backend::template weak_type<int> weak(alias);
    auto locked = weak.lock();
    require(locked.get() == alias.get(), "weak lock preserves interior address");
    root.reset();
    alias.reset();
    require(destroyed == 0 && *locked == 29, "locked weak is strong");
    locked.reset();
    require(destroyed == 1 && weak.expired(), "last locked weak destroys value");
}

template <typename Backend>
void null_alias_still_owns() {
    std::atomic<int> destroyed{0};
    auto root = Backend::template make<value>(destroyed);
    auto alias = Backend::template alias<int>(root, nullptr);
    typename Backend::template weak_type<int> weak(alias);
    root.reset();
    auto locked = weak.lock();
    require(!locked && locked.use_count() == 2, "null alias lock still retains ownership");
    alias.reset();
    require(destroyed == 0, "null strong address still owns");
    locked.reset();
    require(destroyed == 1 && weak.expired(), "null alias last release destroys");
}

template <typename Backend>
void weak_retains_only_storage() {
    experiment_support::allocation_stats stats;
    experiment_support::recording = &stats;
    std::atomic<int> destroyed{0};
    auto root = Backend::template measured_make<value>(destroyed);
    typename Backend::template weak_type<value> weak(root);
    root.reset();
    require(destroyed == 1 && stats.live == 1, "expired weak retains only storage");
    weak.reset();
    require(stats.live == 0 && stats.allocations == stats.deallocations,
            "last weak frees allocation");
    experiment_support::recording = nullptr;
}

template <typename Backend>
void constructor_failure_frees_storage() {
    experiment_support::allocation_stats stats;
    experiment_support::recording = &stats;
    bool threw = false;
    try {
        auto pointer = Backend::template measured_make<throwing_value>(true);
    } catch (const std::runtime_error&) {
        threw = true;
    }
    require(threw && stats.allocations == 1 && stats.deallocations == 1 && stats.live == 0,
            "constructor failure frees storage and propagates exception");
    experiment_support::recording = nullptr;
}

template <typename Backend>
void allocation_failure_propagates() {
    experiment_support::allocation_stats stats;
    stats.fail = true;
    experiment_support::recording = &stats;
    bool threw = false;
    try {
        auto pointer = Backend::template measured_make<int>(12);
    } catch (const std::bad_alloc&) {
        threw = true;
    }
    require(threw && stats.allocations == 0 && stats.live == 0,
            "allocation failure propagates without constructing a value");
    experiment_support::recording = nullptr;
}

template <typename Backend>
void over_aligned_value() {
    auto pointer = Backend::template make<aligned_value>();
    require(reinterpret_cast<std::uintptr_t>(pointer.get()) % 64 == 0 && pointer->member == 42,
            "factory preserves value alignment");
}

template <typename Backend>
void concurrent_lock_release() {
    for (int round = 0; round < 32; ++round) {
        std::atomic<int> destroyed{0};
        auto root = Backend::template make<value>(destroyed);
        auto alias = Backend::template alias<int>(root, &root->second);
        typename Backend::template weak_type<int> weak(alias);
        alias.reset();
        std::barrier start(3);
        auto worker = [weak, &start] {
            {
                auto live = weak.lock();
                require(live && *live == 29, "pre-race weak lock is live");
            }
            start.arrive_and_wait();
            for (int attempt = 0; attempt < 512; ++attempt) {
                auto locked = weak.lock();
                if (locked) {
                    auto copy = locked;
                    require(*copy == 29, "racing lock/copy observes live value");
                }
            }
        };
        std::thread first(worker);
        std::thread second(worker);
        start.arrive_and_wait();
        root.reset();
        first.join();
        second.join();
        require(destroyed == 1 && weak.expired() && !weak.lock(),
                "last release wins without resurrection after race");
    }
}

template <typename Backend>
void run() {
    copy_move_reset<Backend>();
    aliased_member_lifetime<Backend>();
    weak_alias_lifetime<Backend>();
    live_weak_lock<Backend>();
    null_alias_still_owns<Backend>();
    weak_retains_only_storage<Backend>();
    constructor_failure_frees_storage<Backend>();
    allocation_failure_propagates<Backend>();
    over_aligned_value<Backend>();
    concurrent_lock_release<Backend>();
    std::printf("PASS %s: 10 bounded cases (32 lock/release races, 2 workers each)\n",
                Backend::name);
}

} // namespace

int main() {
    run<experiment_support::baseline>();
    run<experiment_support::candidate>();
    bool rejected = false;
    int stack_value = 4;
    try {
        rc_experiment::ptr<int> invalid({}, &stack_value);
    } catch (const std::invalid_argument&) {
        rejected = true;
    }
    require(rejected, "candidate rejects non-null unowned alias");
    auto original = rc_experiment::make<int>(4);
    rc_experiment::weak<int> first(original);
    rc_experiment::weak<int> same(original);
    original.reset();
    auto next = rc_experiment::make<int>(4);
    rc_experiment::weak<int> distinct(next);
    require(first == same && !(first == distinct), "weak identity survives expiration");
    std::puts("PASS candidate alias guard and weak identity");
}
