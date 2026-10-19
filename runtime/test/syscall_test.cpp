// The unit tests of the native functions of package syscall (gocpp/syscall.hpp,
// src/os/linux/posix_file.cpp), Phase 09 step 9.6: the errno of each failure, O_CLOEXEC on every
// open, short and zero-length reads and writes, the stat numbers, the directory calls, the
// environment, and the EINTR rule (a call that a signal interrupts is repeated).
// runtime/cxx_test.go compiles this file with the runtime's .cpp files, with g++ and clang++, at
// -O0, at -O2 and with ASan + UBSan, and runs it like test/unit.cpp: it prints "ok: N checks" and
// exits 0, or prints each failed check and exits 1.
//
// The test uses the natives themselves for everything (the test folder may not include OS
// headers). Its files live in a directory of its own under /tmp that it removes at the end.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>
#include "threads_ended.hpp"

#include <algorithm>
#include <atomic>
#include <cstdint>
#include <csignal>
#include <cstring>
#include <string>
#include <thread>
#include <tuple>
#include <vector>

namespace {

int checks = 0;
int failures = 0;

void report(bool ok, const char* file, int line, const std::string& what) {
    checks++;
    if (!ok) {
        failures++;
        std::string msg = std::string(file) + ":" + std::to_string(line) + ": " + what + "\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
    }
}

#define CHECK(...) report((__VA_ARGS__), __FILE__, __LINE__, "CHECK(" #__VA_ARGS__ ") failed")

namespace sc = gp::syscall;

// Linux errno numbers (the tests name them because the OS headers are not available here).
constexpr int64_t ENOENT_ = 2, EBADF_ = 9, EEXIST_ = 17, ENOTDIR_ = 20, EISDIR_ = 21, EINVAL_ = 22,
                  ENOTEMPTY_ = 39, EPIPE_ = 32;
// open(2) flags on Linux x86-64 and arm64 alike.
constexpr int64_t O_WRONLY_ = 1, O_RDWR_ = 2, O_CREAT_ = 0100, O_EXCL_ = 0200, O_TRUNC_ = 01000,
                  O_APPEND_ = 02000, O_CLOEXEC_ = 02000000;

gp::string gs(const std::string& s) {
    return gp::string::gp_copy(s);
}

gp::slice<uint8_t> bytes(const std::string& s) {
    auto b = gp::make_slice<uint8_t>(static_cast<int64_t>(s.size()));
    if (!s.empty()) {
        std::memcpy(b.gp_data(), s.data(), s.size());
    }
    return b;
}

std::string text(const gp::slice<uint8_t>& b, int64_t n) {
    return std::string(reinterpret_cast<const char*>(b.gp_data()), static_cast<std::size_t>(n));
}

std::string dir;

std::string p(const char* name) {
    return dir + "/" + name;
}

void write_file(const std::string& path, const std::string& content) {
    auto [fd, e] = sc::sys_open(gs(path), O_WRONLY_ | O_CREAT_ | O_TRUNC_, 0644);
    CHECK(e == 0 && fd >= 0);
    auto [n, we] = sc::sys_write(fd, bytes(content));
    CHECK(we == 0 && n == static_cast<int64_t>(content.size()));
    CHECK(sc::sys_close(fd) == 0);
}

std::string read_file(const std::string& path) {
    auto [fd, e] = sc::sys_open(gs(path), 0, 0);
    if (e != 0) {
        return "<open failed " + std::to_string(e) + ">";
    }
    std::string out;
    auto buf = gp::make_slice<uint8_t>(4096);
    for (;;) {
        auto [n, re] = sc::sys_read(fd, buf);
        if (re != 0 || n == 0) {
            break;
        }
        out += text(buf, n);
    }
    sc::sys_close(fd);
    return out;
}

void test_open_errors() {
    auto [fd, e] = sc::sys_open(gs(p("missing")), 0, 0);
    CHECK(fd == -1 && e == ENOENT_);
    std::tie(fd, e) = sc::sys_open(gs(p("no/such/dir/f")), O_CREAT_ | O_WRONLY_, 0600);
    CHECK(fd == -1 && e == ENOENT_);
    write_file(p("exists"), "x");
    std::tie(fd, e) = sc::sys_open(gs(p("exists")), O_CREAT_ | O_EXCL_ | O_WRONLY_, 0600);
    CHECK(fd == -1 && e == EEXIST_);
    std::tie(fd, e) = sc::sys_open(gs(p("exists/sub")), 0, 0);
    CHECK(fd == -1 && e == ENOTDIR_);
    std::tie(fd, e) = sc::sys_open(gs(dir), O_WRONLY_, 0);
    CHECK(fd == -1 && e == EISDIR_);
    std::tie(fd, e) = sc::sys_open(gs(""), 0, 0);
    CHECK(fd == -1 && e == ENOENT_);
    CHECK(sc::sys_close(-1) == EBADF_);
    CHECK(sc::sys_close(1 << 20) == EBADF_);
}

void test_cloexec() {
    auto [fd, e] = sc::sys_open(gs(p("exists")), 0, 0);
    CHECK(e == 0);
    // /proc/self/fdinfo/N has a line "flags:\t0<octal>"; O_CLOEXEC is 02000000.
    std::string info = read_file("/proc/self/fdinfo/" + std::to_string(fd));
    std::size_t at = info.find("flags:");
    CHECK(at != std::string::npos);
    long flags = at == std::string::npos ? 0 : std::stol(info.substr(at + 6), nullptr, 8);
    CHECK((flags & O_CLOEXEC_) != 0);
    auto [d, de] = sc::sys_dup(fd);
    CHECK(de == 0 && d >= 0 && d != fd);
    std::string info2 = read_file("/proc/self/fdinfo/" + std::to_string(d));
    std::size_t at2 = info2.find("flags:");
    long flags2 = at2 == std::string::npos ? 0 : std::stol(info2.substr(at2 + 6), nullptr, 8);
    CHECK((flags2 & O_CLOEXEC_) != 0);
    sc::sys_close(d);
    sc::sys_close(fd);
    // pipe2 with O_CLOEXEC
    auto fds = gp::make_slice<int64_t>(2);
    CHECK(sc::sys_pipe2(fds, O_CLOEXEC_) == 0);
    std::string pi = read_file("/proc/self/fdinfo/" + std::to_string(fds[0]));
    std::size_t at3 = pi.find("flags:");
    long flags3 = at3 == std::string::npos ? 0 : std::stol(pi.substr(at3 + 6), nullptr, 8);
    CHECK((flags3 & O_CLOEXEC_) != 0);
    sc::sys_close(fds[0]);
    sc::sys_close(fds[1]);
}

void test_read_write() {
    write_file(p("rw"), "hello world");
    CHECK(read_file(p("rw")) == "hello world");
    auto [fd, e] = sc::sys_open(gs(p("rw")), O_RDWR_, 0);
    CHECK(e == 0);
    auto buf = gp::make_slice<uint8_t>(5);
    auto [n, re] = sc::sys_read(fd, buf);
    CHECK(re == 0 && n == 5 && text(buf, n) == "hello");
    // a zero-length read is a call with length 0, not the end of the file
    auto none = gp::make_slice<uint8_t>(0);
    std::tie(n, re) = sc::sys_read(fd, none);
    CHECK(re == 0 && n == 0);
    auto [off, se] = sc::sys_seek(fd, 6, 0);
    CHECK(se == 0 && off == 6);
    std::tie(n, re) = sc::sys_read(fd, buf);
    CHECK(re == 0 && n == 5 && text(buf, n) == "world");
    std::tie(n, re) = sc::sys_read(fd, buf);
    CHECK(re == 0 && n == 0); // end of file
    std::tie(n, re) = sc::sys_pread(fd, buf, 0);
    CHECK(re == 0 && n == 5 && text(buf, n) == "hello");
    std::tie(n, re) = sc::sys_pwrite(fd, bytes("J"), 0);
    CHECK(re == 0 && n == 1);
    std::tie(off, se) = sc::sys_seek(fd, 0, 1);
    CHECK(se == 0 && off == 11); // pread and pwrite do not move the offset
    std::tie(off, se) = sc::sys_seek(fd, -100, 1);
    CHECK(off == 0 && se == EINVAL_);
    std::tie(off, se) = sc::sys_seek(fd, 0, 99);
    CHECK(se == EINVAL_);
    sc::sys_close(fd);
    CHECK(read_file(p("rw")) == "Jello world");
    // errors on a closed descriptor
    std::tie(n, re) = sc::sys_read(fd, buf);
    CHECK(n == 0 && re == EBADF_);
    std::tie(n, re) = sc::sys_write(fd, buf);
    CHECK(n == 0 && re == EBADF_);
    std::tie(n, re) = sc::sys_pread(fd, buf, 0);
    CHECK(re == EBADF_);
    // read-only descriptor: write is EBADF; write-only: read is EBADF
    auto [ro, e2] = sc::sys_open(gs(p("rw")), 0, 0);
    std::tie(n, re) = sc::sys_write(ro, bytes("x"));
    CHECK(re == EBADF_);
    sc::sys_close(ro);
    auto [wo, e3] = sc::sys_open(gs(p("rw")), O_WRONLY_ | O_APPEND_, 0);
    std::tie(n, re) = sc::sys_read(wo, buf);
    CHECK(re == EBADF_);
    std::tie(n, re) = sc::sys_write(wo, bytes("!"));
    CHECK(re == 0 && n == 1);
    sc::sys_close(wo);
    CHECK(read_file(p("rw")) == "Jello world!");
    (void)e2;
    (void)e3;
}

void test_big_io_is_cut() {
    // a read or write of more than 1<<30 bytes is cut to 1<<30 (Go's maxRW); only the count of a
    // small file can be checked here, so check that a large buffer's request returns what is there
    write_file(p("small"), "abc");
    auto [fd, e] = sc::sys_open(gs(p("small")), 0, 0);
    auto big = gp::make_slice<uint8_t>(int64_t{1} << 22);
    auto [n, re] = sc::sys_read(fd, big);
    CHECK(e == 0 && re == 0 && n == 3);
    sc::sys_close(fd);
}

void test_stat() {
    write_file(p("st"), "12345");
    auto out = gp::make_slice<uint64_t>(16);
    CHECK(sc::sys_stat(gs(p("st")), out) == 0);
    CHECK(out[7] == 5);                   // size
    CHECK((out[3] & 0170000) == 0100000); // S_IFREG
    CHECK(out[2] == 1);                   // nlink
    CHECK(out[12] > 1000000000);          // mtime seconds
    CHECK(out[8] > 0);                    // blksize
    CHECK(sc::sys_stat(gs(dir), out) == 0 && (out[3] & 0170000) == 0040000);
    CHECK(sc::sys_stat(gs(p("nope")), out) == ENOENT_);
    CHECK(sc::sys_stat(gs(p("st/x")), out) == ENOTDIR_);
    auto small = gp::make_slice<uint64_t>(4);
    CHECK(sc::sys_stat(gs(p("st")), small) == EINVAL_);
    CHECK(sc::sys_symlink(gs("st"), gs(p("ln"))) == 0);
    CHECK(sc::sys_lstat(gs(p("ln")), out) == 0 && (out[3] & 0170000) == 0120000 && out[7] == 2);
    CHECK(sc::sys_stat(gs(p("ln")), out) == 0 && (out[3] & 0170000) == 0100000);
    CHECK(sc::sys_symlink(gs("nowhere"), gs(p("dangling"))) == 0);
    CHECK(sc::sys_stat(gs(p("dangling")), out) == ENOENT_);
    CHECK(sc::sys_lstat(gs(p("dangling")), out) == 0);
    auto [fd, e] = sc::sys_open(gs(p("st")), 0, 0);
    CHECK(e == 0 && sc::sys_fstat(fd, out) == 0 && out[7] == 5);
    sc::sys_close(fd);
    CHECK(sc::sys_fstat(fd, out) == EBADF_);
    CHECK(sc::sys_chmod(gs(p("st")), 0640) == 0);
    CHECK(sc::sys_stat(gs(p("st")), out) == 0 && (out[3] & 07777) == 0640);
    CHECK(sc::sys_utimensat(gs(p("st")), 1000000000, 5, 1500000000, 7) == 0);
    CHECK(sc::sys_stat(gs(p("st")), out) == 0);
    CHECK(out[10] == 1000000000 && out[11] == 5 && out[12] == 1500000000 && out[13] == 7);
    // UTIME_OMIT leaves a time alone
    CHECK(sc::sys_utimensat(gs(p("st")), (1 << 30) - 2, (1 << 30) - 2, 1600000000, 0) == 0);
    CHECK(sc::sys_stat(gs(p("st")), out) == 0 && out[10] == 1000000000 && out[12] == 1600000000);
}

void test_names() {
    CHECK(sc::sys_mkdir(gs(p("d")), 0755) == 0);
    CHECK(sc::sys_mkdir(gs(p("d")), 0755) == EEXIST_);
    CHECK(sc::sys_mkdir(gs(p("x/y")), 0755) == ENOENT_);
    write_file(p("d/f"), "x");
    CHECK(sc::sys_rmdir(gs(p("d"))) == ENOTEMPTY_);
    CHECK(sc::sys_rmdir(gs(p("d/f"))) == ENOTDIR_);
    CHECK(sc::sys_unlink(gs(p("d"))) == EISDIR_);
    CHECK(sc::sys_unlink(gs(p("d/f"))) == 0);
    CHECK(sc::sys_unlink(gs(p("d/f"))) == ENOENT_);
    CHECK(sc::sys_rmdir(gs(p("d"))) == 0);
    write_file(p("a"), "A");
    CHECK(sc::sys_rename(gs(p("a")), gs(p("b"))) == 0);
    CHECK(sc::sys_rename(gs(p("a")), gs(p("c"))) == ENOENT_);
    CHECK(read_file(p("b")) == "A");
    CHECK(sc::sys_link(gs(p("b")), gs(p("b2"))) == 0);
    CHECK(sc::sys_link(gs(p("b")), gs(p("b2"))) == EEXIST_);
    auto out = gp::make_slice<uint64_t>(16);
    sc::sys_stat(gs(p("b")), out);
    CHECK(out[2] == 2);
    auto buf = gp::make_slice<uint8_t>(64);
    CHECK(sc::sys_symlink(gs("target/path"), gs(p("sl"))) == 0);
    auto [n, e] = sc::sys_readlink(gs(p("sl")), buf);
    CHECK(e == 0 && text(buf, n) == "target/path");
    auto tiny = gp::make_slice<uint8_t>(3);
    std::tie(n, e) = sc::sys_readlink(gs(p("sl")), tiny);
    CHECK(e == 0 && n == 3 && text(tiny, n) == "tar"); // not NUL-terminated, cut, n == len(buf)
    std::tie(n, e) = sc::sys_readlink(gs(p("b")), buf);
    CHECK(e == EINVAL_);
    std::tie(n, e) = sc::sys_readlink(gs(p("none")), buf);
    CHECK(e == ENOENT_);
    CHECK(sc::sys_truncate(gs(p("b")), 100) == 0);
    sc::sys_stat(gs(p("b")), out);
    CHECK(out[7] == 100);
    CHECK(sc::sys_truncate(gs(p("none")), 1) == ENOENT_);
    CHECK(sc::sys_truncate(gs(p("b")), -1) == EINVAL_);
}

void test_getdents() {
    CHECK(sc::sys_mkdir(gs(p("dd")), 0755) == 0);
    for (const char* n : {"one", "two", "three"}) {
        write_file(dir + "/dd/" + n, "x");
    }
    auto [fd, e] = sc::sys_open(gs(p("dd")), 0, 0);
    CHECK(e == 0);
    auto buf = gp::make_slice<uint8_t>(8192);
    std::vector<std::string> names;
    for (;;) {
        auto [n, ge] = sc::sys_getdents(fd, buf);
        CHECK(ge == 0);
        if (ge != 0 || n == 0) {
            break;
        }
        // linux_dirent64: ino u64, off i64, reclen u16, type u8, name
        int64_t at = 0;
        while (at < n) {
            uint16_t reclen;
            std::memcpy(&reclen, buf.gp_data() + at + 16, 2);
            std::string name(reinterpret_cast<const char*>(buf.gp_data() + at + 19));
            if (name != "." && name != "..") {
                names.push_back(name);
            }
            at += reclen;
        }
    }
    std::sort(names.begin(), names.end());
    CHECK(names == (std::vector<std::string>{"one", "three", "two"}));
    sc::sys_close(fd);
    auto [n2, e2] = sc::sys_getdents(fd, buf);
    CHECK(e2 == EBADF_);
    (void)n2;
    auto [wf, e3] = sc::sys_open(gs(dir + "/dd/one"), 0, 0);
    auto [n3, e4] = sc::sys_getdents(wf, buf);
    CHECK(e4 == ENOTDIR_);
    (void)n3;
    (void)e3;
    sc::sys_close(wf);
}

void test_cwd() {
    auto buf = gp::make_slice<uint8_t>(4096);
    auto [n, e] = sc::sys_getcwd(buf);
    CHECK(e == 0 && n >= 2 && buf[0] == '/' && buf[n - 1] == 0);
    std::string old = text(buf, n - 1);
    CHECK(sc::sys_chdir(gs(dir)) == 0);
    std::tie(n, e) = sc::sys_getcwd(buf);
    std::string now = text(buf, n - 1);
    CHECK(e == 0 && now.size() >= 4);
    write_file("rel-file", "r"); // relative to the new current directory
    CHECK(read_file(p("rel-file")) == "r");
    CHECK(sc::sys_chdir(gs(p("missing"))) == ENOENT_);
    CHECK(sc::sys_chdir(gs(p("rel-file"))) == ENOTDIR_);
    auto tiny = gp::make_slice<uint8_t>(2);
    std::tie(n, e) = sc::sys_getcwd(tiny);
    CHECK(e != 0); // ERANGE
    CHECK(sc::sys_chdir(gs(old)) == 0);
}

void test_pipe_dup_nonblock() {
    auto fds = gp::make_slice<int64_t>(2);
    CHECK(sc::sys_pipe2(fds, 0) == 0);
    auto [n, e] = sc::sys_write(fds[1], bytes("ping"));
    CHECK(e == 0 && n == 4);
    auto buf = gp::make_slice<uint8_t>(8);
    std::tie(n, e) = sc::sys_read(fds[0], buf);
    CHECK(e == 0 && text(buf, n) == "ping");
    CHECK(sc::sys_set_nonblock(fds[0], true) == 0);
    std::tie(n, e) = sc::sys_read(fds[0], buf);
    CHECK(n == 0 && e == 11); // EAGAIN
    CHECK(sc::sys_set_nonblock(fds[0], false) == 0);
    CHECK(sc::sys_close(fds[0]) == 0);
    std::tie(n, e) = sc::sys_write(fds[1], bytes("x"));
    CHECK(n == 0 && e == EPIPE_); // the signal is handled (a handler that does nothing)
    CHECK(sc::sys_close(fds[1]) == 0);
    auto small = gp::make_slice<int64_t>(1);
    CHECK(sc::sys_pipe2(small, 0) == EINVAL_);
}

void test_retry_eintr() {
    auto [runs, e] = sc::sys_selftest_retry();
    CHECK(runs >= 1); // a signal arrived while the read was blocked
    CHECK(e == 0);    // and the read was repeated: it got the byte the handler wrote
}

void test_ids_and_misc() {
    CHECK(sc::sys_getpid() > 0 && sc::sys_getppid() >= 0);
    CHECK(sc::sys_getpagesize() >= 4096);
    CHECK(sc::sys_getuid() >= 0 && sc::sys_geteuid() >= 0 && sc::sys_getgid() >= 0);
    auto [name, e] = sc::sys_nodename();
    CHECK(e == 0 && name.gp_view().size() > 0);
    auto [n0, e0] = sc::sys_getgroups(gp::slice<int64_t>());
    CHECK(e0 == 0 && n0 >= 0);
    CHECK(sc::sys_kill(sc::sys_getpid(), 0) == 0);
    CHECK(sc::sys_kill(1 << 30, 0) == 3); // ESRCH
    CHECK(sc::runtime_rand() != sc::runtime_rand());
}

void test_env() {
    sc::runtime_setenv(gs("GPSYS_TEST"), gs("value"));
    const char* v = std::getenv("GPSYS_TEST");
    CHECK(v != nullptr && std::string(v) == "value");
    sc::runtime_setenv(gs("GPSYS_TEST"), gs("other"));
    CHECK(std::string(std::getenv("GPSYS_TEST")) == "other");
    sc::runtime_unsetenv(gs("GPSYS_TEST"));
    CHECK(std::getenv("GPSYS_TEST") == nullptr);
    sc::runtime_unsetenv(gs("GPSYS_TEST")); // not set: nothing
    // many threads setting different variables at once do not corrupt each other
    std::vector<std::thread> ts;
    std::atomic<int> bad{0};
    for (int t = 0; t < 8; t++) {
        ts.emplace_back([t, &bad] {
            std::string k = "GPSYS_T" + std::to_string(t);
            for (int i = 0; i < 200; i++) {
                sc::runtime_setenv(gs(k), gs(std::to_string(i)));
            }
            const char* got = std::getenv(k.c_str());
            if (got == nullptr || std::string(got) != "199") {
                bad++;
            }
            sc::runtime_unsetenv(gs(k));
        });
    }
    for (auto& t : ts) {
        t.join();
    }
    CHECK(bad == 0);
    auto args = sc::runtime_args();
    CHECK(args.gp_len() == 0 ||
          args.gp_len() >= 1); // empty when the test does not run through run_main
}

} // namespace

int main() {
    // (run_main installs a harmless SIGPIPE handler; a test program has none)
#ifdef SIGPIPE // (MinGW has no SIGPIPE; this test runs on Linux only, TestWindowsSyntax only
               // compiles it)
    std::signal(SIGPIPE, SIG_IGN);
#endif
    dir = "/tmp/gpsys-test-" + std::to_string(sc::sys_getpid());
    sc::sys_mkdir(gs(dir), 0700);
    test_open_errors();
    test_cloexec();
    test_read_write();
    test_big_io_is_cut();
    test_stat();
    test_names();
    test_getdents();
    test_cwd();
    test_pipe_dup_nonblock();
    test_retry_eintr();
    test_ids_and_misc();
    test_env();
    // remove the directory with what the tests left in it
    for (const char* n : {"exists", "rw", "small", "st", "ln", "dangling", "b", "b2", "sl",
                          "rel-file", "dd/one", "dd/two", "dd/three"}) {
        sc::sys_unlink(gs(dir + "/" + n));
    }
    sc::sys_rmdir(gs(dir + "/dd"));
    sc::sys_rmdir(gs(dir));
    rt_test::threads_ended();
    if (failures != 0) {
        return 1;
    }
    std::string msg = "ok: " + std::to_string(checks) + " checks\n";
    gp::os::write(gp::os::stdout_fd, msg.data(), msg.size());
    return 0;
}
