// A helper header for the tests of unsafe, pointers and slices at the C/C++ boundary (step 4.10):
// C memory that gocpp views through unsafe.Slice and unsafe.String, structs with array members, a
// function that C code gets as an address, and functions that take a std::span.
#pragma once

#include <cstddef>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <span>
#include <string_view>

struct upoint {
    int x, y;
};

struct urec {
    char name[8];
    int vals[4];
    upoint pts[2];
};

inline urec the_rec = {"rec", {10, 20, 30, 40}, {{1, 2}, {3, 4}}};
inline urec* get_rec() { return &the_rec; }

// A linked list that C builds, for the walks over lists built by C and by &T{...}.
struct unode {
    int v;
    unode* next;
};
inline unode* build_list(int n) {
    unode* head = nullptr;
    for (int i = n; i > 0; i--) {
        head = new unode{i, head};
    }
    return head;
}
inline void free_list(unode* p) {
    while (p != nullptr) {
        unode* next = p->next;
        delete p;
        p = next;
    }
}
inline unode* kept_node;
inline void keep_node(unode* p) { kept_node = p; }
inline int kept_value() { return kept_node->v; }
inline int sum_list(const unode* p) {
    int s = 0;
    for (; p != nullptr; p = p->next) {
        s += p->v;
    }
    return s;
}

// A node that counts the live objects (the stores that keep and release objects), a struct with
// pointers in a struct member, in an array member and in a void* member, and a plain struct.
inline int live_counted_count = 0;
struct ucounted {
    int id = 0;
    ucounted* next = nullptr;
    ucounted() { live_counted_count++; }
    ucounted(const ucounted& o) : id(o.id), next(o.next) { live_counted_count++; }
    ~ucounted() { live_counted_count--; }
};
inline int live_counted() { return live_counted_count; }

struct uinner {
    unode* p;
};
struct uholder {
    int v;
    uinner in;
    unode* slots[3];
    void* data;
    unode* next;
};

// A function with a void** out-parameter, as an allocator has.
inline void alloc_block(void** out, std::size_t n) {
    *out = std::malloc(n);
    std::memset(*out, 'z', n);
}
inline void no_write(void**) {}

// An array of C strings, as argv is, over memory that C owns.
inline const char* const words_table[3] = {"alpha", "beta", "gamma"};
inline const char* const* get_words() { return words_table; }
inline char** get_argv() {
    static char a[] = "prog", b[] = "-v", c[] = "file";
    static char* argv[] = {a, b, c, nullptr};
    return argv;
}

// Memory from malloc with no NUL after its bytes (a string over it must not read past them).
inline unsigned char* get_blob(std::size_t n) {
    auto* p = static_cast<unsigned char*>(std::malloc(n));
    for (std::size_t i = 0; i < n; i++) {
        p[i] = static_cast<unsigned char>('a' + i);
    }
    return p;
}

inline int sum_points(const upoint* p, int n) {
    int s = 0;
    for (int i = 0; i < n; i++) {
        s += p[i].x + p[i].y;
    }
    return s;
}
inline int sum_ints(const int* xs, std::size_t n) {
    int s = 0;
    for (std::size_t i = 0; i < n; i++) {
        s += xs[i];
    }
    return s;
}
inline void scale_point(upoint* p, int k) {
    p->x *= k;
    p->y *= k;
}
inline void set_int(int* p, int v) { *p = v; }
inline void set_long(long* p, long v) { *p = v; }
inline std::size_t blob_len(const void* p, std::size_t n) {
    return p == nullptr ? 0 : n;
}
inline std::size_t sizeof_urec() { return sizeof(urec); }
inline std::size_t point_size() { return sizeof(upoint); }
inline upoint make_point(int x, int y) { return upoint{x, y}; }

// A function that goes through unsafe.Pointer and back (dlsym's way), and the function pointer
// type that it comes back as.
inline int triple(int x) { return 3 * x; }
typedef int (*unary_fn)(int);
inline int call_unary(unary_fn f, int x) { return f(x); }

// Functions of std::span, and of std::string_view.
inline int span_sum(std::span<const int> xs) {
    int s = 0;
    for (int x : xs) {
        s += x;
    }
    return s;
}
inline void span_fill(std::span<int> xs, int v) {
    for (int& x : xs) {
        x = v;
    }
}
inline std::size_t span_len(std::span<const unsigned char> bs) { return bs.size(); }
inline int span_first(std::span<const unsigned char> bs) { return bs.empty() ? -1 : bs[0]; }
inline std::size_t view_len(std::string_view s) { return s.size(); }
