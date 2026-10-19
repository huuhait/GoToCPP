// The unit tests of gocpp/concepts.hpp (Phase 07 step 7.9, `gocpp -gp:concepts`): tilde, exact,
// comparable, ptr_elem_t and is_ptr, with the types that the generated concepts see. The checks are
// static_asserts (they fail at compile time); the program prints "ok: N checks" and exits 0, like
// test/unit.cpp. runtime/cxx_test.go compiles it with g++ and clang++.
#include <gocpp/rt.hpp>
#include <gocpp/concepts.hpp>
#include <gocpp/os.hpp>

#include <cstdint>
#include <string>
#include <vector>

namespace {

struct Meters : gp::newtype<Meters, int64_t> {
    using gp::newtype<Meters, int64_t>::newtype;
};
struct Names : gp::newtype<Names, gp::slice<gp::string>> {
    using gp::newtype<Names, gp::slice<gp::string>>::newtype;
};
struct S {
    int64_t n = 0;
    bool operator==(const S&) const = default;
};
struct NotComparable {
    gp::slice<int64_t> xs;
};
struct NPS : gp::newtype<NPS, gp::ptr<S>> {
    using gp::newtype<NPS, gp::ptr<S>>::newtype;
};

using namespace gp::concepts;

// tilde<T, U>: T is U or a defined type over U.
static_assert(tilde<int64_t, int64_t> && tilde<Meters, int64_t> &&
              tilde<Names, gp::slice<gp::string>>);
static_assert(!tilde<int32_t, int64_t> && !tilde<double, int64_t> && !tilde<gp::string, int64_t>);
static_assert(!tilde<Meters, double> && !tilde<gp::slice<gp::string>, int64_t> &&
              !tilde<S, int64_t>);
// exact<T, U>: T is U.
static_assert(exact<int64_t, int64_t> && exact<gp::string, gp::string>);
static_assert(!exact<Meters, int64_t> && !exact<int8_t, int64_t>);
// comparable<T>: ==.
static_assert(comparable<int64_t> && comparable<gp::string> && comparable<Meters> && comparable<S>);
static_assert(comparable<gp::ptr<S>> && comparable<gp::any>);
static_assert(!comparable<NotComparable> && !comparable<gp::slice<int64_t>>);
// ptr_elem_t / is_ptr: only gp::ptr itself, not a defined pointer type.
static_assert(std::same_as<ptr_elem_t<gp::ptr<S>>, S>);
static_assert(is_ptr<gp::ptr<S>> && !is_ptr<S> && !is_ptr<NPS> && !is_ptr<int64_t> && !is_ptr<S*>);
template <typename T>
concept has_elem = requires { typename ptr_elem_t<T>; };
static_assert(has_elem<gp::ptr<S>> && !has_elem<S> && !has_elem<NPS>);

} // namespace

int main() {
    std::string msg = "ok: 8 static checks\n";
    gp::os::write(gp::os::stdout_fd, msg.data(), msg.size());
    return 0;
}
