// C++ equivalent of examples/mixed-sources/main.gp + report.gp.
extern "C" {
#include "checksum.h"
}
#include "textutil.hpp"
#include <cstdint>
#include <initializer_list>
#include <iostream>
#include <string>
#include <vector>
static void show(const std::string& label, std::initializer_list<int64_t> values) {
    std::cout << label << ":";
    for (auto v : values)
        std::cout << " " << v;
    std::cout << "\n";
}
int main() {
    for (std::string s : {"", "a", "gocpp"}) {
        uint32_t h = fnv1a32(s.c_str(), s.size());
        std::cout << "fnv1a32(\"" + s + "\") = " << h << "\n";
    }
    std::vector<int> xs{4, -2, 9, 7};
    span_stats st = stats_of(xs.data(), xs.size());
    show("min max sum", {st.min, st.max, st.sum});
    show("checksum version", {CHECKSUM_VERSION});
    std::string text = "gocpp builds c and c++ files together";
    std::cout << textutil::title(text) << "\n";
    show("words", {textutil::word_count(text)});
    textutil::tally t{};
    for (const char* w : {"go", "cpp", "go", "c", "go", "cpp"})
        t.add(w);
    show("go cpp rust", {t.count("go"), t.count("cpp"), t.count("rust")});
    show("distinct", {t.size()});
    std::cout << "top: " << t.top() << "\n";
}
