// The C++ twin of hresult_generic.gp: the expected output is what this program prints.
#include <cstdio>
#include "../hresult.hpp"

static bool fail(const char* what, long hr_text_seven) {
    std::printf("%s HRESULT\n", what);
    return hr_text_seven < 0;
}

int main() {
    bool first = fail("3", status_of(4));
    bool second = fail("x", status_of(4));
    std::printf("%s %s\n", first ? "true" : "false", second ? "true" : "false");
    HRESULT e = status_of(2);
    std::printf("%s %s\n", e < 0 ? "true" : "false", status_of(0) >= 0 ? "true" : "false");
}
