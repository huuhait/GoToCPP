// A C++ class whose default constructor cannot run at compile time (it counts the objects), so a
// package variable of it, or of a struct that holds one, cannot be constinit.
#pragma once

#include <string>

namespace late {

inline int constructed = 0;

struct Late {
    int v;
    std::string tag;
    Late() : v(++constructed * 100), tag("late") {}
    int get() const { return v; }
    void set(int x) { v = x; }
};

inline int built() { return constructed; }

}  // namespace late
