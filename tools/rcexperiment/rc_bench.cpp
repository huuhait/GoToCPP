// Two backends in one binary, identical inputs/work and an irreversible threaded state.
#include "support.hpp"

#include <array>
#include <cstdint>
#include <ctime>
#include <sys/single_threaded.h>
#include <thread>

using experiment_support::require;
using baseline = experiment_support::baseline;
using candidate = experiment_support::candidate;

namespace {

std::uint64_t destructions = 0;
volatile std::uint64_t sink = 0;

struct payload {
    std::uint64_t first;
    std::uint64_t second;
    payload(std::uint64_t a, std::uint64_t b) : first(a), second(b) {}
    ~payload() { ++destructions; }
};

template <typename T>
void escape(const T& value) {
    asm volatile("" : : "g"(&value) : "memory");
}

std::uint64_t cpu_ns() {
    timespec time{};
    require(clock_gettime(CLOCK_PROCESS_CPUTIME_ID, &time) == 0, "process CPU clock");
    return static_cast<std::uint64_t>(time.tv_sec) * 1000000000ULL + time.tv_nsec;
}

template <typename Owner>
struct string_header {
    const char* data;
    std::size_t length;
    Owner owner;
};

template <typename Owner>
struct slice_header {
    void* data;
    std::size_t length;
    std::size_t capacity;
    Owner owner;
};

template <typename Backend>
void storage() {
    using pointer = typename Backend::template pointer<payload>;
    using weak = typename Backend::template weak_type<payload>;
    using owner = typename Backend::owner_type;
    experiment_support::allocation_stats stats;
    experiment_support::recording = &stats;
    auto root = Backend::template measured_make<payload>(17, 29);
    weak observer(root);
    require(stats.allocations == 1 && stats.live == 1, "one coallocation per backend");
    root.reset();
    require(observer.expired() && stats.live == 1, "weak retains coallocated dead payload storage");
    observer.reset();
    require(stats.live == 0 && stats.deallocations == 1, "storage released by last weak");
    experiment_support::recording = nullptr;
    std::printf(
        "storage,%s,payload=%zu/%zu,owner=%zu/%zu,alias=%zu/%zu,weak=%zu/%zu,"
        "string=%zu,slice=%zu,allocation_requested=%zu,allocation_usable=%zu,"
        "allocation_alignment=%zu,allocations=%zu,deallocations=%zu,"
        "total_owner_requested=%zu,total_alias_requested=%zu,total_alias_weak_requested=%zu\n",
        Backend::name, sizeof(payload), alignof(payload), sizeof(owner), alignof(owner),
        sizeof(pointer), alignof(pointer), sizeof(weak), alignof(weak),
        sizeof(string_header<owner>), sizeof(slice_header<owner>), stats.requested, stats.usable,
        stats.alignment, stats.allocations, stats.deallocations, stats.requested + sizeof(owner),
        stats.requested + sizeof(pointer), stats.requested + sizeof(pointer) + sizeof(weak));
}

template <typename Backend>
void measure(const char* kernel, int sample, std::size_t count, bool print) {
    auto root = Backend::template make<payload>(17, 29);
    auto ownership = Backend::ownership(root);
    auto alias = Backend::template alias<std::uint64_t>(root, &root->second);
    // Keep a weak reference in both arms, also excluding the unique-owner release shortcut.
    typename Backend::template weak_type<std::uint64_t> observer(alias);
    std::uint64_t checksum = 0;
    std::uint64_t expected = count * 29;
    const auto before_destructions = destructions;
    const auto start = cpu_ns();
    if (kernel[0] == 'o') {
        for (std::size_t index = 0; index < count; ++index) {
            auto copy = ownership;
            escape(copy);
            checksum += root->second;
        }
    } else if (kernel[0] == 'a') {
        for (std::size_t index = 0; index < count; ++index) {
            auto copy = alias;
            escape(copy);
            checksum += *copy;
        }
    } else {
        for (std::size_t index = 0; index < count; ++index) {
            auto value = Backend::template make<payload>(17, 29);
            auto member = Backend::template alias<std::uint64_t>(value, &value->second);
            typename Backend::template weak_type<std::uint64_t> weak(member);
            escape(value);
            value.reset();
            auto locked = weak.lock();
            escape(locked);
            checksum += *locked;
            member.reset();
            locked.reset();
            checksum += weak.expired() ? 0 : 1;
            checksum += weak.lock() ? 1 : 0;
        }
    }
    const auto elapsed = cpu_ns() - start;
    sink = checksum;
    require(checksum == expected, "identical kernel checksum");
    const auto expected_destructions = kernel[0] == 'l' ? count : 0;
    require(destructions - before_destructions == expected_destructions,
            "kernel destroys every temporary exactly once");
    require(__libc_single_threaded == 0, "both arms use the threaded libstdc++ state");
    if (print) {
        std::printf("sample,%s,%s,%d,%zu,%llu,%.6f,%llu,%llu\n", kernel, Backend::name, sample,
                    count, static_cast<unsigned long long>(elapsed),
                    static_cast<double>(elapsed) / count, static_cast<unsigned long long>(checksum),
                    static_cast<unsigned long long>(destructions - before_destructions));
    }
}

} // namespace

int main() {
    // glibc's flag stays false after join; never time a pre-thread baseline against atomic RC.
    std::thread([] {}).join();
    require(__libc_single_threaded == 0, "threaded state established before either backend");
    std::printf("protocol,version=1,cpu_clock=process,threaded=%d,iterations_copy=5000000,"
                "iterations_lifetime=500000,samples_each=4,order=AB-BA-AB-BA\n",
                __libc_single_threaded == 0);
    storage<baseline>();
    storage<candidate>();
    std::printf("layout,intrusive_control=%zu/%zu,intrusive_coallocation=%zu/%zu\n",
                sizeof(rc_experiment::detail::control), alignof(rc_experiment::detail::control),
                sizeof(rc_experiment::detail::allocation<payload, std::allocator<std::byte>>),
                alignof(rc_experiment::detail::allocation<payload, std::allocator<std::byte>>));
    for (const char* kernel : std::array{"owner_copy", "alias_copy", "lifetime_weak"}) {
        measure<baseline>(kernel, -1, 10000, false);
        measure<candidate>(kernel, -1, 10000, false);
        const std::size_t count = kernel[0] == 'l' ? 500000 : 5000000;
        for (int sample = 0; sample < 4; ++sample) {
            if (sample % 2 == 0) {
                measure<baseline>(kernel, sample, count, true);
                measure<candidate>(kernel, sample, count, true);
            } else {
                measure<candidate>(kernel, sample, count, true);
                measure<baseline>(kernel, sample, count, true);
            }
        }
    }
    std::printf("verified_sink,%llu\n", static_cast<unsigned long long>(sink));
}
