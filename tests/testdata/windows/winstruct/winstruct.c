#include "winstruct.h"

// The defaults of step 14.4 reach the package's own C files.
#ifndef WIN32_LEAN_AND_MEAN
#error "WIN32_LEAN_AND_MEAN is not defined"
#endif
#ifndef NOMINMAX
#error "NOMINMAX is not defined"
#endif
#ifndef UNICODE
#error "UNICODE is not defined"
#endif
#ifndef _UNICODE
#error "_UNICODE is not defined"
#endif
#ifdef min
#error "min is a macro"
#endif

int WINAPI add_stdcall(int a, int b) {
    return a + b;
}

int WINAPI apply(binop f, int a, int b) {
    return f(a, b);
}

void fill_entry(Entry* e, DWORD id, const WCHAR* name) {
    e->id = id;
    lstrcpynW(e->name, name, 16);
    e->enabled = TRUE;
}

Flags make_flags(unsigned kind, unsigned level, unsigned id) {
    Flags f;
    f.kind = kind;
    f.level = level;
    f.id = id;
    return f;
}

size_t packed_size(void) {
    return sizeof(Packed);
}
