// For TestNoFortify (runtime/cxx_test.go): a file like generated code, which includes
// gocpp/rt.hpp first and then uses the C functions that _FORTIFY_SOURCE replaces. It must
// compile without the fortify warning at -O0, and with GOCPP_EXPECT_FORTIFY at -O2 it checks
// that gocpp/nofortify.h left _FORTIFY_SOURCE alone.
#include <gocpp/rt.hpp>

#include <cstdio>
#include <cstring>

#if defined(GOCPP_EXPECT_FORTIFY) && !(defined(__OPTIMIZE__) && defined(_FORTIFY_SOURCE))
#error "gocpp/nofortify.h removed _FORTIFY_SOURCE from an optimized build"
#endif

int main() {
    char buf[16];
    std::memcpy(buf, "fortify", 8);
    std::printf("%s\n", buf);
    return 0;
}
