// The C++ twin of embed_names.gp: the expected output is what this program prints.
#include <cstdio>
#include "../embed_names.hpp"

struct S {
    Words Words_;
    More More_;
    int Double() { return Words_.select(2) * 2; }
};

int main() {
    S s{};
    std::printf("%d %d %d %d\n", s.Words_.type(), s.Words_.range(1, 4), s.Words_.select(3), s.Double());
    std::printf("%d %d %d %d %d %d\n", s.Words_.func, s.Words_.var, s.Words_.go, s.Words_.map, s.Words_.len(), s.Words_.append(1));
    std::printf("%d %d %d %d\n", s.Words_.interface, s.Words_.defer, s.Words_.chan, s.Words_.fallthrough);
    std::printf("%d %d %d %d\n", s.More_.package, s.More_.import, s.More_.cap(), s.More_.make());
    s.Words_.go = 50;
    s.Words_.func++;
    s.Words_.select(1);
    std::printf("deferred %d %d\n", s.Words_.go, s.Words_.func);
}
