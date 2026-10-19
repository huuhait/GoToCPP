// The C++ twin of defer_members.gp: the expected output is what this program prints.
#include <cstdio>
#include <memory>
#include <vector>
#include "../defer_members.hpp"

int main() {
    std::vector<Guard> v;
    v.emplace_back();
    Guard g;
    Guard* p = &g;
    auto up = std::make_unique<Guard>();
    std::printf("body\n");
    // The deferred calls run last in, first out.
    up->note("d");
    p->note("c");
    v.at(0).note("b");
    v.front().note("a");
    std::printf("%d %s %d %s %d %s\n", v.at(0).hits, v.at(0).name.c_str(), g.hits, g.name.c_str(), up->hits, up->name.c_str());
}
