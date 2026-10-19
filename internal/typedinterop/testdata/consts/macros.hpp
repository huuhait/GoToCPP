// The header zoo of the constant probe (step 12.8): every shape of constant that a program can ask for.
#pragma once
#include <cstdint>
#include <climits>
#include <cstdio>
#include <cmath>
#include <numbers>

// integers
#define M_INT 42
#define M_NEG (-7)
#define M_BIG (1UL << 31)
#define M_U64 0xFFFFFFFFFFFFFFFFULL
#define M_I64MIN (-9223372036854775807LL - 1)
#define M_ALIAS M_INT
#define M_ALIAS2 M_ALIAS
#define M_EXPR (M_INT * 2 + 1)
#define M_CAST ((int)5)
#define M_SIZEOF sizeof(int)
#define M_SHIFT (1 << 10)
#define M_HEX 0xFF
#define M_OCT 0777
#define M_BIN 0b1011
#define M_LL 123456789012LL
#define M_TERNARY (M_INT > 40 ? 1 : 2)
#define M_I128 ((__int128)1 << 100)

// floats
#define M_FLT 0.1
#define M_F32 1.5f
#define M_PI2 (3.141592653589793 * 2)
#define M_HALF (1.0 / 2)
#define M_BIGF 1e300
#define M_TINY 5e-324
#define M_NEGF (-2.5)
#define M_LD 0.5L
#define M_NAN __builtin_nanf("")
#define M_INF __builtin_inf()

// strings and characters
#define M_STR "My" "App\n\"q\" \x41\101"
#define M_EMPTY ""
#define M_NUL "a\0b"
#define M_RAW "x\xffy\x80"
#define M_TAB "\t\\\a"
#define M_UTF8 "héllo"
#define M_CHR 'a'
#define M_CHR2 '\n'
#define M_CHR3 '\\'
#define M_WCHR L'x'
#define M_C16 u'y'

// bool
#define M_BOOL true
#define M_NOTB (!M_BOOL)

// things that are not constants
extern int counter;
int next();
#define M_FN(x) ((x) + 1)
#define M_SIDE (counter++)
#define M_CALL next()
#define M_PTR ((void*)0)
#define M_STMT do { } while (0)

// constexpr and const variables, enumerators
constexpr int kMax = 16;
const int kConstInt = 8;
constexpr double kHalf = 0.5;
constexpr char kSep = '/';
constexpr const char* kName = "kname";
const char kArr[] = "arr";
int kMutable = 3;
const int kRuntime = next();
struct Pt { int x, y; };
constexpr Pt kPt{3, 4};
enum Color { RED, GREEN = 5, BLUE };
enum class Mode : unsigned char { A = 1, B = 2 };
enum : long long { HUGE_ENUM = 1LL << 40 };
constexpr Color kFav = GREEN;
struct Limits { static constexpr int kCap = 64; };
namespace geo { constexpr int kDim = 3; namespace deep { constexpr int kN = geo::kDim * 2; } }

// redefinition, #undef, conditional
#define REDEF 1
#undef REDEF
#define REDEF 2
#define UNDEF_LATER 9
#undef UNDEF_LATER
#ifdef __linux__
#define PLAT 1
#else
#define PLAT 2
#endif
#ifdef FROM_FLAG
#define FLAGGED FROM_FLAG
#else
#define FLAGGED 0
#endif

// macros whose value is the place or the time of the expansion: never constants of the program
#define WHERE __LINE__
#define WHICH __FILE__
#define WHICHNAME __FILE_NAME__
#define CNT __COUNTER__
#define STAMP __DATE__
#define CLOCK __TIME__
#define MODIFIED __TIMESTAMP__
#define BASE __BASE_FILE__
#define LEVEL __INCLUDE_LEVEL__
#define VIA WHERE
#define VIA2 (VIA + 1)
#define VIAFN(x) ((x) + __LINE__)
#define INSTR "mentions __LINE__ and __FILE__ in a string only"
#define NOT_VOLATILE_LOOKALIKE (__LINE__IS_NOT_A_TOKEN + 0)
// a global that a namespace member of the same name hides
constexpr int kDim = 99;
