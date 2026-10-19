// Types for strings_auto.gp and its C++ twin.
#pragma once
#include <cctype>
#include <cstring>
#include <string>
#include <string_view>

inline int count_char(const char* s, char c) {
    int n = 0;
    for (; *s; s++) {
        if (*s == c) n++;
    }
    return n;
}

inline std::string shout(const std::string& s) {
    std::string r = s;
    for (auto& ch : r) ch = (char)std::toupper((unsigned char)ch);
    return r + "!";
}

inline int total_len(const std::string& a, const std::string& b) {
    return (int)(a.size() + b.size());
}

inline int length_sv(std::string_view s) {
    return (int)s.size();
}

// Overloads of different kinds: the call is left to C++, which prefers the string_view for a gocpp string.
inline const char* kind(const char*) { return "cstr"; }
inline const char* kind(std::string_view) { return "view"; }

// A method that reads the string, in a class without a char pointer member.
struct Meter {
    int total = 0;
    int count(const char* s) {
        total += (int)std::strlen(s);
        return total;
    }
};

// A class with a char pointer member: its setter may keep the pointer.
struct Keeper {
    const char* cname = nullptr;
    void set(const char* n) { cname = n; }
};
