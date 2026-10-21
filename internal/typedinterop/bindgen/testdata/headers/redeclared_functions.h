// A function declared again is one function; only a different type is an overload.
// glibc declares strtol twice in a row, the second time with an asm label.
int twice(int x);
int twice(int x) __asm__("twice_other_symbol");
int twice(int);

int pick(int x);
double pick(double x);
double pick(double x);

// Differences that Go does not show are no overload: const, the names of the parameters.
void qualified(int x);
void qualified(const int y);

// These are different overloads in C++, and the view lists each of them.
void a(int *p);
void a(const int *p);
void c(int &r);
void c(const int &r);
void k(char *s);
const char *k(const char *s);
void e(volatile int *p);
void e(int *p);
void i(int &r);
void i(int &&r);

// A top-level const on a parameter makes no overload.
void by_value(int x);
void by_value(const int x);
void by_pointer(int *const p);
void by_pointer(int *p);
