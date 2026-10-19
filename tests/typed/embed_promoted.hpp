// Types for embed_promoted.gp and its C++ twin.
#pragma once
#include <string>
#include <vector>

struct Shape {
    int w = 1, h = 1;
    int area() const { return w * h; }
    void grow(int k) {
        w += k;
        h += k;
    }
};

// Members of a public base are the class's members.
struct Colored : Shape {
    int rgb = 7;
    int color() const { return rgb; }
};

struct Stack {
    std::vector<int> items;
    int top_calls = 0;
    static int made;
    enum Mode { Fast = 3, Slow };
    void push(int v) { items.push_back(v); }
    // const and non-const overloads: the object is not const, so the second is called.
    int top() const { return items.back(); }
    int& top() {
        top_calls++;
        return items.back();
    }
    template <class T>
    T twice(T v) const {
        return v + v;
    }
    static int bump() { return ++made; }
};
inline int Stack::made = 0;

struct Tag {
    std::string text;
    void set(const char* s) { text = s; }
    int length() const { return static_cast<int>(text.size()); }
};
