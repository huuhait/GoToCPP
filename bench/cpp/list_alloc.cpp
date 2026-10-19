// bench: list_alloc (C++ twin of bench/programs/data/list_alloc.gp)
#include <cstdint>
#include <cstdio>
#include <memory>

struct Node {
    int64_t val;
    std::unique_ptr<Node> next;
};
using List = std::unique_ptr<Node>;

static List build(int64_t n, int64_t base) {
    List head;
    for (int64_t i = 0; i < n; i++) {
        auto nd = std::make_unique<Node>();
        nd->val = base + i;
        nd->next = std::move(head);
        head = std::move(nd);
    }
    return head;
}

static uint64_t sum(const List& l) {
    uint64_t s = 0;
    for (const Node* p = l.get(); p; p = p->next.get())
        s += static_cast<uint64_t>(p->val);
    return s;
}

static List reverse(List l) {
    List prev;
    while (l) {
        List next = std::move(l->next);
        l->next = std::move(prev);
        prev = std::move(l);
        l = std::move(next);
    }
    return prev;
}

int main() {
    uint64_t total = 0;
    for (int64_t r = 0; r < BENCH_N; r++) {
        List l = build(1000, r);
        total += sum(l);
        l = reverse(std::move(l));
        total += static_cast<uint64_t>(l->val) * 3;
        total += sum(l);
        l.reset();
    }
    std::printf("%llu\n", static_cast<unsigned long long>(total));
}
