// A helper header for value_members.gp: global objects and raw pointers of a C++ type.
#pragma once

struct S {
    int x;
    int twice() const { return 2 * x; }
};

inline S s_obj{7};
inline S* sp_set = &s_obj;
inline S* sp_nil = nullptr;
