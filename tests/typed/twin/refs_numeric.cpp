// The C++ twin of refs_numeric.gp: the expected output is what this program prints.
#include <cmath>
#include <cstdio>
#include <cstring>
#include <map>
#include <string>
#include <utility>
#include <vector>

int main() {
    std::vector<long> v{10, 20, 30, 40};
    v.at(0) = 5;
    v.back()++;
    v.front() += 2;
    long* p = &v.at(1);
    *p = 99;
    auto it = v.begin() + 2;
    *it = 77;
    std::printf("%ld %ld %ld %ld %ld %ld\n", v.at(0), v.at(1), v.at(2), v.at(3), *p, *it);

    auto c = v.at(1);
    c++;
    std::printf("%ld %ld\n", c, v.at(1));

    long n = std::strlen("hello");
    long m;
    m = std::strlen("12345");
    auto total = std::strlen("abc") + (long)std::strlen("hello");
    int cn = n;
    double f = std::sqrt(16.0);
    long cut = std::floor(3.9);
    std::printf("%ld %ld %ld %d %g %ld\n", n, m, (long)total, cn, f, cut);
    if (std::strlen("abcd") > (unsigned long)(n - 2) && (unsigned long)n < std::strlen("hello world")) {
        std::printf("compared\n");
    }

    std::map<std::string, long> counts;
    for (const char* w : {"a", "b", "a", "c", "a"}) counts[std::string(w)]++;
    std::string key = "b!";
    counts[key] = 42;
    std::printf("%ld %ld %ld %d\n", counts["a"], counts["b"], counts[key], (int)counts.size());

    std::vector<long> a{1, 2, 3};
    auto b = std::move(a);
    std::printf("%d %d\n", (int)b.size(), (int)a.size());

    auto pr = std::make_pair(3L, 4L);
    pr.first += 10;
    std::swap(pr.first, pr.second);
    std::printf("%ld %ld\n", pr.first, pr.second);
}
