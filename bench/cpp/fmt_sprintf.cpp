// bench: fmt_sprintf (C++ twin of bench/programs/runtime/fmt_sprintf.gp)
#include <cstdint>
#include <cstdio>
#include <string>

int main() {
    int64_t total = 0;
    uint64_t h = 14695981039346656037ull;
    char buf[128];
    for (int64_t i = 0; i < BENCH_N; i++) {
        int n = std::snprintf(buf, sizeof buf, "n=%lld name=%s v=%lld f=%.3f", (long long)i,
                              "gopher", (long long)(i * 3), double(i) / 7);
        std::string s(buf, size_t(n));
        total += int64_t(s.size());
        h = (h ^ uint8_t(s.back())) * 1099511628211ull;
        h = (h ^ uint8_t(s[2])) * 1099511628211ull;
    }
    std::printf("%lld %llu\n", (long long)total, (unsigned long long)h);
}
