// bench: struct_copy (C++ twin of bench/programs/data/struct_copy.gp)
#include <array>
#include <cstdint>
#include <cstdio>
#include <vector>

struct Vec4 {
    uint64_t x, y, z, w;
};

struct Body {
    Vec4 pos, vel;
    uint64_t id;
    std::array<uint64_t, 4> hist;
};

static Vec4 add(Vec4 a, Vec4 b) {
    return Vec4{a.x + b.x, a.y + b.y, a.z + b.z, a.w + b.w};
}

static Body step(Body b) {
    b.pos = add(b.pos, b.vel);
    b.hist[b.id & 3] = b.pos.x;
    b.id++;
    return b;
}

static uint64_t fold(std::array<Vec4, 16> g) {
    uint64_t s = 0;
    for (const Vec4& v : g)
        s += v.x ^ v.w;
    return s;
}

int main() {
    std::vector<Body> arr(1024);
    for (size_t i = 0; i < arr.size(); i++) {
        arr[i].pos = Vec4{0, 0, 0, 0};
        arr[i].vel = Vec4{i, i * 3, 5, 7};
        arr[i].id = i;
        arr[i].hist = {0, 0, 0, 0};
    }
    for (int64_t r = 0; r < BENCH_N; r++)
        for (size_t i = 0; i < arr.size(); i++)
            arr[i] = step(arr[i]);
    uint64_t sum = 0;
    for (const Body& b : arr)
        sum += b.pos.x + b.pos.y + b.hist[2] + b.id;

    std::array<Vec4, 16> grid{};
    uint64_t gsum = 0;
    for (int64_t r = 0; r < BENCH_N * 50; r++) {
        std::array<Vec4, 16> g2 = grid;
        g2[r & 15].x += static_cast<uint64_t>(r);
        g2[(r + 1) & 15].w ^= static_cast<uint64_t>(r) * 3;
        gsum += fold(g2);
        grid = g2;
    }
    std::printf("%llu %llu\n", static_cast<unsigned long long>(sum),
                static_cast<unsigned long long>(gsum));
}
