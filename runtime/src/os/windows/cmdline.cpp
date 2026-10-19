// The command line of a Windows program as a list of arguments (gocpp/os.hpp, arguments).
// Plain C++, no Windows header: the runtime's tests compile this file on Linux too.
//
// Windows hands a program one string, and the program splits it. The rules are Go's
// (os/exec_windows.go, which follows daviddeley.com/autohotkey/parameters/parameters.htm): the
// C runtime's rules since 2008, so that a program built with gocpp sees the same arguments as one
// built with Go, and not the ones that CommandLineToArgvW would give (it treats the program name
// by other rules and reads `""` outside of quotes differently):
//   - arguments are separated by spaces and tabs outside of double quotes;
//   - 2n backslashes before a double quote are n backslashes and the quote toggles quoting; 2n+1
//     are n backslashes and a literal quote; backslashes anywhere else are literal;
//   - inside quotes, `""` is one literal double quote (and quoting goes on).
#include "windows.hpp"

namespace gp::os {

namespace {

// read_arg reads the next argument of cmd, starting at i, and returns it; i is left after the
// separator that ended it.
std::string read_arg(std::string_view cmd, std::size_t& i) {
    std::string b;
    bool in_quote = false;
    std::size_t nslash = 0;
    for (; i < cmd.size(); i++) {
        char c = cmd[i];
        switch (c) {
        case ' ':
        case '\t':
            if (!in_quote) {
                b.append(nslash, '\\');
                i++;
                return b;
            }
            break;
        case '"':
            b.append(nslash / 2, '\\');
            if (nslash % 2 == 0) {
                // "Prior to 2008" rule: a doubled quote inside quotes is a quote.
                if (in_quote && i + 1 < cmd.size() && cmd[i + 1] == '"') {
                    b += c;
                    i++;
                }
                in_quote = !in_quote;
            } else {
                b += c;
            }
            nslash = 0;
            continue;
        case '\\':
            nslash++;
            continue;
        default:
            break;
        }
        b.append(nslash, '\\');
        nslash = 0;
        b += c;
    }
    b.append(nslash, '\\');
    return b;
}

} // namespace

std::vector<std::string> split_command_line(std::string_view cmd) {
    std::vector<std::string> args;
    std::size_t i = 0;
    while (i < cmd.size()) {
        if (cmd[i] == ' ' || cmd[i] == '\t') {
            i++;
            continue;
        }
        args.push_back(read_arg(cmd, i));
    }
    return args;
}

} // namespace gp::os
