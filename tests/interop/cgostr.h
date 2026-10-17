// A helper header for the tests of strings at the C/C++ boundary (step 4.9): C functions that
// return C strings (also null, unsigned char, bytes with NULs), take them, keep a copy, and
// count; and C++ functions with std::string_view and std::string.
#pragma once

#include <stdlib.h>
#include <string.h>

#include <string>
#include <string_view>

inline const char* cs_greeting() {
    return "hello, C";
}
inline char* cs_null() {
    return nullptr;
}
// cs_column is what sqlite3_column_text returns: an unsigned char*.
inline const unsigned char* cs_column() {
    static const unsigned char text[] = "column";
    return text;
}
// cs_bytes has a NUL inside: GoStringN and GoBytes copy it too.
inline const char* cs_bytes() {
    return "ab\0cd";
}
inline int cs_len(const char* s) {
    return s == nullptr ? -1 : (int)strlen(s);
}
// cs_upper_copy returns a malloc'ed upper-case copy, which the caller frees with C.free.
inline char* cs_upper_copy(const char* s) {
    char* r = strdup(s);
    for (char* p = r; *p; p++) {
        if (*p >= 'a' && *p <= 'z') {
            *p = (char)(*p - 'a' + 'A');
        }
    }
    return r;
}
// cs_keep keeps the pointer it gets, as some C APIs do: only a C.CString copy may go there.
inline const char* cs_kept = nullptr;
inline void cs_keep(const char* s) {
    cs_kept = s;
}
inline const char* cs_get_kept() {
    return cs_kept;
}

inline size_t sv_count(std::string_view s, char c) {
    size_t n = 0;
    for (char x : s) {
        n += x == c;
    }
    return n;
}
inline std::string sv_shout(std::string_view s) {
    return std::string(s) + "!";
}
inline std::string_view sv_word() {
    return "word";
}
inline std::string str_join(const std::string& a, const std::string& b) {
    return a + "+" + b;
}

// A pointer field for the composite-holder lifetime regressions (keyed and positional).
struct CStringItem {
    const char* name;
};
