// The runtime's C++ unit tests. runtime/cxx_test.go compiles this file with the runtime's
// .cpp files, with g++ and clang++, at -O0, at -O2 and with ASan + UBSan, and runs it as
//
//     unit arg1 "arg two"     (with GOCPP_RT_TEST=env-ok in the environment)
//
// It prints "ok: N checks" and exits 0, or prints each failed check and exits 1. The expected
// values come from real Go 1.25 on amd64 (runtime/testdata/diff/main.go prints many more,
// and cxx_test.go compares them with test/diff.cpp).
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <bit>
#include <cmath>
#include <concepts>
#include <cstdint>
#include <cstdio>
#include <exception>
#include <limits>
#include <memory>
#include <string>
#include <string_view>
#include <type_traits>
#include <unordered_set>
#include <utility>
#include <vector>

namespace {

int checks = 0;
int failures = 0;

template <typename T>
std::string show(const T& v) {
    if constexpr (std::is_same_v<T, bool>) {
        return v ? "true" : "false";
    } else if constexpr (std::is_integral_v<T> && std::is_signed_v<T>) {
        return std::to_string(static_cast<long long>(v));
    } else if constexpr (std::is_integral_v<T>) {
        return std::to_string(static_cast<unsigned long long>(v));
    } else if constexpr (std::is_same_v<T, gp::string>) {
        return '"' + std::string(v.gp_view()) + '"';
    } else {
        return '"' + std::string(v) + '"';
    }
}

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        std::string msg = std::string(file) + ":" + std::to_string(line) + ": " + what + "\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

// CHECK_EQ also checks that both sides have the same type, so a helper that returns the
// wrong Go type fails.
#define CHECK_EQ(got, want)                                                                        \
    do {                                                                                           \
        auto gp_got = (got);                                                                       \
        decltype(gp_got) gp_want = (want);                                                         \
        report(gp_got == gp_want, __FILE__, __LINE__,                                              \
               #got " = " + show(gp_got) + ", want " + show(gp_want));                             \
    } while (0)

#define CHECK_TYPE(expr, T)                                                                        \
    report(std::is_same_v<decltype(expr), T>, __FILE__, __LINE__, #expr " doesn't have type " #T)

// panic_text runs f and returns the text of the panic it throws, or "<no panic>".
template <typename F>
std::string panic_text(F f) {
    try {
        f();
    } catch (const gp::panic_error& e) {
        return std::string(e.gp_text.gp_view());
    }
    return "<no panic>";
}

#define CHECK_PANIC(stmt, want) CHECK_EQ(panic_text([&] { stmt; }), std::string(want))

const char* const divide_msg = "runtime error: integer divide by zero";
const char* const shift_msg = "runtime error: negative shift amount";

// Values the compiler can't fold, so the helpers really run.
template <typename T>
T opaque(T v) {
    static volatile T sink;
    sink = v;
    return sink;
}

// --- strings ---

void test_string() {
    gp::string empty;
    CHECK_EQ(gp::len(empty), INT64_C(0));
    CHECK(empty == ""_gs);
    CHECK_EQ(gp::len("hello"_gs), INT64_C(5));
    CHECK_EQ(gp::len("a\000b"_gs), INT64_C(3)); // a NUL inside a literal counts
    CHECK_EQ(gp::len(""_gs), INT64_C(0));
    CHECK_TYPE(gp::len(empty), int64_t);
    CHECK(sizeof(gp::string) == 32);

    // A literal points at the literal's storage: no copy.
    const char* lit = "static";
    gp::string s(gp::static_bytes, lit, 6);
    CHECK(s.gp_data() == lit);
    CHECK(s == "static"_gs);

    // Concatenation.
    gp::string ab = "ab"_gs + "cd"_gs;
    CHECK(ab == "abcd"_gs);
    CHECK_EQ(gp::len(ab), INT64_C(4));
    CHECK(("ab"_gs + ""_gs).gp_data() == ("ab"_gs).gp_data()); // an empty side shares the other
    CHECK((""_gs + "cd"_gs) == "cd"_gs);
    CHECK((""_gs + ""_gs) == ""_gs);
    gp::string acc = "x"_gs;
    for (int i = 0; i < 3; i++) {
        acc += "yz"_gs;
    }
    CHECK(acc == "xyzyzyz"_gs);
    gp::string nul = "a\000"_gs + "\000b"_gs;
    CHECK_EQ(gp::len(nul), INT64_C(4));
    CHECK_EQ(nul[1], uint8_t(0));
    CHECK_EQ(nul[3], uint8_t('b'));
    {
        // A concatenation owns its bytes: they outlive the strings it was made from.
        gp::string keep;
        {
            gp::string a = gp::string::gp_copy("left");
            gp::string b = gp::string::gp_copy("right");
            keep = a + b;
        }
        CHECK(keep == "leftright"_gs);
        gp::string copy = keep;
        CHECK(copy.gp_data() == keep.gp_data());
    }
    CHECK(gp::string::gp_copy("") == ""_gs);
    CHECK(gp::string::gp_copy("abc") == "abc"_gs);

    // Comparison: byte by byte, unsigned.
    CHECK("abc"_gs == "abc"_gs);
    CHECK("abc"_gs != "abd"_gs);
    CHECK("ab"_gs != "abc"_gs);
    CHECK("abc"_gs < "abd"_gs);
    CHECK("ab"_gs < "abc"_gs);
    CHECK(""_gs < "a"_gs);
    CHECK("b"_gs > "abc"_gs);
    CHECK("\xff"_gs > "a"_gs); // 0xff is a big unsigned byte, not -1
    CHECK("a\000"_gs > "a"_gs);
    CHECK(("abc"_gs <=> "abc"_gs) == std::strong_ordering::equal);
    CHECK(("abc"_gs <=> "abd"_gs) == std::strong_ordering::less);
    CHECK(("abd"_gs <=> "abc"_gs) == std::strong_ordering::greater);
    CHECK("abc"_gs <= "abc"_gs && "abc"_gs >= "abc"_gs);

    // Index: a byte, with Go's bounds check.
    gp::string abc = "abc"_gs;
    CHECK_EQ(abc[0], uint8_t('a'));
    CHECK_EQ(abc[INT64_C(2)], uint8_t('c'));
    CHECK_EQ(abc[uint8_t(1)], uint8_t('b'));
    CHECK_EQ("\xe9"_gs[0], uint8_t(0xe9));
    CHECK_TYPE(abc[0], uint8_t);
    CHECK_PANIC(abc[opaque(INT64_C(5))], "runtime error: index out of range [5] with length 3");
    CHECK_PANIC(abc[opaque(INT64_C(3))], "runtime error: index out of range [3] with length 3");
    CHECK_PANIC(abc[opaque(INT64_C(-1))], "runtime error: index out of range [-1]");
    CHECK_PANIC(abc[opaque(int8_t(-128))], "runtime error: index out of range [-128]");
    CHECK_PANIC(abc[opaque(UINT64_MAX)],
                "runtime error: index out of range [18446744073709551615] with length 3");
    CHECK_PANIC(abc[opaque(uint32_t(3))], "runtime error: index out of range [3] with length 3");
    CHECK_PANIC(empty[opaque(0)], "runtime error: index out of range [0] with length 0");
    CHECK_PANIC(abc[opaque(INT64_MIN)], "runtime error: index out of range [-9223372036854775808]");
}

// --- strings: substrings, concatenation, UTF-8, string(r) (step 3.4) ---

// runes ranges over s like `for i, r := range s` and writes "i:r " for each rune.
std::string runes(const gp::string& s) {
    std::string out;
    for (auto it = gp::range_string(s); gp::range_next(it);) {
        out += std::to_string(it.gp_i) + ":" + std::to_string(it.gp_r) + " ";
    }
    return out;
}

void test_substring() {
    gp::string hello = gp::string::gp_copy("hello, world");
    gp::string w = gp::sub(hello, 7, 12);
    CHECK(w == "world"_gs);
    CHECK(w.gp_data() == hello.gp_data() + 7); // shares the bytes
    CHECK(gp::sub(hello, 7) == "world"_gs);
    CHECK(gp::sub(hello, 0, 5) == "hello"_gs);
    CHECK(gp::sub(hello, 0) == hello);
    CHECK(gp::sub(hello, 12) == ""_gs);
    CHECK(gp::sub(hello, uint8_t(3), INT64_C(3)) == ""_gs);
    CHECK(gp::sub(hello, 3, 3).gp_data() == nullptr); // an empty substring keeps nothing alive
    CHECK_EQ(gp::len(gp::sub(hello, int8_t(1), uint64_t(4))), INT64_C(3));
    CHECK(gp::sub(""_gs, 0, 0) == ""_gs);
    CHECK(gp::sub(""_gs, 0) == ""_gs);
    {
        // A substring keeps the bytes alive after the string it came from is gone.
        gp::string keep;
        {
            gp::string a = gp::string::gp_copy("abcdef");
            keep = gp::sub(a, 2, 4);
        }
        CHECK(keep == "cd"_gs);
        CHECK(gp::sub(keep, 1) == "d"_gs);
    }
    // A substring of a literal points into the literal, and has no owner of its own.
    gp::string lit = "literal"_gs;
    CHECK(gp::sub(lit, 3).gp_data() == lit.gp_data() + 3);

    // Go's messages, and Go's order of the checks: hi <= len first, then lo <= hi.
    gp::string abc = "abc"_gs;
    const char* pre = "runtime error: slice bounds out of range ";
    auto msg = [pre](const char* m) { return std::string(pre) + m; };
    CHECK_PANIC(gp::sub(abc, opaque(0), opaque(5)), msg("[:5] with length 3"));
    CHECK_PANIC(gp::sub(abc, opaque(4), opaque(5)), msg("[:5] with length 3"));
    CHECK_PANIC(gp::sub(abc, opaque(3), opaque(2)), msg("[3:2]"));
    CHECK_PANIC(gp::sub(abc, opaque(-1), opaque(2)), msg("[-1:]"));
    CHECK_PANIC(gp::sub(abc, opaque(0), opaque(-1)), msg("[:-1]"));
    CHECK_PANIC(gp::sub(abc, opaque(4)), msg("[4:3]"));
    CHECK_PANIC(gp::sub(abc, opaque(-2)), msg("[-2:]"));
    CHECK_PANIC(gp::sub(abc, opaque(UINT64_MAX)), msg("[18446744073709551615:3]"));
    CHECK_PANIC(gp::sub(abc, opaque(0), opaque(UINT64_MAX)),
                msg("[:18446744073709551615] with length 3"));
    CHECK_PANIC(gp::sub(abc, opaque(uint8_t(3)), opaque(int8_t(1))), msg("[3:1]"));
    CHECK_PANIC(gp::sub(abc, opaque(INT64_MIN), opaque(1)), msg("[-9223372036854775808:]"));
    CHECK_PANIC(gp::sub(""_gs, opaque(1)), msg("[1:0]"));
    CHECK_PANIC(gp::sub(abc, opaque(1), opaque(INT64_C(1) << 40)),
                msg("[:1099511627776] with length 3"));
    CHECK_EQ(gp::sub(abc, opaque(3), opaque(3)), ""_gs);
}

void test_concat() {
    gp::string a = gp::string::gp_copy("ab"), e;
    CHECK(gp::concat(a, "cd"_gs, "e"_gs) == "abcde"_gs);
    CHECK(gp::concat(e, e, e) == ""_gs);
    CHECK(gp::concat(e, a, e).gp_data() == a.gp_data()); // one non-empty part: no copy
    CHECK(gp::concat(""_gs, "x"_gs, ""_gs, "y"_gs) == "xy"_gs);
    CHECK(gp::concat("a\000"_gs, "\000"_gs, "b"_gs) == "a\000\000b"_gs);
    CHECK_EQ(gp::len(gp::concat(a, a, a, a)), INT64_C(8));
    gp::string s;
    for (int i = 0; i < 100; i++) {
        s = gp::concat(s, "x"_gs, "y"_gs);
    }
    CHECK_EQ(gp::len(s), INT64_C(200));
    CHECK(gp::sub(s, 198) == "xy"_gs);
}

void test_utf8() {
    int64_t w = 0;
    auto dec = [&w](std::string_view b) {
        int32_t r = gp::detail::decode_rune(b.data(), static_cast<int64_t>(b.size()), w);
        return std::to_string(r) + "/" + std::to_string(w);
    };
    // The cases of Go's utf8 tests: valid forms of each length, the edges of each range, and
    // every way a sequence can be invalid.
    CHECK_EQ(dec("a"), std::string("97/1"));
    CHECK_EQ(dec(std::string_view("\0", 1)), std::string("0/1"));
    CHECK_EQ(dec("\x7f"), std::string("127/1"));
    CHECK_EQ(dec("\xc2\x80"), std::string("128/2"));
    CHECK_EQ(dec("\xdf\xbf"), std::string("2047/2"));
    CHECK_EQ(dec("\xe0\xa0\x80"), std::string("2048/3"));
    CHECK_EQ(dec("\xe2\x82\xac"), std::string("8364/3"));  // €
    CHECK_EQ(dec("\xed\x9f\xbf"), std::string("55295/3")); // U+D7FF
    CHECK_EQ(dec("\xee\x80\x80"), std::string("57344/3")); // U+E000
    CHECK_EQ(dec("\xef\xbf\xbd"), std::string("65533/3"));
    CHECK_EQ(dec("\xef\xbf\xbf"), std::string("65535/3"));
    CHECK_EQ(dec("\xf0\x90\x80\x80"), std::string("65536/4"));
    CHECK_EQ(dec("\xf0\x9f\x98\x80"), std::string("128512/4"));  // 😀
    CHECK_EQ(dec("\xf4\x8f\xbf\xbf"), std::string("1114111/4")); // U+10FFFF
    CHECK_EQ(dec("\x80"), std::string("65533/1"));               // a continuation byte
    CHECK_EQ(dec("\xbf"), std::string("65533/1"));
    CHECK_EQ(dec("\xc0\x80"), std::string("65533/1")); // overlong
    CHECK_EQ(dec("\xc1\xbf"), std::string("65533/1"));
    CHECK_EQ(dec("\xe0\x9f\xbf"), std::string("65533/1"));     // overlong 3-byte
    CHECK_EQ(dec("\xed\xa0\x80"), std::string("65533/1"));     // a surrogate, U+D800
    CHECK_EQ(dec("\xed\xbf\xbf"), std::string("65533/1"));     // U+DFFF
    CHECK_EQ(dec("\xf0\x8f\xbf\xbf"), std::string("65533/1")); // overlong 4-byte
    CHECK_EQ(dec("\xf4\x90\x80\x80"), std::string("65533/1")); // above U+10FFFF
    CHECK_EQ(dec("\xf5\x80\x80\x80"), std::string("65533/1"));
    CHECK_EQ(dec("\xff"), std::string("65533/1"));
    CHECK_EQ(dec("\xc2"), std::string("65533/1")); // incomplete
    CHECK_EQ(dec("\xe2\x82"), std::string("65533/1"));
    CHECK_EQ(dec("\xf0\x9f\x98"), std::string("65533/1"));
    CHECK_EQ(dec("\xc2\x41"), std::string("65533/1")); // a bad continuation byte
    CHECK_EQ(dec("\xe2\x41\xac"), std::string("65533/1"));
    CHECK_EQ(dec("\xe2\x82\x41"), std::string("65533/1"));
    CHECK_EQ(dec("\xf0\x9f\x98\x41"), std::string("65533/1"));
    CHECK_EQ(dec("\xc2\xc2"), std::string("65533/1"));

    // range: the index of each rune's first byte, invalid bytes one at a time.
    CHECK_EQ(runes(""_gs), std::string(""));
    CHECK_EQ(runes("ab"_gs), std::string("0:97 1:98 "));
    CHECK_EQ(runes("h\xc3\xa9llo"_gs), std::string("0:104 1:233 3:108 4:108 5:111 "));
    CHECK_EQ(runes("\xe4\xb8\x96\xf0\x9f\x98\x80"_gs), std::string("0:19990 3:128512 "));
    CHECK_EQ(runes("a\xffz"_gs), std::string("0:97 1:65533 2:122 "));
    CHECK_EQ(runes("\xe2\x82"_gs), std::string("0:65533 1:65533 "));
    CHECK_EQ(runes("\xed\xa0\x80x"_gs), std::string("0:65533 1:65533 2:65533 3:120 "));
    CHECK_EQ(runes("a\000b"_gs), std::string("0:97 1:0 2:98 "));
    // The range holds its own copy of the string.
    gp::string s = gp::string::gp_copy("xy");
    auto it = gp::range_string(s);
    s = "other"_gs;
    std::string seen;
    while (gp::range_next(it)) {
        seen += static_cast<char>(it.gp_r);
    }
    CHECK_EQ(seen, std::string("xy"));
    CHECK(!gp::range_next(it)); // stays done

    // string(r): UTF-8, and U+FFFD for what is not a code point.
    CHECK(gp::rune_to_string(int32_t(65)) == "A"_gs);
    CHECK(gp::rune_to_string(int32_t(0)) == "\000"_gs);
    CHECK(gp::rune_to_string(uint8_t(0xe9)) == "\xc3\xa9"_gs); // string(byte) is a rune too
    CHECK(gp::rune_to_string(int32_t(0x7ff)) == "\xdf\xbf"_gs);
    CHECK(gp::rune_to_string(int32_t(0x800)) == "\xe0\xa0\x80"_gs);
    CHECK(gp::rune_to_string(int32_t(0x20ac)) == "\xe2\x82\xac"_gs);
    CHECK(gp::rune_to_string(int32_t(0xffff)) == "\xef\xbf\xbf"_gs);
    CHECK(gp::rune_to_string(int32_t(0x10000)) == "\xf0\x90\x80\x80"_gs);
    CHECK(gp::rune_to_string(int32_t(0x10ffff)) == "\xf4\x8f\xbf\xbf"_gs);
    gp::string bad = "\xef\xbf\xbd"_gs;
    CHECK(gp::rune_to_string(int32_t(0x110000)) == bad);
    CHECK(gp::rune_to_string(int32_t(-1)) == bad);
    CHECK(gp::rune_to_string(INT32_MIN) == bad);
    CHECK(gp::rune_to_string(int32_t(0xd800)) == bad);
    CHECK(gp::rune_to_string(int32_t(0xdfff)) == bad);
    CHECK(gp::rune_to_string(int32_t(0xd7ff)) == "\xed\x9f\xbf"_gs);
    CHECK(gp::rune_to_string(int32_t(0xe000)) == "\xee\x80\x80"_gs);
    CHECK(gp::rune_to_string(INT64_C(0x100000041)) == bad); // not truncated to 'A'
    CHECK(gp::rune_to_string(INT64_MIN) == bad);
    CHECK(gp::rune_to_string(UINT64_MAX) == bad);
    CHECK(gp::rune_to_string(uint64_t(0x41)) == "A"_gs);
    CHECK(gp::rune_to_string(uint32_t(0xffffffff)) == bad);
    CHECK(gp::rune_to_string(int16_t(-3)) == bad);
    // ASCII doesn't allocate: the same static bytes each time.
    CHECK(gp::rune_to_string(int32_t('q')).gp_data() == gp::rune_to_string(int32_t('q')).gp_data());

    // gp_owner: static bytes give the static owner, never an empty one (ptr.hpp).
    auto same = [](const std::shared_ptr<const void>& a, const std::shared_ptr<const void>& b) {
        return !a.owner_before(b) && !b.owner_before(a);
    };
    std::shared_ptr<const void> none;
    CHECK(same(("lit"_gs).gp_owner(), gp::static_owner()));
    CHECK(same(gp::sub("lit"_gs, 1).gp_owner(), gp::static_owner()));
    CHECK(same(gp::string().gp_owner(), none));
    gp::string own = gp::string::gp_copy("own");
    CHECK(!same(own.gp_owner(), none) && !same(own.gp_owner(), gp::static_owner()));
    CHECK(same(gp::sub(own, 1).gp_owner(), own.gp_owner()));
}

// --- div and rem ---

template <typename T>
void test_div_type() {
    using L = std::numeric_limits<T>;
    T mn = L::min(), mx = L::max();
    CHECK_TYPE(gp::div(mn, T(1)), T);
    CHECK_TYPE(gp::rem(mn, T(1)), T);
    CHECK_EQ(gp::div(opaque(T(7)), opaque(T(2))), T(3));
    CHECK_EQ(gp::rem(opaque(T(7)), opaque(T(2))), T(1));
    CHECK_EQ(gp::div(opaque(mx), opaque(T(1))), mx);
    CHECK_EQ(gp::div(opaque(mx), opaque(mx)), T(1));
    CHECK_EQ(gp::rem(opaque(mx), opaque(mx)), T(0));
    CHECK_EQ(gp::div(opaque(T(0)), opaque(T(5))), T(0));
    CHECK_PANIC(gp::div(opaque(T(1)), opaque(T(0))), divide_msg);
    CHECK_PANIC(gp::rem(opaque(T(1)), opaque(T(0))), divide_msg);
    CHECK_PANIC(gp::div(opaque(T(0)), opaque(T(0))), divide_msg);
    CHECK_PANIC(gp::rem(opaque(mn), opaque(T(0))), divide_msg);
    if constexpr (L::is_signed) {
        // MIN / -1 is MIN and MIN % -1 is 0, as Go defines them.
        CHECK_EQ(gp::div(opaque(mn), opaque(T(-1))), mn);
        CHECK_EQ(gp::rem(opaque(mn), opaque(T(-1))), T(0));
        CHECK_EQ(gp::div(opaque(mx), opaque(T(-1))), T(-mx));
        CHECK_EQ(gp::rem(opaque(mx), opaque(T(-1))), T(0));
        CHECK_EQ(gp::div(opaque(T(5)), opaque(T(-1))), T(-5));
        // Truncation toward zero; the remainder has the sign of the dividend.
        CHECK_EQ(gp::div(opaque(T(-7)), opaque(T(2))), T(-3));
        CHECK_EQ(gp::rem(opaque(T(-7)), opaque(T(2))), T(-1));
        CHECK_EQ(gp::div(opaque(T(7)), opaque(T(-2))), T(-3));
        CHECK_EQ(gp::rem(opaque(T(7)), opaque(T(-2))), T(1));
        CHECK_EQ(gp::div(opaque(mn), opaque(T(2))), T(mn / 2));
        CHECK_EQ(gp::rem(opaque(mn), opaque(mx)), T(-1));
        CHECK_EQ(gp::div(opaque(mn), opaque(mn)), T(1));
    } else {
        CHECK_EQ(gp::div(opaque(mx), opaque(T(2))), T(mx / 2));
        CHECK_EQ(gp::rem(opaque(mx), opaque(T(10))), T(mx % 10));
        CHECK_EQ(gp::div(opaque(T(1)), opaque(mx)), T(0));
    }
    // The second operand converts to the type of the first.
    CHECK_EQ(gp::div(opaque(T(9)), 3), T(3));
}

void test_div() {
    test_div_type<int8_t>();
    test_div_type<int16_t>();
    test_div_type<int32_t>();
    test_div_type<int64_t>();
    test_div_type<uint8_t>();
    test_div_type<uint16_t>();
    test_div_type<uint32_t>();
    test_div_type<uint64_t>();
    test_div_type<uintptr_t>();
    CHECK_EQ(gp::div(opaque(INT64_MIN), opaque(INT64_C(-1))), INT64_MIN);
    CHECK_EQ(gp::div(opaque(int8_t(-128)), opaque(int8_t(-1))), int8_t(-128));
    CHECK_EQ(gp::div(opaque(int16_t(-32768)), opaque(int16_t(-1))), int16_t(-32768));
}

// --- shl and shr ---

template <typename T, typename N>
void test_shift_count() {
    constexpr int w = std::numeric_limits<std::make_unsigned_t<T>>::digits;
    using L = std::numeric_limits<T>;
    CHECK_TYPE(gp::shl(T(1), N(1)), T);
    CHECK_TYPE(gp::shr(T(1), N(1)), T);
    CHECK_EQ(gp::shl(opaque(T(1)), opaque(N(0))), T(1));
    CHECK_EQ(gp::shr(opaque(T(1)), opaque(N(0))), T(1));
    CHECK_EQ(gp::shl(opaque(T(3)), opaque(N(1))), T(6));
    CHECK_EQ(gp::shr(opaque(T(6)), opaque(N(1))), T(3));
    // width - 1: the top bit.
    CHECK_EQ(gp::shl(opaque(T(1)), opaque(N(w - 1))),
             T(L::is_signed ? L::min() : T(L::max() / 2 + 1)));
    CHECK_EQ(gp::shl(opaque(T(3)), opaque(N(w - 1))),
             T(L::is_signed ? L::min() : T(L::max() / 2 + 1)));
    CHECK_EQ(gp::shr(opaque(L::max()), opaque(N(w - 1))), T(L::is_signed ? 0 : 1));
    // width and more: 0, or -1 for a negative signed >>.
    CHECK_EQ(gp::shl(opaque(T(1)), opaque(N(w))), T(0));
    CHECK_EQ(gp::shl(opaque(L::max()), opaque(N(w))), T(0));
    CHECK_EQ(gp::shr(opaque(L::max()), opaque(N(w))), T(0));
    if constexpr (std::numeric_limits<N>::max() >= 200) {
        CHECK_EQ(gp::shl(opaque(T(1)), opaque(N(200))), T(0));
        CHECK_EQ(gp::shr(opaque(L::max()), opaque(N(200))), T(0));
    }
    CHECK_EQ(gp::shl(opaque(T(1)), opaque(std::numeric_limits<N>::max())), T(0));
    if constexpr (L::is_signed) {
        CHECK_EQ(gp::shr(opaque(T(-1)), opaque(N(w))), T(-1));
        CHECK_EQ(gp::shr(opaque(L::min()), opaque(N(w))), T(-1));
        CHECK_EQ(gp::shr(opaque(L::min()), opaque(N(w - 1))), T(-1));
        CHECK_EQ(gp::shr(opaque(T(-8)), opaque(N(1))), T(-4)); // arithmetic shift
        CHECK_EQ(gp::shr(opaque(T(-1)), opaque(N(1))), T(-1));
        CHECK_EQ(gp::shl(opaque(T(-1)), opaque(N(1))), T(-2));
        CHECK_EQ(gp::shl(opaque(L::max()), opaque(N(1))), T(-2)); // wraps, as in Go
        CHECK_EQ(gp::shl(opaque(L::min()), opaque(N(1))), T(0));
        if constexpr (std::numeric_limits<N>::max() >= 200) {
            CHECK_EQ(gp::shr(opaque(T(-5)), opaque(N(200))), T(-1));
        }
    } else {
        CHECK_EQ(gp::shr(opaque(L::max()), opaque(N(1))), T(L::max() / 2)); // logical shift
        CHECK_EQ(gp::shl(opaque(L::max()), opaque(N(1))), T(L::max() - 1));
    }
    if constexpr (std::is_signed_v<N>) {
        CHECK_PANIC(gp::shl(opaque(T(1)), opaque(N(-1))), shift_msg);
        CHECK_PANIC(gp::shr(opaque(T(1)), opaque(N(-1))), shift_msg);
        CHECK_PANIC(gp::shl(opaque(T(0)), opaque(std::numeric_limits<N>::min())), shift_msg);
        CHECK_PANIC(gp::shr(opaque(T(-1)), opaque(std::numeric_limits<N>::min())), shift_msg);
    }
}

template <typename T>
void test_shift_type() {
    test_shift_count<T, int8_t>();
    test_shift_count<T, int64_t>();
    test_shift_count<T, uint8_t>();
    test_shift_count<T, uint64_t>();
    test_shift_count<T, int32_t>();
    test_shift_count<T, uint32_t>();
}

void test_shift() {
    test_shift_type<int8_t>();
    test_shift_type<int16_t>();
    test_shift_type<int32_t>();
    test_shift_type<int64_t>();
    test_shift_type<uint8_t>();
    test_shift_type<uint16_t>();
    test_shift_type<uint32_t>();
    test_shift_type<uint64_t>();
    CHECK_EQ(gp::shl(opaque(INT64_C(1)), opaque(INT64_C(62))), INT64_C(4611686018427387904));
    CHECK_EQ(gp::shl(opaque(uint8_t(200)), opaque(1)), uint8_t(144));
    CHECK_EQ(gp::shr(opaque(int16_t(-32768)), opaque(15)), int16_t(-1));
}

// --- f2i ---

void test_f2i() {
    const double inf = std::numeric_limits<double>::infinity();
    const double nan = std::numeric_limits<double>::quiet_NaN();
    CHECK_TYPE(gp::f2i<int8_t>(1.0), int8_t);
    CHECK_TYPE(gp::f2i<uint64_t>(1.0), uint64_t);

    // In range: truncation toward zero.
    CHECK_EQ(gp::f2i<int64_t>(opaque(1.9)), INT64_C(1));
    CHECK_EQ(gp::f2i<int64_t>(opaque(-1.9)), INT64_C(-1));
    CHECK_EQ(gp::f2i<int64_t>(opaque(-0.5)), INT64_C(0));
    CHECK_EQ(gp::f2i<uint8_t>(opaque(255.5)), uint8_t(255));
    CHECK_EQ(gp::f2i<int32_t>(opaque(-2147483648.9)), INT32_MIN);
    CHECK_EQ(gp::f2i<int64_t>(opaque(9.2233720368547748e18)), INT64_C(9223372036854774784));
    CHECK_EQ(gp::f2i<uint64_t>(opaque(1.8446744073709550e19)), UINT64_C(18446744073709549568));

    // int64 and int32: MIN out of range, for NaN and for ±Inf.
    for (double x : {9223372036854775808.0, -9.223372036854777e18, 1e300, -1e300, inf, -inf, nan}) {
        CHECK_EQ(gp::f2i<int64_t>(opaque(x)), INT64_MIN);
    }
    CHECK_EQ(gp::f2i<int64_t>(opaque(-9223372036854775808.0)), INT64_MIN);
    for (double x : {2147483648.0, -2147483649.0, 3e9 + 7, -3e9, 1e300, inf, -inf, nan}) {
        CHECK_EQ(gp::f2i<int32_t>(opaque(x)), INT32_MIN);
    }
    CHECK_EQ(gp::f2i<int32_t>(opaque(2147483647.0)), INT32_MAX);

    // int16, int8, uint16, uint8: through int32, then the low bits.
    CHECK_EQ(gp::f2i<int8_t>(opaque(128.0)), int8_t(-128));
    CHECK_EQ(gp::f2i<int8_t>(opaque(-129.0)), int8_t(127));
    CHECK_EQ(gp::f2i<int8_t>(opaque(300.0)), int8_t(44));
    CHECK_EQ(gp::f2i<int8_t>(opaque(-300.0)), int8_t(-44));
    CHECK_EQ(gp::f2i<uint8_t>(opaque(300.0)), uint8_t(44));
    CHECK_EQ(gp::f2i<uint8_t>(opaque(-300.0)), uint8_t(212));
    CHECK_EQ(gp::f2i<uint8_t>(opaque(-1.0)), uint8_t(255));
    CHECK_EQ(gp::f2i<int16_t>(opaque(32768.0)), int16_t(-32768));
    CHECK_EQ(gp::f2i<int16_t>(opaque(65543.0)), int16_t(7));
    CHECK_EQ(gp::f2i<uint16_t>(opaque(-32769.0)), uint16_t(32767));
    CHECK_EQ(gp::f2i<int16_t>(opaque(2147483647.0)), int16_t(-1));
    CHECK_EQ(gp::f2i<uint8_t>(opaque(3e9 + 7)), uint8_t(0)); // int32 MIN: through int32, not int64
    CHECK_EQ(gp::f2i<int16_t>(opaque(3e9 + 7)), int16_t(0));
    for (double x : {1e300, -1e300, inf, -inf, nan}) {
        CHECK_EQ(gp::f2i<int16_t>(opaque(x)), int16_t(0));
        CHECK_EQ(gp::f2i<int8_t>(opaque(x)), int8_t(0));
        CHECK_EQ(gp::f2i<uint16_t>(opaque(x)), uint16_t(0));
        CHECK_EQ(gp::f2i<uint8_t>(opaque(x)), uint8_t(0));
    }

    // uint32: through int64, then the low 32 bits.
    CHECK_EQ(gp::f2i<uint32_t>(opaque(4294967305.0)), uint32_t(9));
    CHECK_EQ(gp::f2i<uint32_t>(opaque(-3e9)), uint32_t(1294967296));
    CHECK_EQ(gp::f2i<uint32_t>(opaque(-1.0)), uint32_t(4294967295u));
    CHECK_EQ(gp::f2i<uint32_t>(opaque(3e9 + 7)), uint32_t(3000000007u));
    CHECK_EQ(gp::f2i<uint32_t>(opaque(-2147483649.0)), uint32_t(2147483647));
    for (double x : {9223372036854775808.0, 1e300, -1e300, inf, -inf, nan}) {
        CHECK_EQ(gp::f2i<uint32_t>(opaque(x)), uint32_t(0));
    }

    // uint64: Go's two ranges.
    CHECK_EQ(gp::f2i<uint64_t>(opaque(-1.0)), UINT64_MAX);
    CHECK_EQ(gp::f2i<uint64_t>(opaque(-129.0)), UINT64_C(18446744073709551487));
    CHECK_EQ(gp::f2i<uint64_t>(opaque(-3e9)), UINT64_C(18446744070709551616));
    CHECK_EQ(gp::f2i<uint64_t>(opaque(9223372036854775808.0)), UINT64_C(9223372036854775808));
    CHECK_EQ(gp::f2i<uint64_t>(opaque(18446744073709551616.0)), UINT64_C(9223372036854775808));
    for (double x :
         {-9223372036854775808.0, -9.223372036854777e18, 1e300, -1e300, inf, -inf, nan}) {
        CHECK_EQ(gp::f2i<uint64_t>(opaque(x)), UINT64_C(9223372036854775808));
    }

    // float32 arguments give Go's float32 conversions.
    CHECK_EQ(gp::f2i<int64_t>(opaque(2147483648.0f)), INT64_C(2147483648));
    CHECK_EQ(gp::f2i<uint32_t>(opaque(4294967296.0f)), uint32_t(0));
    CHECK_EQ(gp::f2i<uint64_t>(opaque(9.223372e18f)), UINT64_C(9223372036854775808));
    CHECK_EQ(gp::f2i<int64_t>(opaque(std::numeric_limits<float>::infinity())), INT64_MIN);
    CHECK_EQ(gp::f2i<uint8_t>(opaque(300.0f)), uint8_t(44));
}

// --- the print format ---

std::string printed(double v) {
    std::string out;
    gp::detail::print_float(out, v);
    return out;
}

template <typename... Ts>
std::string print_all(const Ts&... v) {
    std::string out;
    (gp::detail::append_print(out, v), ...);
    return out;
}

void test_print() {
    // The plan's example: println(1.5, 3.0, -0.25, 'x') (print puts nothing between them).
    CHECK_EQ(print_all(1.5, 3.0, -0.25, int32_t('x')),
             std::string("+1.500000e+000+3.000000e+000-2.500000e-001120"));
    CHECK_EQ(printed(1.5), std::string("+1.500000e+000"));
    CHECK_EQ(printed(3.0), std::string("+3.000000e+000"));
    CHECK_EQ(printed(-0.25), std::string("-2.500000e-001"));
    CHECK_EQ(printed(0.0), std::string("+0.000000e+000"));
    CHECK_EQ(printed(-0.0), std::string("-0.000000e+000"));
    CHECK_EQ(printed(1e21), std::string("+1.000000e+021"));
    CHECK_EQ(printed(1e300), std::string("+1.000000e+300"));
    CHECK_EQ(printed(5e-324), std::string("+4.940656e-324"));
    CHECK_EQ(printed(123456789.0), std::string("+1.234568e+008"));
    CHECK_EQ(printed(1.7976931348623157e308), std::string("+1.797693e+308"));
    CHECK_EQ(printed(2.5e-10), std::string("+2.500000e-010"));
    CHECK_EQ(printed(0.1), std::string("+1.000000e-001"));
    CHECK_EQ(printed(static_cast<double>(0.1f)), std::string("+1.000000e-001"));
    CHECK_EQ(printed(9.9999999), std::string("+1.000000e+001"));
    CHECK_EQ(printed(std::numeric_limits<double>::infinity()), std::string("+Inf"));
    CHECK_EQ(printed(-std::numeric_limits<double>::infinity()), std::string("-Inf"));
    CHECK_EQ(printed(std::numeric_limits<double>::quiet_NaN()), std::string("NaN"));

    CHECK_EQ(print_all(true, false), std::string("truefalse"));
    CHECK_EQ(print_all(INT64_C(-5), int8_t(-128), uint8_t(255), uint16_t(65535)),
             std::string("-5-12825565535"));
    CHECK_EQ(print_all(UINT64_MAX), std::string("18446744073709551615"));
    CHECK_EQ(print_all(INT64_MIN), std::string("-9223372036854775808"));
    CHECK_EQ(print_all(INT64_C(0)), std::string("0"));
    CHECK_EQ(print_all("str"_gs, ""_gs), std::string("str"));
    int* nil = nullptr;
    CHECK_EQ(print_all(nil), std::string("0x0"));
    CHECK_EQ(print_all(nullptr), std::string("0x0"));
    CHECK_EQ(print_all(reinterpret_cast<int*>(uintptr_t(0xc000012345))),
             std::string("0xc000012345"));
    CHECK_EQ(print_all(1.5f), std::string("+1.500000e+000"));
}

// --- the fmt formats ---

std::string fv(double v, int bits = 64) {
    std::string out;
    gp::fmt::detail::float_v(out, v, bits);
    return out;
}

std::string ff(double v) {
    std::string out;
    gp::fmt::detail::float_f(out, v);
    return out;
}

template <typename... Ts>
std::string fmt_sprintf(std::string_view format, const Ts&... v) {
    std::string out;
    if constexpr (sizeof...(Ts) > 0) {
        const gp::fmt::detail::arg a[] = {gp::fmt::make_arg(v)...};
        gp::fmt::detail::sprintf(out, format, a, sizeof...(Ts));
    } else {
        gp::fmt::detail::sprintf(out, format, nullptr, 0);
    }
    return out;
}

template <typename... Ts>
std::string fmt_sprint(const Ts&... v) {
    std::string out;
    const gp::fmt::detail::arg a[] = {gp::fmt::make_arg(v)...};
    gp::fmt::detail::sprint(out, a, sizeof...(Ts));
    return out;
}

template <typename... Ts>
std::string fmt_sprintln(const Ts&... v) {
    std::string out;
    const gp::fmt::detail::arg a[] = {gp::fmt::make_arg(v)...};
    gp::fmt::detail::sprintln(out, a, sizeof...(Ts));
    return out;
}

void test_fmt() {
    // The plan's examples (Phase 02 §2.3).
    CHECK_EQ(fv(123456.0), std::string("123456"));
    CHECK_EQ(fv(1000000.0), std::string("1e+06"));
    CHECK_EQ(fv(1234567.0), std::string("1.234567e+06"));
    CHECK_EQ(fv(0.0001), std::string("0.0001"));
    CHECK_EQ(fv(0.00001), std::string("1e-05"));
    CHECK_EQ(fv(std::numeric_limits<double>::infinity()), std::string("+Inf"));
    CHECK_EQ(fv(-std::numeric_limits<double>::infinity()), std::string("-Inf"));
    CHECK_EQ(fv(std::numeric_limits<double>::quiet_NaN()), std::string("NaN"));
    // More, checked with real Go.
    CHECK_EQ(fv(1e20), std::string("1e+20"));
    CHECK_EQ(fv(1e21), std::string("1e+21"));
    CHECK_EQ(fv(3.0), std::string("3"));
    CHECK_EQ(fv(2.5), std::string("2.5"));
    CHECK_EQ(fv(0.0), std::string("0"));
    CHECK_EQ(fv(-0.0), std::string("-0"));
    CHECK_EQ(fv(5e-324), std::string("5e-324"));
    CHECK_EQ(fv(1.7976931348623157e308), std::string("1.7976931348623157e+308"));
    CHECK_EQ(fv(-1.5e-7), std::string("-1.5e-07"));
    CHECK_EQ(fv(100000.0), std::string("100000"));
    CHECK_EQ(fv(999999.0), std::string("999999"));
    CHECK_EQ(fv(123456.7), std::string("123456.7"));
    CHECK_EQ(fv(0.1), std::string("0.1"));
    CHECK_EQ(fv(0.1, 32), std::string("0.1"));
    CHECK_EQ(fv(static_cast<double>(0.1f)),
             std::string("0.10000000149011612")); // a float32 seen as float64
    CHECK_EQ(fv(16777216.0, 32), std::string("1.6777216e+07"));
    CHECK_EQ(fv(1e-5, 32), std::string("1e-05"));
    CHECK_EQ(fv(3.4028234663852886e38, 32), std::string("3.4028235e+38"));
    CHECK_EQ(ff(3.14159265), std::string("3.141593"));
    CHECK_EQ(ff(0.0), std::string("0.000000"));
    CHECK_EQ(ff(1e-7), std::string("0.000000"));
    CHECK_EQ(ff(2.5), std::string("2.500000"));
    CHECK_EQ(ff(static_cast<double>(0.1f)), std::string("0.100000"));
    CHECK_EQ(ff(-std::numeric_limits<double>::infinity()), std::string("-Inf"));
    CHECK_EQ(ff(std::numeric_limits<double>::quiet_NaN()), std::string("NaN"));
    CHECK_EQ(ff(1e300).size(), std::size_t(301 + 7));

    // Print adds a space only between two operands that are not strings.
    CHECK_EQ(fmt_sprint("a"_gs, "b"_gs, INT64_C(1), INT64_C(2), true, "c"_gs, 3.5),
             std::string("ab1 2 truec3.5"));
    CHECK_EQ(fmt_sprint("a"_gs, INT64_C(1), INT64_C(2), "b"_gs), std::string("a1 2b"));
    CHECK_EQ(
        fmt_sprintln("x"_gs, INT64_C(1), true, int8_t(-3), UINT64_MAX, int32_t('a'), uint8_t(7)),
        std::string("x 1 true -3 18446744073709551615 97 7\n"));
    CHECK_EQ(fmt_sprintln(1e6, 123456.0, 0.1f), std::string("1e+06 123456 0.1\n"));

    // Printf: the verbs, and Go's error forms (checked with real Go).
    CHECK_EQ(fmt_sprintf("%v|%d|%s|%t|%f|%v|%v", 1.5, INT64_C(-42), "s"_gs, true, 3.14159265,
                         "str"_gs, int8_t(5)),
             std::string("1.5|-42|s|true|3.141593|str|5"));
    CHECK_EQ(fmt_sprintf("%d %s %t %f", "x"_gs, INT64_C(5), INT64_C(1), INT64_C(7)),
             std::string("%!d(string=x) %!s(int=5) %!t(int=1) %!f(int=7)"));
    CHECK_EQ(fmt_sprintf("%v %d", INT64_C(1)), std::string("1 %!d(MISSING)"));
    CHECK_EQ(fmt_sprintf("%d", INT64_C(1), INT64_C(2)), std::string("1%!(EXTRA int=2)"));
    CHECK_EQ(fmt_sprintf("%d", INT64_C(1), INT64_C(2), "x"_gs),
             std::string("1%!(EXTRA int=2, string=x)"));
    CHECK_EQ(fmt_sprintf("100%% %z %", INT64_C(3)), std::string("100% %!z(int=3) %!(NOVERB)"));
    CHECK_EQ(fmt_sprintf("%v %v", true, int32_t('x')), std::string("true 120"));
    CHECK_EQ(fmt_sprintf("%d %d %v", uint8_t(200), UINT64_MAX, 0.1f),
             std::string("200 18446744073709551615 0.1"));
    CHECK_EQ(fmt_sprintf("%s", true), std::string("%!s(bool=true)"));
    CHECK_EQ(fmt_sprintf("%d", 1.5), std::string("%!d(float64=1.5)"));
    CHECK_EQ(fmt_sprintf("%t", 0.5f), std::string("%!t(float32=0.5)"));
    CHECK_EQ(fmt_sprintf("%d%d", uint16_t(1), int16_t(2)), std::string("12"));
    CHECK_EQ(fmt_sprintf("%s", uint32_t(1)), std::string("%!s(uint32=1)"));
    CHECK_EQ(fmt_sprintf("%!"), std::string("%!!(MISSING)"));
    CHECK_EQ(fmt_sprintf("end%"), std::string("end%!(NOVERB)"));
    CHECK_EQ(fmt_sprintf("%\xc3\xa9", INT64_C(1)), std::string("%!\xc3\xa9(int=1)"));
    CHECK_EQ(fmt_sprintf("%\xff", INT64_C(1)), std::string("%!\xef\xbf\xbd(int=1)"));
    CHECK_EQ(fmt_sprintf("no verbs"), std::string("no verbs"));
    CHECK_EQ(fmt_sprintf(""), std::string(""));
    CHECK_EQ(fmt_sprintf("%f %v", std::numeric_limits<double>::infinity(),
                         std::numeric_limits<double>::quiet_NaN()),
             std::string("+Inf NaN"));
}

// --- panic ---

void test_panic() {
    CHECK_PANIC(gp::panic("boom"_gs), "boom");
    CHECK_PANIC(gp::panic(""_gs), "");
    CHECK_PANIC(gp::panic(INT64_C(42)), "42");
    CHECK_PANIC(gp::panic(1.5), "+1.500000e+000");
    CHECK_PANIC(gp::panic(true), "true");
    CHECK_PANIC(gp::panic(uint8_t(7)), "7");
    gp::detail::clear_signal(); // in case a nil pointer panic ran before
    std::string out;
    gp::detail::panic_report(out, "a\nb\nc"_gs);
    CHECK_EQ(out, std::string("panic: a\n\tb\n\tc\n\ngoroutine 1 [running]:\n"));
    out.clear();
    gp::detail::panic_report(out, "boom"_gs);
    CHECK_EQ(out, std::string("panic: boom\n\ngoroutine 1 [running]:\n"));
    // A panic is not a std::exception, so C++ code can't catch it as one by accident.
    CHECK(!std::is_base_of_v<std::exception, gp::panic_error>);
}

// --- complex numbers (gocpp/complex.hpp) ---

// A named complex type, as the emitter writes type N complex128 (step 3.9).
struct NamedC : gp::newtype<NamedC, gp::complex128> {
    using gp::newtype<NamedC, gp::complex128>::newtype;
};

// cbits returns the bits of c's parts, so that -0 and +0 differ (NaN parts are compared
// with each other only where both are NaN, below).
std::string cbits(gp::complex128 c) {
    return std::to_string(std::bit_cast<uint64_t>(gp::real(c))) + "," +
           std::to_string(std::bit_cast<uint64_t>(gp::imag(c)));
}

bool is_nan(double f) {
    return f != f;
}

void test_complex() {
    using c128 = gp::complex128;
    using c64 = gp::complex64;
    const double inf = std::numeric_limits<double>::infinity();
    const double nan = std::numeric_limits<double>::quiet_NaN();

    static_assert(sizeof(c64) == 8 && alignof(c64) == 4);
    static_assert(sizeof(c128) == 16 && alignof(c128) == 8);
    static_assert(std::is_trivially_copyable_v<c128> && std::is_standard_layout_v<c64>);
    static_assert(gp::go_complex<c64> && gp::go_complex<const c128> && !gp::go_complex<double>);
    static_assert(!std::is_convertible_v<double, c128>); // complex(re) is explicit
    static_assert(!std::is_convertible_v<c64, c128> && !std::is_convertible_v<c128, c64>);
    // constexpr: a package variable with a constant value is constinit.
    constexpr c128 k = c128(1, 2) * c128(3, 4) + c128(1, 0) - -c128(0, 1);
    static_assert(k == c128(-4, 11));
    CHECK_TYPE(gp::real(c64(1, 2)), float);
    CHECK_TYPE(gp::imag(c128(1, 2)), double);
    CHECK_TYPE(c64(1, 2) / c64(3, 4), c64);
    CHECK(c128() == c128(0, 0) && cbits(c128()) == cbits(c128(0.0, 0.0)));

    // + - * / and unary - on ordinary values (Go: (1+2i)/(3+4i) = (0.44+0.08i)).
    CHECK(c128(1, 2) + c128(3, 4) == c128(4, 6));
    CHECK(c128(1, 2) - c128(3, 4) == c128(-2, -2));
    CHECK(c128(1, 2) * c128(3, 4) == c128(-5, 10));
    CHECK_EQ(cbits(c128(1, 2) / c128(3, 4)), cbits(c128(0.44, 0.08)));
    CHECK_EQ(cbits(-c128(0, 0)), cbits(c128(-0.0, -0.0)));
    CHECK(c128(1, nan) != c128(1, nan) && !(c128(nan, 0) == c128(nan, 0)));
    CHECK(c128(-0.0, 0) == c128(0, -0.0));

    // Division by zero: infinities or NaN, no panic (Go's complex128div, C99's G.5.1).
    c128 q = c128(1, 1) / c128(0, 0);
    CHECK(gp::real(q) == inf && gp::imag(q) == inf);
    q = c128(-1, 0) / c128(-0.0, 0);
    CHECK(gp::real(q) == inf && is_nan(gp::imag(q))); // copysign(inf, -0) * -1, ... * 0
    q = c128(0, 0) / c128(0, 0);
    CHECK(is_nan(gp::real(q)) && is_nan(gp::imag(q)));
    q = c128(inf, 1) / c128(2, 3);
    CHECK(gp::real(q) == inf && gp::imag(q) == -inf);
    q = c128(1, 2) / c128(inf, -inf);
    CHECK_EQ(cbits(q), cbits(c128(-0.0, 0.0)));
    // Multiplication has no recovery of infinities, as in Go: (Inf+0i)*2 = (+Inf+NaNi).
    q = c128(inf, 0) * c128(2, 0);
    CHECK(gp::real(q) == inf && is_nan(gp::imag(q)));
    q = c128(inf, inf) * c128(0, 1);
    CHECK(is_nan(gp::real(q)) && is_nan(gp::imag(q)));

    // complex64: * in float64, rounded back. In float32, (1+2^-13)(1-2^-13) - 1*1 would be 0.
    const float e = 1.0f / 8192;
    c64 p = c64(1 + e, 1) * c64(1 - e, 1);
    CHECK(gp::real(p) == -1.0f / 67108864);
    CHECK(gp::imag(p) == 2.0f);
    // / through complex128: (1+1i)/(3+0i) rounds each part of 0.333... once.
    CHECK(gp::real(c64(1, 1) / c64(3, 0)) == 1.0f / 3);
    c64 z64 = c64(1, -1) / c64(0, 0);
    CHECK(gp::real(z64) == std::numeric_limits<float>::infinity() &&
          gp::imag(z64) == -std::numeric_limits<float>::infinity());

    // Conversions round each part.
    c64 r = c64(c128(0.1, 1e40));
    CHECK(gp::real(r) == 0.1f);
    CHECK(gp::imag(r) == std::numeric_limits<float>::infinity());
    CHECK_EQ(cbits(c128(c64(0.1f, -0.0f))), cbits(c128(static_cast<double>(0.1f), -0.0)));

    // c++ and c-- add (1+0i): a -0 imaginary part becomes +0, as in Go.
    c128 inc = c128(-0.0, -0.0);
    c128 old = inc++;
    CHECK_EQ(cbits(old), cbits(c128(-0.0, -0.0)));
    CHECK_EQ(cbits(inc), cbits(c128(1, 0)));
    --inc;
    CHECK_EQ(cbits(inc), cbits(c128(0, 0)));
    c64 acc = c64(1, 1);
    acc += c64(1, 0);
    acc *= c64(0, 1);
    acc -= c64(1, 1);
    acc /= c64(0, 2);
    CHECK(acc == c64(0.5f, 1.0f));

    // A named complex type: newtype's operators, ++ through (1+0i).
    NamedC n(c128(1, -0.0));
    n++;
    CHECK_EQ(cbits(n.gp_value()), cbits(c128(2, 0)));
    n = n * NamedC(c128(0, 1)) / NamedC(c128(2, 0));
    CHECK(n == NamedC(c128(0, 1)));
    CHECK(-n == NamedC(c128(-0.0, -1)) && n != NamedC(c128(0, 2)));
    CHECK_EQ(gp::hash_of(n), gp::hash_of(c128(0, 1)));

    // Hashes: +0 and -0 are equal, every NaN hashes alike.
    CHECK_EQ(gp::hash_of(c128(0.0, -0.0)), gp::hash_of(c128(-0.0, 0.0)));
    CHECK_EQ(gp::hash_of(c128(nan, 1)), gp::hash_of(c128(-nan, 1)));
    CHECK(gp::hash_of(c128(1, 2)) != gp::hash_of(c128(2, 1)));
    CHECK_EQ(gp::hash_of(c64(1, 2)), gp::hash_of(c64(1, 2)));

    // print and panic: Go's runtime format, (+1.000000e+000-2.000000e+000i).
    CHECK_EQ(print_all(c128(1, -2)), std::string("(+1.000000e+000-2.000000e+000i)"));
    CHECK_EQ(print_all(c64(nan, std::numeric_limits<float>::infinity())),
             std::string("(NaN+Infi)"));
    CHECK_EQ(print_all(c128(-0.0, 0), NamedC(c128(0.5, 0))),
             std::string("(-0.000000e+000+0.000000e+000i)(+5.000000e-001+0.000000e+000i)"));
    CHECK_PANIC(gp::panic(c128(1.5, 2)), "(+1.500000e+000+2.000000e+000i)");
    CHECK_PANIC(gp::panic_constant(c64(0, 1)), "(+0.000000e+000+1.000000e+000i)");
}

// --- pointers (gocpp/ptr.hpp) ---

const char* const nil_msg = "runtime error: invalid memory address or nil pointer dereference";

// report_of returns what an unrecovered panic with this text prints now, on this thread.
std::string report_of(const char* text) {
    std::string out;
    gp::detail::panic_report(out, gp::string::gp_copy(text));
    return out;
}

struct Point {
    int64_t X = 0;
    int64_t Y = 0;
    int64_t Sum() const { return X + Y; }
};

// Same layout as Point, as for `type Other Point` in Go.
struct Other {
    int64_t X = 0;
    int64_t Y = 0;
};

// Tracked counts its live objects, so a test can see when a block is freed. (Go structs have
// no destructors; this is only for the tests.)
int live = 0;

struct Tracked {
    int64_t n = 0;
    Point pt;
    Tracked() { live++; }
    explicit Tracked(int64_t v) : n(v) { live++; }
    Tracked(const Tracked& o) : n(o.n), pt(o.pt) { live++; }
    ~Tracked() { live--; }
};

// A recursive type: the templates accept an incomplete T.
struct Node {
    int64_t v = 0;
    gp::ptr<Node> next;
    gp::weak_ptr<Node> prev;
    Tracked t;
};

// A parent that owns its child, and a child that points back with a weak pointer: no cycle.
struct Child;
struct Parent {
    gp::ptr<Child> child;
    Tracked t;
};
struct Child {
    gp::weak_ptr<Parent> parent;
    gp::ptr<Parent> strong; // for the cycle test
    Tracked t;
};

Point global_point;
int64_t global_arr[3];

int64_t use_count(const gp::ptr<Tracked>& p) {
    return static_cast<int64_t>(p.gp_shared().use_count());
}

// A boxed local whose address escapes (func f() *T { x := T{...}; return &x }).
gp::ptr<Tracked> boxed_escape(int64_t v) {
    gp::ptr<Tracked> x = gp::new_<Tracked>(Tracked(v)); // boxed: &x is taken below
    x->n++;
    return x; // &x is the box
}

template <typename P>
constexpr bool derefable = requires(P v) { *v; };

void test_ptr_basics() {
    CHECK(sizeof(gp::ptr<int64_t>) == 16);
    CHECK(sizeof(gp::ptr<Node>) == 16);
    CHECK(sizeof(gp::weak_ptr<int64_t>) == 24);

    gp::ptr<int64_t> nil;
    CHECK(nil == nullptr);
    CHECK(nullptr == nil);
    CHECK(nil == gp::ptr<int64_t>());
    CHECK(nil.gp_get() == nullptr);

    // new(int) is a zero int; &T{...} copies the value in.
    gp::ptr<int64_t> p = gp::new_<int64_t>();
    CHECK(p != nullptr);
    CHECK_EQ(*p, INT64_C(0));
    *p = 5;
    gp::ptr<int64_t> q = p;
    *q += 2;
    CHECK_EQ(*p, INT64_C(7));
    CHECK(p == q);
    gp::ptr<int64_t> r = gp::new_<int64_t>(INT64_C(7));
    CHECK(*r == *p);
    CHECK(r != p); // equal values, different objects
    CHECK(p != nil);
    CHECK_TYPE(*p, int64_t&);

    gp::ptr<Point> pt = gp::new_<Point>();
    CHECK_EQ(pt->X, INT64_C(0));
    CHECK_EQ(pt->Y, INT64_C(0));
    pt = gp::new_<Point>(Point{.X = 1, .Y = 2});
    CHECK_EQ(pt->Sum(), INT64_C(3));
    (*pt).Y = 5;
    CHECK_EQ(pt->Y, INT64_C(5));
    pt = nullptr;
    CHECK(pt == nullptr);

    // A recursive type: a list of three nodes, with weak back pointers.
    {
        gp::ptr<Node> head = gp::new_<Node>();
        head->next = gp::new_<Node>();
        head->next->v = 2;
        head->next->prev = gp::weak_make(head);
        head->next->next = gp::new_<Node>();
        head->next->next->v = 3;
        CHECK_EQ(live, 3);
        CHECK(gp::weak_value(head->next->prev) == head);
        CHECK_EQ(head->next->next->v, INT64_C(3));
        CHECK(head->next->next->next == nullptr);
    }
    CHECK_EQ(live, 0);

    // Conversions: none to another pointer type or bool, as in Go; nil converts. The raw
    // pointer is the one implicit conversion, for C and C++ (Phase 04 §2.7; interop_test.cpp).
    CHECK(!(std::is_convertible_v<gp::ptr<Point>, gp::ptr<Other>>));
    CHECK(!(std::is_constructible_v<gp::ptr<Other>, gp::ptr<Point>>));
    CHECK((std::is_convertible_v<gp::ptr<int64_t>, int64_t*>));
    CHECK(!(std::is_convertible_v<gp::ptr<int64_t>, bool>));
    CHECK(!(std::is_constructible_v<bool, gp::ptr<int64_t>>));
    CHECK(!(std::is_convertible_v<int64_t*, gp::ptr<int64_t>>));
    CHECK(!(std::is_convertible_v<std::shared_ptr<int64_t>, gp::ptr<int64_t>>)); // explicit only
    CHECK((std::is_convertible_v<std::nullptr_t, gp::ptr<int64_t>>));
    CHECK(!(std::equality_comparable_with<gp::ptr<Point>, gp::ptr<Other>>));

    // ptr_cast: Go's (*Other)(p) between types with identical underlying types.
    gp::ptr<Point> src = gp::new_<Point>(Point{.X = 4, .Y = 6});
    gp::ptr<Other> o = gp::ptr_cast<Other>(src);
    o->X = 11;
    CHECK_EQ(src->X, INT64_C(11));
    CHECK(static_cast<void*>(o.gp_get()) == static_cast<void*>(src.gp_get()));
    CHECK(gp::ptr_cast<Point>(o) == src);
    CHECK_EQ(src.gp_shared().use_count(), 2L); // the conversion shares the owner
    CHECK(gp::ptr_cast<Other>(gp::ptr<Point>()) == nullptr);

    // unsafe.Pointer is gp::ptr<void> (Phase 04): it exists and compares; it has no *.
    gp::ptr<void> vp;
    CHECK(vp == nullptr);
    CHECK(!derefable<gp::ptr<void>>);
    CHECK(derefable<gp::ptr<int64_t>>);
}

void test_ptr_nil() {
    gp::ptr<int64_t> n;
    gp::ptr<Point> pn;
    CHECK_PANIC((void)*n, nil_msg);
    CHECK_PANIC(*n = 1, nil_msg);
    CHECK_PANIC((void)pn->X, nil_msg);
    CHECK_PANIC(pn->Y = 1, nil_msg);
    CHECK_PANIC((void)(*pn).Y, nil_msg);
    CHECK_PANIC(pn->Sum(), nil_msg);                    // a value method through a nil pointer
    CHECK_PANIC(gp::ptr<int64_t>(pn, &pn->Y), nil_msg); // &p.Y with a nil p panics in Go too
    CHECK_PANIC(*gp::weak_value(gp::weak_ptr<int64_t>()), nil_msg);

    // The goroutine's signal: a nil pointer panic sets it, and the report of an unrecovered
    // panic prints Go's signal line after the text. The pc is the caller's.
    gp::detail::clear_signal();
    CHECK_EQ(report_of(nil_msg), std::string("panic: ") + nil_msg + "\n\ngoroutine 1 [running]:\n");
    CHECK_PANIC(*n = 1, nil_msg);
    std::string out = report_of(nil_msg);
    std::string head = std::string("panic: ") + nil_msg +
                       "\n[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x";
#ifdef _WIN32
    head = std::string("panic: ") + nil_msg + "\n[signal 0xc0000005 code=0x0 addr=0x0 pc=0x";
#endif
    std::string tail = "]\n\ngoroutine 1 [running]:\n";
    CHECK_EQ(out.substr(0, head.size()), head);
    CHECK(out.size() > head.size() + tail.size());
    if (out.size() > head.size() + tail.size()) {
        CHECK_EQ(out.substr(out.size() - tail.size()), tail);
        std::string pc = out.substr(head.size(), out.size() - head.size() - tail.size());
        CHECK(pc.find_first_not_of("0123456789abcdef") == std::string::npos);
        CHECK(pc != "0");
    }
    // The signal belongs to the goroutine, not to the panic value: a later panic, as from a
    // deferred call that panics while the nil pointer panic goes on (step 3.13), still prints
    // it, once, after its text. In Go:
    //
    //     panic: runtime error: invalid memory address or nil pointer dereference
    //         panic: boom
    //     [signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x47e84b]
    //
    // Step 3.13 passes the chain; here the report of the last panic shows where the line goes.
    std::string boom = report_of("boom");
    CHECK_EQ(boom.substr(0, 12), std::string("panic: boom\n"));
    CHECK(boom.find("[signal ") == 12);
    // A recover that stops the panic clears it (Phase 05 calls clear_signal); an ordinary
    // panic after that has no signal line.
    gp::detail::clear_signal();
    CHECK_EQ(report_of("later"), std::string("panic: later\n\ngoroutine 1 [running]:\n"));
    CHECK_PANIC(gp::panic("x"_gs), "x");
    CHECK_PANIC(gp::panic_divide(), divide_msg);
    CHECK_EQ(report_of("x"), std::string("panic: x\n\ngoroutine 1 [running]:\n"));
}

void test_ptr_ownership() {
    // A boxed variable outlives its function through the pointer.
    {
        gp::ptr<Tracked> p = boxed_escape(41);
        CHECK_EQ(live, 1);
        CHECK_EQ(p->n, INT64_C(42));
        CHECK_EQ(use_count(p), INT64_C(1));
    }
    CHECK_EQ(live, 0);

    // An interior pointer (&p.pt.Y) keeps the whole block alive.
    gp::ptr<int64_t> field;
    gp::ptr<Point> sub;
    {
        gp::ptr<Tracked> p = gp::new_<Tracked>(Tracked(1));
        field = gp::ptr<int64_t>(p, &p->pt.Y);
        sub = gp::ptr<Point>(p, &p->pt);
        CHECK_EQ(use_count(p), INT64_C(3));
        *field = 9;
        CHECK_EQ(p->pt.Y, INT64_C(9));
        CHECK(field.gp_get() == &sub->Y);
    }
    CHECK_EQ(live, 1); // the Tracked lives on through field and sub
    CHECK_EQ(*field, INT64_C(9));
    CHECK_EQ(sub->Y, INT64_C(9));
    sub = nullptr;
    CHECK_EQ(live, 1);
    field = nullptr;
    CHECK_EQ(live, 0);

    // An interior pointer made from a pointer that is moved in takes over its count.
    {
        gp::ptr<Tracked> p = gp::new_<Tracked>(Tracked(1));
        gp::ptr<Tracked> keep = p;
        gp::ptr<int64_t> n(std::move(p), &keep->n);
        CHECK(p == nullptr);
        CHECK_EQ(use_count(keep), INT64_C(2));
        CHECK(n.gp_get() == &keep->n);
    }
    CHECK_EQ(live, 0);

    // &s[i]: an element of a shared array (a slice's backing array, step 3.5) keeps the array.
    gp::ptr<int64_t> elem;
    {
        std::shared_ptr<int64_t[]> arr = std::make_shared<int64_t[]>(4);
        elem = gp::ptr<int64_t>(arr, &arr[2]);
        *elem = 42;
        CHECK_EQ(arr[2], INT64_C(42));
        CHECK_EQ(arr[1], INT64_C(0)); // make_shared<T[]> value-initializes: Go's zero values
        CHECK_EQ(arr.use_count(), 2L);
        gp::ptr<int64_t> e2(arr, &arr[2]);
        CHECK(e2 == elem);
        CHECK(gp::ptr<int64_t>(arr, &arr[3]) != elem);
    }
    CHECK_EQ(*elem, INT64_C(42)); // the array is still there
    CHECK_EQ(elem.gp_shared().use_count(), 1L);
    elem = nullptr;

    // Copies and moves (what they cost: one count change per copy, none per move).
    gp::ptr<Tracked> a = gp::new_<Tracked>();
    CHECK_EQ(use_count(a), INT64_C(1));
    gp::ptr<Tracked> b = a;
    CHECK_EQ(use_count(a), INT64_C(2));
    gp::ptr<Tracked> c = std::move(b);
    CHECK(b == nullptr);
    CHECK_EQ(use_count(a), INT64_C(2));
    c = std::move(a);
    CHECK(a == nullptr);
    CHECK_EQ(use_count(c), INT64_C(1));
    CHECK_EQ(live, 1);
    c = nullptr;
    CHECK_EQ(live, 0);
    CHECK(std::is_nothrow_copy_constructible_v<gp::ptr<Tracked>>);
    CHECK(std::is_nothrow_move_constructible_v<gp::ptr<Tracked>>);
    CHECK(std::is_nothrow_copy_assignable_v<gp::ptr<Tracked>>);
    CHECK(std::is_nothrow_move_assignable_v<gp::ptr<Tracked>>);
    // A dereference and -> don't copy the pointer: they return the pointee itself.
    CHECK_TYPE(gp::ptr<Tracked>().operator->(), Tracked*);
    CHECK_TYPE(*gp::ptr<Tracked>(), Tracked&);

    // Hashing, for map keys (step 3.7): by address, like ==.
    gp::ptr<int64_t> h1 = gp::new_<int64_t>(), h2 = gp::new_<int64_t>();
    gp::ptr<int64_t> h1b = h1;
    std::hash<gp::ptr<int64_t>> hash;
    CHECK(hash(h1) == hash(h1b));
    CHECK(hash(gp::ptr<int64_t>()) == hash(nullptr));
    std::unordered_set<gp::ptr<int64_t>> set{h1, h2, h1b, nullptr};
    CHECK_EQ(set.size(), std::size_t(3));
    CHECK(set.count(h1b) == 1);
}

void test_ptr_borrowed() {
    // gp::bptr<T> (ADR-086): a raw pointer with gp::ptr's checks and no count.
    static_assert(sizeof(gp::bptr<Tracked>) == sizeof(Tracked*));
    static_assert(std::is_trivially_copyable_v<gp::bptr<Tracked>>);
    static_assert(std::is_same_v<gp::detail::owning_t<gp::bptr<int64_t>>, gp::ptr<int64_t>>);
    static_assert(std::is_same_v<gp::detail::owning_t<gp::ptr<int64_t>>, gp::ptr<int64_t>>);
    static_assert(std::is_same_v<gp::detail::owning_t<int64_t>, int64_t>);
    static_assert(std::is_same_v<gp::detail::owning_t<const gp::string&>, gp::string>);
    static_assert(std::is_same_v<gp::detail::owning_t<const gp::ptr<int64_t>&>, gp::ptr<int64_t>>);
    static_assert(std::is_same_v<gp::detail::owning_t<const gp::bptr<int64_t>&>, gp::ptr<int64_t>>);

    gp::ptr<Point> p = gp::new_<Point>(Point{3, 4});
    long uses = p.gp_shared().use_count();
    {
        gp::bptr<Point> b = gp::lend(p); // no count
        CHECK_EQ(p.gp_shared().use_count(), uses);
        CHECK(b == p);
        CHECK(b != nullptr);
        CHECK(b.gp_get() == p.gp_get());
        b->X = 7;
        CHECK_EQ((*b).Y, INT64_C(4));
        CHECK_EQ(p->X, INT64_C(7));
        gp::bptr<Point> c = b;
        CHECK(c == b);
        gp::bptr<int64_t> y = gp::borrow(&p->Y); // a field: no aliasing pointer
        *y = 9;
        CHECK_EQ(p->Y, INT64_C(9));
        CHECK_EQ(p.gp_shared().use_count(), uses);
        gp::bptr<Point> implicit = p; // a gp::ptr converts (C++ callers)
        CHECK(implicit == b);
        CHECK(gp::lend(b) == b);
    }
    // nil: ==, and the first dereference panics as gp::ptr's does
    gp::bptr<Point> nil;
    CHECK(nil == nullptr);
    CHECK(nil == gp::ptr<Point>());
    CHECK(gp::bptr<Point>(nullptr) == nil);
    CHECK(gp::lend(gp::ptr<Point>()) == nullptr);
    CHECK_PANIC((void)nil->X, nil_msg);
    CHECK_PANIC((void)*nil, nil_msg);
    CHECK_PANIC(gp::check_wrap(nil, "value method main.T.M called using nil *T pointer"),
                "value method main.T.M called using nil *T pointer");
    // a temporary lives until the end of the full expression
    auto sum = [](gp::bptr<Point> q) { return q->X + q->Y; };
    CHECK_EQ(sum(gp::new_<Point>(Point{1, 2})), INT64_C(3));
    CHECK_EQ(sum(p), INT64_C(16));
    // the raw pointer for C
    Point* raw = gp::bptr<Point>(p);
    CHECK(raw == p.gp_get());
    // field_ptr of a borrowed pointer: a borrowed pointer to the field, nil panics (the nil check
    // of a promoted method's receiver)
    gp::bptr<int64_t> fy = gp::field_ptr(gp::lend(p), &Point::Y);
    CHECK(fy.gp_get() == &p->Y);
    CHECK_PANIC((void)*gp::field_ptr(nil, &Point::Y), nil_msg);
}

int64_t adapt_target(gp::bptr<Point> p, const gp::string& s, int64_t k) {
    return p->X + gp::len(s) + k;
}
void adapt_void(const gp::slice<int64_t>& xs, gp::bptr<Point> p) {
    p->Y = gp::len(xs);
}

void test_adapt() {
    // gp::adapt<&f>::call takes the Go-level parameters of f (ADR-086): an owning pointer for a
    // borrowed one, a value for a const reference.
    gp::ptr<Point> p = gp::new_<Point>(Point{3, 4});
    long uses = p.gp_shared().use_count();
    auto f = &gp::adapt<&adapt_target>::call;
    static_assert(std::is_same_v<decltype(f), int64_t (*)(gp::ptr<Point>, gp::string, int64_t)>);
    CHECK_EQ(f(p, "four"_gs, 10), INT64_C(17));
    CHECK_EQ(p.gp_shared().use_count(), uses); // the temporary parameter copy is gone again
    auto g = &gp::adapt<&adapt_void>::call;
    static_assert(std::is_same_v<decltype(g), void (*)(gp::slice<int64_t>, gp::ptr<Point>)>);
    g(gp::slice_of<int64_t>({1, 2, 3}), p);
    CHECK_EQ(p->Y, INT64_C(3));
    // one adapter for each function: a func value of it has one address
    CHECK(&gp::adapt<&adapt_target>::call == &gp::adapt<&adapt_target>::call);
    gp::func<int64_t(gp::ptr<Point>, gp::string, int64_t)> fv =
        gp::func_of<&gp::adapt<&adapt_target>::call>();
    CHECK_EQ(fv(p, "ab"_gs, 1), INT64_C(6));
}

void test_ptr_static() {
    // &global and &global.f share the static owner, which is never freed.
    CHECK(&gp::static_owner() == &gp::static_owner());
    long before = gp::static_owner().use_count();
    CHECK(before >= 1);
    gp::weak_ptr<Point> w;
    {
        gp::ptr<Point> g = gp::static_ptr(global_point);
        CHECK(g == gp::static_ptr(global_point));
        CHECK(g.gp_get() == &global_point);
        g->X = 3;
        CHECK_EQ(global_point.X, INT64_C(3));
        gp::ptr<int64_t> gy = gp::static_ptr(global_point.Y);
        CHECK(gy == gp::ptr<int64_t>(g, &g->Y)); // &global.Y == &(&global).Y
        CHECK(gp::static_ptr(global_point.X) != gy);
        CHECK_EQ(gp::static_owner().use_count(), before + 2);
        gp::ptr<int64_t> e = gp::static_ptr(global_arr[1]);
        *e = 9;
        CHECK_EQ(global_arr[1], INT64_C(9));
        w = gp::weak_make(g);
    }
    CHECK_EQ(gp::static_owner().use_count(), before);
    // No pointer to the global is left, but it is never freed: weak.Make(&global) never
    // expires. (ASan would report a delete of a global.)
    CHECK(gp::weak_value(w) == gp::static_ptr(global_point));
    CHECK(w == gp::weak_make(gp::static_ptr(global_point)));
    for (int i = 0; i < 1000; i++) {
        gp::ptr<Point> g = gp::static_ptr(global_point);
        g->Y = i;
    }
    CHECK_EQ(global_point.Y, INT64_C(999));
    CHECK(gp::static_ptr(global_point) != nullptr);
}

void test_ptr_weak() {
    // The zero weak pointer and weak.Make(nil): Value is nil, and they are equal.
    gp::weak_ptr<Tracked> zero;
    CHECK(gp::weak_value(zero) == nullptr);
    CHECK(gp::weak_make(gp::ptr<Tracked>()) == zero);
    CHECK(gp::weak_make(gp::ptr<Tracked>()).gp_addr() == 0);

    gp::weak_ptr<Tracked> w, w2;
    gp::weak_ptr<int64_t> wf;
    {
        gp::ptr<Tracked> p = gp::new_<Tracked>();
        w = gp::weak_make(p);
        w2 = gp::weak_make(p);
        wf = gp::weak_make(gp::ptr<int64_t>(p, &p->n));
        CHECK(gp::weak_value(w) == p);
        CHECK(w == w2);
        CHECK(w != zero);
        CHECK_EQ(use_count(p), INT64_C(1)); // a weak pointer doesn't count
        CHECK(gp::weak_value(wf) == gp::ptr<int64_t>(p, &p->n));
        gp::ptr<Tracked> other = gp::new_<Tracked>();
        CHECK(gp::weak_make(other) != w);
        CHECK_EQ(live, 2);
    }
    // The last pointer went away: the object is freed, and Value is nil.
    CHECK_EQ(live, 0);
    CHECK(gp::weak_value(w) == nullptr);
    CHECK(gp::weak_value(wf) == nullptr);
    // Still equal after the target is freed, as in Go; and not equal to the zero value.
    CHECK(w == w2);
    CHECK(w != zero);
    // A new object gives a different weak pointer (the allocator may or may not reuse the
    // address; the case below makes the same address with different owners on purpose).
    gp::ptr<Tracked> again = gp::new_<Tracked>();
    CHECK(gp::weak_make(again) != w);
    {
        // Two pointers to one address with two owners: equal as pointers, but their weak
        // pointers differ, because the owners differ. (This breaks the invariant that one
        // object has one owner, which generated code never does; it shows that weak == also
        // compares the owner, which is what tells a freed object from a new one at its
        // address.)
        gp::ptr<Tracked> o1 = gp::new_<Tracked>(), o2 = gp::new_<Tracked>();
        gp::ptr<int64_t> a(o1, &o1->n), b(o2, &o1->n);
        CHECK(a == b);
        CHECK(gp::weak_make(a) != gp::weak_make(b));
        CHECK(gp::weak_make(a) == gp::weak_make(gp::ptr<int64_t>(o1, &o1->n)));
        CHECK(gp::weak_make(a).gp_addr() == gp::weak_make(b).gp_addr());
    }
    std::hash<gp::weak_ptr<Tracked>> hash;
    CHECK(hash(w) == hash(w2));
    again = nullptr;

    // A parent and a child that points back with a weak pointer: freeing the parent frees both
    // (LeakSanitizer checks it in the asan mode).
    {
        gp::ptr<Parent> parent = gp::new_<Parent>();
        parent->child = gp::new_<Child>();
        parent->child->parent = gp::weak_make(parent);
        CHECK(gp::weak_value(parent->child->parent) == parent);
        CHECK_EQ(live, 2);
    }
    CHECK_EQ(live, 0);

    // A cycle of strong pointers is never freed (D1). The test breaks it by hand afterwards,
    // so LeakSanitizer stays quiet.
    gp::weak_ptr<Parent> wp;
    {
        gp::ptr<Parent> parent = gp::new_<Parent>();
        parent->child = gp::new_<Child>();
        parent->child->strong = parent; // the cycle
        wp = gp::weak_make(parent);
    }
    CHECK_EQ(live, 2); // leaked
    gp::ptr<Parent> back = gp::weak_value(wp);
    CHECK(back != nullptr);
    back->child->strong = nullptr; // break the cycle
    back = nullptr;
    CHECK_EQ(live, 0);
    CHECK(gp::weak_value(wp) == nullptr);
}

// --- freeing long chains (detail::release) ---

// Link and Tree count their frees. (Only for the tests: Go types have no destructors.)
int64_t freed = 0;

struct Link {
    gp::ptr<Link> next;
    ~Link() { freed++; }
};

struct Tree {
    gp::ptr<Tree> left;
    gp::ptr<Tree> right;
    ~Tree() { freed++; }
};

// list builds a list of n links, without recursion.
gp::ptr<Link> list(int64_t n) {
    gp::ptr<Link> head;
    for (int64_t i = 0; i < n; i++) {
        gp::ptr<Link> l = gp::new_<Link>();
        l->next = std::move(head);
        head = std::move(l);
    }
    return head;
}

// spine builds a tree that is n nodes deep on one side, with a leaf on the other side of each
// node: 2n nodes.
gp::ptr<Tree> spine(int64_t n, bool left) {
    gp::ptr<Tree> top;
    for (int64_t i = 0; i < n; i++) {
        gp::ptr<Tree> t = gp::new_<Tree>();
        (left ? t->left : t->right) = std::move(top);
        (left ? t->right : t->left) = gp::new_<Tree>();
        top = std::move(t);
    }
    return top;
}

bool nothing_pending() {
    return gp::detail::release_tls.gp_depth == 0 && gp::detail::release_tls.gp_pending == nullptr;
}

// big_tests: GOCPP_TEST_BIG=1 in the environment also runs the big cases (2,000,000 nodes,
// about 100 MB, more with ASan), which make ci leaves out.
bool big_tests() {
    for (const gp::string& e : gp::envs()) {
        if (e == "GOCPP_TEST_BIG=1"_gs) {
            return true;
        }
    }
    return false;
}

// The chain cases, each run with n nodes. Without detail::release, freeing 100,000 links one
// from the destructor of the next overflowed the 8 MB stack at -O0 (1,000,000 at -O2); each
// case below crashes at -O0 with release's depth limit taken away (checked by hand). n is
// 200,000 in make ci, and 2,000,000 more with GOCPP_TEST_BIG=1.
void chain_list_assign(int64_t n) {
    freed = 0;
    gp::ptr<Link> l = list(n);
    l = nullptr; // dropped by an assignment
    CHECK_EQ(freed, n);
}

void chain_list_scope(int64_t n) {
    freed = 0;
    {
        gp::ptr<Link> scoped = list(n); // dropped at the end of the scope (~ptr)
    }
    CHECK_EQ(freed, n);
}

void chain_list_middle(int64_t n) {
    // The last pointer into the middle of a list goes; a weak pointer to it then gives nil.
    freed = 0;
    gp::ptr<Link> a = list(n);
    gp::ptr<Link> b = a->next;
    gp::weak_ptr<Link> w = gp::weak_make(b);
    a = nullptr;
    CHECK_EQ(freed, INT64_C(1));
    b = nullptr;
    CHECK_EQ(freed, n);
    CHECK(gp::weak_value(w) == nullptr);
}

// Trees n nodes deep on one side, with a leaf on the other side of each node.
void chain_tree_left(int64_t n) {
    freed = 0;
    gp::ptr<Tree> t = spine(n, true);
    t = nullptr;
    CHECK_EQ(freed, 2 * n);
}

void chain_tree_right(int64_t n) {
    freed = 0;
    gp::ptr<Tree> t = spine(n, false);
    t = nullptr;
    CHECK_EQ(freed, 2 * n);
}

void test_ptr_chains() {
    void (*const cases[])(int64_t) = {chain_list_assign, chain_list_scope, chain_list_middle,
                                      chain_tree_left, chain_tree_right};
    std::vector<int64_t> sizes = {200'000};
    if (big_tests()) {
        sizes.push_back(2'000'000);
    }
    for (int64_t n : sizes) {
        for (auto f : cases) {
            f(n);
            CHECK(nothing_pending());
        }
    }

    // A short list is freed at once, without the pending list.
    freed = 0;
    gp::ptr<Link> s = list(gp::detail::release_max_depth - 10);
    s = nullptr;
    CHECK_EQ(freed, INT64_C(gp::detail::release_max_depth - 10));
    CHECK(nothing_pending());
}

// The "no plain-named members" rule (Phase 03 §2.1): a runtime type has only operators and
// gp_-prefixed members, so a Go method or field of any other name can't clash with one.
// C++20 can't list a class's members, so the test asks for a set of names that a wrapper
// would likely have (the std::shared_ptr and std::string ones) and common Go method names.
// has_member_<name><T> finds a member of any kind, public or private, a data member, a member
// function (also overloaded or a template) or a member type: the probe inherits from T and
// from a base with a member of that name, so &probe::name is ambiguous exactly when T has a
// member with the name too.
#define GP_MEMBER_PROBE(name)                                                                      \
    struct probe_base_##name {                                                                     \
        int name;                                                                                  \
    };                                                                                             \
    template <typename T>                                                                          \
    struct probe_##name : T, probe_base_##name {};                                                 \
    template <typename T>                                                                          \
    constexpr bool has_member_##name = !requires { &probe_##name<T>::name; };

// clang-format off
#define GP_PLAIN_NAMES(X)                                                                          \
    X(get) X(reset) X(swap) X(use_count) X(unique) X(owner_before) X(lock) X(expired)              \
    X(release) X(value) X(data) X(size) X(len) X(cap) X(length) X(empty) X(begin) X(end)           \
    X(front) X(back) X(push_back) X(append) X(c_str) X(str) X(at) X(find) X(insert) X(erase)       \
    X(clear) X(copy) X(view) X(text) X(signal) X(next) X(type) X(kind) X(hash) X(element_type)     \
    X(value_type) X(Value) X(String) X(Error) X(Len) X(Get) X(Set) X(Reset) X(Make)
// clang-format on

GP_PLAIN_NAMES(GP_MEMBER_PROBE)

#define GP_HAS_ANY_PLAIN_MEMBER(name) || has_member_##name<T>

template <typename T>
constexpr bool has_plain_member = false GP_PLAIN_NAMES(GP_HAS_ANY_PLAIN_MEMBER);

// The probe works: std::shared_ptr has get, reset and element_type; a struct with a private
// member is found too.
struct with_private {
private:
    [[maybe_unused]] int len = 0;
};
static_assert(has_member_get<std::shared_ptr<int>>);
static_assert(has_member_reset<std::shared_ptr<int>>);
static_assert(has_member_element_type<std::shared_ptr<int>>);
static_assert(has_member_lock<std::weak_ptr<int>>);
static_assert(has_member_len<with_private>);
static_assert(!has_member_get<with_private>);
static_assert(has_plain_member<std::string>);

static_assert(!has_plain_member<gp::string>);
static_assert(!has_plain_member<gp::ptr<int64_t>>);
static_assert(!has_plain_member<gp::ptr<Node>>);
static_assert(!has_plain_member<gp::slice<int64_t>>);
static_assert(!has_plain_member<gp::slice<Node>>);
static_assert(!has_plain_member<gp::ptr<void>>);
static_assert(!has_plain_member<gp::weak_ptr<int64_t>>);
static_assert(!has_plain_member<gp::weak_ptr<Node>>);
static_assert(!has_plain_member<gp::panic_error>);
static_assert(!has_plain_member<gp::static_bytes_t>);

// --- run_main ---

bool init_ran = false;

void test_init() {
    init_ran = true;
}

void test_run_main() {
    CHECK(init_ran);
    const auto& args = gp::args();
    CHECK_EQ(args.size(), std::size_t(3));
    if (args.size() == 3) {
        CHECK_EQ(args[1], "arg1"_gs);
        CHECK_EQ(args[2], "arg two"_gs);
    }
    bool found = false;
    for (const gp::string& e : gp::envs()) {
        found = found || e == "GOCPP_RT_TEST=env-ok"_gs;
    }
    CHECK(found);
}

void run_tests() {
    test_run_main();
    test_string();
    test_substring();
    test_concat();
    test_utf8();
    test_div();
    test_shift();
    test_f2i();
    test_print();
    test_fmt();
    test_panic();
    test_complex();
    test_ptr_basics();
    test_ptr_nil();
    test_ptr_ownership();
    test_ptr_static();
    test_ptr_borrowed();
    test_adapt();
    test_ptr_weak();
    test_ptr_chains();
    if (failures > 0) {
        std::string msg =
            "FAIL: " + std::to_string(failures) + " of " + std::to_string(checks) + " checks\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
        gp::os::exit(1);
    }
    gp::fmt::Printf("ok: %d checks\n"_gs, int64_t(checks));
}

} // namespace

int main(int argc, char** argv) {
    return gp::run_main(argc, argv, &test_init, &run_tests);
}
