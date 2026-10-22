// The C++ twin of adapt_format.gp: the expected output is what this program prints.
#include <cstdio>
#include <string>
#include "../adapt.hpp"

int main() {
    std::string text = "50% done";
    std::printf("%d %d\n", say("%s!", text.c_str()), say("[%s|%d]", text.c_str(), 7));
    std::printf("%d\n", say("plain"));
}
