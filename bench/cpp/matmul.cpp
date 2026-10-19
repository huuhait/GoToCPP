// bench: matmul (C++ twin of bench/programs/cpu/matmul.gp)
#include <cstdint>
#include <cstdio>
#include <vector>

int main() {
    const int64_t n = BENCH_N;
    using Row = std::vector<double>;
    std::vector<Row> a(n, Row(n)), b(n, Row(n)), c(n, Row(n));
    for (int64_t i = 0; i < n; i++)
        for (int64_t j = 0; j < n; j++) {
            a[i][j] = double((i * j) % 7) + 0.5;
            b[i][j] = double((i + j) % 5) - 1.25;
        }
    for (int64_t i = 0; i < n; i++) {
        Row& ci = c[i];
        for (int64_t k = 0; k < n; k++) {
            const double aik = a[i][k];
            const Row& bk = b[k];
            for (int64_t j = 0; j < n; j++)
                ci[j] += aik * bk[j];
        }
    }
    double trace = 0, sum = 0;
    for (int64_t i = 0; i < n; i++) {
        trace += c[i][i];
        for (int64_t j = 0; j < n; j++)
            sum += c[i][j];
    }
    std::printf("%.6f %.6f\n", trace, sum);
}
