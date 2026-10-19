// What a Windows program built by gocpp sees of its command line and environment: the runtime's
// own view (gp::args), printed one item a line.
#pragma once

#include <gocpp/main.hpp>

#include <cstdio>
#include <cstdlib>

inline void print_args() {
    for (const auto& a : gp::args()) {
        auto v = a.gp_view();
        std::fwrite(v.data(), 1, v.size(), stdout);
        std::fputc('\n', stdout);
    }
    std::fflush(stdout);
}
