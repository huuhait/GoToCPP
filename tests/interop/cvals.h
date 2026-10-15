// A helper header for the tests of operations on C/C++ values (step 4.4): a linked list of C
// structs, function pointers, an object returned by reference, a global that a call changes, and
// a member that is a macro, as glibc's st_mtime is.
#pragma once

#include <memory>
#include <string>
#include <vector>

struct node {
    int value;
    struct node* next;
};

inline node n3{3, nullptr};
inline node n2{2, &n3};
inline node n1{1, &n2};

inline node* list_head() { return &n1; }

typedef int (*binop)(int, int);
inline int add_ints(int a, int b) { return a + b; }
inline binop get_add() { return add_ints; }

struct point {
    int x, y;
    int sum() const { return x + y; }
};

// the_ints is one vector for the whole program, returned by reference: the_ints()[0] = 1 and
// the_ints().push_back(2) change it.
inline std::vector<int>& the_ints() {
    static std::vector<int> v;
    return v;
}

inline int counter = 0;
inline int bump() { return ++counter; }

struct stamp {
    long when_sec;
};
#define when when_sec

// For object_temps.gp: functions that return references into their arguments.
struct bag {
    std::vector<int> items{1, 2, 3};
};
inline bag make_bag() { return bag{}; }
inline const bag& same_bag(const bag& b) { return b; }

// A function that takes and returns references to its arguments' places.
struct pt {
    int x;
};
inline pt pts[2] = {{1}, {2}};
inline pt& pick(pt& a, pt& b) { return a.x >= b.x ? a : b; }

// For move_only.gp: a parameter that can only be moved into.
inline int take_ptr(std::unique_ptr<int> p, int x) { return *p + x; }

// For c_comparisons.gp: a global whose address C code takes.
inline int the_box = 41;
inline void bump_at(int* p) { ++*p; }
