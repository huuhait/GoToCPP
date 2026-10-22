// The C++ twin of hresult_raw.gp: the expected output is what this program prints.
#include <cstdint>
#include <cstdio>
#include "../hresult.hpp"

int main() {
    ISwapChain* chain = new_swap_chain();
    chain->Present(1, 0);
    std::printf("%d\n", present_count());
    int32_t code = static_cast<int32_t>(status_of(2));
    std::printf("%x\n", static_cast<uint32_t>(code));
    bool occluded = static_cast<int32_t>(chain->Present(1, 1)) == static_cast<int32_t>(DXGI_STATUS_OCCLUDED);
    std::printf("%s %s\n", occluded ? "true" : "false", "true");
    long wide = static_cast<long>(status_of(1));
    std::printf("%ld\n", wide);
    std::printf("%ld %ld %d %ld\n", window_result(2), nt_result(0), bool_result(1), my_result(1));
    long lresult = window_result(1);
    std::printf("%s\n", lresult != 0 ? "true" : "false");
    Surface* surface = nullptr;
    chain->GetBuffer(0, reinterpret_cast<void**>(&surface));
    code = static_cast<int32_t>(chain->GetBuffer(1, reinterpret_cast<void**>(&surface)));
    (void)chain->GetBuffer(0, reinterpret_cast<void**>(&surface));
    std::printf("%x %d\n", static_cast<uint32_t>(code), surface->value);
    chain->Present(1, 0);
}
