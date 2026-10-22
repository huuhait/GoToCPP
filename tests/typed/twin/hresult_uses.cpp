// The C++ twin of hresult_uses.gp: the expected output is what this program prints. The Go
// libraries have no C++ counterpart, so the program prints the text that they make of the code.
#include <algorithm>
#include <cctype>
#include <cstdint>
#include <cstdio>
#include <map>
#include <string>
#include <vector>
#include "../hresult.hpp"

static std::string show(HRESULT hr) {
    switch (static_cast<uint32_t>(static_cast<int32_t>(hr))) {
    case 0x80004005u:
        return "HRESULT 0x80004005 (E_FAIL)";
    case 0x887A0005u:
        return "HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)";
    }
    return "<nil>";
}

static int describe(const std::vector<HRESULT>& errs) {
    int n = 0;
    for (HRESULT e : errs) {
        if (e < 0) {
            n++;
        }
    }
    return n;
}

int main() {
    HRESULT e = status_of(2);
    std::printf("failed: %s\n", show(e).c_str());
    std::printf("%s!\n", show(e).c_str());
    std::string msg = show(e);
    std::string upper = msg;
    std::transform(upper.begin(), upper.end(), upper.begin(), [](unsigned char c) { return std::toupper(c); });
    std::printf("%s %s %zu\n", upper.c_str(), msg.find("E_FAIL") != std::string::npos ? "true" : "false", msg.size());
    std::string t = msg;
    std::printf("%s %s %s %c\n", t == msg ? "true" : "false", t.substr(0, 4).c_str(), msg.substr(0, 4).c_str(), msg[5]);
    int runes = 0;
    for (char r : msg) {
        if (r == 'E') {
            runes++;
        }
    }
    std::printf("%d\n", runes);

    std::vector<HRESULT> xs;
    xs.push_back(e);
    xs.push_back(status_of(0));
    std::vector<HRESULT> items;
    items.push_back(status_of(4));
    items.push_back(e);
    items.push_back(0);
    std::printf("%zu %s %zu %s\n", xs.size(), xs[1] >= 0 ? "true" : "false", items.size(), items[2] >= 0 ? "true" : "false");
    std::vector<HRESULT> lit{e, status_of(0), status_of(4)};
    std::printf("%d %d\n", describe(lit), describe({e, status_of(0)}));

    HRESULT f = status_of(4);
    f = 0;
    std::printf("%s %s\n", f >= 0 ? "true" : "false", f != e ? "true" : "false");
    f = status_of(4);
    std::printf("%s %s\n", f >= 0 ? "true" : "false", show(f).c_str());

    std::map<long, std::string> m{{e, "dev"}};
    std::map<long, int> n{{status_of(4), 1}, {0, 2}};
    std::printf("%zu %s %zu %d %d\n", m.size(), m[e].c_str(), n.size(), n[f], n[0]);

    std::printf("<nil>\n");
    if (status_of(4) >= 0) {
        std::printf("not reached\n");
    } else {
        std::printf("removed true\n");
    }
}
