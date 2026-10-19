// The native functions of package testing (gocpp/testing.hpp, Phase 09 step 9.9). Standard C++
// only: the OS is reached through gocpp/os.hpp, and the parts that need system calls (the
// standard output capture, the environment, temporary folders) are in src/os/<os>/testing_os.cpp.
#include <gocpp/nofortify.h>
#include <gocpp/main.hpp>
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>
#include <gocpp/testing.hpp>
#include <gocpp/trace.hpp>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cerrno>
#include <condition_variable>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <filesystem>
#include <map>
#include <mutex>
#include <string>
#include <system_error>
#include <thread>
#include <unordered_map>
#include <vector>

namespace gp::testing {

namespace {

std::recursive_mutex state_mu;

// The alarm of the -test.timeout flag.
std::mutex alarm_mu;
std::condition_variable alarm_cv;
bool alarm_canceled = false;

// The running tests, by name.
std::mutex running_mu;
std::map<std::string, int64_t> running;

} // namespace

int64_t now() noexcept {
    using namespace std::chrono;
    return duration_cast<nanoseconds>(steady_clock::now().time_since_epoch()).count();
}

void sleep_ns(int64_t ns) {
    if (ns > 0) {
        std::this_thread::sleep_for(std::chrono::nanoseconds(ns));
    }
}

void lock() noexcept {
    state_mu.lock();
}

void unlock() noexcept {
    state_mu.unlock();
}

gp::slice<gp::string> args() {
    const std::vector<gp::string>& all = gp::args();
    int64_t n = all.empty() ? 0 : static_cast<int64_t>(all.size()) - 1;
    auto out = gp::make_slice<gp::string>(n);
    for (int64_t i = 0; i < n; i++) {
        out[i] = all[static_cast<std::size_t>(i) + 1];
    }
    return out;
}

gp::string program_name() {
    const std::vector<gp::string>& all = gp::args();
    return all.empty() ? gp::string() : all[0];
}

int64_t ncpu() noexcept {
    return os::cpu_count();
}

gp::string goos() {
#if defined(_WIN32)
    return gp::string::gp_copy("windows");
#elif defined(__APPLE__)
    return gp::string::gp_copy("darwin");
#else
    return gp::string::gp_copy("linux");
#endif
}

gp::string goarch() {
#if defined(__x86_64__) || defined(_M_X64)
    return gp::string::gp_copy("amd64");
#elif defined(__aarch64__) || defined(_M_ARM64)
    return gp::string::gp_copy("arm64");
#else
    return gp::string::gp_copy("unknown");
#endif
}

gp::string frames(int64_t max) {
    if (max <= 0) {
        max = 64;
    }
    // The stack of the caller from the runtime's own reader (frames.cpp): the program's file and
    // its line table are read once, and a call site that was asked about is kept.
    std::vector<uintptr_t> pcs = gp::detail::shown_frames(static_cast<std::size_t>(max));
    std::vector<gp::detail::frame_info> infos(pcs.size());
    gp::detail::resolve_pcs(pcs.data(), pcs.size(), true, infos.data());
    std::string out;
    for (const gp::detail::frame_info& f : infos) {
        if (!f.ok) {
            continue;
        }
        out += f.fn.name;
        out += '\t';
        out += f.file;
        out += '\t';
        out += std::to_string(f.line);
        out += '\n';
    }
    return gp::string::gp_copy(out);
}

std::tuple<gp::string, bool> lookup_env(gp::string key) {
    std::string k(key.gp_view());
    const char* v = std::getenv(k.c_str());
    if (v == nullptr) {
        return {gp::string(), false};
    }
    return {gp::string::gp_copy(v), true};
}

gp::string remove_all(gp::string path) {
    std::error_code ec;
    std::filesystem::remove_all(std::filesystem::path(std::string(path.gp_view())), ec);
    return ec ? gp::string::gp_copy(ec.message()) : gp::string();
}

gp::string make_dir(gp::string path) {
    std::error_code ec;
    std::filesystem::path p(std::string(path.gp_view()));
    if (std::filesystem::exists(p, ec)) {
        return gp::string::gp_copy("mkdir " + p.string() + ": file exists");
    }
    ec.clear();
    std::filesystem::create_directory(p, ec);
    return ec ? gp::string::gp_copy("mkdir " + p.string() + ": " + ec.message()) : gp::string();
}

std::tuple<gp::string, gp::string> getwd() {
    std::error_code ec;
    std::filesystem::path p = std::filesystem::current_path(ec);
    if (ec) {
        return {gp::string(), gp::string::gp_copy("getwd: " + ec.message())};
    }
    return {gp::string::gp_copy(p.string()), gp::string()};
}

gp::string chdir(gp::string dir) {
    std::error_code ec;
    std::string d(dir.gp_view());
    std::filesystem::current_path(std::filesystem::path(d), ec);
    return ec ? gp::string::gp_copy("chdir " + d + ": " + ec.message()) : gp::string();
}

std::tuple<gp::string, gp::string> read_files(gp::string dir) {
    namespace fs = std::filesystem;
    std::error_code ec;
    fs::path d{std::string(dir.gp_view())};
    if (!fs::exists(d, ec)) {
        return {gp::string(), gp::string()};
    }
    std::vector<std::string> names;
    for (fs::directory_iterator it(d, ec), end; !ec && it != end; it.increment(ec)) {
        std::error_code e2;
        if (!it->is_directory(e2)) {
            names.push_back(it->path().filename().string());
        }
    }
    if (ec) {
        return {gp::string(), gp::string::gp_copy(ec.message())};
    }
    std::sort(names.begin(), names.end());
    std::string out;
    for (const std::string& n : names) {
        out += n;
        out += '\n';
    }
    return {gp::string::gp_copy(out), gp::string()};
}

std::tuple<gp::string, gp::string> read_file(gp::string path) {
    std::string p(path.gp_view());
    std::FILE* f = std::fopen(p.c_str(), "rb");
    if (f == nullptr) {
        return {gp::string(), gp::string::gp_copy("open " + p + ": " + std::strerror(errno))};
    }
    std::string data;
    char buf[4096];
    std::size_t n;
    while ((n = std::fread(buf, 1, sizeof buf, f)) > 0) {
        data.append(buf, n);
    }
    std::fclose(f);
    return {gp::string::gp_copy(data), gp::string()};
}

std::tuple<double, bool> parse_float(gp::string s) {
    std::string text;
    for (char c : s.gp_view()) {
        if (c != '_') {
            text += c;
        }
    }
    if (text.empty()) {
        return {0.0, false};
    }
    char* end = nullptr;
    errno = 0;
    double v = std::strtod(text.c_str(), &end);
    return {v, end == text.c_str() + text.size()};
}

void exit(int64_t code) noexcept {
    os::exit(static_cast<int>(code));
}

bool alarm_wait(int64_t ns) {
    if (ns <= 0) {
        return false;
    }
    std::unique_lock<std::mutex> lk(alarm_mu);
    alarm_canceled = false;
    return !alarm_cv.wait_for(lk, std::chrono::nanoseconds(ns), [] { return alarm_canceled; });
}

void alarm_cancel() {
    {
        std::lock_guard<std::mutex> lk(alarm_mu);
        alarm_canceled = true;
    }
    alarm_cv.notify_all();
}

void running_add(gp::string name, int64_t start) {
    std::lock_guard<std::mutex> g(running_mu);
    running[std::string(name.gp_view())] = start;
}

void running_delete(gp::string name) {
    std::lock_guard<std::mutex> g(running_mu);
    running.erase(std::string(name.gp_view()));
}

gp::string running_list() {
    std::lock_guard<std::mutex> g(running_mu);
    std::string out;
    for (const auto& [name, start] : running) {
        out += name;
        out += '\t';
        out += std::to_string(start);
        out += '\n';
    }
    return gp::string::gp_copy(out);
}

} // namespace gp::testing
