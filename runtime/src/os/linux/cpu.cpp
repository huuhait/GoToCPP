// The CPU limit of the process's control group (gocpp/os.hpp, cpu_limit), found as Go 1.25's
// runtime finds it (internal/runtime/cgroup): /proc/self/mountinfo says where the cpu controller
// is mounted, /proc/self/cgroup which group the process is in, and the limit is in a file of
// that group: cgroup v2's cpu.max ("max 100000", or "200000 100000": the quota and the period), or
// v1's cpu.cfs_quota_us (-1: no limit) and cpu.cfs_period_us. Like Go it reads only the group of
// the process itself, not the limits of its parents, and it reads once, at the start.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>

#include <cstdint>
#include <cstdio>
#include <string>
#include <string_view>

namespace gp::os {

namespace {

// containsCPU reports whether the comma-separated list has the entry "cpu".
bool contains_cpu(std::string_view list) {
    while (!list.empty()) {
        std::size_t i = list.find(',');
        std::string_view one = list.substr(0, i);
        if (one == "cpu") {
            return true;
        }
        if (i == std::string_view::npos) {
            break;
        }
        list.remove_prefix(i + 1);
    }
    return false;
}

// unescape decodes the \040 style escapes that mountinfo writes (a space, a tab, a newline and
// a backslash in a path). It returns false for a malformed escape.
bool unescape(std::string_view in, std::string& out) {
    out.clear();
    for (std::size_t i = 0; i < in.size(); i++) {
        if (in[i] != '\\') {
            out += in[i];
            continue;
        }
        if (i + 3 >= in.size()) {
            return false;
        }
        int c = 0;
        for (int k = 1; k <= 3; k++) {
            char d = in[i + static_cast<std::size_t>(k)];
            if (d < '0' || d > '9') {
                return false;
            }
            c = c * 8 + (d - '0');
        }
        out += static_cast<char>(c);
        i += 3;
    }
    return true;
}

// next_line splits the first line off text.
std::string_view next_line(std::string_view& text) {
    std::size_t i = text.find('\n');
    std::string_view line = text.substr(0, i);
    text.remove_prefix(i == std::string_view::npos ? text.size() : i + 1);
    return line;
}

// cpu_mount finds the mount point of the cpu controller in the contents of mountinfo: a cgroup
// v1 mount whose super options have "cpu", or the cgroup2 mount (the last one wins when there
// are several, as in Go). version is 1 or 2; false when there is none.
bool cpu_mount(std::string_view mountinfo, std::string& mount, int& version) {
    bool found = false;
    while (!mountinfo.empty()) {
        std::string_view line = next_line(mountinfo);
        // id parent major:minor root mount-point options optional-fields - fstype source
        // super-options
        for (int k = 0; k < 4; k++) {
            std::size_t i = line.find(' ');
            if (i == std::string_view::npos) {
                return false;
            }
            line.remove_prefix(i + 1);
        }
        std::size_t i = line.find(' ');
        if (i == std::string_view::npos) {
            return false;
        }
        std::string_view mnt = line.substr(0, i);
        line.remove_prefix(i + 1);
        for (;;) {
            i = line.find(' ');
            if (i == std::string_view::npos || i + 3 >= line.size()) {
                return false;
            }
            if (line.substr(i, 3) == " - ") {
                line.remove_prefix(i + 3);
                break;
            }
            line.remove_prefix(i + 1);
        }
        i = line.find(' ');
        if (i == std::string_view::npos) {
            return false;
        }
        std::string_view fstype = line.substr(0, i);
        line.remove_prefix(i + 1);
        if (fstype == "cgroup2") {
            if (!unescape(mnt, mount)) {
                return false;
            }
            version = 2;
            found = true;
            continue;
        }
        if (fstype != "cgroup") {
            continue;
        }
        i = line.find(' ');
        if (i == std::string_view::npos) {
            return false;
        }
        line.remove_prefix(i + 1); // the source
        if (contains_cpu(line)) {
            if (!unescape(mnt, mount)) {
                return false;
            }
            version = 1;
            return true;
        }
    }
    return found;
}

// cpu_path finds the control group of the process in the contents of /proc/self/cgroup: the
// path of the v1 hierarchy that has the cpu controller, else the v2 one ("0::/path").
bool cpu_path(std::string_view cgroup, std::string& path) {
    bool found = false;
    while (!cgroup.empty()) {
        std::string_view line = next_line(cgroup);
        std::size_t i = line.find(':');
        if (i == std::string_view::npos) {
            return false;
        }
        std::string_view hierarchy = line.substr(0, i);
        line.remove_prefix(i + 1);
        i = line.find(':');
        if (i == std::string_view::npos) {
            return false;
        }
        std::string_view controllers = line.substr(0, i);
        std::string_view p = line.substr(i + 1);
        if (hierarchy == "0") {
            path = std::string(p);
            found = !path.empty();
        } else if (contains_cpu(controllers)) {
            path = std::string(p);
            return true;
        }
    }
    return found;
}

// parse_int reads a decimal number (with a sign) from the start of s, up to a space or a
// newline.
bool parse_int(std::string_view s, int64_t& v) {
    std::size_t i = 0;
    bool neg = false;
    if (i < s.size() && (s[i] == '-' || s[i] == '+')) {
        neg = s[i] == '-';
        i++;
    }
    if (i == s.size()) {
        return false;
    }
    int64_t n = 0;
    for (; i < s.size(); i++) {
        if (s[i] < '0' || s[i] > '9') {
            return false;
        }
        n = n * 10 + (s[i] - '0');
        if (n > (int64_t(1) << 52)) {
            return false;
        }
    }
    v = neg ? -n : n;
    return true;
}

std::string_view trim_end(std::string_view s) {
    while (!s.empty() && (s.back() == '\n' || s.back() == ' ')) {
        s.remove_suffix(1);
    }
    return s;
}

// read_file reads the file at path whole (small files only: the limit files).
bool read_whole(const std::string& path, std::string& out) {
    std::FILE* f = std::fopen(path.c_str(), "re");
    if (f == nullptr) {
        return false;
    }
    char buf[4096];
    std::size_t n = std::fread(buf, 1, sizeof buf, f);
    bool ok = std::ferror(f) == 0 && n < sizeof buf;
    std::fclose(f);
    if (ok) {
        out.assign(buf, n);
    }
    return ok;
}

} // namespace

double cgroup_cpu_limit(
    std::string_view proc_cgroup, std::string_view mountinfo,
    const std::function<bool(const std::string& path, std::string& out)>& read_file) noexcept {
    try {
        std::string mount;
        int version = 0;
        std::string rel;
        if (!cpu_mount(mountinfo, mount, version) || !cpu_path(proc_cgroup, rel)) {
            return 0;
        }
        if (rel == "/") {
            rel.clear(); // the root group: the files are in the mount point itself
        }
        std::string base = mount + rel;
        std::string text;
        if (version == 2) {
            // "max 100000" or "200000 100000"
            if (!read_file(base + "/cpu.max", text)) {
                return 0;
            }
            std::string_view v = trim_end(text);
            std::size_t sp = v.find(' ');
            if (sp == std::string_view::npos || v.substr(0, sp) == "max") {
                return 0;
            }
            int64_t quota = 0;
            int64_t period = 0;
            if (!parse_int(v.substr(0, sp), quota) || !parse_int(v.substr(sp + 1), period) ||
                quota <= 0 || period <= 0) {
                return 0;
            }
            return static_cast<double>(quota) / static_cast<double>(period);
        }
        std::string quota_text;
        std::string period_text;
        if (!read_file(base + "/cpu.cfs_quota_us", quota_text) ||
            !read_file(base + "/cpu.cfs_period_us", period_text)) {
            return 0;
        }
        int64_t quota = 0;
        int64_t period = 0;
        if (!parse_int(trim_end(quota_text), quota) || !parse_int(trim_end(period_text), period) ||
            quota <= 0 || period <= 0) {
            return 0; // a quota of -1 is no limit
        }
        return static_cast<double>(quota) / static_cast<double>(period);
    } catch (...) {
        return 0;
    }
}

double cpu_limit() noexcept {
    try {
        std::string cgroup;
        std::string mountinfo;
        if (!read_whole("/proc/self/cgroup", cgroup)) {
            return 0;
        }
        // mountinfo can be bigger than the small files that read_whole is for.
        std::FILE* f = std::fopen("/proc/self/mountinfo", "re");
        if (f == nullptr) {
            return 0;
        }
        char buf[4096];
        std::size_t n;
        while ((n = std::fread(buf, 1, sizeof buf, f)) > 0) {
            mountinfo.append(buf, n);
        }
        std::fclose(f);
        return cgroup_cpu_limit(cgroup, mountinfo, read_whole);
    } catch (...) {
        return 0;
    }
}

} // namespace gp::os
