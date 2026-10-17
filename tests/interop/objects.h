// C++ objects for the tests of step 4.8 (closure_*.gp, defer_*.gp, result_*.gp, policy_*.gp): types
// with a constructor and a destructor that print (so the order of lifetimes is visible), a class
// with no default constructor, one that can't be copied or moved, and a class that can only be
// moved.
#pragma once

#include <cstdio>
#include <memory>
#include <mutex>
#include <string>
#include <utility>
#include <vector>

// Counter has methods that change it, and shows how it was made.
struct Counter {
    int n = 0;
    void inc() { n++; }
    void add(int d) { n += d; }
    int get() const { return n; }
};

// Guard prints when it is made, copied, moved and destroyed: an RAII object (a lock_guard).
struct Guard {
    std::string name;
    explicit Guard(std::string n) : name(std::move(n)) { std::printf("lock %s\n", name.c_str()); }
    Guard(const Guard& o) : name(o.name + "'") { std::printf("copy %s\n", o.name.c_str()); }
    Guard(Guard&& o) noexcept : name(std::move(o.name)) { std::printf("move %s\n", name.c_str()); }
    ~Guard() {
        if (!name.empty()) {
            std::printf("unlock %s\n", name.c_str());
        }
    }
    void note(const char* what) const { std::printf("%s: %s\n", name.c_str(), what); }
    void rename(std::string n) { name = std::move(n); }
};

// Widget has no default constructor, and can be copied.
struct Widget {
    int id;
    explicit Widget(int i) : id(i) {}
    int get() const { return id; }
    void set(int i) { id = i; }
};

// Pinned can be neither copied nor moved (it holds a std::mutex in real code).
struct Pinned {
    int v;
    explicit Pinned(int x) : v(x) { std::printf("pinned %d\n", v); }
    Pinned(const Pinned&) = delete;
    Pinned& operator=(const Pinned&) = delete;
    ~Pinned() { std::printf("unpinned %d\n", v); }
    void bump() { v++; }
    int get() const { return v; }
};

inline Pinned make_pinned(int x) {
    return Pinned(x);
}

// Handle can only be moved; it has a default constructor (empty).
struct Handle {
    int fd = -1;
    Handle() = default;
    explicit Handle(int f) : fd(f) {}
    Handle(const Handle&) = delete;
    Handle& operator=(const Handle&) = delete;
    Handle(Handle&& o) noexcept : fd(std::exchange(o.fd, -1)) {}
    Handle& operator=(Handle&& o) noexcept {
        fd = std::exchange(o.fd, -1);
        return *this;
    }
    ~Handle() {
        if (fd >= 0) {
            std::printf("close %d\n", fd);
        }
    }
    int get() const { return fd; }
};

inline Handle make_handle(int fd) {
    return Handle(fd);
}

inline void bump(Counter& c) {
    c.n += 10;
}

inline void bump_ptr(Counter* c) {
    c->n += 100;
}

inline int total(const std::vector<int>& v) {
    int t = 0;
    for (int x : v) {
        t += x;
    }
    return t;
}

inline void take(Handle h) {
    std::printf("take %d\n", h.fd);
}

inline void shout(int x) {
    std::printf("shout %d\n", x);
}
using shout_fn = void (*)(int);
inline shout_fn get_shout() {
    return &shout;
}

// Functions that print when they run, to see the order in which a deferred call evaluates its
// object and its arguments.
inline int mk(const char* s) {
    std::printf("mk %s\n", s);
    return 1;
}
inline void show2(int a, int b) {
    std::printf("show2 %d %d\n", a, b);
}
inline Counter mkc(const char* s) {
    std::printf("mkc %s\n", s);
    return Counter{};
}

inline void rename_ptr(Guard* g, const char* n) {
    g->rename(n);
}

// A struct that holds a Guard and a Counter, to see them through a gocpp pointer.
inline void use_pinned(Pinned) {}

// Lock can be neither copied nor moved (it holds a std::mutex), and has a default constructor.
struct Lock {
    std::mutex m;
    int n = 0;
    void lock() { m.lock(); }
    void unlock() { m.unlock(); }
    void bump() { n++; }
};
inline void use_lock(Lock) {}

// Live counts the objects that exist, to see how long a local lives.
struct Live {
    static inline int count = 0;
    int n = 0;
    Live() { count++; }
    Live(const Live& o) : n(o.n) { count++; }
    ~Live() { count--; }
    void inc() { n++; }
    int get() const { return n; }
};
inline int live_count() {
    return Live::count;
}
