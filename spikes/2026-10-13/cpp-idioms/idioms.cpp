// Proof that the C++ idioms the gocpp plan relies on are valid C++20 on GCC and Clang.
#include <cstdint>
#include <cstdio>
#include <exception>
#include <functional>
#include <memory>
#include <string>
#include <tuple>
#include <type_traits>
#include <vector>

// --- 1. ARC pointers: interior pointers share ownership (aliasing ctor); globals are non-owning
struct Inner { int64_t v = 7; };
struct Outer { int64_t a = 1; Inner in; };
Outer g_outer;

// --- 2. single-exit function shape: defers run on return AND on panic; recover stops the panic
struct panic_error { std::string msg; };
struct defer_frame {
    std::vector<std::function<void(defer_frame&)>> fns;
    std::exception_ptr panic;
    bool recovered = false;
    void push(std::function<void(defer_frame&)> f) { fns.push_back(std::move(f)); }
    void on_panic(std::exception_ptr p) { panic = p; }
    std::string recover() {                 // only meaningful inside a deferred call
        if (!panic || recovered) return "";
        recovered = true;
        try { std::rethrow_exception(panic); } catch (const panic_error& e) { return e.msg; }
    }
    void run() {
        for (auto it = fns.rbegin(); it != fns.rend(); ++it) (*it)(*this);   // LIFO
        if (panic && !recovered) std::rethrow_exception(panic);
    }
};

// func f(fail bool) (n int) { defer func() { if r := recover(); r != "" { n = -1 } else { n *= 10 } }(); if fail { panic("boom") }; return 4 }
int64_t f(bool fail) {
    auto n = std::make_shared<int64_t>(0);          // named result, boxed (captured + modified by a defer)
    defer_frame gp_df;
    try {
        gp_df.push([n](defer_frame& df) { auto r = df.recover(); if (!r.empty()) *n = -1; else *n *= 10; });
        if (fail) throw panic_error{"boom"};
        { *n = 4; goto gp_exit; }                  // `return 4`
    } catch (...) {
        gp_df.on_panic(std::current_exception());
    }
gp_exit:
    gp_df.run();
    return *n;
}

// --- 3. method calls on type-parameter values: pick the call form at compile time
template <class T> struct pointee { using type = void; };
template <class T> struct pointee<std::shared_ptr<T>> { using type = T; };
template <class T> using pointee_t = typename pointee<T>::type;

struct ValM { std::string String() const { return "value-method"; } };
struct PtrM { static std::string String(std::shared_ptr<PtrM>) { return "pointer-method"; } };

template <typename T>
std::string gp_call_String(const T& x) {
    if constexpr (requires { x.String(); }) return x.String();
    else if constexpr (requires { pointee_t<T>::String(x); }) return pointee_t<T>::String(x);
    else return x->String();
}

// --- 4. structs: default member init (zero values), designated init, defaulted ==
struct Vec2 { double X = 0; double Y = 0; bool operator==(const Vec2&) const = default; };

// --- 5. multiple results
std::tuple<int64_t, bool> add(int64_t a, int64_t b) { return {a + b, a + b > 10}; }

// --- 6. constant probe helper must at least compile
template <auto V> struct gp_val {};
using probe = gp_val<BUFSIZ>;

int main() {
    // 1
    std::shared_ptr<Inner> field;
    {
        auto owner = std::make_shared<Outer>();
        field = std::shared_ptr<Inner>(owner, &owner->in);   // &owner.in keeps owner alive
    }
    std::shared_ptr<Outer> global_ptr(std::shared_ptr<void>{}, &g_outer);   // non-owning
    std::printf("1 interior=%lld use_count(global)=%ld\n", (long long)field->v, global_ptr.use_count());

    // 2
    std::printf("2 f(false)=%lld f(true)=%lld\n", (long long)f(false), (long long)f(true));

    // 3
    std::printf("3 %s %s %s\n", gp_call_String(ValM{}).c_str(),
                gp_call_String(std::make_shared<PtrM>()).c_str(),
                gp_call_String(std::make_shared<ValM>()).c_str());

    // 4 + 5
    Vec2 v{.X = 3, .Y = 4};
    auto t = add(7, 5);
    std::printf("4 eq=%d zero=%d  5 sum=%lld big=%d\n", v == Vec2{.X = 3, .Y = 4}, Vec2{} == Vec2{0, 0},
                (long long)std::get<0>(t), std::get<1>(t));

    // 7. slice backing array: make_shared<T[]> (C++20)
    auto arr = std::make_shared<int64_t[]>(4);
    std::shared_ptr<int64_t> elem(arr, &arr[2]);   // &s[2]
    *elem = 42;
    std::printf("7 arr[2]=%lld\n", (long long)arr[2]);
}
