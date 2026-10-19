// bench: alloc_churn (C++ twin of bench/programs/runtime/alloc_churn.gp)
// Items are shared_ptr nodes, like the Go program's pointers (a chain of a dropped slot is freed
// with its head).
#include <array>
#include <cstdint>
#include <cstdio>
#include <memory>
#include <string>
#include <vector>

struct Item {
    int64_t id = 0;
    std::string name;
    std::vector<int64_t> attrs;
    std::shared_ptr<Item> next;
    ~Item() { // free long chains iteratively, as a garbage collector would
        auto n = std::move(next);
        while (n && n.use_count() == 1)
            n = std::move(n->next);
    }
};

int main() {
    std::array<std::shared_ptr<Item>, 64> keep{};
    int64_t sum = 0;
    uint64_t seed = 5;
    for (int64_t i = 0; i < BENCH_N; i++) {
        seed = seed * 6364136223846793005ull + 1442695040888963407ull;
        auto it = std::make_shared<Item>();
        it->id = i;
        it->attrs.assign(size_t(seed >> 60) + 1, 0);
        it->attrs[0] = i;
        if (i % 7 == 0)
            it->name = std::string("item-") + char('a' + i % 26);
        size_t slot = size_t(seed >> 33) & 63;
        it->next = keep[slot];
        keep[slot] = it;
        if ((i & 15) == 0)
            keep[slot] = nullptr;
        sum += int64_t(it->attrs.size()) + int64_t(it->name.size()) + it->attrs[0] % 3;
    }
    int64_t live = 0;
    for (auto& p : keep)
        for (Item* q = p.get(); q; q = q->next.get())
            live++;
    std::printf("%lld %lld\n", (long long)sum, (long long)live);
}
