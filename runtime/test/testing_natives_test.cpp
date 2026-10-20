// The unit tests of the operating system's natives of package testing (gocpp/testing.hpp: the
// environment, temporary folders, the standard output capture, the processor's name, and the
// folder and file functions of the test binary), Phase 14 step 14.9. The code is in
// src/os/linux/testing_os.cpp and src/os/windows/testing_os.cpp and src/testing.cpp; the program
// is built with the natives of package testing (rtfiles.TestingSources) and runs on Linux
// (runtime/testing_natives_test.go) and on Windows (TestWindowsRuntime: a real PC over ssh). It
// prints "ok: N checks" and exits 0, or prints each failed check and exits 1.
//
// The cases that differ by OS are guarded by _WIN32: the names of environment variables are case
// insensitive on Windows (SetEnvironmentVariable), the folder of temporary files comes from TMP
// there and from TMPDIR on Linux, the processor's name is always known on Windows (the registry).
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>
#include <gocpp/testing.hpp>

#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <filesystem>
#include <fstream>
#include <set>
#include <string>
#include <string_view>
#include <thread>
#include <tuple>
#include <vector>

namespace {

namespace fs = std::filesystem;

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

#define CHECK_STR(got, want)                                                                       \
    do {                                                                                           \
        std::string g_ = (got), w_ = (want);                                                       \
        report(g_ == w_, __FILE__, __LINE__,                                                       \
               "CHECK_STR(" #got ", " #want ") failed: got \"" + g_ + "\", want \"" + w_ + "\"");  \
    } while (0)

gp::string gs(std::string_view s) {
    return gp::string::gp_copy(s);
}

std::string str(const gp::string& s) {
    return std::string(s.gp_view());
}

// Paths: the UTF-8 of a Go string and std::filesystem::path without the narrow code page.
fs::path to_path(std::string_view s) {
    return fs::path(std::u8string(reinterpret_cast<const char8_t*>(s.data()), s.size()));
}

std::string utf8_of(const fs::path& p) {
    std::u8string u = p.u8string();
    return std::string(reinterpret_cast<const char*>(u.data()), u.size());
}

// The name of the variable that holds the folder of temporary files.
#ifdef _WIN32
constexpr const char* tmp_var = "TMP";
#else
constexpr const char* tmp_var = "TMPDIR";
#endif

namespace T = gp::testing;

// --- environment ---

std::string lookup(const char* key, bool& found) {
    auto [v, ok] = T::lookup_env(gs(key));
    found = ok;
    return str(v);
}

void test_env() {
    bool found = true;

    // A variable that nobody set.
    lookup("GOCPP_TN_UNSET", found);
    CHECK(!found);
    CHECK_STR(str(T::unset_env(gs("GOCPP_TN_UNSET"))), ""); // not an error

    // Set, look up, replace, remove.
    CHECK_STR(str(T::set_env(gs("GOCPP_TN_A"), gs("v1"))), "");
    CHECK_STR(lookup("GOCPP_TN_A", found), "v1");
    CHECK(found);
    CHECK(std::getenv("GOCPP_TN_A") != nullptr && std::string(std::getenv("GOCPP_TN_A")) == "v1");
    CHECK_STR(str(T::set_env(gs("GOCPP_TN_A"), gs("v2"))), "");
    CHECK_STR(lookup("GOCPP_TN_A", found), "v2");
    CHECK_STR(std::getenv("GOCPP_TN_A") != nullptr ? std::getenv("GOCPP_TN_A") : "(null)", "v2");
    CHECK_STR(str(T::unset_env(gs("GOCPP_TN_A"))), "");
    lookup("GOCPP_TN_A", found);
    CHECK(!found);
    CHECK(std::getenv("GOCPP_TN_A") == nullptr);

    // A variable that is set to "" is set (os.LookupEnv says true).
    CHECK_STR(str(T::set_env(gs("GOCPP_TN_EMPTY"), gs(""))), "");
    CHECK_STR(lookup("GOCPP_TN_EMPTY", found), "");
    CHECK(found);
    CHECK_STR(str(T::unset_env(gs("GOCPP_TN_EMPTY"))), "");
    lookup("GOCPP_TN_EMPTY", found);
    CHECK(!found);

    // UTF-8, and a value longer than the first buffer of a lookup.
    const std::string utf8 = "h\xc3\xa9llo \xe6\x97\xa5\xe6\x9c\xac \xf0\x9f\x8e\x89";
    CHECK_STR(str(T::set_env(gs("GOCPP_TN_UTF8"), gs(utf8))), "");
    CHECK_STR(lookup("GOCPP_TN_UTF8", found), utf8);
    const std::string big(5000, 'x');
    CHECK_STR(str(T::set_env(gs("GOCPP_TN_BIG"), gs(big))), "");
    CHECK_STR(lookup("GOCPP_TN_BIG", found), big);
    CHECK(found);
    CHECK_STR(str(T::set_env(gs("GOCPP_TN_V=="), gs("a=b=c"))).substr(0, 6),
              "setenv"); // a key with =
    CHECK_STR(str(T::set_env(gs("GOCPP_TN_EQ"), gs("a=b=c"))), "");
    CHECK_STR(lookup("GOCPP_TN_EQ", found), "a=b=c"); // a value may have =
    T::unset_env(gs("GOCPP_TN_UTF8"));
    T::unset_env(gs("GOCPP_TN_BIG"));
    T::unset_env(gs("GOCPP_TN_EQ"));

    // Arguments that no system takes: the error text, and nothing changes.
    CHECK_STR(str(T::set_env(gs(""), gs("x"))), "setenv: invalid argument");
    CHECK_STR(str(T::set_env(gs("A=B"), gs("x"))), "setenv: invalid argument");
    CHECK_STR(str(T::set_env(gs(std::string("A\0B", 3)), gs("x"))), "setenv: invalid argument");
    CHECK_STR(str(T::set_env(gs("GOCPP_TN_NUL"), gs(std::string("a\0b", 3)))),
              "setenv: invalid argument");
    lookup("GOCPP_TN_NUL", found);
    CHECK(!found);
    CHECK_STR(str(T::unset_env(gs(""))), "unsetenv: invalid argument");
    CHECK_STR(str(T::unset_env(gs("A=B"))), "unsetenv: invalid argument");

#ifdef _WIN32
    // The names of variables are case insensitive, and the case of the first spelling stays.
    CHECK_STR(str(T::set_env(gs("GoCpp_Tn_Case"), gs("one"))), "");
    CHECK_STR(lookup("GOCPP_TN_CASE", found), "one");
    CHECK(found);
    CHECK_STR(lookup("gocpp_tn_case", found), "one");
    CHECK_STR(str(T::set_env(gs("gocpp_tn_case"), gs("two"))), "");
    CHECK_STR(lookup("GoCpp_Tn_Case", found), "two");
    CHECK_STR(str(T::unset_env(gs("GOCPP_TN_CASE"))), "");
    lookup("gocpp_tn_case", found);
    CHECK(!found);
    // The C library's copy follows (getenv of C and C++ code reads it).
    CHECK_STR(str(T::set_env(gs("GOCPP_TN_CRT"), gs("crt"))), "");
    CHECK(std::getenv("gocpp_tn_crt") != nullptr);
    T::unset_env(gs("GOCPP_TN_CRT"));
#else
    // Linux: the names are case sensitive.
    CHECK_STR(str(T::set_env(gs("GoCpp_Tn_Case"), gs("one"))), "");
    lookup("GOCPP_TN_CASE", found);
    CHECK(!found);
    CHECK_STR(lookup("GoCpp_Tn_Case", found), "one");
    T::unset_env(gs("GoCpp_Tn_Case"));
#endif
}

// --- temporary folders, remove_all, make_dir ---

void test_temp_dirs() {
    // The new folders: they exist, are below the folder of temporary files, have the prefix and a
    // number, and no two have the same name.
    std::set<std::string> seen;
    std::vector<std::string> made;
    for (int i = 0; i < 100; i++) {
        auto [dir, err] = T::make_temp_dir(gs("gocpp-tn-"));
        CHECK_STR(str(err), "");
        std::string d = str(dir);
        made.push_back(d);
        CHECK(seen.insert(d).second);
        std::error_code ec;
        CHECK(fs::is_directory(to_path(d), ec));
        std::string name = utf8_of(to_path(d).filename());
        CHECK(name.rfind("gocpp-tn-", 0) == 0 && name.size() > 9);
        CHECK(d.find_last_of("/\\") != d.size() - 1); // no separator at the end
#ifdef _WIN32
        CHECK(d.find('\\') != std::string::npos && d.find('/') == std::string::npos);
#endif
    }
    CHECK(seen.size() == 100);

    // The folder comes from TMPDIR (TMP on Windows), and a name outside ASCII is fine.
    auto [dir, err] = T::make_temp_dir(gs("gocpp-tn-base-"));
    CHECK_STR(str(err), "");
    std::string base = str(dir);
    std::string savedTmp;
    bool hadTmp = false;
    {
        auto [v, ok] = T::lookup_env(gs(tmp_var));
        savedTmp = str(v);
        hadTmp = ok;
    }
    CHECK_STR(str(T::set_env(gs(tmp_var), gs(base))), "");
    auto [dir2, err2] = T::make_temp_dir(gs("gocpp-tn-\xc3\xa9\xe6\x97\xa5-"));
    CHECK_STR(str(err2), "");
    std::string d2 = str(dir2);
    CHECK(fs::equivalent(to_path(d2).parent_path(), to_path(base)));
    CHECK(utf8_of(to_path(d2).filename()).rfind("gocpp-tn-\xc3\xa9\xe6\x97\xa5-", 0) == 0);
    CHECK(fs::is_directory(to_path(d2)));
    // A folder of temporary files that is not there: the error text names it.
    std::string missing = base + "/no-such-folder";
    CHECK_STR(str(T::set_env(gs(tmp_var), gs(missing))), "");
    auto [dir3, err3] = T::make_temp_dir(gs("p-"));
    CHECK(str(dir3).empty());
    CHECK(!str(err3).empty());
    if (hadTmp) {
        T::set_env(gs(tmp_var), gs(savedTmp));
    } else {
        T::unset_env(gs(tmp_var));
    }

    // make_dir: one level, mode 0777; an existing path is an error that names it.
    std::string sub = d2 + "/sub \xc3\xa9";
    CHECK_STR(str(T::make_dir(gs(sub))), "");
    CHECK(fs::is_directory(to_path(sub)));
    std::string again = str(T::make_dir(gs(sub)));
    CHECK(again.rfind("mkdir ", 0) == 0 && again.find("exists") != std::string::npos);
#ifdef _WIN32
    // The system's own text, as Go's errors have it (not the C library's).
    // (exactly: the text of FormatMessage has a CR LF at its end, which is cut)
    CHECK_STR(again, "mkdir " + sub + ": Cannot create a file when that file already exists.");
#endif
    std::string deep = d2 + "/no/such/parent";
    CHECK(!str(T::make_dir(gs(deep))).empty());

    // remove_all: a tree with files, also a read-only one and a name outside ASCII; a path that is
    // not there is not an error.
    std::string tree = d2 + "/tree";
    CHECK_STR(str(T::make_dir(gs(tree))), "");
    CHECK_STR(str(T::make_dir(gs(tree + "/inner"))), "");
    for (const std::string& f :
         {tree + "/a.txt", tree + "/inner/\xe6\x97\xa5.txt", tree + "/inner/ro.txt"}) {
        std::ofstream(to_path(f), std::ios::binary) << "data";
    }
    std::error_code ec;
    fs::permissions(to_path(tree + "/inner/ro.txt"),
                    fs::perms::owner_read | fs::perms::group_read | fs::perms::others_read,
                    fs::perm_options::replace, ec);
    CHECK_STR(str(T::remove_all(gs(tree))), "");
    CHECK(!fs::exists(to_path(tree), ec));
    CHECK_STR(str(T::remove_all(gs(tree))), "");
    // A trailing separator (either kind on Windows) is no problem.
    for (const char* tail : {"/", "//",
#ifdef _WIN32
                             "\\", "\\\\", "/\\"
#endif
         }) {
        CHECK_STR(str(T::make_dir(gs(tree))), "");
        CHECK_STR(str(T::make_dir(gs(tree + "/inner"))), "");
        std::ofstream(to_path(tree + "/inner/f.txt"), std::ios::binary) << "x";
        CHECK_STR(str(T::remove_all(gs(tree + tail))), "");
        CHECK(!fs::exists(to_path(tree), ec));
    }
    CHECK_STR(str(T::remove_all(gs(base + "/never-made"))), "");
    CHECK_STR(str(T::remove_all(gs(d2))), "");
    CHECK(!fs::exists(to_path(d2), ec));

    for (const std::string& d : made) {
        CHECK_STR(str(T::remove_all(gs(d))), "");
        CHECK(!fs::exists(to_path(d), ec));
    }
    CHECK_STR(str(T::remove_all(gs(base))), "");
    CHECK(!fs::exists(to_path(base), ec));
}

#ifdef _WIN32
// A folder of temporary files whose path is over the 248 characters that CreateDirectory takes
// (the program has no manifest that lifts the limit): make_temp_dir and remove_all add the
// long-path prefix themselves. The path of TMP stays below the 261 that GetTempPath can give.
void test_long_paths() {
    auto [dir, err] = T::make_temp_dir(gs("gocpp-tn-long-"));
    CHECK_STR(str(err), "");
    std::string deep = str(dir);
    std::error_code ec;
    // One level at a time, each with the prefix (this test's own calls): 236 characters in all.
    auto level = [&](size_t n) {
        deep += "\\" + std::string(n, 'q');
        fs::create_directory(to_path("\\\\?\\" + deep), ec);
        CHECK(!ec);
    };
    while (deep.size() + 41 <= 236) {
        level(40);
    }
    if (deep.size() < 234) {
        level(236 - deep.size() - 1);
    }
    CHECK(deep.size() >= 234 && deep.size() <= 236);
    std::string saved;
    bool had = false;
    {
        auto [v, ok] = T::lookup_env(gs(tmp_var));
        saved = str(v);
        had = ok;
    }
    CHECK_STR(str(T::set_env(gs(tmp_var), gs(deep))), "");
    auto [made, merr] = T::make_temp_dir(gs("gocpp-tn-long-sub-"));
    CHECK_STR(str(merr), "");
    std::string m = str(made);
    CHECK(m.size() > 260);
    CHECK(fs::is_directory(to_path("\\\\?\\" + m), ec));
    // make_dir of a path over the limit of CreateDirectory
    CHECK_STR(str(T::make_dir(gs(m + "\\sub"))), "");
    CHECK(fs::is_directory(to_path("\\\\?\\" + m + "\\sub"), ec));
    // and read_file
    std::ofstream(to_path("\\\\?\\" + m + "\\sub\\g.txt"), std::ios::binary) << "long";
    auto [lc, lerr] = T::read_file(gs(m + "\\sub\\g.txt"));
    CHECK_STR(str(lerr), "");
    CHECK_STR(str(lc), "long");
    // remove_all of a path that long, with a file in it
    std::ofstream(to_path("\\\\?\\" + m + "\\f.txt"), std::ios::binary) << "x";
    CHECK_STR(str(T::remove_all(gs(m))), "");
    CHECK(!fs::exists(to_path("\\\\?\\" + m), ec));
    if (had) {
        T::set_env(gs(tmp_var), gs(saved));
    } else {
        T::unset_env(gs(tmp_var));
    }
    CHECK_STR(str(T::remove_all(gs(str(dir)))), "");
    CHECK(!fs::exists(to_path("\\\\?\\" + str(dir)), ec));
}
#endif

// remove_all removes a link itself and does not follow it (the folder it points to keeps its
// files). On Windows the link is a junction (mklink /J needs no privilege); the names are ASCII,
// because system() reads its command line in the ANSI code page.
void test_remove_links() {
    auto [dir, err] = T::make_temp_dir(gs("gocpp-tn-link-"));
    CHECK_STR(str(err), "");
    std::string base = str(dir);
#ifdef _WIN32
    const char sep = '\\';
#else
    const char sep = '/';
#endif
    std::string tree = base + sep + "tree", outside = base + sep + "outside";
    CHECK_STR(str(T::make_dir(gs(tree))), "");
    CHECK_STR(str(T::make_dir(gs(outside))), "");
    std::ofstream(to_path(outside + sep + "keep.txt"), std::ios::binary) << "keep";
    std::error_code ec;
#ifdef _WIN32
    int rc = std::system(("mklink /J \"" + tree + "\\link\" \"" + outside + "\" >nul").c_str());
    bool linked = rc == 0;
#else
    fs::create_directory_symlink(to_path(outside), to_path(tree + "/link"), ec);
    bool linked = !ec;
#endif
    CHECK(linked);
    CHECK(fs::exists(to_path(tree + sep + "link" + sep + "keep.txt"), ec) == linked);
    CHECK_STR(str(T::remove_all(gs(tree))), "");
    CHECK(!fs::exists(to_path(tree), ec));
    CHECK(fs::is_regular_file(to_path(outside + sep + "keep.txt"), ec));
    // The link itself, written with a trailing separator, is removed and not followed.
    std::string jl = base + sep + "jl";
#ifdef _WIN32
    linked = std::system(("mklink /J \"" + jl + "\" \"" + outside + "\" >nul").c_str()) == 0;
#else
    fs::create_directory_symlink(to_path(outside), to_path(jl), ec);
    linked = !ec;
#endif
    CHECK(linked);
    CHECK_STR(str(T::remove_all(gs(jl + sep))), "");
    CHECK(!fs::exists(fs::symlink_status(to_path(jl), ec)));
    CHECK(fs::is_regular_file(to_path(outside + sep + "keep.txt"), ec));
    CHECK_STR(str(T::remove_all(gs(base))), "");
}

#ifdef _WIN32
// A file that is open cannot be deleted on Windows (the library's streams do not share delete
// access): remove_all tries again for two seconds, and then reports the error. A file that is
// closed in the meantime lets it succeed.
void test_remove_busy() {
    auto [dir, err] = T::make_temp_dir(gs("gocpp-tn-busy-"));
    CHECK_STR(str(err), "");
    std::string base = str(dir);
    std::ofstream busy(to_path(base + "\\busy.txt"), std::ios::binary);
    busy << "x";
    busy.flush();
    std::string e = str(T::remove_all(gs(base)));
    CHECK(e.rfind("RemoveAll ", 0) == 0);
    CHECK(e.find(base) != std::string::npos);      // the path as it was given
    CHECK(e.find("\\\\?\\") == std::string::npos); // not the internal prefix
    std::thread closer([&] {
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
        busy.close();
    });
    CHECK_STR(str(T::remove_all(gs(base))), "");
    closer.join();
    std::error_code ec;
    CHECK(!fs::exists(to_path(base), ec));
}
#endif

// --- the working folder, files ---

void test_files() {
    auto [dir, err] = T::make_temp_dir(gs("gocpp-tn-files-"));
    CHECK_STR(str(err), "");
    std::string base = str(dir);
    std::string uni = base + "/d\xc3\xa9\xe6\x97\xa5";
    CHECK_STR(str(T::make_dir(gs(uni))), "");

    // read_file: the bytes as they are (also NUL and CRLF); a missing file is an error.
    const std::string bytes("a\r\nb\0c\n\xff", 8);
    {
        std::ofstream f(to_path(uni + "/data.bin"), std::ios::binary);
        f.write(bytes.data(), static_cast<std::streamsize>(bytes.size()));
    }
    {
        std::ofstream(to_path(uni + "/b.txt"), std::ios::binary) << "b";
        std::ofstream(to_path(uni + "/a.txt"), std::ios::binary) << "a";
    }
    CHECK_STR(str(T::make_dir(gs(uni + "/subfolder"))), "");
    auto [content, rerr] = T::read_file(gs(uni + "/data.bin"));
    CHECK_STR(str(rerr), "");
    CHECK(str(content) == bytes);
    auto [none, rerr2] = T::read_file(gs(uni + "/missing.txt"));
    CHECK(str(none).empty());
    CHECK(str(rerr2).rfind("open ", 0) == 0);
#ifndef _WIN32
    auto [dnone, derr] = T::read_file(gs(uni + "/subfolder"));
    CHECK(str(dnone).empty());
    CHECK(str(derr).rfind("read " + uni + "/subfolder: ", 0) == 0);
#endif
#ifdef _WIN32
    CHECK_STR(str(rerr2),
              "open " + uni + "/missing.txt: The system cannot find the file specified.");
    // A folder is not a file: Go says "read X: Incorrect function." on Windows.
    auto [dnone, derr] = T::read_file(gs(uni + "/subfolder"));
    CHECK(str(dnone).empty());
    CHECK_STR(str(derr), "read " + uni + "/subfolder: Incorrect function.");
#endif

    // read_files: the files, not the folders, sorted; a folder that is not there is empty.
    auto [names, lerr] = T::read_files(gs(uni));
    CHECK_STR(str(lerr), "");
    CHECK_STR(str(names), "a.txt\nb.txt\ndata.bin\n");
    auto [nonames, lerr2] = T::read_files(gs(base + "/no-such"));
    CHECK_STR(str(nonames), "");
    CHECK_STR(str(lerr2), "");

    // getwd and chdir.
    auto [wd0, werr0] = T::getwd();
    CHECK_STR(str(werr0), "");
    CHECK(!str(wd0).empty());
    CHECK_STR(str(T::chdir(gs(uni))), "");
    auto [wd1, werr1] = T::getwd();
    CHECK_STR(str(werr1), "");
    std::error_code ec;
    CHECK(fs::equivalent(to_path(str(wd1)), to_path(uni), ec));
    CHECK(utf8_of(to_path(str(wd1)).filename()) == "d\xc3\xa9\xe6\x97\xa5");
    CHECK_STR(str(T::chdir(gs("subfolder"))), ""); // relative to the new folder
    auto [wd2, werr2] = T::getwd();
    CHECK(utf8_of(to_path(str(wd2)).filename()) == "subfolder");
    CHECK_STR(str(T::chdir(gs(str(wd0)))), "");
    std::string bad = str(T::chdir(gs(base + "/no-such")));
    CHECK(bad.rfind("chdir ", 0) == 0);
#ifdef _WIN32
    CHECK_STR(bad, "chdir " + base + "/no-such: The system cannot find the file specified.");
#endif
    auto [wd3, werr3] = T::getwd();
    CHECK(fs::equivalent(to_path(str(wd3)), to_path(str(wd0)), ec));

    CHECK_STR(str(T::remove_all(gs(base))), "");
}

// --- the standard output capture ---

void test_capture() {
    // C's stdio and the runtime's own writes both end up in the capture (C's buffer is flushed
    // into it when it stops, so the order of the two lines is not fixed); what was written before
    // and after goes to the real output.
    CHECK_STR(str(T::capture_start()), "");
    std::printf("from printf\n"); // (stays in C's buffer until the capture is stopped)
    const std::string line = "from write\n";
    gp::os::write(gp::os::stdout_fd, line.data(), line.size());
    std::string got = str(T::capture_stop());
    CHECK(got == "from printf\nfrom write\n" || got == "from write\nfrom printf\n");

    // Again, with a big output and with nothing; stop without start gives "".
    CHECK_STR(str(T::capture_start()), "");
    std::string big;
    for (int i = 0; i < 20000; i++) {
        big += "line " + std::to_string(i) + "\n";
    }
    gp::os::write(gp::os::stdout_fd, big.data(), big.size());
    CHECK(str(T::capture_stop()) == big);
    CHECK_STR(str(T::capture_start()), "");
    CHECK_STR(str(T::capture_stop()), "");
    CHECK_STR(str(T::capture_stop()), "");

    // Bytes as they are: a text-mode file would turn CRLF into LF and end at Ctrl-Z (0x1a) or
    // change what a NUL does.
    const std::string raw("a\r\nb\x1a"
                          "c\0d\r\r\n\xff\n",
                          13);
    CHECK_STR(str(T::capture_start()), "");
    gp::os::write(gp::os::stdout_fd, raw.data(), raw.size());
    std::string rawGot = str(T::capture_stop());
    CHECK(rawGot == raw);

    // Captures do not nest: the second start says so, changes nothing, and the first capture goes
    // on and ends with its own text.
    CHECK_STR(str(T::capture_start()), "");
    const std::string first = "first\n";
    gp::os::write(gp::os::stdout_fd, first.data(), first.size());
    std::string nested = str(T::capture_start());
    CHECK(nested.find("captured already") != std::string::npos);
    const std::string second = "second\n";
    gp::os::write(gp::os::stdout_fd, second.data(), second.size());
    CHECK_STR(str(T::capture_stop()), "first\nsecond\n");

    // The captures leave no file behind: they run in a folder of their own (TMP or TMPDIR).
    auto [dir, err] = T::make_temp_dir(gs("gocpp-tn-cap-"));
    CHECK_STR(str(err), "");
    std::string base = str(dir);
    auto [v, had] = T::lookup_env(gs(tmp_var));
    std::string saved = str(v);
    CHECK_STR(str(T::set_env(gs(tmp_var), gs(base))), "");
    for (int i = 0; i < 300; i++) {
        CHECK_STR(str(T::capture_start()), "");
        CHECK_STR(str(T::capture_stop()), "");
    }
    std::error_code ec;
    int left = 0;
    for (fs::directory_iterator it(to_path(base), ec), end; !ec && it != end; it.increment(ec)) {
        left++;
    }
    CHECK(left == 0);

    // A folder of temporary files that is not there: the start says why, and the output is still
    // the real one (a later capture works again).
    CHECK_STR(str(T::set_env(gs(tmp_var), gs(base + "/no-such-folder"))), "");
    std::string failed = str(T::capture_start());
    CHECK(failed.find("cannot capture the standard output") != std::string::npos);
    CHECK_STR(str(T::capture_stop()), "");
    CHECK_STR(str(T::set_env(gs(tmp_var), gs(base))), "");
    CHECK_STR(str(T::capture_start()), "");
    CHECK_STR(str(T::capture_stop()), "");
    if (had) {
        T::set_env(gs(tmp_var), gs(saved));
    } else {
        T::unset_env(gs(tmp_var));
    }
    CHECK_STR(str(T::remove_all(gs(base))), "");

    // After the captures the real output works (the report of this program is its proof: the
    // runner reads "ok: " on standard output).
}

// --- cpu_name, remove_target ---

void test_trim_and_target() {
    CHECK_STR(str(T::trim_cpu_name(gs("  Intel(R) Core(TM) i5 \t\r\n"))), "Intel(R) Core(TM) i5");
    CHECK_STR(str(T::trim_cpu_name(gs("\t x  y\n"))), "x  y");
    CHECK_STR(str(T::trim_cpu_name(gs("   \r\n"))), "");
    CHECK_STR(str(T::trim_cpu_name(gs(""))), "");
    CHECK_STR(str(T::trim_cpu_name(gs("plain"))), "plain");

    // Trailing separators go; the text of the path stays; roots are refused; "" is nothing to do.
    auto target = [](const std::string& p, std::string& err) {
        auto [t, e] = T::remove_target(gs(p));
        err = str(e);
        return str(t);
    };
    std::string err;
    auto refused = [&](const std::string& p) {
        // the message names the path as it was given, in quotes
        std::string t = target(p, err);
        return t.empty() && err.rfind("RemoveAll \"" + p + "\": refusing to remove ", 0) == 0;
    };
    auto allowed = [&](const std::string& p, const std::string& want) {
        std::string t = target(p, err);
        return t == want && err.empty();
    };
#ifdef _WIN32
    CHECK(allowed("C:\\x\\var\\", "C:\\x\\var"));
    CHECK(allowed("C:/x/var/", "C:\\x\\var"));
    CHECK(allowed("C:\\x\\var\\\\", "C:\\x\\var"));
    CHECK(allowed("var", "var"));
    CHECK(allowed("\\\\?\\C:\\x\\", "\\\\?\\C:\\x"));
    CHECK(allowed("\\\\server\\share\\dir\\", "\\\\server\\share\\dir"));
    CHECK(allowed("a\\.hidden", "a\\.hidden")); // names that only start with a dot are names
    CHECK(allowed("a\\..b", "a\\..b"));
    CHECK(allowed("", ""));
    for (const char* root :
         {"C:\\", "C:/", "C:", "c:\\\\", "\\", "/", "\\\\?\\C:\\", "\\\\?\\c:\\",
          "\\\\server\\share", "\\\\server\\share\\", "\\\\?\\UNC\\server\\share\\", "\\\\server",
          "\\\\.\\C:\\", "\\\\.\\PhysicalDrive0",
          "\\\\?\\Volume{01234567-89ab-cdef-0123-456789abcdef}\\",
          "\\\\?\\GLOBALROOT\\Device\\HarddiskVolume1\\x", "\\\\?\\",
          "\\\\nonexistent-srv\\share"}) {
        CHECK(refused(root));
    }
    // The paths that name a folder other than the one written: the dots are resolved by the
    // system, and C:\foo\.. is the drive's root.
    for (const char* dots : {".",
                             "..",
                             "..\\..",
                             "x\\.\\y",
                             "x\\..",
                             "C:.",
                             "C:..",
                             "C:\\.",
                             "C:\\..",
                             "C:\\foo\\..",
                             "C:\\Windows\\..\\",
                             "\\.",
                             "\\..",
                             "\\foo\\..",
                             "\\\\?\\C:\\foo\\..",
                             "C:/foo/../",
                             "...",
                             "x\\... ",
                             "a\\. \\b",
                             "\\\\server\\share\\dir\\.."}) {
        CHECK(refused(dots));
    }
    // The folder of temporary files and the profile themselves, not what is in them.
    auto [tdir, terr] = T::make_temp_dir(gs("gocpp-tn-target-"));
    CHECK_STR(str(terr), "");
    std::string tbase = str(tdir);
    auto [tv, thad] = T::lookup_env(gs("TMP"));
    auto [pv, phad] = T::lookup_env(gs("USERPROFILE"));
    std::string tsaved = str(tv), psaved = str(pv);
    CHECK_STR(str(T::set_env(gs("TMP"), gs(tbase))), "");
    CHECK_STR(str(T::set_env(gs("USERPROFILE"), gs(tbase + "\\profile"))), "");
    CHECK(refused(tbase));
    CHECK(refused(tbase + "\\"));
    CHECK(refused(tbase + "/"));
    CHECK(refused("\\\\?\\" + tbase));
    std::string upper = tbase;
    for (char& c : upper) {
        c = static_cast<char>(std::toupper(static_cast<unsigned char>(c)));
    }
    CHECK(refused(upper));
    CHECK(allowed(tbase + "\\sub", tbase + "\\sub"));
    CHECK(refused(tbase + "\\profile"));
    CHECK(refused(tbase + "\\PROFILE\\"));
    CHECK(allowed(tbase + "\\profile\\x", tbase + "\\profile\\x"));
    if (thad) {
        T::set_env(gs("TMP"), gs(tsaved));
    } else {
        T::unset_env(gs("TMP"));
    }
    if (phad) {
        T::set_env(gs("USERPROFILE"), gs(psaved));
    } else {
        T::unset_env(gs("USERPROFILE"));
    }
    // remove_all asks remove_target (a refused path gives the refusal and deletes nothing). Only
    // paths inside this test's own folder, or a server that is not there, are used.
    CHECK_STR(str(T::make_dir(gs(tbase + "\\tree"))), "");
    CHECK_STR(str(T::make_dir(gs(tbase + "\\tree\\inner"))), "");
    std::ofstream(to_path(tbase + "\\tree\\f.txt"), std::ios::binary) << "x";
    for (const std::string& p :
         {tbase + "\\tree\\inner\\..", tbase + "\\tree\\.", tbase + "\\tree\\..\\tree",
          std::string("\\\\nonexistent-srv\\share")}) {
        std::string e = str(T::remove_all(gs(p)));
        CHECK(e.rfind("RemoveAll \"", 0) == 0 && e.find("refusing") != std::string::npos);
        std::error_code ec;
        CHECK(fs::exists(to_path(tbase + "\\tree\\f.txt"), ec));
    }
    CHECK_STR(str(T::remove_all(gs(tbase))), "");
#else
    CHECK(allowed("/x/var/", "/x/var"));
    CHECK(allowed("/x/var///", "/x/var"));
    CHECK(allowed("var", "var"));
    CHECK(allowed("a/.hidden", "a/.hidden"));
    CHECK(allowed("a//b", "a//b"));
    CHECK(allowed("", ""));
    for (const char* bad :
         {"/", "//", "///", ".", "..", "../..", "/.", "/..", "/tmp/..", "/foo/..", "x/./y", "x/..",
          "./x", "../x", "/proc", "/proc/", "/proc/self/root", "/proc/1/cwd/x"}) {
        CHECK(refused(bad));
    }
    auto [tdir, terr] = T::make_temp_dir(gs("gocpp-tn-target-"));
    CHECK_STR(str(terr), "");
    std::string tbase = str(tdir);
    auto [tv, thad] = T::lookup_env(gs("TMPDIR"));
    auto [hv, hhad] = T::lookup_env(gs("HOME"));
    std::string tsaved = str(tv), hsaved = str(hv);
    CHECK_STR(str(T::set_env(gs("TMPDIR"), gs(tbase))), "");
    CHECK_STR(str(T::set_env(gs("HOME"), gs(tbase + "/home/"))), "");
    CHECK(refused(tbase));
    CHECK(refused(tbase + "/"));
    CHECK(allowed(tbase + "/sub", tbase + "/sub"));
    CHECK(refused(tbase + "/home"));
    CHECK(refused(tbase + "/home//"));
    CHECK(allowed(tbase + "/home/x", tbase + "/home/x"));
    if (thad) {
        T::set_env(gs("TMPDIR"), gs(tsaved));
    } else {
        T::unset_env(gs("TMPDIR"));
    }
    if (hhad) {
        T::set_env(gs("HOME"), gs(hsaved));
    } else {
        T::unset_env(gs("HOME"));
    }
    // remove_all asks remove_target (a refused path gives the refusal and deletes nothing); only
    // paths inside this test's own folder are used. A symbolic link with a trailing slash is
    // removed itself: the folder it points to keeps its files.
    CHECK_STR(str(T::make_dir(gs(tbase + "/tree"))), "");
    CHECK_STR(str(T::make_dir(gs(tbase + "/tree/inner"))), "");
    std::ofstream(to_path(tbase + "/tree/f.txt"), std::ios::binary) << "x";
    for (const std::string& p :
         {tbase + "/tree/inner/..", tbase + "/tree/.", tbase + "/tree/../tree"}) {
        std::string e = str(T::remove_all(gs(p)));
        CHECK(e.rfind("RemoveAll \"", 0) == 0 && e.find("refusing") != std::string::npos);
        std::error_code ec;
        CHECK(fs::exists(to_path(tbase + "/tree/f.txt"), ec));
    }
    std::error_code lec;
    fs::create_directory_symlink(to_path(tbase + "/tree"), to_path(tbase + "/link"), lec);
    CHECK(!lec);
    CHECK_STR(str(T::remove_all(gs(tbase + "/link/"))), "");
    std::error_code ec;
    CHECK(!fs::exists(fs::symlink_status(to_path(tbase + "/link"), ec)));
    CHECK(fs::exists(to_path(tbase + "/tree/f.txt"), ec));
    CHECK_STR(str(T::remove_all(gs(tbase))), "");
#endif
}

// --- the processor's name ---

void test_cpu_name() {
    std::string name = str(T::cpu_name());
    // No white space at either end, and no line end.
    CHECK(name.empty() || (name.front() != ' ' && name.front() != '\t' && name.back() != ' ' &&
                           name.back() != '\t' && name.find('\n') == std::string::npos &&
                           name.find('\r') == std::string::npos));
#ifdef _WIN32
    CHECK(!name.empty()); // the registry has it, also under Wine
#elif defined(__x86_64__)
    CHECK(!name.empty()); // /proc/cpuinfo has "model name" on x86
#endif
}

} // namespace

int main() {
    test_env();
    test_temp_dirs();
    test_remove_links();
#ifdef _WIN32
    test_long_paths();
    test_remove_busy();
#endif
    test_files();
    test_capture();
    test_trim_and_target();
    test_cpu_name();
    if (failures != 0) {
        return 1;
    }
    std::string msg = "ok: " + std::to_string(checks) + " checks\n";
    gp::os::write(gp::os::stdout_fd, msg.data(), msg.size());
    return 0;
}
