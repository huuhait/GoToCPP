// The Linux part of the natives of package testing (gocpp/testing.hpp): what needs system calls.
// This folder is the only place in the runtime that may include OS headers.
#include <gocpp/nofortify.h>
#include <gocpp/rt.hpp>
#include <gocpp/testing.hpp>

#include <cerrno>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <string>

#include <fcntl.h>
#include <unistd.h>

namespace gp::testing {

namespace {

int saved_stdout = -1; // the real standard output while it is captured
int capture_fd = -1;   // the temporary file that gets it

gp::string errno_text(const std::string& what) {
    return gp::string::gp_copy(what + ": " + std::strerror(errno));
}

} // namespace

void capture_start() {
    std::fflush(nullptr);
    const char* dir = std::getenv("TMPDIR");
    std::string name =
        std::string(dir != nullptr && *dir != '\0' ? dir : "/tmp") + "/gocpp-capture-XXXXXX";
    int tmp = ::mkstemp(name.data());
    if (tmp < 0) {
        return;
    }
    ::unlink(name.c_str()); // the file lives while it is open
    saved_stdout = ::dup(1);
    capture_fd = tmp;
    ::dup2(tmp, 1);
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
                while (!result.empty() && (result.front() == ' ' || result.front() == '\t')) {
                    result.erase(result.begin());
                }
                while (!result.empty() && (result.back() == '\n' || result.back() == '\r')) {
                    result.pop_back();
                }
            }
            break;
        }
    }
    std::fclose(f);
    return gp::string::gp_copy(result);
}

} // namespace gp::testing
