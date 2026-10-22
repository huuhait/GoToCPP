// The Windows part of the natives of package testing (gocpp/testing.hpp): what needs system calls.
// The semantics are those of the Linux part (src/os/linux/testing_os.cpp) and of Go's os package on
// Windows: the environment is the process's (SetEnvironmentVariable and GetEnvironmentVariable,
// case-insensitive names, like syscall.Setenv and Getenv) and also the C library's copy, the
// temporary folder is GetTempPath, the text is UTF-8 outside and UTF-16 for the system. This
// folder is the only place in the runtime that may include OS headers.
#include <gocpp/nofortify.h>
#include <gocpp/rt.hpp>
#include <gocpp/testing.hpp>
#include "windows.hpp"

#include <atomic>
#include <chrono>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cwchar>
#include <string>
#include <string_view>

#include <windows.h>

#include <fcntl.h>
#include <io.h>
#include <stdlib.h>

namespace gp::testing {

namespace {

// error_text is the system's text of a Windows error code, as Go's Errno.Error() has it (the
// trailing white space and line ends are cut).
std::string error_text(DWORD code) {
    wchar_t* msg = nullptr;
    DWORD n = FormatMessageW(FORMAT_MESSAGE_ALLOCATE_BUFFER | FORMAT_MESSAGE_FROM_SYSTEM |
                                 FORMAT_MESSAGE_IGNORE_INSERTS,
                             nullptr, code, 0, reinterpret_cast<LPWSTR>(&msg), 0, nullptr);
    std::string text;
    if (n > 0 && msg != nullptr) {
        while (n > 0 && (msg[n - 1] == L'\r' || msg[n - 1] == L'\n' || msg[n - 1] == L' ')) {
            n--;
        }
        text = gp::os::to_utf8(msg, n);
    } else {
        text = "Windows error " + std::to_string(code);
    }
    if (msg != nullptr) {
        LocalFree(msg);
    }
    return text;
}

gp::string error_with(const std::string& what, DWORD code) {
    return gp::string::gp_copy(what + ": " + error_text(code));
}

// fix_long_path is Go's fixLongPath: a path of 248 characters or more (the limit of
// CreateDirectory) gets the \\?\ prefix that lifts it, and then it must have backslashes only. The
// path must be absolute and have no "." or ".." element; others are returned as they are.
std::wstring fix_long_path(const std::wstring& path, std::size_t from = 248) {
    if (path.size() < from || path.rfind(L"\\\\?\\", 0) == 0 || path.rfind(L"\\\\.\\", 0) == 0) {
        return path;
    }
    std::wstring p = path;
    for (wchar_t& c : p) {
        if (c == L'/') {
            c = L'\\';
        }
    }
    // The prefix turns off the system's cleaning of the path: "." and ".." elements stay as they
    // are, so such a path is left alone.
    for (std::size_t i = 0; i < p.size();) {
        std::size_t e = p.find(L'\\', i);
        if (e == std::wstring::npos) {
            e = p.size();
        }
        std::wstring element = p.substr(i, e - i);
        if (element == L"." || element == L"..") {
            return path;
        }
        i = e + 1;
    }
    if (p.rfind(L"\\\\", 0) == 0) {
        return L"\\\\?\\UNC\\" + p.substr(2); // \\server\share\...
    }
    if (p.size() >= 3 && p[1] == L':' && p[2] == L'\\') {
        return L"\\\\?\\" + p;
    }
    return path;
}

bool has_nul(const std::string& s) {
    return s.find('\0') != std::string::npos;
}

// The standard output capture. The temporary file is held through a C descriptor, which becomes
// the C library's descriptor 1 (C's printf and the children that inherit it write to it) and the
// process's standard output handle (STD_OUTPUT_HANDLE: what the runtime's own writes use,
// gp::os::write).
int saved_stdout = -1; // a duplicate of the real standard output while it is captured
int capture_fd = -1;   // the temporary file that gets it

// set_std_output makes the handle of descriptor 1 the standard output handle of the process.
void set_std_output() {
    intptr_t h = _get_osfhandle(1);
    if (h != -1) {
        SetStdHandle(STD_OUTPUT_HANDLE, reinterpret_cast<HANDLE>(h));
    }
}

std::wstring temp_path() {
    std::wstring buf(16, L'\0');
    for (int tries = 0; tries < 4; tries++) {
        DWORD n = GetTempPathW(static_cast<DWORD>(buf.size()), buf.data());
        if (n == 0) {
            return {};
        }
        if (n < buf.size()) {
            buf.resize(n);
            return buf;
        }
        buf.assign(n + 1, L'\0');
    }
    return {};
}

// A random number for the names of temporary folders: not secret (a name that exists is tried
// again), but different in every process and every call.
uint64_t next_random() {
    static std::atomic<uint64_t> counter{0};
    LARGE_INTEGER qpc;
    QueryPerformanceCounter(&qpc);
    uint64_t x = static_cast<uint64_t>(qpc.QuadPart) ^ (uint64_t(GetCurrentProcessId()) << 32) ^
                 (uint64_t(GetCurrentThreadId()) << 16) ^ GetTickCount64() ^
                 (counter.fetch_add(1, std::memory_order_relaxed) * 0x9E3779B97F4A7C15ull);
    x += 0x9E3779B97F4A7C15ull; // splitmix64
    x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9ull;
    x = (x ^ (x >> 27)) * 0x94D049BB133111EBull;
    return x ^ (x >> 31);
}

} // namespace

gp::string capture_start() {
    if (capture_fd >= 0) {
        return gp::string::gp_copy("testing: the standard output is captured already");
    }
    std::fflush(nullptr);
    std::wstring dir = temp_path();
    if (dir.empty()) {
        return error_with("cannot capture the standard output: GetTempPath", GetLastError());
    }
    std::wstring name = dir + L"gocpp-capture-" + std::to_wstring(GetCurrentProcessId()) + L"-" +
                        std::to_wstring(next_random() & 0xFFFFFFFF) + L".tmp";
    SECURITY_ATTRIBUTES sa{sizeof sa, nullptr, TRUE}; // inheritable: a child writes into it too
    HANDLE h = CreateFileW(fix_long_path(name).c_str(), GENERIC_READ | GENERIC_WRITE,
                           FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE, &sa, CREATE_NEW,
                           FILE_ATTRIBUTE_TEMPORARY | FILE_FLAG_DELETE_ON_CLOSE, nullptr);
    if (h == INVALID_HANDLE_VALUE) {
        return error_with("cannot capture the standard output: create " +
                              gp::os::to_utf8(name.data(), name.size()),
                          GetLastError());
    }
    int tmp = _open_osfhandle(reinterpret_cast<intptr_t>(h), _O_BINARY | _O_RDWR);
    if (tmp < 0) {
        CloseHandle(h);
        return gp::string::gp_copy("cannot capture the standard output: no file descriptor");
    }
    // The file is deleted when its last handle is closed. (Descriptor 1's first handle is closed
    // by _dup2: saved_stdout holds the real output meanwhile.)
    saved_stdout = _dup(1);
    if (saved_stdout < 0) {
        _close(tmp);
        return gp::string::gp_copy("cannot capture the standard output: no standard output");
    }
    capture_fd = tmp;
    _dup2(tmp, 1);
    set_std_output();
    return gp::string();
}

gp::string capture_stop() {
    if (capture_fd < 0) {
        return gp::string();
    }
    std::fflush(nullptr);
    if (saved_stdout >= 0) {
        _dup2(saved_stdout, 1);
        _close(saved_stdout);
        saved_stdout = -1;
    }
    set_std_output();
    std::string out;
    _lseeki64(capture_fd, 0, SEEK_SET);
    char buf[4096];
    for (;;) {
        int n = _read(capture_fd, buf, sizeof buf);
        if (n <= 0) {
            break;
        }
        out.append(buf, static_cast<std::size_t>(n));
    }
    _close(capture_fd);
    capture_fd = -1;
    return gp::string::gp_copy(out);
}

std::tuple<gp::string, bool> lookup_env(gp::string key) {
    std::string k(key.gp_view());
    if (has_nul(k)) {
        return {gp::string(), false};
    }
    std::wstring wk = gp::os::to_wide(k);
    std::wstring buf(128, L'\0');
    for (;;) {
        // (A variable that is set to "" gives 0 and leaves the last error as it was.)
        SetLastError(0);
        DWORD n = GetEnvironmentVariableW(wk.c_str(), buf.data(), static_cast<DWORD>(buf.size()));
        if (n == 0 && GetLastError() == ERROR_ENVVAR_NOT_FOUND) {
            return {gp::string(), false};
        }
        if (n < buf.size()) { // the length without the NUL
            return {gp::string::gp_copy(gp::os::to_utf8(buf.data(), n)), true};
        }
        buf.assign(n, L'\0'); // the size with the NUL
    }
}

gp::string set_env(gp::string key, gp::string value) {
    std::string k(key.gp_view()), v(value.gp_view());
    if (k.empty() || k.find('=') != std::string::npos || has_nul(k) || has_nul(v)) {
        return gp::string::gp_copy("setenv: invalid argument");
    }
    std::wstring wk = gp::os::to_wide(k), wv = gp::os::to_wide(v);
    // The C library keeps its own copy of the environment (getenv of C and C++ code reads it); an
    // empty value deletes the variable there, so the system's block is set after it.
    _wputenv_s(wk.c_str(), wv.c_str());
    if (!SetEnvironmentVariableW(wk.c_str(), wv.c_str())) {
        return error_with("setenv", GetLastError());
    }
    return gp::string();
}

gp::string unset_env(gp::string key) {
    std::string k(key.gp_view());
    if (k.empty() || k.find('=') != std::string::npos || has_nul(k)) {
        return gp::string::gp_copy("unsetenv: invalid argument");
    }
    std::wstring wk = gp::os::to_wide(k);
    _wputenv_s(wk.c_str(), L"");
    // A variable that is not set is not an error (os.Unsetenv on Linux; T.Setenv's cleanup).
    if (!SetEnvironmentVariableW(wk.c_str(), nullptr) && GetLastError() != ERROR_ENVVAR_NOT_FOUND) {
        return error_with("unsetenv", GetLastError());
    }
    return gp::string();
}

std::tuple<gp::string, gp::string> make_temp_dir(gp::string prefix) {
    std::wstring dir = temp_path();
    if (dir.empty()) {
        return {gp::string(), error_with("TempDir", GetLastError())};
    }
    // (GetTempPath ends in a backslash, also for a root like C:\: no separator is added.)
    std::wstring wp = gp::os::to_wide(prefix.gp_view());
    // os.MkdirTemp: the prefix and a random number, tried again while the name exists.
    for (int tries = 0; tries < 10000; tries++) {
        std::wstring path = dir + wp + std::to_wstring(static_cast<uint32_t>(next_random()));
        if (CreateDirectoryW(fix_long_path(path).c_str(), nullptr)) {
            return {gp::string::gp_copy(gp::os::to_utf8(path.data(), path.size())), gp::string()};
        }
        DWORD err = GetLastError();
        if (err != ERROR_ALREADY_EXISTS && err != ERROR_FILE_EXISTS) {
            return {gp::string(),
                    error_with("mkdir " + gp::os::to_utf8(path.data(), path.size()), err)};
        }
    }
    return {gp::string(), gp::string::gp_copy("mkdir " + gp::os::to_utf8(dir.data(), dir.size()) +
                                              ": too many attempts to find a free name")};
}

namespace {

// remove_tree removes the file or folder path (in the form that fix_long_path gives) with all it
// holds, and returns 0 or the first Windows error. A path that is not there is not an error. The
// read-only attribute is cleared first (DeleteFile and RemoveDirectory refuse such a file), and a
// link (a symbolic link or a junction to a folder) is removed itself, not followed. (It is not
// std::filesystem::remove_all: with MinGW-w64's library that never returns for a path of over
// 260 characters.)
DWORD remove_tree(const std::wstring& path) {
    DWORD attrs = GetFileAttributesW(path.c_str());
    if (attrs == INVALID_FILE_ATTRIBUTES) {
        DWORD err = GetLastError();
        return err == ERROR_FILE_NOT_FOUND || err == ERROR_PATH_NOT_FOUND ? 0 : err;
    }
    if ((attrs & FILE_ATTRIBUTE_READONLY) != 0) {
        SetFileAttributesW(path.c_str(), attrs & ~DWORD(FILE_ATTRIBUTE_READONLY));
    }
    if ((attrs & FILE_ATTRIBUTE_DIRECTORY) == 0) {
        return DeleteFileW(path.c_str()) ? 0 : GetLastError();
    }
    DWORD first = 0;
    if ((attrs & FILE_ATTRIBUTE_REPARSE_POINT) == 0) {
        WIN32_FIND_DATAW fd;
        HANDLE h = FindFirstFileExW((path + L"\\*").c_str(), FindExInfoBasic, &fd,
                                    FindExSearchNameMatch, nullptr, FIND_FIRST_EX_LARGE_FETCH);
        if (h == INVALID_HANDLE_VALUE) {
            DWORD err = GetLastError();
            if (err != ERROR_FILE_NOT_FOUND) {
                first = err;
            }
        } else {
            do {
                std::wstring name = fd.cFileName;
                if (name == L"." || name == L"..") {
                    continue;
                }
                DWORD err = remove_tree(path + L"\\" + name);
                if (err != 0 && first == 0) {
                    first = err;
                }
            } while (FindNextFileW(h, &fd));
            FindClose(h);
        }
    }
    if (!RemoveDirectoryW(path.c_str()) && first == 0) {
        first = GetLastError();
    }
    return first;
}

} // namespace

// to_slashes_wide is the path as UTF-16 with backslashes only.
std::wstring to_backslashes(std::string_view utf8) {
    std::wstring w = gp::os::to_wide(utf8);
    for (wchar_t& c : w) {
        if (c == L'/') {
            c = L'\\';
        }
    }
    return w;
}

namespace {

// same_path compares two paths with backslashes and no trailing separator, ignoring case (the
// file systems of Windows are case-insensitive).
bool same_path(const std::wstring& a, const std::wstring& b) {
    return !a.empty() && a.size() == b.size() &&
           CompareStringOrdinal(a.c_str(), static_cast<int>(a.size()), b.c_str(),
                                static_cast<int>(b.size()), TRUE) == CSTR_EQUAL;
}

// plain_path is a path as UTF-16 with backslashes, no trailing separator and no \\?\ prefix.
std::wstring plain_path(std::wstring w) {
    for (wchar_t& c : w) {
        if (c == L'/') {
            c = L'\\';
        }
    }
    while (!w.empty() && w.back() == L'\\') {
        w.pop_back();
    }
    if (w.rfind(L"\\\\?\\UNC\\", 0) == 0) {
        w = L"\\\\" + w.substr(8);
    } else if (w.rfind(L"\\\\?\\", 0) == 0) {
        w = w.substr(4);
    }
    return w;
}

} // namespace

// remove_target returns the path to remove, or the refusal for a path that names a folder that must
// stay. The checks are on the spelling (drives, shares, device paths, the dots, the temp folder and
// the profile), not on what the links resolve to, unlike the Linux helper (which resolves the
// working folder and the links of the folder part, and refuses the folders at the top of the
// tree): a relative name is not joined to the working folder, a short 8.3 name ("PROGRA~1") is not
// expanded, and a UNC name is only checked for its share. A test that removes such a name must
// not be able to name a system folder: the folder of a test is in the temp folder.
std::tuple<gp::string, gp::string> remove_target(gp::string path) {
    std::string given(path.gp_view());
    std::wstring w = to_backslashes(given);
    if (w.empty()) {
        return {gp::string(), gp::string()}; // nothing to remove (os.RemoveAll("") is nil)
    }
    auto refuse = [&](const char* why) -> std::tuple<gp::string, gp::string> {
        return {gp::string(),
                gp::string::gp_copy("RemoveAll \"" + given + "\": refusing to remove " + why)};
    };
    // The system stops at a NUL: "C:\\x\0" is "C:\\x", and "\\\\\0" would pass the checks below.
    if (given.find('\0') != std::string::npos) {
        return refuse("a path with a NUL byte in it");
    }
    while (!w.empty() && w.back() == L'\\') {
        w.pop_back();
    }
    if (w.empty()) {
        return refuse("the root");
    }
    // Device paths and volume names (\\.\C:, \\?\Volume{guid}\, \\?\GLOBALROOT\...) name no
    // folder that a test made. The prefix with a drive or UNC is fine.
    if (w.rfind(L"\\\\.\\", 0) == 0) {
        return refuse("a device path");
    }
    std::wstring rest = plain_path(w);
    if (w.rfind(L"\\\\?\\", 0) == 0 && w.rfind(L"\\\\?\\UNC\\", 0) != 0 &&
        !(rest.size() >= 2 && rest[1] == L':' && iswalpha(rest[0]))) {
        return refuse("a volume or device path");
    }
    // The elements. A drive letter ("C:") is cut off first: "C:." and "C:.." are the working
    // folder of that drive and its parent.
    bool unc = rest.rfind(L"\\\\", 0) == 0;
    std::wstring names = rest;
    bool drive = false;
    if (!unc && names.size() >= 2 && names[1] == L':') {
        names = names.substr(2);
        drive = true;
    }
    std::size_t elements = 0;
    for (std::size_t i = unc ? 2 : 0; i < names.size();) {
        std::size_t e = names.find(L'\\', i);
        if (e == std::wstring::npos) {
            e = names.size();
        }
        std::wstring element = names.substr(i, e - i);
        i = e + 1;
        if (element.empty()) {
            continue;
        }
        elements++;
        // Windows drops the dots and spaces at the end of a name: "." and ".." are folders
        // that are not the one written, and so are "..." and ". ".
        if (element.find_first_not_of(L". ") == std::wstring::npos) {
            return refuse("a path with . or .. in it");
        }
    }
    if (drive && elements == 0) {
        return refuse("a drive");
    }
    if (unc && elements < 3) {
        return refuse("a server or a share");
    }
    if (!drive && !unc && elements == 0) {
        return refuse("the root");
    }
    // The folder of temporary files and the user's profile themselves (what is in them is fine).
    std::wstring tmp = plain_path(temp_path());
    if (same_path(rest, tmp)) {
        return refuse("the folder of temporary files");
    }
    wchar_t profile[MAX_PATH * 2];
    DWORD n = GetEnvironmentVariableW(L"USERPROFILE", profile, sizeof profile / sizeof profile[0]);
    if (n > 0 && n < sizeof profile / sizeof profile[0] &&
        same_path(rest, plain_path(std::wstring(profile, n)))) {
        return refuse("the user profile folder");
    }
    return {gp::string::gp_copy(gp::os::to_utf8(w.data(), w.size())), gp::string()};
}

gp::string remove_all(gp::string path) {
    auto [target, bad] = remove_target(path);
    if (!bad.gp_view().empty()) {
        return bad;
    }
    if (target.gp_view().empty()) {
        return gp::string();
    }
    // The prefix always: the names below a folder are as long as its path and more, and a tree can
    // be deeper than the limit although its root is not. A name that is not absolute is left as it
    // is.
    std::wstring p = fix_long_path(to_backslashes(target.gp_view()), 0);
    // A file that another process (a virus scanner, a search indexer) has open for a moment cannot
    // be deleted: try again for up to two seconds, as Go's testing does for the folders of TempDir.
    const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(2);
    DWORD wait = 1;
    for (;;) {
        DWORD err = remove_tree(p);
        if (err == 0) {
            return gp::string();
        }
        if (std::chrono::steady_clock::now() >= deadline) {
            // (The message names the path as it was given, without the \\?\ prefix.)
            return error_with("RemoveAll " + std::string(target.gp_view()), err);
        }
        Sleep(wait);
        wait = wait * 2 > 200 ? 200 : wait * 2;
    }
}

gp::string make_dir(gp::string path) {
    std::string p(path.gp_view());
    if (!CreateDirectoryW(fix_long_path(to_backslashes(p)).c_str(), nullptr)) {
        return error_with("mkdir " + p, GetLastError());
    }
    return gp::string();
}

std::tuple<gp::string, gp::string> getwd() {
    std::wstring buf(16, L'\0'); // small: the growth below is the usual path of a long folder
    for (int tries = 0; tries < 4; tries++) {
        DWORD n = GetCurrentDirectoryW(static_cast<DWORD>(buf.size()), buf.data());
        if (n == 0) {
            return {gp::string(), error_with("getwd", GetLastError())};
        }
        if (n < buf.size()) {
            return {gp::string::gp_copy(gp::os::to_utf8(buf.data(), n)), gp::string()};
        }
        buf.assign(n + 1, L'\0');
    }
    return {gp::string(), gp::string::gp_copy("getwd: the working folder keeps changing")};
}

gp::string chdir(gp::string dir) {
    std::string d(dir.gp_view());
    if (!SetCurrentDirectoryW(to_backslashes(d).c_str())) {
        return error_with("chdir " + d, GetLastError());
    }
    return gp::string();
}

std::tuple<gp::string, gp::string> read_file(gp::string path) {
    std::string p(path.gp_view());
    HANDLE h =
        CreateFileW(fix_long_path(to_backslashes(p)).c_str(), GENERIC_READ,
                    FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE, nullptr, OPEN_EXISTING,
                    FILE_FLAG_SEQUENTIAL_SCAN | FILE_FLAG_BACKUP_SEMANTICS, nullptr);
    if (h == INVALID_HANDLE_VALUE) {
        return {gp::string(), error_with("open " + p, GetLastError())};
    }
    std::string data;
    char buf[65536];
    for (;;) {
        DWORD n = 0;
        if (!ReadFile(h, buf, sizeof buf, &n, nullptr)) {
            DWORD err = GetLastError();
            CloseHandle(h);
            return {gp::string(), error_with("read " + p, err)};
        }
        if (n == 0) {
            break;
        }
        data.append(buf, n);
    }
    CloseHandle(h);
    return {gp::string::gp_copy(data), gp::string()};
}

gp::string cpu_name() {
    // The processor's name is in the registry, as the CPU brand string: "Intel(R) Core(TM) ..."
    // (left-padded with spaces by some). Empty when the system does not say.
    wchar_t buf[256];
    DWORD size = sizeof buf - sizeof buf[0];
    LSTATUS st =
        RegGetValueW(HKEY_LOCAL_MACHINE, L"HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0",
                     L"ProcessorNameString", RRF_RT_REG_SZ, nullptr, buf, &size);
    if (st != ERROR_SUCCESS) {
        return gp::string();
    }
    return trim_cpu_name(gp::string::gp_copy(gp::os::to_utf8(buf, std::wcslen(buf))));
}

} // namespace gp::testing
