// gocpp/gc/slice.cpp: the part of slices that is not a template: append's growth, as Go's
// runtime/slice.go gives it (phase-16 README §2.2; job 16.1e).
//
// The numbers are gc's own: nextslicecap (double below 256 elements, then about 1.25x plus 192
// elements), then roundupsize to a malloc size class (up to 32 KiB) or to an 8 KiB page. A
// block of pointer-holding elements above 512 bytes carries gc's 8-byte malloc header, which
// the size class includes and the result does not ([]string reaches 71 elements at the 33rd
// append, not 64). The element size is Go's (gocpp/gc/golayout.hpp), never sizeof where the two
// differ.
//
// The size-class table, roundupsize, nextslicecap and grow_cap below are a verbatim copy of
// runtime/src/slice.cpp:47-180, the reference-counting engine's (which are Go's numbers too).
// The ticket allowed "reuse it or move it": reuse would link the ARC runtime's src/slice.cpp
// into every GC program, and editing the ARC engine's sources is outside this job. The copy is
// the in-scope option, and the risk is that a change to Go's numbers is made in one engine and
// not the other. Steps 16.5/16.8 should move the pure algorithm (the table, roundupsize,
// nextslicecap and grow_cap) into a shared header that both engines include.
#include <gocpp/nofortify.h>
#include <gocpp/gc/checks.hpp>
#include <gocpp/gc/slice.hpp>

#include <cstddef>
#include <cstdint>

namespace gp::gc {

namespace detail {

namespace {

// Go's malloc size classes (internal/runtime/gc/sizeclasses.go, Go 1.25), without class 0.
constexpr std::uint16_t size_classes[] = {
    8,     16,    24,    32,    48,    64,    80,    96,    112,   128,   144,   160,
    176,   192,   208,   224,   240,   256,   288,   320,   352,   384,   416,   448,
    480,   512,   576,   640,   704,   768,   896,   1024,  1152,  1280,  1408,  1536,
    1792,  2048,  2304,  2688,  3072,  3200,  3456,  4096,  4864,  5376,  6144,  6528,
    6784,  6912,  8192,  9472,  9728,  10240, 10880, 12288, 13568, 14336, 16384, 18432,
    19072, 20480, 21760, 24576, 27264, 28672, 32768,
};

constexpr std::uint64_t max_small_size = 32768;
constexpr std::uint64_t malloc_header_size = 8;
constexpr std::uint64_t page_size = 8192;
// gc.MinSizeForMallocHeader: goarch.PtrSize * ptrBits (8 * 64). Above it, an object with
// pointers has a malloc header instead of heap bits in its span.
constexpr std::uint64_t min_size_for_malloc_header = 8 * 64;

// roundupsize: Go's roundupsize(size, noscan) (runtime/msize.go): the usable size of the block
// that Go's malloc gives for size bytes. A small size goes up to its size class; a block of
// elements with pointers over 512 bytes has an 8-byte header that the size class includes and
// the result does not, so such a size is rounded up as size + 8 and the 8 are taken off again.
// A large size goes up to a multiple of the page size.
std::uint64_t roundupsize(std::uint64_t size, bool noscan) {
    if (size == 0) {
        return 0; // size class 0: Go's malloc(0) is zerobase
    }
    if (size <= max_small_size - malloc_header_size) {
        std::uint64_t req = size;
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
    std::uint64_t req = size + page_size - 1;
    if (req < size) {
        return size;
    }
    return req & ~(page_size - 1);
}

// nextslicecap: Go's nextslicecap (runtime/slice.go), in unsigned arithmetic. new_len > 0.
std::uint64_t nextslicecap(std::uint64_t new_len, std::uint64_t old_cap) {
    std::uint64_t newcap = old_cap;
    std::uint64_t doublecap = newcap + newcap;
    if (new_len > doublecap) {
        return new_len;
    }
    constexpr std::uint64_t threshold = 256;
    if (old_cap < threshold) {
        return doublecap;
    }
    do {
        // Go's smooth transition from growing 2x for small slices to 1.25x for large ones.
        newcap += (newcap + 3 * threshold) >> 2;
    } while (newcap < new_len);
    return newcap;
}

} // namespace

std::int64_t grow_cap(std::int64_t new_len, std::int64_t old_cap, std::uint64_t size, bool noscan) {
    constexpr std::uint64_t max_alloc = (std::uint64_t{1} << 48) - 1; // Go's maxAlloc
    if (new_len < 0) {
        panic_growslice();
    }
    if (size == 0) {
        // Go's growslice for a zero-size element: no memory, and cap == len.
        return new_len;
    }
    std::uint64_t newcap =
        nextslicecap(static_cast<std::uint64_t>(new_len), static_cast<std::uint64_t>(old_cap));
    if (newcap > max_alloc / size) {
        panic_growslice();
    }
    std::uint64_t mem = roundupsize(newcap * size, noscan);
    if (mem > max_alloc) {
        panic_growslice();
    }
    return static_cast<std::int64_t>(mem / size);
}

} // namespace detail

} // namespace gp::gc
