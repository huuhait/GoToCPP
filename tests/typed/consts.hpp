// Constants for macro_consts.gp and its C++ twin (step 12.8): every kind of C++ constant that Go can use.
#pragma once
#include <cstdint>

#define NEG (-7)
#define ZERO 0
#define BIG (1UL << 31)
#define MASK 0xFFFFu
#define SHIFT 5
#define ROWS 3
#define COLS (ROWS + 1)
#define FRAME (ROWS * COLS * 2)
#define ALIAS COLS
#define TENTH 0.1
#define GREETING "hello"
#define SEP '/'
#define ESC "tab\there \"q\" \\ \x41\102"
#define ON true

// Redefined and removed: the value is the one at the end of the headers.
#define LIMIT 10
#undef LIMIT
#define LIMIT 20
#ifdef __linux__
#define OS_ID 1
#else
#define OS_ID 2
#endif

// Not constants: a function-like macro and a macro with a side effect.
inline int counter = 0;
#define NEXT (counter++)
#define TWICE(x) ((x) * 2)

constexpr int kRows = 4;
constexpr double kScale = 2.5;
struct Dim { static constexpr int kWidth = 6; int w, h; };
constexpr Dim kDim{7, 8};
enum Color { RED, GREEN = 4, BLUE };
namespace geo { constexpr int kSides = 3; }

inline int next() { return NEXT; }
