// A helper header for reflect_cpp_values_set.gp (step 9.3): C and C++ types of every shape that
// reflect meets: a plain C struct, an enum, a C array, a union, bit-fields, a wide integer, a class
// that can't be copied, a function pointer.
#pragma once

#include <string>
#include <string_view>

namespace rc {

enum shade { DARK, MID = 2, LIGHT };
enum class tone : unsigned char { low = 1, high = 9 };

struct point {
    int x;
    int y;
};

struct bits {
    unsigned a : 3;
    int b : 5;
    unsigned char tail;
};

union either {
    int i;
    float f;
};

struct pinned {
    pinned() = default;
    pinned(const pinned&) = delete;
    pinned& operator=(const pinned&) = delete;
    int n = 4;
};

struct wide {
    __int128 big;
    long long small;
};

inline int twice(int x) { return 2 * x; }
using binop = int (*)(int, int);
inline int add(int a, int b) { return a + b; }

} // namespace rc
