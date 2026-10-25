// The natives of package syscall for Windows (gocpp/syscall.hpp, Phase 14 step 14.3, ADR-076): the
// process's arguments and environment for os, the call of an arbitrary system function
// (Go's runtime.syscall_Syscall), and LoadLibrary/GetProcAddress. Everything else that Go's os
// needs from the system is Go's own code over these (stdlib/syscall, internal/syscall/windows).
// This folder is the only place in the runtime that may include OS headers.
#include <gocpp/nofortify.h>
#include <gocpp/main.hpp>
#include <gocpp/os.hpp>
#include <gocpp/print.hpp>
#include <gocpp/shims.hpp>
#include <gocpp/syscall.hpp>
#include "windows.hpp"

#include <atomic>
#include <chrono>
#include <cstdint>
#include <cstdlib>
#include <cwchar>
#include <string>
#include <thread>
#include <vector>

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

namespace {

// block_entries is the process's environment block (the system's, the one package syscall
// reads), one "name=value" entry each, in its order.
std::vector<std::wstring> block_entries() {
    std::vector<std::wstring> out;
    wchar_t* block = GetEnvironmentStringsW();
    if (block == nullptr) {
        return out;
    }
    for (const wchar_t* p = block; *p != 0; p += std::wcslen(p) + 1) {
        out.emplace_back(p);
    }
    FreeEnvironmentStringsW(block);
    return out;
}

// has_name says whether entry is a variable named key, the way the system compares names
// (without case). A name may begin with '=' ("=C:=C:\x"): its '=' is looked for from the second
// character on, as Go's syscall.Clearenv does.
bool has_name(const std::wstring& entry, const wchar_t* key) {
    std::size_t eq = entry.find(L'=', 1);
    if (eq == std::wstring::npos) {
        return false;
    }
    return CompareStringOrdinal(entry.c_str(), static_cast<int>(eq), key, -1, TRUE) == CSTR_EQUAL;
}

std::vector<std::wstring> entries_named(const std::vector<std::wstring>& block,
                                        const wchar_t* key) {
    std::vector<std::wstring> out;
    for (const std::wstring& e : block) {
        if (has_name(e, key)) {
            out.push_back(e);
        }
    }
    return out;
}

// The functions of ntdll that copy the environment block exactly (also two variables whose names
// differ only in case, which a parent can pass and which SetEnvironmentStringsW would merge) and
// put a copy back as the process's block. Exported by every ntdll since NT, and by Wine's.
struct env_block_api {
    LONG(WINAPI* create)(BOOLEAN clone, PWSTR* env) = nullptr;  // RtlCreateEnvironment
    void(WINAPI* set_current)(PWSTR env, PWSTR* old) = nullptr; // RtlSetCurrentEnvironment
    LONG(WINAPI* destroy)(PWSTR env) = nullptr;                 // RtlDestroyEnvironment
};

const env_block_api& block_api() {
    static const env_block_api api = [] {
        env_block_api a;
        HMODULE ntdll = GetModuleHandleW(L"ntdll.dll");
        if (ntdll != nullptr) {
            a.create = reinterpret_cast<decltype(a.create)>(
                reinterpret_cast<void*>(GetProcAddress(ntdll, "RtlCreateEnvironment")));
            a.set_current = reinterpret_cast<decltype(a.set_current)>(
                reinterpret_cast<void*>(GetProcAddress(ntdll, "RtlSetCurrentEnvironment")));
            a.destroy = reinterpret_cast<decltype(a.destroy)>(
                reinterpret_cast<void*>(GetProcAddress(ntdll, "RtlDestroyEnvironment")));
        }
        if (a.create == nullptr || a.set_current == nullptr || a.destroy == nullptr) {
            a = env_block_api{};
        }
        return a;
    }();
    return api;
}

// env_lock serializes os.Setenv and os.Unsetenv (win_setenv): the change of the system's block,
// the C library's copy and the block put back are one step for the other goroutines that change
// the environment, and for those that read it (syscall.Getenv and Environ hold it shared,
// win_env_read_lock): while crt_sync works, the block may lack the variable for a moment. C or
// C++ code that calls SetEnvironmentVariableW itself, on another thread, in that moment does not
// take the lock: its change can be undone when the block is put back. (SRWLOCK_INIT is a
// constant: no initializer at program start.)
SRWLOCK env_lock = SRWLOCK_INIT;

// crt_sync makes the C library's copy of the environment (what getenv of C and C++ code reads)
// say for key what the system's block says, which package syscall has just changed: the value of
// the block's FIRST variable of the name (Windows compares names without case, and a block can
// hold several that differ in case, "godebug" and "GODEBUG", when the parent passed them), or no
// variable when the block has none or its value is "" (the library cannot hold an empty value).
// The library's calls (_wputenv_s) change the system's block too (SetEnvironmentVariableW, one
// variable each), so the block is put back as it was afterwards: Go's syscall.Unsetenv removes
// one variable of the name, and syscall.Setenv(k, "") keeps the variable (LookupEnv says true).
void crt_sync(const wchar_t* key) {
    const env_block_api& api = block_api();
    PWSTR saved = nullptr;
    if (api.create != nullptr && api.create(TRUE, &saved) < 0) {
        saved = nullptr;
    }
    std::vector<std::wstring> before = block_entries();
    std::vector<std::wstring> want = entries_named(before, key);
    // Every variable of the name out of the library's copy (each call removes one), then the first
    // value of the block in. A name the library refuses (empty, with '=') is left alone. The
    // library has two copies, the narrow one that getenv reads and the wide one of _wgetenv, which
    // it may make only at its first use, from the block as it is then (without the variable that
    // syscall just removed): both are asked.
    std::string narrow;
    if (int n = WideCharToMultiByte(CP_ACP, 0, key, -1, nullptr, 0, nullptr, nullptr); n > 0) {
        narrow.resize(static_cast<std::size_t>(n));
        WideCharToMultiByte(CP_ACP, 0, key, -1, narrow.data(), n, nullptr, nullptr);
        narrow.resize(static_cast<std::size_t>(n - 1));
    }
    auto has = [&] {
        return _wgetenv(key) != nullptr ||
               (!narrow.empty() && std::getenv(narrow.c_str()) != nullptr);
    };
    for (int i = 0; i < 256 && has(); i++) {
        if (_wputenv_s(key, L"") != 0) {
            break;
        }
    }
    if (!want.empty()) {
        const wchar_t* v = want[0].c_str() + want[0].find(L'=', 1) + 1;
        if (*v != 0) {
            _wputenv_s(key, v);
        }
    }
    if (block_entries() == before) {
        if (saved != nullptr) {
            api.destroy(saved);
        }
        return;
    }
    if (saved != nullptr) {
        PWSTR old = nullptr;
        api.set_current(saved, &old);
        if (old != nullptr) {
            api.destroy(old);
        }
        return;
    }
    // (No ntdll functions: the variables of the name come back one at a time; only the first of
    // several can.)
    while (!entries_named(block_entries(), key).empty()) {
        if (!SetEnvironmentVariableW(key, nullptr)) {
            break;
        }
    }
    if (!want.empty()) {
        std::size_t eq = want[0].find(L'=', 1);
        std::wstring name = want[0].substr(0, eq);
        SetEnvironmentVariableW(name.c_str(), want[0].c_str() + eq + 1);
    }
}

// set_env is syscall.Setenv's (value != null) or syscall.Unsetenv's (null) change of the system's
// block, then of the C library's copy, under env_lock: 0, or the system's error.
uintptr_t set_env(const wchar_t* key, const wchar_t* value) {
    AcquireSRWLockExclusive(&env_lock);
    uintptr_t err = 0;
    if (SetEnvironmentVariableW(key, value)) {
        crt_sync(key);
    } else {
        err = GetLastError();
    }
    ReleaseSRWLockExclusive(&env_lock);
    if (err == 0 && std::wcscmp(key, L"GODEBUG") == 0) {
        // (as Go's runtime: syscall_runtimeSetenv and syscall_runtimeUnsetenv)
        gp::godebug::env_changed(value == nullptr ? std::string()
                                                  : gp::os::to_utf8(value, std::wcslen(value)));
    }
    return err;
}

} // namespace

uintptr_t win_setenv(uintptr_t key, uintptr_t value) {
    // The two arguments are the NUL-terminated UTF-16 that Go's Setenv made (lone surrogates
    // intact).
    return set_env(reinterpret_cast<const wchar_t*>(key), reinterpret_cast<const wchar_t*>(value));
}

uintptr_t win_unsetenv(uintptr_t key) {
    return set_env(reinterpret_cast<const wchar_t*>(key), nullptr);
}

void win_env_read_lock() {
    AcquireSRWLockShared(&env_lock);
}

void win_env_read_unlock() {
    ReleaseSRWLockShared(&env_lock);
}

void runtime_setenv(const gp::string& key, const gp::string& value) {
    std::wstring k = wtf8_to_wide(key.gp_view());
    std::wstring v = wtf8_to_wide(value.gp_view());
    (void)win_setenv(reinterpret_cast<uintptr_t>(k.c_str()),
                     reinterpret_cast<uintptr_t>(v.c_str()));
}

void runtime_unsetenv(const gp::string& key) {
    std::wstring k = wtf8_to_wide(key.gp_view());
    (void)win_unsetenv(reinterpret_cast<uintptr_t>(k.c_str()));
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
