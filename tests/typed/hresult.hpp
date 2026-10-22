// A COM-like header for the HRESULT tests of typed interop (ADR-137): the typedef is the Windows
// one (typedef long HRESULT), so that the tests run on Linux, where long is 64 bits: a code is
// the 32 bits of a negative number, sign-extended as the Windows headers do on 64-bit targets.
#pragma once
#include <cstdint>

typedef long HRESULT;

#define S_OK ((HRESULT)0L)
#define S_FALSE ((HRESULT)1L)
#define E_FAIL ((HRESULT)(int32_t)0x80004005u)
#define E_INVALIDARG ((HRESULT)(int32_t)0x80070057u)
#define DXGI_STATUS_OCCLUDED ((HRESULT)(int32_t)0x087A0001u)
#define DXGI_ERROR_DEVICE_REMOVED ((HRESULT)(int32_t)0x887A0005u)

// Things that look like an HRESULT and are not: another typedef of long, an int, and a typedef of
// HRESULT itself.
typedef long LRESULT;
typedef long NTSTATUS;
typedef int BOOL;
typedef HRESULT MYRESULT;

inline HRESULT status_of(int kind) {
    switch (kind) {
    case 0:
        return S_OK;
    case 1:
        return S_FALSE;
    case 2:
        return E_FAIL;
    case 3:
        return DXGI_STATUS_OCCLUDED;
    case 4:
        return DXGI_ERROR_DEVICE_REMOVED;
    }
    return (HRESULT)-5;
}

inline HRESULT create(int* out, int value) {
    *out = value;
    return value < 0 ? E_INVALIDARG : S_OK;
}

inline LRESULT window_result(int kind) {
    return status_of(kind);
}

inline NTSTATUS nt_result(int kind) {
    return status_of(kind);
}

inline BOOL bool_result(int kind) {
    return kind == 0 ? 0 : 1;
}

inline MYRESULT my_result(int kind) {
    return status_of(kind);
}

struct Surface {
    int value;
};

// A COM-like interface: virtual methods with the HRESULT result, called through the vtable.
struct ISwapChain {
    virtual HRESULT Present(unsigned sync, unsigned flags) = 0;
    virtual HRESULT ResizeBuffers(unsigned width, unsigned height) = 0;
    virtual HRESULT GetBuffer(unsigned index, void** surface) = 0;
    virtual unsigned long Release() = 0;
    virtual ~ISwapChain() = default;
};

struct SwapChain : ISwapChain {
    int presents = 0;
    Surface surface_storage{7};
    HRESULT Present(unsigned sync, unsigned flags) override {
        presents++;
        if (flags == 1) {
            return DXGI_STATUS_OCCLUDED;
        }
        return sync > 4 ? DXGI_ERROR_DEVICE_REMOVED : S_OK;
    }
    HRESULT ResizeBuffers(unsigned width, unsigned height) override {
        return width == 0 || height == 0 ? E_INVALIDARG : S_OK;
    }
    HRESULT GetBuffer(unsigned index, void** surface) override {
        if (index != 0) {
            return E_INVALIDARG;
        }
        *surface = &surface_storage;
        return S_OK;
    }
    unsigned long Release() override { return 1; }
};

inline ISwapChain* new_swap_chain() {
    static SwapChain chain;
    return &chain;
}

inline int surface_value(const Surface* surface) {
    return surface == nullptr ? -1 : surface->value;
}

inline int present_count() {
    return static_cast<SwapChain*>(new_swap_chain())->presents;
}
