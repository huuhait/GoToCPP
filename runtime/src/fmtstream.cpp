// gocpp/fmt.hpp: the text of a C++ value that has only an operator<<. This is the one place of the
// runtime that uses std::ostream: a std::ostringstream pulls libstdc++'s whole locale machinery
// (about 750 KB) into the program, so the function is a file of its own, which the linker takes
// only when a program prints such a value.
#include <gocpp/fmt.hpp>

#include <sstream>
#include <string>
#include <utility>

namespace gp::fmt::detail {

void stream_text(std::string& out, void (*put)(std::ostream&, const void*), const void* v) {
    std::ostringstream os;
    put(os, v);
    out += std::move(os).str();
}

} // namespace gp::fmt::detail
