// The C++ twin of hresult_shadow.gp: the expected output is what this program prints.
#include <cstdio>
#include "../hresult.hpp"

int main() {
    HRESULT e = status_of(4);
    std::printf("text %s HRESULT\n", e < 0 ? "true" : "false");
    HRESULT f = status_of(2);
    std::printf("7 %s %s\n", f < 0 ? "true" : "false", status_of(0) >= 0 ? "true" : "false");
}
