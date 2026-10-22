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
#include <mutex>
#include <set>
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

namespace {

// The folders that make_temp_dir made, by the name it returned: a folder that is directly under
// the root (TMPDIR=/, as in some containers) is a top-level folder, and remove_target refuses
// those, except these.
std::mutex made_mutex;
std::set<std::string>& made_folders() {
    static std::set<std::string> folders;
    return folders;
}

} // namespace

std::tuple<gp::string, gp::string> make_temp_dir(gp::string prefix) {
    const char* base = std::getenv("TMPDIR");
    std::string dir = base != nullptr && *base != '\0' ? base : "/tmp";
    while (!dir.empty() && dir.back() == '/') {
        dir.pop_back(); // (TMPDIR=/ leaves "": the folder is /prefixXXXXXX, not //prefixXXXXXX)
    }
    std::string tmpl = dir + "/" + std::string(prefix.gp_view()) + "XXXXXX";
    if (::mkdtemp(tmpl.data()) == nullptr) {
        return {gp::string(), errno_text("mkdtemp " + tmpl)};
    }
    {
        std::lock_guard<std::mutex> lock(made_mutex);
        made_folders().insert(tmpl);
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

// collapse_slashes makes every run of separators one ("//tmp" and "/tmp" are the same folder).
std::string collapse_slashes(const std::string& p) {
    std::string out;
    for (char c : p) {
        if (c != '/' || out.empty() || out.back() != '/') {
            out.push_back(c);
        }
    }
    return out;
}

// resolved_spelling returns the absolute, canonical name of the folder or file that the path p
// (no . or .. in it) names, for the comparison with the folders that must stay: a relative name
// is joined to the working folder, and the links in the folder of the file are resolved, so
// "/tmp/link/x" with a link to "/" is "/x". The last element is not resolved: remove_all
// removes a symbolic link itself. A folder that is not there stays as it is spelled.
std::string resolved_spelling(const std::string& p) {
    std::string abs = p;
    if (abs.empty() || abs[0] != '/') {
        char buf[PATH_MAX];
        if (::getcwd(buf, sizeof buf) != nullptr) {
            abs = std::string(buf) + "/" + abs;
        }
    }
    abs = cut_slashes(collapse_slashes(abs));
    std::size_t slash = abs.rfind('/');
    if (slash == std::string::npos || abs == "/") {
        return abs;
    }
    std::string parent = slash == 0 ? "/" : abs.substr(0, slash);
    char real[PATH_MAX];
    if (::realpath(parent.c_str(), real) == nullptr) {
        return abs;
    }
    std::string resolved(real);
    return cut_slashes(resolved) == "/" ? "/" + abs.substr(slash + 1)
                                        : cut_slashes(resolved) + abs.substr(slash);
}

// names_folder reports whether the path p, as it was written or with its links and its working
// folder resolved, is the folder named by the environment variable var (or fallback when it is
// not set): what is in the folder is fine, the folder is not.
bool names_folder(const std::string& p, const std::string& resolved, const char* var,
                  const char* fallback) {
    const char* value = std::getenv(var);
    std::string folder = value != nullptr && *value != '\0' ? value : fallback;
    if (folder.empty()) {
        return false;
    }
    folder = cut_slashes(collapse_slashes(folder));
    if (p == folder || resolved == folder || resolved == resolved_spelling(folder)) {
        return true;
    }
    // The variable may name the folder through a link (HOME=/home/me with /home/me -> /data/me),
    // and the path the folder behind it: compare with the folder with every link resolved too.
    char real[PATH_MAX];
    return ::realpath(folder.c_str(), real) != nullptr && cut_slashes(real) == resolved;
}

// top_level reports whether the absolute path p has one element: /home, /etc, /usr, /var, ... A
// test has no reason to remove one, and whatever the tree below it is, removing it is a disaster.
bool top_level(const std::string& p) {
    return p.size() > 1 && p[0] == '/' && p.find('/', 1) == std::string::npos;
}

// under reports whether the path p is the folder prefix or in it.
bool under(const std::string& p, const std::string& prefix) {
    return p == prefix || p.rfind(prefix + "/", 0) == 0;
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
    // The system stops at a NUL: "/\0" would delete "/", whatever the check made of the rest.
    if (given.find('\0') != std::string::npos) {
        return refuse("a path with a NUL byte in it");
    }
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
    // The checks below look at the folder that the path really names: separators repeated
    // ("//tmp"), a relative spelling ("tmp" in "/") and a link in the folder ("/tmp/lnk/self"
    // with lnk -> /proc) give the same answer as the plain spelling.
    std::string plain = cut_slashes(collapse_slashes(p));
    std::string resolved = resolved_spelling(p);
    if (resolved == "/" || plain == "/") {
        return refuse("the root");
    }
    if (under(plain, "/proc") || under(resolved, "/proc")) {
        return refuse("the process information folder");
    }
    if (under(plain, "/sys") || under(resolved, "/sys")) {
        return refuse("the system information folder");
    }
    // The device folder: a test makes its files in /dev/shm and /dev/mqueue at most.
    for (const std::string& name : {plain, resolved}) {
        bool made_by_test = name.rfind("/dev/shm/", 0) == 0 || name.rfind("/dev/mqueue/", 0) == 0;
        if (under(name, "/dev") && !made_by_test) {
            return refuse("the device folder");
        }
    }
    // The folders at the top of the tree (/home, /etc, /usr, ...), also through a link to them.
    if (top_level(plain) || top_level(resolved)) {
        bool made = false;
        {
            std::lock_guard<std::mutex> lock(made_mutex);
            made = made_folders().count(plain) != 0 || made_folders().count(resolved) != 0;
        }
        if (!made) {
            return refuse("a folder at the top of the file system");
        }
    }
    // The home folder and the folder of temporary files themselves (what is in them is fine).
    if (names_folder(plain, resolved, "HOME", "")) {
        return refuse("the home folder");
    }
    if (names_folder(plain, resolved, "TMPDIR", "/tmp")) {
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
