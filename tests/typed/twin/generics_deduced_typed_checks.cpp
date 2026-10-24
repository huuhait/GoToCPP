// The C++ twin of generics_deduced_typed_checks.gp: the expected output is what this program prints.
#include <cstdio>
#include <map>
#include <string>

static bool isE(const std::string& s) { return s.empty(); }

int main() {
    std::map<long, std::string> k = {{1, "a"}};
    std::map<long, std::string> k2 = k; // maps.Clone(k)
    bool ok = k[1].empty();
    auto n = 3 + k[1].size();
    std::printf("%zu %s %s %d\n", k2.size(), isE(k[1]) ? "true" : "false", ok ? "true" : "false", int(n));
}
