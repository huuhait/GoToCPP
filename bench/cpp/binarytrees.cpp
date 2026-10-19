// bench: binarytrees (C++ twin of bench/programs/cpu/binarytrees.gp)
#include <cstdint>
#include <cstdio>
#include <memory>

struct Node {
    std::unique_ptr<Node> left, right;
    int64_t check() const { return left ? 1 + left->check() + right->check() : 1; }
};

static std::unique_ptr<Node> bottomUp(int64_t depth) {
    auto n = std::make_unique<Node>();
    if (depth > 0) {
        n->left = bottomUp(depth - 1);
        n->right = bottomUp(depth - 1);
    }
    return n;
}

int main() {
    const int64_t maxDepth = BENCH_N, minDepth = 4, stretch = maxDepth + 1;
    std::printf("stretch tree of depth %lld check: %lld\n", (long long)stretch,
                (long long)bottomUp(stretch)->check());
    auto longLived = bottomUp(maxDepth);
    for (int64_t d = minDepth; d <= maxDepth; d += 4) {
        int64_t iters = int64_t(1) << (maxDepth - d + minDepth), check = 0;
        for (int64_t i = 0; i < iters; i++)
            check += bottomUp(d)->check();
        std::printf("%lld trees of depth %lld check: %lld\n", (long long)iters, (long long)d,
                    (long long)check);
    }
    std::printf("long lived tree of depth %lld check: %lld\n", (long long)maxDepth,
                (long long)longLived->check());
}
