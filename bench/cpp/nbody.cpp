// bench: nbody (C++ twin of bench/programs/cpu/nbody.gp)
#include <cmath>
#include <cstdio>
#include <vector>

namespace {

double pi = 3.141592653589793;
double solarMass = 4 * pi * pi;
double daysPerYear = 365.24;

struct Body {
    double x, y, z, vx, vy, vz, mass;
};

std::vector<Body> newBodies() {
    return {
        {0, 0, 0, 0, 0, 0, solarMass},
        {4.84143144246472090e+00, -1.16032004402742839e+00, -1.03622044471123109e-01,
         1.66007664274403694e-03 * daysPerYear, 7.69901118419740425e-03 * daysPerYear,
         -6.90460016972063023e-05 * daysPerYear, 9.54791938424326609e-04 * solarMass},
        {8.34336671824457987e+00, 4.12479856412430479e+00, -4.03523417114321381e-01,
         -2.76742510726862411e-03 * daysPerYear, 4.99852801234917238e-03 * daysPerYear,
         2.30417297573763929e-05 * daysPerYear, 2.85885980666130812e-04 * solarMass},
        {1.28943695621391310e+01, -1.51111514016986312e+01, -2.23307578892655734e-01,
         2.96460137564761618e-03 * daysPerYear, 2.37847173959480950e-03 * daysPerYear,
         -2.96589568540237556e-05 * daysPerYear, 4.36624404335156298e-05 * solarMass},
        {1.53796971148509165e+01, -2.59193146099879641e+01, 1.79258772950371181e-01,
         2.68067772490389322e-03 * daysPerYear, 1.62824170038242295e-03 * daysPerYear,
         -9.51592254519715870e-05 * daysPerYear, 5.15138902046611451e-05 * solarMass},
    };
}

void offsetMomentum(std::vector<Body>& b) {
    double px = 0, py = 0, pz = 0;
    for (const auto& p : b) {
        px += p.vx * p.mass;
        py += p.vy * p.mass;
        pz += p.vz * p.mass;
    }
    b[0].vx = -px / solarMass;
    b[0].vy = -py / solarMass;
    b[0].vz = -pz / solarMass;
}

double energy(const std::vector<Body>& b) {
    double e = 0.0;
    for (size_t i = 0; i < b.size(); i++) {
        e += 0.5 * b[i].mass * (b[i].vx * b[i].vx + b[i].vy * b[i].vy + b[i].vz * b[i].vz);
        for (size_t j = i + 1; j < b.size(); j++) {
            double dx = b[i].x - b[j].x;
            double dy = b[i].y - b[j].y;
            double dz = b[i].z - b[j].z;
            e -= b[i].mass * b[j].mass / std::sqrt(dx * dx + dy * dy + dz * dz);
        }
    }
    return e;
}

void advance(std::vector<Body>& b, double dt) {
    size_t n = b.size();
    for (size_t i = 0; i < n; i++) {
        for (size_t j = i + 1; j < n; j++) {
            double dx = b[i].x - b[j].x;
            double dy = b[i].y - b[j].y;
            double dz = b[i].z - b[j].z;
            double dSq = dx * dx + dy * dy + dz * dz;
            double dist = std::sqrt(dSq);
            double mag = dt / (dSq * dist);
            b[i].vx -= dx * b[j].mass * mag;
            b[i].vy -= dy * b[j].mass * mag;
            b[i].vz -= dz * b[j].mass * mag;
            b[j].vx += dx * b[i].mass * mag;
            b[j].vy += dy * b[i].mass * mag;
            b[j].vz += dz * b[i].mass * mag;
        }
    }
    for (auto& p : b) {
        p.x += dt * p.vx;
        p.y += dt * p.vy;
        p.z += dt * p.vz;
    }
}

} // namespace

int main() {
    auto bodies = newBodies();
    offsetMomentum(bodies);
    std::printf("%.9f\n", energy(bodies));
    for (long long i = 0; i < BENCH_N; i++)
        advance(bodies, 0.01);
    std::printf("%.9f\n", energy(bodies));
}
