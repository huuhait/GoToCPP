// Types for member_arrow.gp and its C++ twin.
#pragma once
#include <memory>
#include <optional>
#include <string>
#include <vector>

struct Node {
    int value;
    Node* next;
    int doubled() const { return value * 2; }
    void bump() { value++; }
};

struct Counter {
    int n = 0;
    int inc() { return ++n; }
    int get() const { return n; }
};

struct Holder {
    std::vector<int> items;
    std::shared_ptr<Counter> shared;
    Node* head = nullptr;
};

inline Node* make_list(int n) {
    static Node pool[8];
    Node* head = nullptr;
    for (int i = n; i >= 1; i--) {
        pool[i] = Node{i * 10, head};
        head = &pool[i];
    }
    return head;
}
