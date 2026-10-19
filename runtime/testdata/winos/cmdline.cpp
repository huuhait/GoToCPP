// The harness of TestWindowsCommandLine (runtime/wincmdline_test.go): it reads command lines from
// standard input, one per line, and prints how the Windows OS layer splits each (the rules of
// src/os/windows/cmdline.cpp, which is plain C++ and so is also built for the host): the number
// of arguments, then each argument as "length:text". The Go test compares that with Go's own
// splitting (os.commandLineToArgv, copied into the test).
#include <cstddef>
#include <cstdio>
#include <iostream>
#include <string>
#include <string_view>
#include <vector>

namespace gp::os {
std::vector<std::string> split_command_line(std::string_view cmd);
} // namespace gp::os

int main() {
    std::string line;
    while (std::getline(std::cin, line)) {
        std::vector<std::string> args = gp::os::split_command_line(line);
        std::printf("%zu\n", args.size());
        for (const std::string& a : args) {
            std::printf("%zu:", a.size());
            std::fwrite(a.data(), 1, a.size(), stdout);
            std::printf("\n");
        }
    }
    return 0;
}
