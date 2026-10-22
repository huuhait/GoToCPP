// The C++ twin of hresult_error.gp: the expected output is what this program prints.
#include <cstdint>
#include <cstdio>
#include <string>
#include "../hresult.hpp"

// What Go prints for an error made of an HRESULT: <nil> for a code that is not negative.
static std::string show(HRESULT hr) {
    switch (static_cast<uint32_t>(static_cast<int32_t>(hr))) {
    case 0x80004005u:
        return "HRESULT 0x80004005 (E_FAIL)";
    case 0x80070057u:
        return "HRESULT 0x80070057 (E_INVALIDARG)";
    case 0x887A0005u:
        return "HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)";
    case 0xFFFFFFFBu:
        return "HRESULT 0xFFFFFFFB";
    }
    return "<nil>";
}

int main() {
    for (int kind = 0; kind <= 5; kind++) {
        HRESULT hr = status_of(kind);
        std::printf("%d %s %s\n", kind, hr >= 0 ? "true" : "false", show(hr).c_str());
    }
    std::printf("%s %s %s\n", show(status_of(2)).c_str(), show(status_of(3)).c_str(),
                show(status_of(4)).c_str());
    std::printf("%x\n", static_cast<uint32_t>(status_of(4)));
    std::printf("%s %s %s\n", "true", "false", "true");
    std::printf("%d\n", 0);

    int value = 0;
    HRESULT hr = create(&value, -3);
    if (hr < 0) {
        std::printf("create failed: %s %d\n", show(hr).c_str(), value);
    }
    hr = create(&value, 9);
    if (hr >= 0) {
        std::printf("create ok: %d\n", value);
    }
    ISwapChain* chain = new_swap_chain();
    hr = chain->ResizeBuffers(0, 5);
    if (hr < 0) {
        std::printf("resize failed: %s\n", show(hr).c_str());
    }
    bool resized = chain->ResizeBuffers(4, 5) >= 0;
    bool presented = chain->Present(1, 1) >= 0;
    std::printf("%s %s %s\n", resized ? "true" : "false", presented ? "true" : "false",
                show(chain->Present(9, 0)).c_str());
    Surface* surface = nullptr;
    hr = chain->GetBuffer(0, reinterpret_cast<void**>(&surface));
    std::printf("%s %d\n", show(hr).c_str(), surface_value(surface));
    std::printf("%s\n", show(chain->GetBuffer(1, reinterpret_cast<void**>(&surface))).c_str());
}
