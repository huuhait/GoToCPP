// Micro-benchmark of gp::map (gocpp/map.hpp): insert, lookup (hit and miss), delete and range
// for int64 and string keys at 10, 1000, 100000 and 10000000 entries. Prints one line per
// measurement: "<op> <key> <n> <ns/op>" (CPU time, the best of a few repeats) and, per key
// type and size, the heap bytes per entry. tools/mapbench/map_bench.go is the same program in Go.
//
//   g++ -O2 -std=c++20 -Iruntime/include tools/mapbench/map_bench.cpp runtime/src/*.cpp
//   runtime/src/os/linux/*.cpp -pthread
//
// Not part of any test: it is a tool (Phase 15, step 15.5).
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <ctime>
#include <cstdio>
#include <cstdlib>
#include <malloc.h>
#include <random>
#include <string>
#include <vector>

namespace {

double cpu_ns() {
    timespec ts;
    clock_gettime(CLOCK_PROCESS_CPUTIME_ID, &ts);
    return static_cast<double>(ts.tv_sec) * 1e9 + static_cast<double>(ts.tv_nsec);
}

size_t heap_in_use() {
    struct mallinfo2 mi = mallinfo2();
    return mi.uordblks + mi.hblkhd;
}

template <typename F>
double best(int repeats, int64_t ops, F&& f) {
    double b = 1e30;
    for (int r = 0; r < repeats; r++) {
        double t0 = cpu_ns();
        f();
        double t1 = cpu_ns();
        b = std::min(b, (t1 - t0) / static_cast<double>(ops));
    }
    return b;
}

volatile int64_t sink;

template <typename K, typename Make>
void run(const char* kname, int64_t n, Make make) {
    using M = gp::map<K, int64_t>;
    std::vector<K> keys;
    keys.reserve(static_cast<size_t>(n));
    std::mt19937_64 rng(12345);
    for (int64_t i = 0; i < n; i++) {
        keys.push_back(make(static_cast<int64_t>(rng() >> 1)));
    }
    std::vector<K> misses;
    misses.reserve(static_cast<size_t>(n));
    for (int64_t i = 0; i < n; i++) {
        misses.push_back(make(static_cast<int64_t>(rng() >> 1)));
    }
    int64_t reps = std::max<int64_t>(1, 4000000 / n);
    int tries = n >= 1000000 ? 2 : 3;
    auto report = [&](const char* op, double ns) {
        std::printf("%s %s %lld %.2f\n", op, kname, (long long)n, ns);
    };

    report("insert", best(tries, n * reps, [&] {
               for (int64_t r = 0; r < reps; r++) {
                   M m = gp::make_map<K, int64_t>();
                   for (int64_t i = 0; i < n; i++) {
                       gp::set(m, keys[static_cast<size_t>(i)], i);
                   }
                   sink = gp::len(m);
               }
           }));
    report("insert_hint", best(tries, n * reps, [&] {
               for (int64_t r = 0; r < reps; r++) {
                   M m = gp::make_map<K, int64_t>(n);
                   for (int64_t i = 0; i < n; i++) {
                       gp::set(m, keys[static_cast<size_t>(i)], i);
                   }
                   sink = gp::len(m);
               }
           }));

    size_t before = heap_in_use();
    M m = gp::make_map<K, int64_t>();
    for (int64_t i = 0; i < n; i++) {
        gp::set(m, keys[static_cast<size_t>(i)], i);
    }
    size_t after = heap_in_use();
    if (n >= 1000) {
        std::printf("bytes_per_entry %s %lld %.1f\n", kname, (long long)n,
                    static_cast<double>(after - before) / static_cast<double>(n));
    }

    int64_t lreps = std::max<int64_t>(1, 8000000 / n);
    report("lookup_hit", best(tries, n * lreps, [&] {
               int64_t s = 0;
               for (int64_t r = 0; r < lreps; r++) {
                   for (int64_t i = 0; i < n; i++) {
                       s += gp::get(m, keys[static_cast<size_t>(i)]);
                   }
               }
               sink = s;
           }));
    report("lookup_miss", best(tries, n * lreps, [&] {
               int64_t s = 0;
               for (int64_t r = 0; r < lreps; r++) {
                   for (int64_t i = 0; i < n; i++) {
                       s += gp::get(m, misses[static_cast<size_t>(i)]);
                   }
               }
               sink = s;
           }));
    report("range", best(tries, n * lreps, [&] {
               int64_t s = 0;
               for (int64_t r = 0; r < lreps; r++) {
                   for (auto it = gp::range_map(m); gp::range_next(it);) {
                       s += it.gp_value();
                   }
               }
               sink = s;
           }));
    report("update", best(tries, n * lreps, [&] {
               for (int64_t r = 0; r < lreps; r++) {
                   for (int64_t i = 0; i < n; i++) {
                       gp::ref(m, keys[static_cast<size_t>(i)])++;
                   }
               }
           }));
    // delete: all the keys of full tables (built outside the timer: reps tables for a small n).
    {
        double del = 1e30;
        for (int t = 0; t < tries + 2; t++) {
            std::vector<M> tables;
            tables.reserve(static_cast<size_t>(reps));
            for (int64_t r = 0; r < reps; r++) {
                tables.push_back(gp::make_map<K, int64_t>());
                for (int64_t i = 0; i < n; i++) {
                    gp::set(tables.back(), keys[static_cast<size_t>(i)], i);
                }
            }
            double t0 = cpu_ns();
            for (M& d : tables) {
                for (int64_t i = 0; i < n; i++) {
                    gp::erase(d, keys[static_cast<size_t>(i)]);
                }
            }
            del = std::min(del, (cpu_ns() - t0) / static_cast<double>(n * reps));
        }
        report("delete", del);
    }
    // churn: a map that holds n/2 entries, replace the oldest by a new one n times.
    {
        M c = gp::make_map<K, int64_t>();
        int64_t half = n / 2 > 0 ? n / 2 : 1;
        for (int64_t i = 0; i < half; i++) {
            gp::set(c, keys[static_cast<size_t>(i)], i);
        }
        double ns = best(tries, n * reps, [&] {
            for (int64_t r = 0; r < reps; r++) {
                for (int64_t i = 0; i < n; i++) {
                    gp::erase(c, keys[static_cast<size_t>(i % n)]);
                    gp::set(c, keys[static_cast<size_t>((i + half) % n)], i);
                }
            }
        });
        report("churn", ns);
    }
}

} // namespace

// SKey: a struct key of two integers, hashed with gp::hash_fields (the emitter's protocol).
struct SKey {
    int64_t a = 0;
    int64_t b = 0;
    bool operator==(const SKey&) const = default;
    std::size_t gp_hash() const { return gp::hash_fields(a, b); }
};

int main(int argc, char** argv) {
    std::vector<int64_t> sizes = {10, 1000, 100000, 10000000};
    bool ints = true, strs = true, structs = true;
    if (argc > 1) {
        sizes.clear();
        bool picked = false;
        ints = strs = structs = false;
        for (int i = 1; i < argc; i++) {
            std::string a = argv[i];
            if (a == "int") {
                ints = picked = true;
            } else if (a == "str") {
                strs = picked = true;
            } else if (a == "struct") {
                structs = picked = true;
            } else {
                sizes.push_back(std::atoll(a.c_str()));
            }
        }
        if (!picked) {
            ints = strs = structs = true;
        }
        if (sizes.empty()) {
            sizes = {10, 1000, 100000, 10000000};
        }
    }
    for (int64_t n : sizes) {
        if (ints) {
            run<int64_t>("int", n, [](int64_t v) { return v; });
        }
        if (strs) {
            run<gp::string>("string", n, [](int64_t v) {
                char buf[32];
                int len = std::snprintf(buf, sizeof buf, "key-%lld", (long long)v);
                return gp::string::gp_copy(std::string_view(buf, static_cast<size_t>(len)));
            });
        }
        if (structs) {
            run<SKey>("struct", n, [](int64_t v) { return SKey{v, v >> 7}; });
        }
        std::fflush(stdout);
    }
    return 0;
}
