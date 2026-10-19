// The natives of package syscall for Windows (gocpp/syscall.hpp, Phase 14 step 14.3, ADR-076): the
// process's arguments and environment for os, the call of an arbitrary system function
// (Go's runtime.syscall_Syscall), and LoadLibrary/GetProcAddress. Everything else that Go's os
// needs from the system is Go's own code over these (stdlib/syscall, internal/syscall/windows).
// This folder is the only place in the runtime that may include OS headers.
#include <gocpp/nofortify.h>
#include <gocpp/main.hpp>
#include <gocpp/os.hpp>
#include <gocpp/print.hpp>
#include <gocpp/syscall.hpp>
#include "windows.hpp"

#include <atomic>
#include <chrono>
#include <cstdint>
#include <cstdlib>
#include <string>
#include <thread>

#include <windows.h>

#include <stdlib.h>

namespace gp::syscall {

gp::slice<gp::string> runtime_envs() {
    const auto& v = gp::envs();
    auto s = gp::make_slice<gp::string>(static_cast<int64_t>(v.size()));
    for (std::size_t i = 0; i < v.size(); i++) {
        s[static_cast<int64_t>(i)] = v[i];
    }
    return s;
}

gp::slice<gp::string> runtime_args() {
    const auto& v = gp::args();
    auto s = gp::make_slice<gp::string>(static_cast<int64_t>(v.size()));
    for (std::size_t i = 0; i < v.size(); i++) {
        s[static_cast<int64_t>(i)] = v[i];
    }
    return s;
}

uint64_t runtime_rand() {
    // One generator per thread, seeded from the clock, the thread and a counter (xorshift64*).
    thread_local uint64_t state = 0;
    static std::atomic<uint64_t> counter{0};
    if (state == 0) {
        uint64_t seed =
            static_cast<uint64_t>(std::chrono::steady_clock::now().time_since_epoch().count());
        seed ^= std::hash<std::thread::id>{}(std::this_thread::get_id()) * 0x9E3779B97F4A7C15ull;
        seed += counter.fetch_add(1) * 0xBF58476D1CE4E5B9ull;
        state = seed | 1;
    }
    state ^= state >> 12;
    state ^= state << 25;
    state ^= state >> 27;
    return state * 0x2545F4914F6CDD1Dull;
}

namespace {

// wtf8_to_wide decodes WTF-8 (UTF-8 that may hold the code points of unpaired surrogates, as Go's
// syscall.UTF16FromString encodes them) to UTF-16, with the surrogates as they are; a byte that is
// not part of such a sequence becomes U+FFFD, one for each byte.
std::wstring wtf8_to_wide(std::string_view s) {
    std::wstring w;
    w.reserve(s.size());
    std::size_t i = 0;
    while (i < s.size()) {
        unsigned char c = static_cast<unsigned char>(s[i]);
        uint32_t cp = 0xFFFD;
        std::size_t n = 1;
        auto cont = [&](std::size_t k) {
            return i + k < s.size() && (static_cast<unsigned char>(s[i + k]) & 0xC0) == 0x80;
        };
        auto bits = [&](std::size_t k) {
            return static_cast<uint32_t>(static_cast<unsigned char>(s[i + k]) & 0x3F);
        };
        if (c < 0x80) {
            cp = c;
        } else if (c >= 0xC2 && c <= 0xDF && cont(1)) {
            cp = (static_cast<uint32_t>(c & 0x1F) << 6) | bits(1);
            n = 2;
        } else if (c >= 0xE0 && c <= 0xEF && cont(1) && cont(2)) {
            uint32_t v = (static_cast<uint32_t>(c & 0x0F) << 12) | (bits(1) << 6) | bits(2);
            if (v >= 0x800) { // surrogates (0xD800 to 0xDFFF) are allowed: that is WTF-8
                cp = v;
                n = 3;
            }
        } else if (c >= 0xF0 && c <= 0xF4 && cont(1) && cont(2) && cont(3)) {
            uint32_t v = (static_cast<uint32_t>(c & 0x07) << 18) | (bits(1) << 12) |
                         (bits(2) << 6) | bits(3);
            if (v >= 0x10000 && v <= 0x10FFFF) {
                cp = v;
                n = 4;
            }
        }
        if (cp >= 0x10000) {
            cp -= 0x10000;
            w.push_back(static_cast<wchar_t>(0xD800 + (cp >> 10)));
            w.push_back(static_cast<wchar_t>(0xDC00 + (cp & 0x3FF)));
        } else {
            w.push_back(static_cast<wchar_t>(cp));
        }
        i += n;
    }
    return w;
}

} // namespace

void win_setenv(uintptr_t key, uintptr_t value) {
    // The C library keeps its own copy of the environment (getenv of C and C++ code reads it);
    // SetEnvironmentVariable, which package syscall called, changed the process's. The two
    // arguments are the NUL-terminated UTF-16 that Go's Setenv already made (lone surrogates
    // intact). A key the library refuses (empty, with '=') is ignored.
    auto k = reinterpret_cast<const wchar_t*>(key);
    auto v = reinterpret_cast<const wchar_t*>(value);
    _wputenv_s(k, v);
    // The C library deletes the variable for an empty value (and so does it in the process's
    // block); Go keeps a variable that is set to "" (LookupEnv says true), so the system's block
    // gets the value again. (C's getenv then answers NULL for it: no way to hold an empty value in
    // the C library's copy.)
    SetEnvironmentVariableW(k, v);
}

void win_unsetenv(uintptr_t key) {
    auto k = reinterpret_cast<const wchar_t*>(key);
    _wputenv_s(k, L"");
    // (The C library's call may leave the variable in the system's block, empty.)
    SetEnvironmentVariableW(k, nullptr);
}

void runtime_setenv(const gp::string& key, const gp::string& value) {
    std::wstring k = wtf8_to_wide(key.gp_view());
    std::wstring v = wtf8_to_wide(value.gp_view());
    win_setenv(reinterpret_cast<uintptr_t>(k.c_str()), reinterpret_cast<uintptr_t>(v.c_str()));
}

void runtime_unsetenv(const gp::string& key) {
    std::wstring k = wtf8_to_wide(key.gp_view());
    win_unsetenv(reinterpret_cast<uintptr_t>(k.c_str()));
}

void sys_exit(int64_t code) {
    gp::os::exit(static_cast<int>(code));
}

void sys_flush_stdio() {
    gp::detail::flush_c_stdio();
}

std::tuple<uintptr_t, uintptr_t, uintptr_t>
win_syscall15(uintptr_t trap, uintptr_t nargs, uintptr_t a1, uintptr_t a2, uintptr_t a3,
              uintptr_t a4, uintptr_t a5, uintptr_t a6, uintptr_t a7, uintptr_t a8, uintptr_t a9,
              uintptr_t a10, uintptr_t a11, uintptr_t a12, uintptr_t a13, uintptr_t a14,
              uintptr_t a15) {
    (void)nargs;
    // The x64 convention has one way to call: the first four integer arguments in registers, the
    // others on the stack, the callee ignoring the ones it does not take. A function that returns a
    // double leaves it in XMM0: the second result (Go's r2) is its bits.
    using fn_t = uintptr_t(WINAPI*)(uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t,
                                    uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t,
                                    uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t);
    auto fn = reinterpret_cast<fn_t>(trap);
    // Go's asmstdcall clears the thread's last error first: a function that succeeds without
    // touching it (GetEnvironmentVariableW of an empty value returns 0) must not show an old error.
    SetLastError(0);
    uintptr_t r1 = fn(a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15);
    // GetLastError first: nothing between the call and this read may call the system.
    uintptr_t err = GetLastError();
    return {r1, 0, err};
}

std::tuple<uintptr_t, uintptr_t, uintptr_t> win_syscall_n(uintptr_t trap,
                                                          const gp::slice<uintptr_t>& args) {
    // Up to 42 arguments (Go's limit): the first four in registers, the others on the stack, the
    // callee ignoring the ones it does not take. As win_syscall15, the last error is cleared before
    // and read right after.
    constexpr std::size_t max_args = 42;
    uintptr_t a[max_args] = {};
    std::size_t n = static_cast<std::size_t>(args.gp_len());
    for (std::size_t i = 0; i < n && i < max_args; i++) {
        a[i] = args.gp_data()[i];
    }
    using fn_t = uintptr_t(WINAPI*)(
        uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t,
        uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t,
        uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t,
        uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t,
        uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t,
        uintptr_t, uintptr_t);
    auto fn = reinterpret_cast<fn_t>(trap);
    SetLastError(0);
    uintptr_t r1 = fn(a[0], a[1], a[2], a[3], a[4], a[5], a[6], a[7], a[8], a[9], a[10], a[11],
                      a[12], a[13], a[14], a[15], a[16], a[17], a[18], a[19], a[20], a[21], a[22],
                      a[23], a[24], a[25], a[26], a[27], a[28], a[29], a[30], a[31], a[32], a[33],
                      a[34], a[35], a[36], a[37], a[38], a[39], a[40], a[41]);
    uintptr_t err = GetLastError();
    return {r1, 0, err};
}

std::tuple<uintptr_t, uintptr_t> win_loadlibrary(uintptr_t name) {
    HMODULE h = LoadLibraryW(reinterpret_cast<LPCWSTR>(name));
    if (h == nullptr) {
        return {0, GetLastError()};
    }
    return {reinterpret_cast<uintptr_t>(h), 0};
}

std::tuple<uintptr_t, uintptr_t> win_loadsystemlibrary(uintptr_t name) {
    HMODULE h =
        LoadLibraryExW(reinterpret_cast<LPCWSTR>(name), nullptr, LOAD_LIBRARY_SEARCH_SYSTEM32);
    if (h == nullptr) {
        return {0, GetLastError()};
    }
    return {reinterpret_cast<uintptr_t>(h), 0};
}

std::tuple<uintptr_t, uintptr_t> win_getprocaddress(uintptr_t handle, uintptr_t name) {
    FARPROC p = GetProcAddress(reinterpret_cast<HMODULE>(handle), reinterpret_cast<LPCSTR>(name));
    if (p == nullptr) {
        return {0, GetLastError()};
    }
    return {reinterpret_cast<uintptr_t>(p), 0};
}

} // namespace gp::syscall
