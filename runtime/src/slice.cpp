// The parts of slices (gocpp/slice.hpp) and of the bounds checks (gocpp/bounds.hpp) that are
// not templates: the panics with Go's messages, the capacity of a growing slice, and the
// conversions between strings and slices.
#include <gocpp/rt.hpp>

#include <cstddef>
#include <cstdint>
#include <cstring>
#include <memory>
#include <string>
#include <string_view>

namespace gp {

namespace {

// Go's boundsErrorFmts and boundsNegErrorFmts (runtime/error.go), in the order of
// gp::bounds. %x is the index that failed, %y what it was compared with. The second table is
// for a negative signed %x, which Go prints without %y.
constexpr const char* bounds_fmts[] = {
    "slice bounds out of range [:%x] with length %y",
    "slice bounds out of range [:%x] with capacity %y",
    "slice bounds out of range [%x:%y]",
    "slice bounds out of range [::%x] with length %y",
    "slice bounds out of range [::%x] with capacity %y",
    "slice bounds out of range [:%x:%y]",
    "slice bounds out of range [%x:%y:]",
    "cannot convert slice with length %y to array or pointer to array with length %x",
};

constexpr const char* bounds_neg_fmts[] = {
    "slice bounds out of range [:%x]",
    "slice bounds out of range [:%x]",
    "slice bounds out of range [%x:]",
    "slice bounds out of range [::%x]",
    "slice bounds out of range [::%x]",
    "slice bounds out of range [:%x:]",
    "slice bounds out of range [%x::]",
    "cannot convert slice with length %y to array or pointer to array with length %x",
};

[[noreturn]] void panic_text(const std::string& text) {
    detail::throw_runtime(detail::rt_error::bounds, gp::string::gp_copy(text));
}

// Go's malloc size classes (internal/runtime/gc/sizeclasses.go, Go 1.25), without class 0.
constexpr uint16_t size_classes[] = {
    8,     16,    24,    32,    48,    64,    80,    96,    112,   128,   144,   160,
    176,   192,   208,   224,   240,   256,   288,   320,   352,   384,   416,   448,
    480,   512,   576,   640,   704,   768,   896,   1024,  1152,  1280,  1408,  1536,
    1792,  2048,  2304,  2688,  3072,  3200,  3456,  4096,  4864,  5376,  6144,  6528,
    6784,  6912,  8192,  9472,  9728,  10240, 10880, 12288, 13568, 14336, 16384, 18432,
    19072, 20480, 21760, 24576, 27264, 28672, 32768,
};

constexpr uint64_t max_small_size = 32768;
constexpr uint64_t malloc_header_size = 8;
constexpr uint64_t page_size = 8192;
// gc.MinSizeForMallocHeader: goarch.PtrSize * ptrBits (8 * 64): above it, an object with
// pointers has a malloc header instead of heap bits in its span.
constexpr uint64_t min_size_for_malloc_header = 8 * 64;

// roundupsize: Go's roundupsize(size, noscan) (runtime/msize.go): the usable size of the block
// that Go's malloc gives for size bytes. A small size goes up to its size class; a block of
// elements with pointers over 512 bytes (gc.MinSizeForMallocHeader) has an 8-byte header that
// the size class includes and the result doesn't, so such a size is rounded up as size + 8 and
// the 8 are taken off again. A large size goes up to a multiple of the page size.
uint64_t roundupsize(uint64_t size, bool noscan) {
    if (size == 0) {
        return 0; // size class 0: Go's malloc(0) is zerobase
    }
    if (size <= max_small_size - malloc_header_size) {
        uint64_t req = size;
        if (!noscan && req > min_size_for_malloc_header) {
            req += malloc_header_size;
        }
        std::size_t lo = 0, hi = sizeof size_classes / sizeof size_classes[0];
        while (lo < hi) { // the first class >= req
            std::size_t mid = (lo + hi) / 2;
            if (size_classes[mid] < req) {
                lo = mid + 1;
            } else {
                hi = mid;
            }
        }
        return size_classes[lo] - (req - size);
    }
    uint64_t req = size + page_size - 1;
    if (req < size) {
        return size;
    }
    return req & ~(page_size - 1);
}

// nextslicecap: Go's nextslicecap (runtime/slice.go), in unsigned arithmetic. new_len > 0.
uint64_t nextslicecap(uint64_t new_len, uint64_t old_cap) {
    uint64_t newcap = old_cap;
    uint64_t doublecap = newcap + newcap;
    if (new_len > doublecap) {
        return new_len;
    }
    constexpr uint64_t threshold = 256;
    if (old_cap < threshold) {
        return doublecap;
    }
    do {
        // Go's smooth transition from growing 2x for small slices to 1.25x for large ones.
        newcap += (newcap + 3 * threshold) >> 2;
    } while (newcap < new_len);
    return newcap;
}

[[noreturn]] void panic_growslice() {
    throw panic_error{"runtime error: growslice: len out of range"_gs,
                      {panic_identity::gp_runtime, 0, 0}};
}

} // namespace

void panic_bounds(bounds code, int64_t x, bool is_signed, int64_t y) {
    auto i = static_cast<std::size_t>(code);
    const char* fmt = is_signed && x < 0 ? bounds_neg_fmts[i] : bounds_fmts[i];
    std::string text = "runtime error: ";
    for (const char* p = fmt; *p != '\0'; p++) {
        if (*p != '%') {
            text += *p;
            continue;
        }
        p++;
        if (*p == 'x') {
            if (is_signed) {
                detail::print_int(text, x);
            } else {
                detail::print_uint(text, static_cast<uint64_t>(x));
            }
        } else {
            detail::print_int(text, y);
        }
    }
    panic_text(text);
}

namespace detail {

void panic_make(int64_t len, int64_t /*cap*/, std::size_t size) {
    // Go's makeslice: when the capacity is bad, it reports the length if the length alone is
    // bad too (make([]T, huge) has only a length), and the capacity otherwise.
    // make([]T, huge_len, small_cap) says "len out of range" here, which is what Go says when
    // the slice escapes to the heap (runtime.makeslice). For a slice that doesn't escape, Go's
    // compiler emits its own checks, and they say "cap out of range"; gocpp always gives the
    // heap form.
    auto limit =
        size == 0 ? static_cast<uint64_t>(INT64_MAX) : static_cast<uint64_t>(max_alloc) / size;
    if (len < 0 || static_cast<uint64_t>(len) > limit) {
        throw panic_error{"runtime error: makeslice: len out of range"_gs,
                          {panic_identity::gp_runtime, 0, 0}};
    }
    throw panic_error{"runtime error: makeslice: cap out of range"_gs,
                      {panic_identity::gp_runtime, 0, 0}};
}

uint64_t roundup_size(uint64_t size, bool noscan) {
    return roundupsize(size, noscan);
}

int64_t grow_cap(int64_t new_len, int64_t old_cap, uint64_t size, bool noscan) {
    if (new_len < 0) {
        panic_growslice();
    }
    if (size == 0) {
        // Go's growslice for a zero-size element: no memory, and cap == len.
        return new_len;
    }
    uint64_t newcap = nextslicecap(static_cast<uint64_t>(new_len), static_cast<uint64_t>(old_cap));
    if (newcap > static_cast<uint64_t>(max_alloc) / size) {
        panic_growslice();
    }
    uint64_t mem = roundupsize(newcap * size, noscan);
    if (mem > static_cast<uint64_t>(max_alloc)) {
        panic_growslice();
    }
    return static_cast<int64_t>(mem / size);
}

} // namespace detail

// --- the conversions between strings and slices (slice.hpp) ---

namespace {

// rune_len: the number of bytes of the UTF-8 encoding of r, or of U+FFFD (3) if r is not a
// code point, as Go's encoderune does.
int64_t rune_len(int32_t r) {
    if (r < 0 || r > 0x10FFFF || (r >= 0xD800 && r <= 0xDFFF)) {
        return 3;
    }
    return r < 0x80 ? 1 : r < 0x800 ? 2 : r < 0x10000 ? 3 : 4;
}

// put_rune writes the UTF-8 of r (U+FFFD if r is not a code point) at p and returns the end.
char* put_rune(char* p, int32_t r) {
    if (r < 0 || r > 0x10FFFF || (r >= 0xD800 && r <= 0xDFFF)) {
        r = 0xFFFD;
    }
    auto u = static_cast<uint32_t>(r);
    if (u < 0x80) {
        *p++ = static_cast<char>(u);
    } else if (u < 0x800) {
        *p++ = static_cast<char>(0xC0 | u >> 6);
        *p++ = static_cast<char>(0x80 | (u & 0x3F));
    } else if (u < 0x10000) {
        *p++ = static_cast<char>(0xE0 | u >> 12);
        *p++ = static_cast<char>(0x80 | (u >> 6 & 0x3F));
        *p++ = static_cast<char>(0x80 | (u & 0x3F));
    } else {
        *p++ = static_cast<char>(0xF0 | u >> 18);
        *p++ = static_cast<char>(0x80 | (u >> 12 & 0x3F));
        *p++ = static_cast<char>(0x80 | (u >> 6 & 0x3F));
        *p++ = static_cast<char>(0x80 | (u & 0x3F));
    }
    return p;
}

template <typename T>
slice<T> empty_slice() {
    return slice<T>(std::shared_ptr<void>(), detail::empty_data<T>(), 0, 0);
}

} // namespace

namespace {

// make_bytes: a slice of n bytes that are the string's, with capacity cap >= n.
slice<uint8_t> make_bytes(const string& s, int64_t n, int64_t cap) {
    std::shared_ptr<uint8_t[]> a = detail::new_array<uint8_t>(cap, n);
    uint8_t* d = a.get();
    std::memcpy(d, s.gp_data(), static_cast<std::size_t>(n));
    return slice<uint8_t>(std::shared_ptr<void>(std::move(a), d), d, n, cap);
}

// count_runes: Go's countrunes: the number of runes that range over s gives.
int64_t count_runes(const char* p, int64_t n) {
    int64_t count = 0;
    for (int64_t i = 0; i < n; count++) {
        if (static_cast<uint8_t>(p[i]) < 0x80) {
            i++;
            continue;
        }
        int64_t width = 1;
        detail::decode_rune(p + i, n - i, width);
        i += width;
    }
    return count;
}

// make_runes: a slice of count runes, the string's, with capacity cap >= count.
slice<int32_t> make_runes(const string& s, int64_t count, int64_t cap) {
    const char* p = s.gp_data();
    int64_t n = gp::len(s);
    std::shared_ptr<int32_t[]> a = detail::new_array<int32_t>(cap, count);
    int32_t* d = a.get();
    for (int64_t i = 0, k = 0; i < n; k++) {
        auto c = static_cast<uint8_t>(p[i]);
        if (c < 0x80) {
            d[k] = c;
            i++;
            continue;
        }
        int64_t width = 1;
        d[k] = detail::decode_rune(p + i, n - i, width);
        i += width;
    }
    return slice<int32_t>(std::shared_ptr<void>(std::move(a), d), d, count, cap);
}

} // namespace

slice<uint8_t> to_bytes(const string& s) {
    int64_t n = gp::len(s);
    if (n == 0) {
        return empty_slice<uint8_t>();
    }
    // Go's rawbyteslice: the capacity is the size class that its malloc gives.
    return make_bytes(s, n, static_cast<int64_t>(roundupsize(static_cast<uint64_t>(n), true)));
}

slice<uint8_t> to_bytes_const(const string& s) {
    int64_t n = gp::len(s);
    if (n == 0) {
        return empty_slice<uint8_t>();
    }
    return make_bytes(s, n, n);
}

slice<int32_t> to_runes(const string& s) {
    int64_t n = gp::len(s);
    if (n == 0) {
        return empty_slice<int32_t>();
    }
    int64_t count = count_runes(s.gp_data(), n);
    // Go's rawruneslice: the capacity is the size class of count * 4 bytes, in runes.
    uint64_t mem = roundupsize(static_cast<uint64_t>(count) * 4, true);
    return make_runes(s, count, static_cast<int64_t>(mem / 4));
}

slice<int32_t> to_runes_const(const string& s) {
    int64_t n = gp::len(s);
    if (n == 0) {
        return empty_slice<int32_t>();
    }
    int64_t count = count_runes(s.gp_data(), n);
    return make_runes(s, count, count);
}

string to_string(const slice<uint8_t>& b) {
    if (gp::len(b) == 0) {
        return string();
    }
    return string::gp_copy(std::string_view(reinterpret_cast<const char*>(b.gp_data()),
                                            static_cast<std::size_t>(gp::len(b))));
}

string to_string(const slice<int32_t>& r) {
    int64_t n = gp::len(r);
    const int32_t* d = r.gp_data();
    int64_t total = 0;
    for (int64_t i = 0; i < n; i++) {
        total += rune_len(d[i]);
    }
    if (total == 0) {
        return string();
    }
    auto bytes = std::make_shared_for_overwrite<char[]>(static_cast<std::size_t>(total));
    char* p = bytes.get();
    for (int64_t i = 0; i < n; i++) {
        p = put_rune(p, d[i]);
    }
    const char* data = bytes.get();
    return string(std::move(bytes), data, total);
}

} // namespace gp
