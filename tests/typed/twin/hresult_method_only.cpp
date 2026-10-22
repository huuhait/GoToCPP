// The C++ twin of hresult_method_only.gp: the expected output is what this program prints.
#include <cstdio>
#include "../hresult.hpp"

int main() {
    ISwapChain* chain = new_swap_chain();
    Surface* surface = nullptr;
    HRESULT err = chain->GetBuffer(1, reinterpret_cast<void**>(&surface));
    std::printf("%s\n", err < 0 ? "HRESULT 0x80070057 (E_INVALIDARG)" : "<nil>");
    HRESULT second = chain->GetBuffer(0, reinterpret_cast<void**>(&surface));
    std::printf("%s %d\n", second < 0 ? "failed" : "<nil>", surface->value);
}
