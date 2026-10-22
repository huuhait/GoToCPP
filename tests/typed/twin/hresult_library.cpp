// The C++ twin of hresult_library.gp: the expected output is what this program prints. The Go
// libraries have no C++ counterpart, so the program prints the text that they make of the code.
#include <cstdint>
#include <cstdio>
#include <map>
#include <string>
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

int main() {
    HRESULT e = status_of(4);
    std::printf("true %x\n", static_cast<uint32_t>(e));
    std::printf("%s %s\n", status_of(4) == e ? "true" : "false", status_of(2) == e ? "true" : "false");
    std::printf("<nil>\n");
    std::printf("%s\n", show(e).c_str());
    std::printf("device: %s true\n", show(e).c_str());
    std::printf("%s|\"%s\"\n", show(e).c_str(), show(e).c_str());
    std::printf("%zu <%s>\n", show(e).size(), show(e).c_str());
    std::printf("%s %s\n", show(e) == "HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)" ? "true" : "false",
                "false");
    std::printf("asserted %s\n", e < 0 ? "true" : "false");
    std::printf("type switch %s\n", e < 0 ? "true" : "false");
    if (e == status_of(4)) {
        std::printf("the same code\n");
    }
    if (status_of(0) >= 0) {
        std::printf("S_OK is nil\n");
    }
    std::map<long, int> counts;
    counts[status_of(4)]++;
    counts[status_of(4)]++;
    counts[status_of(2)]++;
    std::printf("%zu %d\n", counts.size(), counts[e]);
    std::printf("recovered: true %s\n", show(e).c_str());
}
