// small cases: macros named like enumerators, function typedefs, UTF-8 names, hostile names
#pragma once
enum Sock { SOCK_STREAM = 1, SOCK_DGRAM = 2 };
#define SOCK_STREAM SOCK_STREAM
#define SOCK_DGRAM SOCK_DGRAM
typedef void (Fn)(int);
typedef Fn *FnPtr;
typedef FnPtr *FnPtrPtr;
void install(FnPtr cb, Fn *other);
enum Flagbit { FLAG_TOP = 0x80000000u, FLAG_LOW = 1 };
#define UNI_ÄÖ 3
#define NEG_ZERO (-0.0)
#define POS_ZERO 0.0
int plain(int);
#define map 4
#define auto 5
#define bool 6
#define val 7
#define size 8
