// The C++ twin of hresult_reassign.gp: the expected output is what this program prints.
#include <cstdio>
#include "../hresult.hpp"

int main() {
    std::printf("HRESULT 0x80004005 (E_FAIL)\n");
    std::printf("a Go error\n");
    std::printf("%s\n", status_of(0) >= 0 ? "true" : "false");

    ISwapChain* chain = new_swap_chain();
    Surface* surface = nullptr;
    HRESULT second = chain->GetBuffer(1, reinterpret_cast<void**>(&surface));
    if (second < 0) {
        std::printf("wrapped: HRESULT 0x80070057 (E_INVALIDARG)\n");
    }
    std::printf("another Go error\n");
    second = chain->GetBuffer(0, reinterpret_cast<void**>(&surface));
    std::printf("%s %d\n", second >= 0 ? "<nil>" : "failed", surface->value);
}
