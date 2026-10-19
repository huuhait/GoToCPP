#pragma once
// The header of the checker's zoo: every declaration is inline, so the programs
// that use it also link and run (the accepted ones).
#include <cstddef>
#include <string>

struct Point {
  int x, y;
  int sum() const { return x + y; }
  void move(int dx) { x += dx; }
  static Point origin() { return Point{0, 0}; }
};
enum Color { Red, Green, Blue };
enum class Mode { Fast, Slow };

inline int area(Point p) { return p.x * p.y; }
inline int area(const Point &p, int scale) { return p.x * p.y * scale; }
inline void paint(Color c) { (void)c; }
inline void setmode(Mode m) { (void)m; }
inline void scale(Point &p, int k) { p.x *= k; p.y *= k; }
inline int count(const Point *p) { return p ? 1 : 0; }
inline void ovl(int) {}
inline void ovl(long) {}
inline void ovl2(long) {}
inline void ovl2(unsigned long) {}
inline int defaults(int a, int b = 2) { return a + b; }
inline double mix(double a, int b) { return a + b; }

struct Counter {
  int n;
  Counter(int v) : n(v) {}
  int get() const { return n; }
  void inc() { n++; }
  static int make() { return 1; }
  int &ref() { return n; }
};
inline const Counter &gcounter() {
  static Counter c(1);
  return c;
}
inline Counter make_counter() { return Counter(3); }
inline Point make_point() { return Point{1, 2}; }
inline const Point &cpoint() {
  static Point p{1, 2};
  return p;
}
inline void nothing() {}

inline int global_value = 7;
#define SIZE 10

inline int takes_str(const std::string &s) { return (int)s.size(); }
inline int takes_cstr(const char *s) { return s ? 1 : 0; }

struct Opaque;
inline Opaque *opaque_new() { return nullptr; }
inline int opaque_use(Opaque *o) { return o ? 1 : 0; }

struct NoCopy {
  NoCopy() = default;
  NoCopy(const NoCopy &) = delete;
  int v = 0;
};
inline void take_nocopy(NoCopy n) { (void)n; }
inline void take_nocopy_ref(NoCopy &n) { (void)n; }

struct Meters {
  double v = 2.5;
  operator double() const { return v; }
};
inline Meters make_meters() { return Meters{}; }

class Secret {
  int hidden = 1;
 public:
  int shown = 2;
};
inline NoCopy make_nocopy() { return NoCopy(); }
