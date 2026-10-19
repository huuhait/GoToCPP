// bench: tree_alloc (C++ twin of bench/programs/data/tree_alloc.gp)
#include <cstdint>
#include <cstdio>
#include <memory>

static uint64_t seed = 99;

static uint64_t rnd() {
    seed = seed * 6364136223846793005ULL + 1442695040888963407ULL;
    return seed >> 33;
}

struct Tree {
    int64_t key;
    std::unique_ptr<Tree> left, right;
};
using Ptr = std::unique_ptr<Tree>;

static void insert(Ptr& t, int64_t k) {
    if (!t) {
        t = std::make_unique<Tree>();
        t->key = k;
    } else if (k < t->key) {
        insert(t->left, k);
    } else if (k > t->key) {
        insert(t->right, k);
    }
}

static bool find(const Tree* t, int64_t k) {
    while (t) {
        if (k < t->key)
            t = t->left.get();
        else if (k > t->key)
            t = t->right.get();
        else
            return true;
    }
    return false;
}

static int64_t height(const Tree* t) {
    if (!t)
        return 0;
    int64_t l = height(t->left.get()), r = height(t->right.get());
    return (l > r ? l : r) + 1;
}

static void walk(const Tree* t, uint64_t& acc, int64_t& count) {
    if (!t)
        return;
    walk(t->left.get(), acc, count);
    count++;
    acc = acc * 31 + static_cast<uint64_t>(t->key);
    walk(t->right.get(), acc, count);
}

int main() {
    const int64_t n = BENCH_N;
    Ptr root;
    for (int64_t i = 0; i < n; i++)
        insert(root, static_cast<int64_t>(rnd() % static_cast<uint64_t>(4 * n)));
    int64_t hits = 0;
    for (int64_t i = 0; i < n; i++)
        if (find(root.get(), static_cast<int64_t>(rnd() % static_cast<uint64_t>(4 * n))))
            hits++;
    uint64_t acc = 0;
    int64_t count = 0;
    walk(root.get(), acc, count);
    std::printf("%lld %lld %lld %llu\n", static_cast<long long>(count),
                static_cast<long long>(hits), static_cast<long long>(height(root.get())),
                static_cast<unsigned long long>(acc));
}
