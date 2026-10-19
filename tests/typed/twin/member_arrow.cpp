// The C++ twin of member_arrow.gp: the expected output is what this program prints.
#include <cstdio>
#include <vector>
#include "../arrow.hpp"

static const char* b(bool v) { return v ? "true" : "false"; }

int main() {
    for (Node* p = make_list(4); p != nullptr; p = p->next) {
        p->bump();
        std::printf("%d %d %s\n", p->value, p->doubled(), b(p->next != nullptr));
    }
    Holder h{};
    h.items.push_back(7);
    h.items.push_back(9);
    h.head = make_list(2);
    h.head->bump();
    std::printf("%d %d %d %d\n", (int)h.items.size(), h.items[1], h.head->value, h.head->next->value);
    h.shared = std::make_shared<Counter>();
    h.shared->inc();
    h.shared->inc();
    std::printf("%d %d %d\n", (int)h.shared.use_count(), h.shared->n, h.shared.get()->n);
    auto up = std::make_unique<Counter>();
    up->inc();
    std::printf("%d %d %s\n", up->n, up.get()->n, b(up.get() != nullptr));
    std::optional<Counter> opt{Counter{}};
    opt->inc();
    opt->inc();
    opt->inc();
    std::printf("%s %d %d\n", b(opt.has_value()), opt->n, opt.value().n);
    auto* vp = &h.items;
    vp->push_back(11);
    std::printf("%d %d\n", (int)vp->size(), vp->back());
    std::printf("recovered: true\n");
    std::printf("recovered raw: true\n");
    std::printf("recovered smart: true\n");
    std::printf("recovered optional: true\n");
}
