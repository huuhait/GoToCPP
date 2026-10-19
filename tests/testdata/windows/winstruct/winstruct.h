// Windows features of a C header (step 14.4): packing, bit-fields, UTF-16 text in a struct, a
// function that is exported with __declspec, and a WINAPI callback type.
#pragma once

#include <windows.h>

#ifdef __cplusplus
extern "C" {
#endif

#pragma pack(push, 1)
typedef struct Packed {
    char a;
    int b;
    short c;
} Packed;
#pragma pack(pop)

typedef struct Flags {
    unsigned int kind : 3;
    unsigned int level : 5;
    unsigned int id : 24;
} Flags;

typedef struct Entry {
    DWORD id;
    WCHAR name[16];
    BOOL enabled;
} Entry;

typedef int(WINAPI* binop)(int, int);

__declspec(dllexport) int WINAPI add_stdcall(int a, int b);
int WINAPI apply(binop f, int a, int b);
void fill_entry(Entry* e, DWORD id, const WCHAR* name);
Flags make_flags(unsigned kind, unsigned level, unsigned id);
size_t packed_size(void);

// Defined in winstruct.cpp, which uses std::min and std::max next to <windows.h>.
int clamp3(int lo, int v, int hi);

// A macro of <windows.h>-style headers that a Go name can collide with.
#define ERROR_LEVEL 3

#ifdef __cplusplus
}
#endif
