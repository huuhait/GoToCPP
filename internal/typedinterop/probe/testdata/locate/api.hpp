#pragma once
#include <cstdio>
#include <string>

// Group header
/// Adds two numbers.
/// Second line.
int add(int a, int b = 2);
int add(double a);  // the double overload

/* block doc */
struct Widget {
  int w;  ///< the width
  int size() const;
  int size(int x) const;
  static Widget make();
  virtual ~Widget();

 private:
  int hidden;
};

struct Derived : Widget {
  int own;
  void tick(const char *label = "x", ...);
};

namespace geo {
inline namespace v1 {
const int kMax = 7;  // largest size
typedef Widget W;
}
enum class Kind : unsigned char { A, B = 5 };
template <class T, int N = 3>
T pick(T first, int index = N);
}

enum Color { RED, GREEN = 5 };
typedef struct Vector2 {
  float x;  // horizontal
  float y;  // vertical
} Vector2;

#define RLAPI __attribute__((visibility("default")))
// Window-related functions
RLAPI void InitWindow(int width, int height, const char *title);  // Initialize window
RLAPI bool WindowShouldClose(void);                               // Check if closing
#define PI 3.14159265358979323846f  // Pi
// Maximum of two.
#define MAX(a, b) ((a) > (b) ? (a) : (b))
extern int counter;
using Name = std::string;
#define MAKE(n) int make_##n(int);
MAKE(one)
struct Sealed final { int x; };
