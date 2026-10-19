// The C++ twin of strings_auto.gp: the expected output is what this program prints.
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <string>
#include "../strs.hpp"

static std::string build(int n) {
    std::string s = "x";
    for (int i = 0; i < n; i++) s += "y";
    return s;
}

int main() {
    std::string s = "banana";
    std::printf("%d %d\n", count_char(s.c_str(), 'a'), count_char((s + "!").c_str(), 'n'));
    std::printf("%d %d %d\n", (int)std::strlen(s.c_str()), (int)std::strlen(s.substr(1, 3).c_str()),
                (int)std::strlen(build(4).c_str()));
    std::printf("%d %s %d\n", std::strcmp(s.c_str(), "banana"), std::strcmp(s.c_str(), "apple") > 0 ? "true" : "false",
                std::strcmp(s.substr(0, 3).c_str(), "ban"));
    std::printf("%d\n", std::atoi((build(0) + "41").c_str()) + 1);
    std::printf("%s %d %d\n", shout(s).c_str(), total_len(s, "xy"), length_sv(s));
    std::printf("%d\n", std::stoi(std::string("123")));
    std::printf("%s\n", kind(std::string_view(s)));
    char buf[32];
    int n = std::snprintf(buf, 32, "%s-%d-%s", s.c_str(), 7, build(1).c_str());
    std::printf("%d %s\n", n, buf);
    Meter meter;
    meter.count(s.c_str());
    std::printf("%d\n", meter.count((s + "!").c_str()));
    if (std::strstr(s.c_str(), "nan") != nullptr) std::printf("found %s\n", std::strstr(s.c_str(), "nan"));
    std::string t("ab\0cd", 5);
    std::printf("%d %d\n", (int)std::strlen(t.c_str()), (int)t.size());
}
