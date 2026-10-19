// The unit tests of gocpp/lazy.hpp (Phase 06 step 6.4, ADR-093): gp::lazy<T> and gp::lazy_boot,
// the storage of a package variable whose type is a C++ class with a constructor that is not
// constexpr. The program prints "ok: N checks" and exits 0, like test/unit.cpp.
// runtime/cxx_test.go compiles it with g++ and clang++.
#include <gocpp/rt.hpp>
#include <gocpp/os.hpp>

#include <cstdint>
#include <string>

namespace {

int checks = 0;
bool failed = false;

void check(bool ok, const char* what) {
    checks++;
    if (!ok) {
        failed = true;
        std::string msg = std::string("FAIL: ") + what + "\n";
        gp::os::write(gp::os::stderr_fd, msg.data(), msg.size());
    }
}

int constructed = 0;
int destroyed = 0;
int next_seed() {
    return ++constructed;
}

// A class whose constructor cannot run at compile time, as std::ofstream's cannot.
struct Late {
    int v;
    Late() : v(next_seed() * 100) {}
    ~Late() { destroyed++; }
};

// constinit with no object: the compiler checks that the reference is a constant.
gp::lazy<Late> store;
constinit Late& var = store.gp_ref();

// A host's static initializer that calls the library before this file's dynamic initializers
// have run (here: before gp_boot, in the order of the file): gp_var_init's gp_ensure, then a store.
struct Early {
    Early() {
        check(!store.gp_exists(), "no object before gp_ensure");
        store.gp_ensure();
        var.v = 7;
    }
} early;

// The dynamic initializer, after the call: it must not build the object again (it would wipe 7).
const gp::lazy_boot boot{store};

// A variable nobody ensured: the dynamic initializer builds it, the zero value of C++.
gp::lazy<Late> quiet;
constinit Late& quiet_var = quiet.gp_ref();
const gp::lazy_boot quiet_boot{quiet};

} // namespace

int main() {
    check(var.v == 7, "the dynamic initializer did not wipe the stored value");
    check(store.gp_exists(), "the object exists");
    store.gp_ensure();
    check(var.v == 7, "gp_ensure is idempotent");
    check(quiet.gp_exists() && quiet_var.v == 200,
          "a variable nobody ensured is built by the boot");
    check(constructed == 2, "one construction per variable");

    {
        gp::lazy<Late> local;
        local.gp_ensure();
        check(constructed == 3, "a local lazy builds on gp_ensure");
    }
    check(destroyed == 0, "the object is never destroyed");

    std::string msg = "ok: " + std::to_string(checks) + " checks\n";
    gp::os::write(gp::os::stdout_fd, msg.data(), msg.size());
    return failed ? 1 : 0;
}
