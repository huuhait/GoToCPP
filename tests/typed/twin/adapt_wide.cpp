// The C++ twin of adapt_wide.gp: the expected output is what this program prints.
#include <cstdio>
#include "../adapt.hpp"

int main() {
    std::printf("%d %d %d\n", wide_length(L"héllo"), wide_length(L"abcd"), wide_length(L""));
    std::printf("%d %d\n", unit16_length(u"a\U0001F600"), unit16_length(u"xyz"));
    std::printf("%d %d\n", wide_length(L"caf"), wide_length(L"café"));
}
