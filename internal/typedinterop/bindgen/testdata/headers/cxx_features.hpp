// C++: namespaces, classes, templates, references, overloads, constants.
#pragma once
#include <cstddef>
#include <string>

namespace geo {

struct Vec {
    double x = 0, y = 0;
    Vec() = default;
    Vec(double x, double y);
    double length() const;
    void scale(double k);
    Vec operator+(const Vec &o) const;
    static Vec zero();
    static int count;
    explicit operator bool() const;
private:
    int hidden_;
protected:
    int guarded_;
};

class Shape {
public:
    virtual ~Shape();
    virtual double area() const = 0;
    virtual void move(Vec by);
    void move(double dx, double dy);
    Vec &position();
    const std::string &name() const;
    enum Kind { Circle, Square };
    struct Style { int width; bool dashed; };
    typedef int Id;
    Id id;
    Style style;
    Kind kind;
private:
    int secret;
};

class Circle : public Shape {
public:
    double radius;
    double area() const override;
};

namespace detail {
int helper(int);
constexpr int kLimit = 64;
}

inline namespace v2 {
int version();
}

namespace {
int hidden_in_anonymous_namespace();
}

namespace _private_ns {
int in_private_ns(int);
}

enum class Level : unsigned char { Low = 1, Mid, High = 10 };
enum Plain { P_ONE = 1, P_TWO };

double dot(const Vec &a, const Vec &b);
double dot(const Vec &a, const Vec &b, double w);
Vec &self(Vec &v);
int sum(const int *values, std::size_t n);
void greet(std::string name);
int defaulted(int a, int b = 2);
Vec operator*(const Vec &a, double k);

struct Derived : private Vec { int d; };
struct Open : public Vec, protected Circle { int o; };
int deleted_fn(int) = delete;

template <class T> struct Box { T value; };
template <class T, int N> T pick(const T (&values)[N], int i);
template <class T> using Vector = Box<T>;

using Id = unsigned;
typedef Vec Point;
constexpr int kMax = 3 * 4;
const double kPi = 3.14159;
constexpr bool kFlag = true;
extern Vec origin;
extern int counter;

}  // namespace geo

namespace outer::inner {
struct Deep { int d; };
int deep_fn(Deep);
}

int global_fn(geo::Vec v, outer::inner::Deep d);
