// The unit tests of the native functions of package internal/bytealg (gocpp/bytealg.hpp,
// runtime/src/bytealg.cpp, Phase 09 step 9.2): every function against a plain loop written here,
// on seeded random inputs of every length from 0 to 300 at every alignment, with nil and empty
// operands, and the two ways to count (words and the vectorizable loop) on their own, whichever one
// the build picks. runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++
// and clang++, at -O0, at -O2 and with ASan + UBSan, and runs it like test/unit.cpp. It prints "ok:
// N checks" and exits 0, or prints each failed check and exits 1.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <cstdint>
#include <string>
#include <string_view>
#include <utility>
#include <vector>

namespace {

int checks = 0;
int failures = 0;

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        std::string msg = std::string(file) + ":" + std::to_string(line) + ": " + what + "\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

// A generator that does not depend on the library (xorshift64*), so every run sees the same bytes.
struct rng {
    uint64_t s;
    uint64_t next() {
        s ^= s >> 12;
        s ^= s << 25;
        s ^= s >> 27;
        return s * 0x2545F4914F6CDD1DULL;
    }
    uint32_t below(uint32_t n) { return static_cast<uint32_t>(next() % n); }
};

// The naive versions.
int64_t naive_index(const uint8_t* p, int64_t n, uint8_t c) {
    for (int64_t i = 0; i < n; i++) {
        if (p[i] == c) {
            return i;
        }
    }
    return -1;
}
int64_t naive_last(const uint8_t* p, int64_t n, uint8_t c) {
    for (int64_t i = n - 1; i >= 0; i--) {
        if (p[i] == c) {
            return i;
        }
    }
    return -1;
}
int64_t naive_count(const uint8_t* p, int64_t n, uint8_t c) {
    int64_t k = 0;
    for (int64_t i = 0; i < n; i++) {
        k += p[i] == c;
    }
    return k;
}
int64_t naive_compare(const uint8_t* a, int64_t na, const uint8_t* b, int64_t nb) {
    for (int64_t i = 0; i < na && i < nb; i++) {
        if (a[i] != b[i]) {
            return a[i] < b[i] ? -1 : 1;
        }
    }
    return na < nb ? -1 : (na > nb ? 1 : 0);
}
int64_t naive_find(const uint8_t* a, int64_t na, const uint8_t* b, int64_t nb) {
    for (int64_t i = 0; i + nb <= na; i++) {
        int64_t j = 0;
        while (j < nb && a[i + j] == b[j]) {
            j++;
        }
        if (j == nb) {
            return i;
        }
    }
    return -1;
}

gp::slice<uint8_t> to_slice(const std::vector<uint8_t>& v) {
    gp::slice<uint8_t> s = gp::make_slice<uint8_t>(static_cast<int64_t>(v.size()));
    for (size_t i = 0; i < v.size(); i++) {
        s[static_cast<int64_t>(i)] = v[i];
    }
    return s;
}

gp::string to_string(const std::vector<uint8_t>& v) {
    return gp::string::gp_copy(std::string_view(reinterpret_cast<const char*>(v.data()), v.size()));
}

// Bytes from a small alphabet, so that the matches are common (and 255 and 0 appear).
std::vector<uint8_t> random_bytes(rng& r, int64_t n, uint32_t alphabet) {
    static const uint8_t pool[] = {0, 1, 'a', 'b', 'c', 0x7f, 0x80, 0xfe, 0xff, 'x'};
    std::vector<uint8_t> v(static_cast<size_t>(n));
    for (auto& b : v) {
        b = pool[r.below(alphabet)];
    }
    return v;
}

// The counting loops, on a buffer at each of eight alignments, for every length.
void test_count_ways() {
    rng r{0x9E3779B97F4A7C15ULL};
    std::vector<uint8_t> big(400);
    for (int64_t n = 0; n <= 300; n++) {
        for (size_t off = 0; off < 8; off++) {
            // The whole pool (10 values: 0, 0x7f, 0x80, 0xfe and 0xff among them) on every second
            // buffer, so bytes with the high bit set reach the word counter, and a byte that
            // differs from c only in its high bit (0x80 against 0) is seen as different.
            std::vector<uint8_t> v = random_bytes(r, n + 8, off % 2 == 0 ? 4 : 10);
            const uint8_t* p = v.data() + off;
            for (uint8_t c : {uint8_t(0), uint8_t(1), uint8_t('a'), uint8_t(0xff), uint8_t('z'),
                              uint8_t(0x80), uint8_t(0x7f), uint8_t(0xfe)}) {
                int64_t want = naive_count(p, n, c);
                CHECK(gp::bytealg::detail::count_words(p, n, c) == want);
                CHECK(gp::bytealg::detail::count_loop(p, n, c) == want);
                CHECK(gp::bytealg::detail::count_raw(p, n, c) == want);
            }
        }
    }
    // Long runs of one value: all bytes equal, the case where every word of the counter adds 8 per
    // step, and a length of a few thousand words so that the lanes of the sum are emptied in
    // between.
    for (int64_t n :
         {int64_t(255 * 8), int64_t(255 * 8 + 1), int64_t(255 * 8 * 2 + 7), int64_t(100000)}) {
        for (uint8_t fill : {uint8_t(0), uint8_t(0xff), uint8_t('q')}) {
            std::vector<uint8_t> v(static_cast<size_t>(n), fill);
            CHECK(gp::bytealg::detail::count_words(v.data(), n, fill) == n);
            CHECK(gp::bytealg::detail::count_loop(v.data(), n, fill) == n);
            CHECK(gp::bytealg::detail::count_words(v.data(), n, uint8_t(fill + 1)) == 0);
            CHECK(gp::bytealg::detail::count_loop(v.data(), n, uint8_t(fill + 1)) == 0);
        }
    }
    CHECK(gp::bytealg::detail::count_words(nullptr, 0, 'a') == 0);
    CHECK(gp::bytealg::detail::count_loop(nullptr, 0, 'a') == 0);
}

void test_single_byte_searches() {
    rng r{12345};
    for (int64_t n = 0; n <= 300; n++) {
        std::vector<uint8_t> v = random_bytes(r, n, 10);
        gp::slice<uint8_t> s = to_slice(v);
        gp::string str = to_string(v);
        for (uint8_t c : {uint8_t(0), uint8_t('a'), uint8_t('c'), uint8_t(0x80), uint8_t(0xff),
                          uint8_t('x'), uint8_t('q')}) {
            CHECK(gp::bytealg::index_byte(s, c) == naive_index(v.data(), n, c));
            CHECK(gp::bytealg::index_byte_string(str, c) == naive_index(v.data(), n, c));
            CHECK(gp::bytealg::last_index_byte(s, c) == naive_last(v.data(), n, c));
            CHECK(gp::bytealg::last_index_byte_string(str, c) == naive_last(v.data(), n, c));
            CHECK(gp::bytealg::count(s, c) == naive_count(v.data(), n, c));
            CHECK(gp::bytealg::count_string(str, c) == naive_count(v.data(), n, c));
        }
    }
    // Nil and empty operands: no function may pass a null pointer to memchr.
    gp::slice<uint8_t> nil;
    gp::string empty;
    CHECK(gp::bytealg::index_byte(nil, 'a') == -1);
    CHECK(gp::bytealg::index_byte_string(empty, 'a') == -1);
    CHECK(gp::bytealg::last_index_byte(nil, 0) == -1);
    CHECK(gp::bytealg::last_index_byte_string(empty, 0) == -1);
    CHECK(gp::bytealg::count(nil, 0) == 0);
    CHECK(gp::bytealg::count_string(empty, 0) == 0);
}

void test_compare_equal() {
    rng r{777};
    for (int64_t na = 0; na <= 70; na++) {
        for (int64_t nb : {int64_t(0), na, na + 1, na > 0 ? na - 1 : 0, int64_t(65)}) {
            for (int rep = 0; rep < 4; rep++) {
                std::vector<uint8_t> a = random_bytes(r, na, 3);
                std::vector<uint8_t> b = random_bytes(r, nb, 3);
                if (rep == 0) { // a prefix relation
                    b = a;
                    b.resize(static_cast<size_t>(nb), 'a');
                }
                if (rep == 1 && na == nb) {
                    b = a;
                }
                int64_t want = naive_compare(a.data(), na, b.data(), nb);
                gp::slice<uint8_t> sa = to_slice(a), sb = to_slice(b);
                CHECK(gp::bytealg::compare(sa, sb) == want);
                CHECK(gp::bytealg::compare_string(to_string(a), to_string(b)) == want);
                CHECK(gp::bytealg::equal(sa, sb) == (want == 0));
            }
        }
    }
    // Bytes above 0x7f compare as unsigned.
    CHECK(gp::bytealg::compare_string(gp::string::gp_copy(std::string_view("\x80", 1)),
                                      gp::string::gp_copy(std::string_view("\x7f", 1))) == 1);
    CHECK(gp::bytealg::compare_string(gp::string::gp_copy(std::string_view("\x7f", 1)),
                                      gp::string::gp_copy(std::string_view("\xff", 1))) == -1);
    gp::slice<uint8_t> nil;
    CHECK(gp::bytealg::compare(nil, nil) == 0);
    CHECK(gp::bytealg::equal(nil, gp::make_slice<uint8_t>(0)));
    CHECK(gp::bytealg::compare(nil, to_slice({1})) == -1);
    CHECK(gp::bytealg::compare(to_slice({1}), nil) == 1);
    CHECK(gp::bytealg::compare_string(gp::string(), gp::string()) == 0);
}

void test_index() {
    rng r{4242};
    for (int64_t na = 0; na <= 120; na++) {
        for (int64_t nb : {int64_t(0), int64_t(1), int64_t(2), int64_t(3), int64_t(7), int64_t(16),
                           int64_t(63), int64_t(64), int64_t(121)}) {
            for (int rep = 0; rep < 3; rep++) {
                std::vector<uint8_t> a = random_bytes(r, na, 2 + rep);
                std::vector<uint8_t> b = random_bytes(r, nb, 2 + rep);
                if (rep == 2 && nb <= na) { // a needle that is there
                    int64_t at = nb == na ? 0 : r.below(static_cast<uint32_t>(na - nb + 1));
                    b.assign(a.begin() + at, a.begin() + at + nb);
                }
                int64_t want = nb == 0 ? 0 : naive_find(a.data(), na, b.data(), nb);
                CHECK(gp::bytealg::index(to_slice(a), to_slice(b)) == want);
                CHECK(gp::bytealg::index_string(to_string(a), to_string(b)) == want);
            }
        }
    }
    CHECK(gp::bytealg::index_string(gp::string::gp_copy(std::string_view("abcabd")),
                                    gp::string::gp_copy(std::string_view("abd"))) == 3);
    CHECK(gp::bytealg::index_string(gp::string::gp_copy(std::string_view("aaaa")),
                                    gp::string::gp_copy(std::string_view("aaaaa"))) == -1);
    CHECK(gp::bytealg::index_string(gp::string(), gp::string()) == 0);
    gp::slice<uint8_t> nil;
    CHECK(gp::bytealg::index(nil, nil) == 0);
    CHECK(gp::bytealg::index(nil, to_slice({'a'})) == -1);
}

void test_make_no_zero() {
    // Go's capacity: the size class of n (runtime.roundupsize).
    const std::pair<int64_t, int64_t> sizes[] = {{0, 0},   {1, 8},       {7, 8},      {9, 16},
                                                 {33, 48}, {4096, 4096}, {5000, 5376}};
    for (auto [n, c] : sizes) {
        gp::slice<uint8_t> s = gp::bytealg::make_no_zero(n);
        CHECK(s.gp_len() == n);
        CHECK(s.gp_cap() == c);
        CHECK((n == 0) == (s.gp_len() == 0));
        for (int64_t i = 0; i < n; i++) { // the bytes are ours to write
            s[i] = static_cast<uint8_t>(i);
        }
        for (int64_t i = 0; i < n; i++) {
            CHECK(s[i] == static_cast<uint8_t>(i));
        }
    }
    auto text = [](int64_t n) {
        try {
            gp::bytealg::make_no_zero(n);
        } catch (const gp::panic_error& e) {
            return std::string(e.gp_text.gp_view());
        }
        return std::string("<no panic>");
    };
    CHECK(text(-1) == "runtime error: makeslice: len out of range");
    CHECK(text(int64_t(1) << 62) == "runtime error: makeslice: len out of range");
}

} // namespace

int main() {
    test_count_ways();
    test_single_byte_searches();
    test_compare_equal();
    test_index();
    test_make_no_zero();
    if (failures != 0) {
        return 1;
    }
    std::string msg = "ok: " + std::to_string(checks) + " checks\n";
    gp::os::write(gp::os::stdout_fd, msg.data(), msg.size());
    return 0;
}
