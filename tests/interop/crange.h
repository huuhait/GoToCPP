// A helper header for the tests of range over C/C++ values (step 4.7): C arrays, containers
// returned by value, and an object that a temporary owns.
#pragma once

#include <list>
#include <map>
#include <memory>
#include <mutex>
#include <string>
#include <string_view>
#include <vector>

inline int table[5] = {10, 20, 30, 40, 50};
inline int grid[2][3] = {{1, 2, 3}, {4, 5, 6}};

// squares returns a new vector: a temporary, which gp::range holds by value.
inline std::vector<int> squares(int n) {
    std::vector<int> v;
    for (int i = 0; i < n; i++) {
        v.push_back(i * i);
    }
    return v;
}

// A box returned by value: range make_box().items walks a member of a temporary, whose strings
// are on the heap (long enough for that), so ASan sees a use after free.
struct box {
    std::vector<std::string> items;
};
inline box make_box() {
    return box{{std::string(20, 'a'), std::string(30, 'b')}};
}

// longer returns a reference to one of its arguments, which can be temporaries.
inline const std::vector<int>& longer(const std::vector<int>& a, const std::vector<int>& b) {
    return a.size() >= b.size() ? a : b;
}

inline std::list<int> make_list() {
    return {3, 1, 2};
}

// evens is one vector for the whole program, returned by reference.
inline std::vector<int>& evens() {
    static std::vector<int> v{0, 2, 4};
    return v;
}

// For minmax_values.gp: first_of takes a pair of values and another argument.
inline int first_of(std::pair<int, int> p, int k) {
    return p.first + k;
}

// Owners that the range expression, not a stored view, creates.
inline auto vector_owner() {
    return std::make_unique<std::vector<int>>(20, 7);
}
struct reader {
    std::vector<int> items = std::vector<int>(20, 7);
    std::vector<int>& operator()() { return items; }
};
inline reader make_reader() {
    return reader{};
}
inline std::map<std::unique_ptr<int>, std::unique_ptr<int>> owners_map() {
    std::map<std::unique_ptr<int>, std::unique_ptr<int>> m;
    m.emplace(std::make_unique<int>(3), std::make_unique<int>(7));
    return m;
}
inline std::map<int, std::unique_ptr<int>> mapped_owners() {
    std::map<int, std::unique_ptr<int>> m;
    m.emplace(3, std::make_unique<int>(7));
    return m;
}
inline std::map<std::unique_ptr<int>, int> keyed_owners() {
    std::map<std::unique_ptr<int>, int> m;
    m.emplace(std::make_unique<int>(3), 7);
    return m;
}

struct immovable {
    int n = 7;
    immovable() = default;
    immovable(const immovable&) = delete;
    immovable(immovable&&) = delete;
};
inline immovable make_immovable() {
    return immovable{};
}
inline std::mutex fresh_mutex() {
    return {};
}
inline int result_copies = 0;
inline int result_moves = 0;
struct counted_result {
    int n = 7;
    counted_result() = default;
    counted_result(const counted_result& x) : n(x.n) { ++result_copies; }
    counted_result(counted_result&& x) noexcept : n(x.n) { ++result_moves; }
};
inline counted_result fresh_result() {
    return counted_result{};
}
inline counted_result& result_ref() {
    static counted_result x;
    return x;
}
inline int read_result(counted_result&& x, int n) {
    return x.n + n;
}
inline int increment_ref(int& n) {
    return ++n;
}

inline std::string_view longer_view(std::string_view a, std::string_view b) {
    return a.size() >= b.size() ? a : b;
}

// A range factory whose parameter is a pointer: the literal 0 is a null pointer constant.
inline std::vector<int> from_cstr(const char* s) {
    return std::vector<int>(s == nullptr ? 2 : 3, 5);
}

// For range_order.gp: an object that logs the order of its calls, and a function that returns a
// reference into one of its (temporary) arguments.
struct tracer {
    std::vector<int> log;
    int tag(int n) {
        log.push_back(n);
        return n;
    }
    std::vector<int> tr(int n) {
        log.push_back(n);
        return std::vector<int>(3, n);
    }
    const std::vector<int>& pick(int a, const std::vector<int>& b, const std::vector<int>& c) {
        return a == 1 ? b : c;
    }
};
inline const std::string& first_ref(const std::string& a) {
    return a;
}
