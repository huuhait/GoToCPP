// The C++ twin of win_std.gp: the expected output is what this program prints.
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <map>
#include <memory>
#include <optional>
#include <string>
#include <vector>

int main() {
    auto up = std::make_unique<std::vector<long long>>();
    up->push_back(4);
    up->push_back(9);
    std::printf("%d %lld %d\n", (int)up->size(), up->back(), (int)up.get()->size());

    std::optional<std::string> opt{std::string("abc")};
    std::printf("%s %d %d\n", opt.has_value() ? "true" : "false", (int)opt->size(), (int)opt->length());

    std::vector<long long> v{10, 20, 30};
    v.at(1) = 21;
    long long* p = &v.back();
    *p += 5;
    std::printf("%lld %lld %lld %lld\n", v.at(0), v.at(1), v.at(2), *p);

    std::string word = "windows" + std::to_string(2);
    long long n = std::strlen(word.c_str());
    long long l = std::labs(-5);
    auto total = std::strlen("abc") + (long long)word.size();
    std::printf("%lld %lld %lld %d\n", n, l, (long long)total, std::atoi(word.substr(7).c_str()));

    std::map<std::string, long long> counts;
    for (const char* w : {"x", "y", "x"}) counts[std::string(w)]++;
    std::printf("%d %lld %lld\n", (int)counts.size(), counts["x"], counts["y"]);
}
