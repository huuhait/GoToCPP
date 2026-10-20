// The Linux part of the natives of package testing (gocpp/testing.hpp): what needs system calls.
// This folder is the only place in the runtime that may include OS headers.
#include <gocpp/nofortify.h>
#include <gocpp/rt.hpp>
#include <gocpp/testing.hpp>

#include <cerrno>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <filesystem>
#include <string>
#include <system_error>

#include <climits>
#include <fcntl.h>
#include <sys/stat.h>
#include <unistd.h>

namespace gp::testing {

namespace {

int saved_stdout = -1; // the real standard output while it is captured
int capture_fd = -1;   // the temporary file that gets it

gp::string errno_text(const std::string& what) {
    return gp::string::gp_copy(what + ": " + std::strerror(errno));
}

} // namespace

gp::string capture_start() {
    if (capture_fd >= 0) {
        return gp::string::gp_copy("testing: the standard output is captured already");
    }
    std::fflush(nullptr);
    const char* dir = std::getenv("TMPDIR");
    std::string name =
        std::string(dir != nullptr && *dir != '\0' ? dir : "/tmp") + "/gocpp-capture-XXXXXX";
    int tmp = ::mkstemp(name.data());
    if (tmp < 0) {
        return errno_text("cannot capture the standard output: mkstemp " + name);
    }
    ::unlink(name.c_str()); // the file lives while it is open
    saved_stdout = ::dup(1);
    capture_fd = tmp;
    ::dup2(tmp, 1);
    return gp::string();
}

gp::string capture_stop() {
    if (capture_fd < 0) {
        return gp::string();
    }
    std::fflush(nullptr);
    ::dup2(saved_stdout, 1);
    ::close(saved_stdout);
    saved_stdout = -1;
    std::string out;
    ::lseek(capture_fd, 0, SEEK_SET);
    char buf[4096];
    for (;;) {
        ssize_t n = ::read(capture_fd, buf, sizeof buf);
        if (n < 0 && errno == EINTR) {
            continue;
        }
        if (n <= 0) {
            break;
        }
        out.append(buf, static_cast<std::size_t>(n));
    }
    ::close(capture_fd);
    capture_fd = -1;
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

gp::string set_env(gp::string key, gp::string value) {
    std::string k(key.gp_view()), v(value.gp_view());
    if (k.empty() || k.find('=') != std::string::npos || k.find('\0') != std::string::npos ||
        v.find('\0') != std::string::npos) {
        return gp::string::gp_copy("setenv: invalid argument");
    }
    if (::setenv(k.c_str(), v.c_str(), 1) != 0) {
        return errno_text("setenv");
    }
    return gp::string();
}

gp::string unset_env(gp::string key) {
    std::string k(key.gp_view());
    if (k.empty() || k.find('=') != std::string::npos) {
        return gp::string::gp_copy("unsetenv: invalid argument");
    }
    if (::unsetenv(k.c_str()) != 0) {
        return errno_text("unsetenv");
    }
    return gp::string();
}

std::tuple<gp::string, gp::string> make_temp_dir(gp::string prefix) {
    const char* base = std::getenv("TMPDIR");
    std::string dir = base != nullptr && *base != '\0' ? base : "/tmp";
    while (dir.size() > 1 && dir.back() == '/') {
        dir.pop_back();
    }
    std::string tmpl = dir + "/" + std::string(prefix.gp_view()) + "XXXXXX";
    if (::mkdtemp(tmpl.data()) == nullptr) {
        return {gp::string(), errno_text("mkdtemp " + tmpl)};
    }
    return {gp::string::gp_copy(tmpl), gp::string()};
}

gp::string remove_all(gp::string path) {
    auto [target, bad] = remove_target(path);
    if (!bad.gp_view().empty()) {
        return bad;
    }
    if (target.gp_view().empty()) {
        return gp::string();
    }
    // (remove_all removes a symbolic link itself, it does not follow it.)
    std::error_code ec;
    std::filesystem::remove_all(std::filesystem::path(std::string(target.gp_view())), ec);
    return ec ? gp::string::gp_copy(ec.message()) : gp::string();
}

namespace {

// cut_slashes removes the trailing separators of a path (but keeps "/").
std::string cut_slashes(std::string p) {
    while (p.size() > 1 && p.back() == '/') {
        p.pop_back();
    }
    return p;
}

} // namespace

std::tuple<gp::string, gp::string> remove_target(gp::string path) {
    std::string given(path.gp_view());
    if (given.empty()) {
        return {gp::string(), gp::string()}; // nothing to remove (os.RemoveAll("") is nil)
    }
    auto refuse = [&](const char* why) -> std::tuple<gp::string, gp::string> {
        return {gp::string(),
                gp::string::gp_copy("RemoveAll \"" + given + "\": refusing to remove " + why)};
    };
    // The trailing separators go, so that a symbolic link is removed itself and not followed
    // ("link/" would name the folder it points to).
    std::string p = cut_slashes(given);
    if (p == "/") {
        return refuse("the root");
    }
    // A "." or ".." element names a folder that is not the one written (Go refuses RemoveAll(".")
    // as an invalid argument): "/tmp/.." is the root, ".." the parent of the working folder.
    for (std::size_t i = 0; i < p.size();) {
        std::size_t e = p.find('/', i);
        if (e == std::string::npos) {
            e = p.size();
        }
        std::string_view element(p.data() + i, e - i);
        if (element == "." || element == "..") {
            return refuse("a path with . or .. in it");
        }
        i = e + 1;
    }
    if (p == "/proc" || p.rfind("/proc/", 0) == 0) {
        return refuse("the process information folder");
    }
    // The home folder and the folder of temporary files themselves (what is in them is fine).
    const char* home = std::getenv("HOME");
    if (home != nullptr && *home != '\0' && cut_slashes(home) == p) {
        return refuse("the home folder");
    }
    const char* tmp = std::getenv("TMPDIR");
    if (cut_slashes(tmp != nullptr && *tmp != '\0' ? tmp : "/tmp") == p) {
        return refuse("the folder of temporary files");
    }
    return {gp::string::gp_copy(p), gp::string()};
}

gp::string make_dir(gp::string path) {
    std::string p(path.gp_view());
    if (::mkdir(p.c_str(), 0777) != 0) {
        return errno_text("mkdir " + p);
    }
    return gp::string();
}

std::tuple<gp::string, gp::string> getwd() {
    char buf[PATH_MAX];
    if (::getcwd(buf, sizeof buf) == nullptr) {
        return {gp::string(), errno_text("getwd")};
    }
    return {gp::string::gp_copy(buf), gp::string()};
}

gp::string chdir(gp::string dir) {
    std::string d(dir.gp_view());
    if (::chdir(d.c_str()) != 0) {
        return errno_text("chdir " + d);
    }
    return gp::string();
}

std::tuple<gp::string, gp::string> read_file(gp::string path) {
    std::string p(path.gp_view());
    std::FILE* f = std::fopen(p.c_str(), "rb");
    if (f == nullptr) {
        return {gp::string(), errno_text("open " + p)};
    }
    std::string data;
    char buf[4096];
    std::size_t n;
    while ((n = std::fread(buf, 1, sizeof buf, f)) > 0) {
        data.append(buf, n);
    }
    bool failed = std::ferror(f) != 0;
    gp::string why = failed ? errno_text("read " + p) : gp::string();
    std::fclose(f);
    if (failed) {
        return {gp::string(), why};
    }
    return {gp::string::gp_copy(data), gp::string()};
}

gp::string cpu_name() {
    std::FILE* f = std::fopen("/proc/cpuinfo", "r");
    if (f == nullptr) {
        return gp::string();
    }
    std::string result;
    char line[1024];
    while (std::fgets(line, sizeof line, f) != nullptr) {
        std::string l(line);
        if (l.rfind("model name", 0) == 0) {
            auto colon = l.find(':');
            if (colon != std::string::npos) {
                result = l.substr(colon + 1);
            }
            break;
        }
    }
    std::fclose(f);
    return trim_cpu_name(gp::string::gp_copy(result));
}

} // namespace gp::testing
