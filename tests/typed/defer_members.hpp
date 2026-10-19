// Types for defer_members.gp and its C++ twin.
#pragma once
#include <string>

struct Guard {
    std::string name;
    int hits = 0;
    void note(const std::string& what) { hits++; name += what; }
};
