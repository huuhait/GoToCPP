// The C++ twin of adapt_enum.gp: the expected output is what this program prints.
#include <cstdio>
#include "../adapt.hpp"

int main() {
    std::printf("%d %d %d\n", mode_value(static_cast<Mode>(2)), mode_value(static_cast<Mode>(0x4)),
                mode_value(static_cast<Mode>(1 + 2)));
    std::printf("%d %d\n", mode_value(static_cast<Mode>(6)), mode_value(MODE_C));
    std::printf("%d %d %d\n", level_value(static_cast<Level>(5)),
                level_value(static_cast<Level>(200)), level_value(Level::High));
}
