// C++ classes for cxx_inheritance.gp and cxx_features.gp (step 4.18): virtual functions and
// inheritance, static members, overloads, default arguments, references, operators, enums, a
// class template and a type with begin()/end().
#pragma once

#include <memory>
#include <string>
#include <vector>

namespace oop {

struct Shape {
    static inline int live = 0;
    int id = 0;
    Shape() { live++; }
    Shape(const Shape&) { live++; }
    virtual ~Shape() { live--; }
    virtual double area() const = 0;
    virtual std::string name() const { return "shape"; }
    std::string describe() const { return name() + ":" + std::to_string(static_cast<int>(area())); }
};

struct Rect : Shape {
    double w, h;
    Rect(double w_, double h_) : w(w_), h(h_) {}
    double area() const override { return w * h; }
    std::string name() const override { return "rect"; }
};

struct Square : Rect {
    explicit Square(double s) : Rect(s, s) {}
    std::string name() const override { return "square"; }
};

struct Circle : Shape {
    double r;
    explicit Circle(double r_) : r(r_) {}
    double area() const override { return 3 * r * r; }
};

inline std::unique_ptr<Shape> make_shape(int kind) {
    if (kind == 0) {
        return std::make_unique<Rect>(2, 3);
    }
    if (kind == 1) {
        return std::make_unique<Square>(4);
    }
    return std::make_unique<Circle>(2);
}

inline double area_of(const Shape& s) { return s.area(); }
inline std::string name_of(const Shape* s) { return s->name(); }
inline double total_area(const std::vector<std::shared_ptr<Shape>>& v) {
    double t = 0;
    for (const auto& s : v) {
        t += s->area();
    }
    return t;
}

// Overloads, default arguments and references.
inline int scale(int x) { return x * 2; }
inline double scale(double x) { return x * 1.5; }
inline std::string scale(const std::string& s) { return s + s; }
inline int volume(int w, int h = 2, int d = 3) { return w * h * d; }
inline void divmod(int a, int b, int& q, int& r) {
    q = a / b;
    r = a % b;
}
inline int& pick(int& a, int& b, bool first) { return first ? a : b; }
inline int sum_all(const std::vector<int>& v) {
    int t = 0;
    for (int x : v) {
        t += x;
    }
    return t;
}

// Enums.
enum class Color { Red, Green = 5, Blue };
enum Level { Low = 1, Mid, High = 10 };
inline const char* color_name(Color c) {
    switch (c) {
    case Color::Red:
        return "red";
    case Color::Green:
        return "green";
    default:
        return "blue";
    }
}

// A value type with operators.
struct Vec2 {
    int x = 0, y = 0;
    Vec2() = default;
    Vec2(int x_, int y_) : x(x_), y(y_) {}
    Vec2 operator+(const Vec2& o) const { return {x + o.x, y + o.y}; }
    Vec2 operator-() const { return {-x, -y}; }
    Vec2& operator+=(const Vec2& o) {
        x += o.x;
        y += o.y;
        return *this;
    }
    bool operator==(const Vec2& o) const { return x == o.x && y == o.y; }
    bool operator<(const Vec2& o) const { return x < o.x || (x == o.x && y < o.y); }
    int operator[](int i) const { return i == 0 ? x : y; }
    int operator()(int k) const { return (x + y) * k; }
};

// A class template and a type that C++ can iterate.
template <typename T>
class Stack {
public:
    void push(T v) { items_.push_back(std::move(v)); }
    T pop() {
        T v = std::move(items_.back());
        items_.pop_back();
        return v;
    }
    size_t size() const { return items_.size(); }
    auto begin() const { return items_.begin(); }
    auto end() const { return items_.end(); }
    static constexpr int kind = 7;

private:
    std::vector<T> items_;
};

template <typename T>
T twice(T x) {
    return x + x;
}

struct Ring {
    int items[4] = {10, 20, 30, 40};
    const int* begin() const { return items; }
    const int* end() const { return items + 4; }
};

}  // namespace oop
