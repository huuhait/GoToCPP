// bench: mandelbrot (C++ twin of bench/programs/cpu/mandelbrot.gp)
#include <cstdint>
#include <cstdio>

int main() {
    const int64_t n = BENCH_N;
    int64_t inside = 0, total = 0;
    for (int64_t y = 0; y < n; y++) {
        double ci = 2.0 * static_cast<double>(y) / static_cast<double>(n) - 1.0;
        for (int64_t x = 0; x < n; x++) {
            double cr = 2.0 * static_cast<double>(x) / static_cast<double>(n) - 1.5;
            double zr = 0.0, zi = 0.0;
            int64_t i = 0;
            for (; i < 50 && zr * zr + zi * zi <= 4.0; i++) {
                double nzr = zr * zr - zi * zi + cr;
                double nzi = 2.0 * zr * zi + ci;
                zr = nzr;
                zi = nzi;
            }
            total += i;
            if (i == 50)
                inside++;
        }
    }
    std::printf("%lld %lld\n", static_cast<long long>(inside), static_cast<long long>(total));
}
