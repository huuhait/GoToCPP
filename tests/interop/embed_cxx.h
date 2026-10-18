// A helper header for the tests of embedded C/C++ types (step 5.8): a C struct with a method,
// a class that holds a std::string, and a counter object whose members change it.
#pragma once

#include <string>

struct epoint {
    int x, y;
    int sum() const { return x + y; }
    void scale(int k) {
        x *= k;
        y *= k;
    }
};

struct elabel {
    std::string text;
    int size() const { return static_cast<int>(text.size()); }
    void append(const char* s) { text += s; }
};
