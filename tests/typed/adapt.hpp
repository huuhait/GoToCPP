// Functions for the argument adaptations of typed interop (ADR-130): adapt_numbers.gp,
// adapt_enum.gp, adapt_wide.gp, adapt_voidout.gp and the err_adapt_*.gp programs.
#pragma once
#include <cstdarg>
#include <cstddef>
#include <cstdio>
#include <cwchar>
#include <string>

// ---- numbers behind a pointer
inline int bump(int* value) {
    *value += 1;
    return *value;
}

inline void set_big(long long* value) {
    *value = 1LL << 40;
}

inline int read_int(const int* value) {
    return *value;
}

inline double halve(const double* value) {
    return *value / 2;
}

inline unsigned long add_one(unsigned long* value) {
    *value += 1;
    return *value;
}

inline void scale(double* value) {
    *value *= 2;
}

inline void store_short(short* value) {
    *value = 12;
}

// ---- enums
enum Mode { MODE_A = 1, MODE_B = 2, MODE_C = 4 };
enum class Level : unsigned char { Low = 0, Mid = 5, High = 9 };

inline int mode_value(Mode mode) {
    return (int)mode;
}

inline int level_value(Level level) {
    return (int)level;
}

// ---- wide strings
inline int wide_length(const wchar_t* text) {
    return (int)std::wcslen(text);
}

inline int unit16_length(const char16_t* text) {
    int n = 0;
    while (text[n] != 0) {
        n++;
    }
    return n;
}

inline void set_title_static(const wchar_t* title) {
    (void)title;
}

// ---- void**, void*
struct Obj {
    int v;
};

inline void make_object(void** out) {
    static Obj object{7};
    *out = &object;
}

inline void make_nothing(void** out) {
    *out = nullptr;
}

inline int object_value(const void* object) {
    return object == nullptr ? -1 : static_cast<const Obj*>(object)->v;
}

inline int mutable_value(void* object) {
    return object == nullptr ? -1 : static_cast<Obj*>(object)->v;
}

// ---- a printf-like function
inline int say(const char* format, ...) __attribute__((format(printf, 1, 2)));
inline int say(const char* format, ...) {
    char buffer[128];
    va_list args;
    va_start(args, format);
    int n = std::vsnprintf(buffer, sizeof buffer, format, args);
    va_end(args);
    return n;
}

// ---- overloads
inline int pick(int*) {
    return 1;
}

inline int pick(long long*) {
    return 2;
}

// ---- numbers by value
inline int take_char(char value) {
    return (int)value;
}

inline int take_uchar(unsigned char value) {
    return (int)value;
}

inline long long take_int(int value) {
    return value;
}

inline long long take_uint(unsigned int value) {
    return value;
}

inline long long take_longlong(long long value) {
    return value;
}

inline double take_float(float value) {
    return value;
}

inline double take_double(double value) {
    return value;
}

inline bool take_bool(bool value) {
    return value;
}

inline void flip(bool* value) {
    *value = !*value;
}

// ---- parameters written as arrays (the callee takes several elements) and pointers with defaults
inline float mix_color(const char* label, float color[3]) {
    (void)label;
    return color[0] + color[1] + color[2];
}

inline void drag_pair(int values[2]) {
    values[0] += 1;
    values[1] += 1;
}

inline int sum_three(const int values[3]) {
    return values[0] + values[1] + values[2];
}

inline void fill_slots(void* slots[2]) {
    slots[0] = nullptr;
    slots[1] = nullptr;
}

struct Palette {
    float first(float rgba[4]) { return rgba[0]; }

    int count(int* counter = nullptr) {
        if (counter != nullptr) {
            *counter += 5;
        }
        return 1;
    }
};

inline int count_up(const char* name, int* counter = nullptr, int flags = 0) {
    (void)name;
    (void)flags;
    if (counter != nullptr) {
        *counter += 1;
        return *counter;
    }
    return 0;
}

// ---- out-parameters read again in the same expression
inline int store_five(int* value) {
    *value = 5;
    return 0;
}

inline int next_value(int* value) {
    *value += 1;
    return *value;
}

inline int bump_by(int* value, int by) {
    *value += by;
    return *value;
}

// ---- objects that keep a wide string, and a printf-like function with the format second
struct Window {
    const wchar_t* title = nullptr;

    void set(const wchar_t* text) { title = text; }

    int length() const { return title == nullptr ? 0 : (int)std::wcslen(title); }
};

inline int say_at(int level, const char* format, ...) __attribute__((format(printf, 2, 3)));
inline int say_at(int level, const char* format, ...) {
    char buffer[128];
    va_list args;
    va_start(args, format);
    int n = std::vsnprintf(buffer, sizeof buffer, format, args);
    va_end(args);
    return n + level;
}

// ---- parameters written with a typedef: the typedef of an array is an array, the typedef of a pointer a pointer
typedef int IVec[3];
typedef float Vec3[3];
typedef float Mat4[4][4];
typedef int* IntPtr;

inline int tdef(IVec values) {
    int sum = values[0] + values[1] + values[2];
    values[2] = 40;
    return sum;
}

inline int tdef_c(const IVec values) {
    return values[0] + values[1] + values[2];
}

inline float tdef_matrix(Mat4 m) {
    return m[0][0] + m[3][3];
}

inline int bump_typedef(IntPtr counter) {
    *counter += 7;
    return *counter;
}

struct Rack {
    int total(IVec values) { return values[0] + values[1] + values[2]; }
};

// ---- the arguments of a call are all evaluated before the value of an adapted &x is read
inline int show_two(int* first, int* second, int third) {
    int shown = 100 * *first + 10 * *second + third;
    *first += 10;
    *second += 10;
    return shown;
}

inline int add_to(int* total, int amount) {
    *total += amount;
    return *total;
}

struct Meter {
    int level = 0;

    int raise(int* gauge, int by) {
        *gauge += by;
        return *gauge + level;
    }
};

// ---- an object that keeps a UTF-16 string (char16_t has no C.WString)
struct Label16 {
    const char16_t* text = nullptr;

    void set(const char16_t* value) { text = value; }
};
