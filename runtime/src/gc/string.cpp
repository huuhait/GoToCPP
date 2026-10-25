// gocpp/gc/string.cpp: the part of gp::gc::string that allocates (phase-16 README §2.4; job
// 16.1c).
#include <gocpp/nofortify.h>
#include <gocpp/gc/heap.hpp>
#include <gocpp/gc/string.hpp>

#include <cstring>

namespace gp::gc {

string concat(string a, string b) {
    const int64_t n = a.gp_len() + b.gp_len();
    auto* p = static_cast<char*>(process_heap().alloc_atomic(static_cast<std::size_t>(n) + 1));
    if (a.gp_len() > 0) {
        std::memcpy(p, a.gp_data(), static_cast<std::size_t>(a.gp_len()));
    }
    if (b.gp_len() > 0) {
        std::memcpy(p + a.gp_len(), b.gp_data(), static_cast<std::size_t>(b.gp_len()));
    }
    // The byte at p[n] is not part of the string: it makes the bytes readable as C text, which
    // the boundary (step 16.14) will need. The string never forms a pointer past p[n].
    p[n] = '\0';
    return string(bytes, p, n);
}

} // namespace gp::gc
