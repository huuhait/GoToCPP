// bench: spectralnorm (C++ twin of bench/programs/cpu/spectralnorm.gp)
#include <cmath>
#include <cstdint>
#include <cstdio>
#include <vector>

namespace {

double evalA(int64_t i, int64_t j) {
    return 1 / static_cast<double>((i + j) * (i + j + 1) / 2 + i + 1);
}

void times(std::vector<double>& out, const std::vector<double>& v) {
    for (size_t i = 0; i < out.size(); i++) {
        double sum = 0.0;
        for (size_t j = 0; j < v.size(); j++)
            sum += evalA(i, j) * v[j];
        out[i] = sum;
    }
}

void timesTransp(std::vector<double>& out, const std::vector<double>& v) {
    for (size_t i = 0; i < out.size(); i++) {
        double sum = 0.0;
        for (size_t j = 0; j < v.size(); j++)
            sum += evalA(j, i) * v[j];
        out[i] = sum;
    }
}

void timesAtA(std::vector<double>& out, const std::vector<double>& v, std::vector<double>& tmp) {
    times(tmp, v);
    timesTransp(out, tmp);
}

} // namespace

int main() {
    const size_t n = BENCH_N;
    std::vector<double> u(n, 1.0), v(n), tmp(n);
    for (int i = 0; i < 10; i++) {
        timesAtA(v, u, tmp);
        timesAtA(u, v, tmp);
    }
    double vBv = 0, vv = 0;
    for (size_t i = 0; i < n; i++) {
        vBv += u[i] * v[i];
        vv += v[i] * v[i];
    }
    std::printf("%.9f\n", std::sqrt(vBv / vv));
}
