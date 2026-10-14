// The parts of slices (gocpp/slice.hpp) and of the bounds checks (gocpp/bounds.hpp) that are
// not templates: the panics with Go's messages, and the capacity of a growing slice.
#include <gocpp/rt.hpp>

#include <cstddef>
#include <cstdint>
#include <string>

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
    throw panic_error{gp::string::gp_copy(text)};
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

// roundupsize: Go's roundupsize(size, noscan=true) (runtime/msize.go): the size of the block
// that Go's malloc gives for size bytes. A small size goes up to its size class, a large one
// to a multiple of the page size.
uint64_t roundupsize(uint64_t size) {
    if (size <= max_small_size - malloc_header_size) {
        std::size_t lo = 0, hi = sizeof size_classes / sizeof size_classes[0];
        while (lo < hi) { // the first class >= size
            std::size_t mid = (lo + hi) / 2;
            if (size_classes[mid] < size) {
                lo = mid + 1;
            } else {
                hi = mid;
            }
        }
        return size_classes[lo];
    }
    return (size + page_size - 1) & ~(page_size - 1);
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
    throw panic_error{"runtime error: growslice: len out of range"_gs};
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
    auto limit = static_cast<uint64_t>(max_alloc) / size;
    if (len < 0 || static_cast<uint64_t>(len) > limit) {
        throw panic_error{"runtime error: makeslice: len out of range"_gs};
    }
    throw panic_error{"runtime error: makeslice: cap out of range"_gs};
}

int64_t grow_cap(int64_t new_len, int64_t old_cap, std::size_t size) {
    if (new_len < 0) {
        panic_growslice();
    }
    uint64_t newcap = nextslicecap(static_cast<uint64_t>(new_len), static_cast<uint64_t>(old_cap));
    if (newcap > static_cast<uint64_t>(max_alloc) / size) {
        panic_growslice();
    }
    uint64_t mem = roundupsize(newcap * size);
    if (mem > static_cast<uint64_t>(max_alloc)) {
        panic_growslice();
    }
    return static_cast<int64_t>(mem / size);
}

} // namespace detail

} // namespace gp
