// bench: iface_call (C++ twin of bench/programs/dispatch/iface_call.gp)
#include <cstdint>
#include <cstdio>
#include <memory>
#include <vector>

struct Shape {
    virtual ~Shape() = default;
    virtual int64_t area() = 0;
    virtual void grow(int64_t k) = 0;
};
struct Square : Shape {
    int64_t s;
    explicit Square(int64_t s) : s(s) {}
    int64_t area() override { return s * s; }
    void grow(int64_t k) override { s += k & 1; }
};
struct Rect : Shape {
    int64_t w, h;
    Rect(int64_t w, int64_t h) : w(w), h(h) {}
    int64_t area() override { return w * h; }
    void grow(int64_t k) override { h += k & 1; }
};
struct Tri : Shape {
    int64_t b, h;
    Tri(int64_t b, int64_t h) : b(b), h(h) {}
    int64_t area() override { return b * h / 2; }
    void grow(int64_t k) override { b += k & 1; }
};

int main() {
    std::vector<std::unique_ptr<Shape>> shapes;
    shapes.push_back(std::make_unique<Square>(3));
    shapes.push_back(std::make_unique<Rect>(4, 5));
    shapes.push_back(std::make_unique<Tri>(6, 7));
    shapes.push_back(std::make_unique<Square>(8));
    shapes.push_back(std::make_unique<Rect>(2, 9));
    shapes.push_back(std::make_unique<Tri>(10, 3));
    shapes.push_back(std::make_unique<Square>(1));
    shapes.push_back(std::make_unique<Rect>(7, 7));
    int64_t idx[1024];
    uint32_t x = 1;
    for (auto& v : idx) {
        x = x * 1664525u + 1013904223u;
        v = (x >> 16) & 7;
    }
    uint64_t sum = 0;
    for (int64_t i = 0; i < BENCH_N; i++) {
        Shape* s = shapes[idx[i & 1023]].get();
        sum += static_cast<uint64_t>(s->area());
        if ((i & 255) == 0)
            s->grow(i);
    }
    std::printf("%llu\n", static_cast<unsigned long long>(sum));
}
