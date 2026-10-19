// The C++ twin of designated_order.gp: the expected output is what this program prints.
#include <cstdio>
#include <vector>
#include "../order.hpp"

int main() {
    Rect r{.x = 1, .y = 5, .w = 640, .h = 480};
    std::printf("%d %d %d %d\n", r.x, r.y, r.w, r.h);

    Config c{.id = 42, .arr = {4, 5, 6}, .in = Inner{.a = 1, .b = 2}, .mode = 5, .scale = 2.5};
    std::printf("%d %s %d %d %d %d %d %d %d %g\n", c.id, c.name, c.arr[0], c.arr[1], c.arr[2], c.in.a, c.in.b,
                (int)c.flags, (int)c.mode, c.scale);

    Der d{.q = 20, .r = 30};
    std::printf("%d %d %d\n", d.p, d.q, d.r);

    Num n{.f = 1.5f};
    std::printf("%g\n", n.f);

    Anon a{.tag = 1, .i = 7, .u = 8, .v = 9};
    std::printf("%d %d %d %d\n", a.tag, a.i, a.u, a.v);

    Anon u{};
    u.tag = 2;
    u.f = 2.5f;
    u.i = 7;
    std::printf("%d %d\n", u.tag, u.i);

    std::vector<int> trace;
    auto step = [&](int n) { trace.push_back(n); return n; };
    int h = step(1), x = step(2), w = step(3), y = step(4);
    Rect s{.x = x, .y = y, .w = w, .h = h};
    std::printf("%d %d %d %d [", s.x, s.y, s.w, s.h);
    for (size_t i = 0; i < trace.size(); i++) std::printf(i ? " %d" : "%d", trace[i]);
    std::printf("]\n");

    std::vector<int> two;
    auto note = [&](int n) { two.push_back(n); return n; };
    int w2h = note(1), w2x = note(2);
    Rect w2{.x = w2x, .h = w2h};
    std::printf("%d %d [%d %d]\n", w2.x, w2.h, two[0], two[1]);

    int cid = 0, c0 = add(1, 2), c2 = mul(2, 2), ia = mul(3, 3), ib = add(1, 1);
    cid = add(5, 6);
    Cfg cfg{.id = cid, .arr = {c0, 3, c2}, .in = Inner{.a = ia, .b = ib}};
    std::printf("%d %d %d %d %d %d\n", cfg.id, cfg.arr[0], cfg.arr[1], cfg.arr[2], cfg.in.a, cfg.in.b);

    Ctor ct;
    ct.b = 2;
    std::printf("%d %d\n", ct.a, ct.b);

    long k = 70;
    unsigned char small = 200;
    Rect t{.x = (int)small, .w = 3, .h = (int)k};
    std::printf("%d %d %d %d\n", t.x, t.y, t.w, t.h);
}
