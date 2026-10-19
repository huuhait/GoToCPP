// gocpp typed interop: probe file (generated)
#include <cstddef>
#include <cstdint>
#include <bit>
#include <compare>
#include <iterator>
#include <string_view>
#include <type_traits>
#include <utility>
#include <initializer_list>
#include <string>
#include <vector>
#include <memory>
#include <tuple>
#include <array>
#include <cstring>
#include <limits>
#include <new>
#include <span>
#include <algorithm>
#include <array>
#include <bitset>
#include <deque>
#include <functional>
#include <list>
#include <map>
#include <queue>
#include <set>
#include <sstream>
#include <stack>
#include <string>
#include <string_view>
#include <tuple>
#include <utility>
namespace gocpp_probe_internal_pr {
struct string {
  string() = default;
  template <size_t gocpp_probe_internal_N> string(const char (&)[gocpp_probe_internal_N]) noexcept;  // a literal copies (Phase 04, step 4.9)
  explicit string(std::string_view) noexcept;  // string(x) of a std::string or a std::string_view copies
  operator std::string_view() const noexcept;
  std::strong_ordering operator<=>(const string&) const noexcept;
  bool operator==(const string&) const noexcept;
  size_t size() const noexcept;
  const char* data() const noexcept;
  const uint8_t& operator[](size_t) const noexcept;
};
template <class gocpp_probe_internal_T> struct slice {
  using value_type = gocpp_probe_internal_T;
  slice() = default;
  slice(std::initializer_list<gocpp_probe_internal_T>) noexcept;
  operator std::span<gocpp_probe_internal_T>() const noexcept;
  gocpp_probe_internal_T* begin() const noexcept;
  gocpp_probe_internal_T* end() const noexcept;
  gocpp_probe_internal_T* data() const noexcept;
  size_t size() const noexcept;
  gocpp_probe_internal_T& operator[](size_t) const noexcept;
};
template <class gocpp_probe_internal_T, size_t gocpp_probe_internal_N> struct array {
  using value_type = gocpp_probe_internal_T;
  array() = default;
  array(std::initializer_list<gocpp_probe_internal_T>) noexcept;
  gocpp_probe_internal_T* begin() const noexcept;
  gocpp_probe_internal_T* end() const noexcept;
  gocpp_probe_internal_T* data() const noexcept;
  constexpr size_t size() const noexcept { return gocpp_probe_internal_N; }
  gocpp_probe_internal_T& operator[](size_t) const noexcept;
};
template <class gocpp_probe_internal_K, class gocpp_probe_internal_V> struct map {
  using key_type = gocpp_probe_internal_K;
  using mapped_type = gocpp_probe_internal_V;
  map() = default;
  map(std::initializer_list<std::pair<const gocpp_probe_internal_K, gocpp_probe_internal_V>>) noexcept;
  size_t size() const noexcept;
  gocpp_probe_internal_V& operator[](const gocpp_probe_internal_K&) const noexcept;
};
template <class gocpp_probe_internal_T> constexpr decltype(auto) deref(gocpp_probe_internal_T&& gocpp_probe_internal_t) {
  if constexpr (std::is_pointer_v<std::remove_cvref_t<gocpp_probe_internal_T>>) return (*gocpp_probe_internal_t);
  else return std::forward<gocpp_probe_internal_T>(gocpp_probe_internal_t);
}
using std::begin;  // begin(c) finds std::begin and the begin of c's own namespace (ADL)
template <class gocpp_probe_internal_C, class = void> struct range_t {
  using key = int64_t;
  using val = std::decay_t<decltype(*begin(std::declval<gocpp_probe_internal_C&>()))>;
};
template <class gocpp_probe_internal_C> struct range_t<gocpp_probe_internal_C, std::void_t<typename gocpp_probe_internal_C::mapped_type>> {
  using key = typename gocpp_probe_internal_C::key_type;
  using val = typename gocpp_probe_internal_C::mapped_type;
};
void not_exact();  // not constexpr: calling it makes the constant evaluation fail
template <class gocpp_probe_internal_T> concept arith = std::is_arithmetic_v<gocpp_probe_internal_T> || std::is_enum_v<gocpp_probe_internal_T> ||
    std::is_same_v<gocpp_probe_internal_T, __int128> || std::is_same_v<gocpp_probe_internal_T, unsigned __int128>;
template <class gocpp_probe_internal_T> requires arith<std::remove_cvref_t<gocpp_probe_internal_T>> constexpr __int128 enc(const gocpp_probe_internal_T& gocpp_probe_internal_v) {
  using gocpp_probe_internal_U = std::remove_cvref_t<gocpp_probe_internal_T>;
  if constexpr (std::is_floating_point_v<gocpp_probe_internal_U>) {
    // A float is read as the bits of a double: a wider type (long double) must round-trip,
    // else the evaluation fails (no constant) instead of giving a rounded value.
    if constexpr (sizeof(gocpp_probe_internal_U) > sizeof(double)) { if (!((gocpp_probe_internal_U)(double)gocpp_probe_internal_v == gocpp_probe_internal_v)) not_exact(); }
    return (__int128)std::bit_cast<uint64_t>((double)gocpp_probe_internal_v);
  } else if constexpr (std::is_same_v<gocpp_probe_internal_U, unsigned __int128>) {
    if (gocpp_probe_internal_v > (unsigned __int128)std::numeric_limits<__int128>::max()) not_exact();  // above 2^127-1: not exact in __int128
    return (__int128)gocpp_probe_internal_v;
  } else return (__int128)gocpp_probe_internal_v;
}
template <size_t gocpp_probe_internal_N> constexpr const char* str_of(const char (&gocpp_probe_internal_a)[gocpp_probe_internal_N]) { return gocpp_probe_internal_a; }
template <class gocpp_probe_internal_T> constexpr int str_of(const gocpp_probe_internal_T&) { return 0; }
}
template <auto gocpp_probe_internal_V> struct gocpp_probe_internal_val {};
template <class gocpp_probe_internal_T> struct gocpp_probe_internal_c {};
using gp_probe_0 = gocpp_probe_internal_c<int>;
using gp_probe_1 = gocpp_probe_internal_c<decltype(((std::declval<int&>() * std::declval<int&>())))>;
using gp_probe_2 = gocpp_probe_internal_c<decltype(((std::declval<int&>() + std::declval<int&>())))>;
using gp_probe_3 = gocpp_probe_internal_c<decltype(((std::declval<::std::stringstream&>() << "n=")))>;
using gp_probe_4 = gocpp_probe_internal_c<decltype((((std::declval<::std::stringstream&>() << "n=") << 5)))>;
using gp_probe_5 = gocpp_probe_internal_c<decltype(((((std::declval<::std::stringstream&>() << "n=") << 5) << ' ')))>;
using gp_probe_6 = gocpp_probe_internal_c<decltype((((((std::declval<::std::stringstream&>() << "n=") << 5) << ' ') << ::std::string("s"))))>;
using gp_probe_7 = gocpp_probe_internal_c<decltype(((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).find(9) == gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).end())))>;
using gp_probe_8 = gocpp_probe_internal_c<decltype((bool(gocpp_probe_internal_pr::deref(std::declval<::std::queue<::std::string>&>()).empty())))>;
using gp_probe_9 = gocpp_probe_internal_c<decltype((double(::std::get<2>(std::declval<std::decay_t<decltype((::std::make_tuple(1, ::std::string("two"), 3.5)))>&>()))))>;
using gp_probe_10 = gocpp_probe_internal_c<decltype((int64_t(std::declval<std::decay_t<typename gocpp_probe_internal_pr::range_t<std::remove_cvref_t<decltype((std::declval<std::decay_t<decltype((::std::list<int64_t>{3, 1, 2}))>&>()))>>::val>&>())))>;
using gp_probe_11 = gocpp_probe_internal_c<decltype((int64_t(std::declval<std::decay_t<typename gocpp_probe_internal_pr::range_t<std::remove_cvref_t<decltype((std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()))>>::val>&>())))>;
using gp_probe_12 = gocpp_probe_internal_c<decltype((int64_t(std::declval<std::decay_t<typename gocpp_probe_internal_pr::range_t<std::remove_cvref_t<decltype((std::declval<std::decay_t<decltype((::std::array<int, 4>{4, 3, 2, 1}))>&>()))>>::val>&>())))>;
using gp_probe_13 = gocpp_probe_internal_c<decltype((int64_t(std::declval<std::decay_t<std::tuple_element_t<0, std::remove_cvref_t<decltype((::std::minmax(17, 5)))>>>&>())))>;
using gp_probe_14 = gocpp_probe_internal_c<decltype((int64_t(std::declval<std::decay_t<std::tuple_element_t<1, std::remove_cvref_t<decltype((::std::minmax(17, 5)))>>>&>())))>;
using gp_probe_17 = gocpp_probe_internal_c<decltype((int64_t(::std::get<int>(std::declval<std::decay_t<decltype((::std::make_tuple(1, ::std::string("two"), 3.5)))>&>()))))>;
using gp_probe_18 = gocpp_probe_internal_c<decltype((int64_t(::std::get<0>(std::declval<std::decay_t<decltype((::std::make_tuple(1, ::std::string("two"), 3.5)))>&>()))))>;
using gp_probe_19 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::bitset<8>(11)))>&>()).count())))>;
using gp_probe_20 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::string_view(std::declval<std::decay_t<decltype((::std::string("hello world")))>&>())))>&>()).find("o"))))>;
using gp_probe_21 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::string_view(std::declval<std::decay_t<decltype((::std::string("hello world")))>&>())))>&>()).size())))>;
using gp_probe_22 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::deque<int64_t>{}))>&>()).back())))>;
using gp_probe_23 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::deque<int64_t>{}))>&>()).front())))>;
using gp_probe_24 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::deque<int64_t>{}))>&>()).size())))>;
using gp_probe_25 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).count(5))))>;
using gp_probe_26 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).erase(1))))>;
using gp_probe_27 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).size())))>;
using gp_probe_28 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::array<int, 4>{4, 3, 2, 1}))>&>()).back())))>;
using gp_probe_29 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::array<int, 4>{4, 3, 2, 1}))>&>()).front())))>;
using gp_probe_30 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::array<int, 4>{4, 3, 2, 1}))>&>()).size())))>;
using gp_probe_32 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<::std::priority_queue<int64_t>&>()).size())))>;
using gp_probe_33 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<::std::priority_queue<int64_t>&>()).top())))>;
using gp_probe_34 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<::std::stack<int64_t>&>()).size())))>;
using gp_probe_35 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<::std::stack<int64_t>&>()).top())))>;
using gp_probe_36 = gocpp_probe_internal_c<decltype((int64_t(::std::size(std::declval<std::decay_t<decltype((::std::array<int, 4>{4, 3, 2, 1}))>&>()))))>;
using gp_probe_37 = gocpp_probe_internal_c<decltype((int64_t((std::declval<std::decay_t<decltype((::std::deque<int64_t>{}))>&>())[1])))>;
using gp_probe_38 = gocpp_probe_internal_c<decltype((int64_t(::std::tuple_size_v<::std::tuple<int64_t, int64_t, int64_t>>)))>;
using gp_probe_41 = gocpp_probe_internal_c<decltype((int64_t((*gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).lower_bound(2)))))>;
using gp_probe_42 = gocpp_probe_internal_c<decltype((int64_t((*gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).insert(4)))>&>()).first))))>;
using gp_probe_45 = gocpp_probe_internal_c<decltype((::std::bitset<8>(11)))>;
using gp_probe_46 = gocpp_probe_internal_c<decltype((::std::get<int>(std::declval<std::decay_t<decltype((::std::make_tuple(1, ::std::string("two"), 3.5)))>&>())))>;
using gp_probe_47 = gocpp_probe_internal_c<decltype((::std::get<0>(std::declval<std::decay_t<decltype((::std::make_tuple(1, ::std::string("two"), 3.5)))>&>())))>;
using gp_probe_48 = gocpp_probe_internal_c<decltype((::std::get<1>(std::declval<std::decay_t<decltype((::std::make_tuple(1, ::std::string("two"), 3.5)))>&>())))>;
using gp_probe_49 = gocpp_probe_internal_c<decltype((::std::get<2>(std::declval<std::decay_t<decltype((::std::make_tuple(1, ::std::string("two"), 3.5)))>&>())))>;
using gp_probe_50 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::bitset<8>(11)))>&>()).count()))>;
using gp_probe_51 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::bitset<8>(11)))>&>()).set(7)))>;
using gp_probe_52 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::bitset<8>(11)))>&>()).test(1)))>;
using gp_probe_53 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::bitset<8>(11)))>&>()).test(2)))>;
using gp_probe_54 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::bitset<8>(11)))>&>()).to_string()))>;
using gp_probe_55 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::string_view(std::declval<std::decay_t<decltype((::std::string("hello world")))>&>())))>&>()).find("o")))>;
using gp_probe_56 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::string_view(std::declval<std::decay_t<decltype((::std::string("hello world")))>&>())))>&>()).size()))>;
using gp_probe_57 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::string_view(std::declval<std::decay_t<decltype((::std::string("hello world")))>&>())))>&>()).starts_with("hell")))>;
using gp_probe_58 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::string_view(std::declval<std::decay_t<decltype((::std::string("hello world")))>&>())))>&>()).substr(6)))>;
using gp_probe_59 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::deque<int64_t>{}))>&>()).back()))>;
using gp_probe_60 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::deque<int64_t>{}))>&>()).front()))>;
using gp_probe_61 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::deque<int64_t>{}))>&>()).push_back(2)))>;
using gp_probe_62 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::deque<int64_t>{}))>&>()).push_back(3)))>;
using gp_probe_63 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::deque<int64_t>{}))>&>()).push_front(1)))>;
using gp_probe_64 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::deque<int64_t>{}))>&>()).size()))>;
using gp_probe_65 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::list<int64_t>{3, 1, 2}))>&>()).push_front(0)))>;
using gp_probe_66 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::list<int64_t>{3, 1, 2}))>&>()).sort()))>;
using gp_probe_67 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).count(5)))>;
using gp_probe_68 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).end()))>;
using gp_probe_69 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).erase(1)))>;
using gp_probe_70 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).find(9)))>;
using gp_probe_71 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).insert(3)))>;
using gp_probe_72 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).insert(4)))>;
using gp_probe_73 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).lower_bound(2)))>;
using gp_probe_74 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).size()))>;
using gp_probe_75 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::array<int, 4>{4, 3, 2, 1}))>&>()).back()))>;
using gp_probe_76 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::array<int, 4>{4, 3, 2, 1}))>&>()).front()))>;
using gp_probe_77 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::array<int, 4>{4, 3, 2, 1}))>&>()).size()))>;
using gp_probe_79 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::priority_queue<int64_t>&>()).pop()))>;
using gp_probe_80 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::priority_queue<int64_t>&>()).push(std::declval<int64_t&>())))>;
using gp_probe_81 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::priority_queue<int64_t>&>()).size()))>;
using gp_probe_82 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::priority_queue<int64_t>&>()).top()))>;
using gp_probe_83 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::queue<::std::string>&>()).back()))>;
using gp_probe_84 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::queue<::std::string>&>()).empty()))>;
using gp_probe_85 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::queue<::std::string>&>()).front()))>;
using gp_probe_86 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::queue<::std::string>&>()).pop()))>;
using gp_probe_87 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::queue<::std::string>&>()).push("a")))>;
using gp_probe_88 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::queue<::std::string>&>()).push("b")))>;
using gp_probe_89 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::stack<int64_t>&>()).pop()))>;
using gp_probe_90 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::stack<int64_t>&>()).push(1)))>;
using gp_probe_91 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::stack<int64_t>&>()).push(2)))>;
using gp_probe_92 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::stack<int64_t>&>()).size()))>;
using gp_probe_93 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::stack<int64_t>&>()).top()))>;
using gp_probe_94 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::stringstream&>()).str()))>;
using gp_probe_95 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::string(::std::get<1>(std::declval<std::decay_t<decltype((::std::make_tuple(1, ::std::string("two"), 3.5)))>&>()))))>;
using gp_probe_96 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::string(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::bitset<8>(11)))>&>()).to_string())))>;
using gp_probe_97 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::string(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::string_view(std::declval<std::decay_t<decltype((::std::string("hello world")))>&>())))>&>()).substr(6))))>;
using gp_probe_98 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::string(gocpp_probe_internal_pr::deref(std::declval<::std::queue<::std::string>&>()).back())))>;
using gp_probe_99 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::string(gocpp_probe_internal_pr::deref(std::declval<::std::queue<::std::string>&>()).front())))>;
using gp_probe_100 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::string(gocpp_probe_internal_pr::deref(std::declval<::std::stringstream&>()).str())))>;
using gp_probe_101 = gocpp_probe_internal_c<decltype((::std::make_pair(std::declval<int64_t>(), std::declval<int64_t>())))>;
using gp_probe_102 = gocpp_probe_internal_c<decltype((::std::make_tuple(1, ::std::string("two"), 3.5)))>;
using gp_probe_103 = gocpp_probe_internal_c<decltype((::std::minmax(17, 5)))>;
using gp_probe_104 = gocpp_probe_internal_c<decltype((::std::size(std::declval<std::decay_t<decltype((::std::array<int, 4>{4, 3, 2, 1}))>&>())))>;
using gp_probe_105 = gocpp_probe_internal_c<decltype((::std::string_view(std::declval<std::decay_t<decltype((::std::string("hello world")))>&>())))>;
using gp_probe_106 = gocpp_probe_internal_c<decltype((::std::string("hello world")))>;
using gp_probe_107 = gocpp_probe_internal_c<decltype((::std::string("s")))>;
using gp_probe_108 = gocpp_probe_internal_c<decltype((::std::string("two")))>;
using gp_probe_110 = gocpp_probe_internal_c<std::decay_t<decltype((::std::bitset<8>(11)))>>;
using gp_probe_111 = gocpp_probe_internal_c<std::decay_t<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).insert(3)))>>;
using gp_probe_112 = gocpp_probe_internal_c<std::decay_t<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).insert(4)))>>;
using gp_probe_113 = gocpp_probe_internal_c<std::decay_t<decltype((::std::make_tuple(1, ::std::string("two"), 3.5)))>>;
using gp_probe_114 = gocpp_probe_internal_c<std::decay_t<decltype((::std::string_view(std::declval<std::decay_t<decltype((::std::string("hello world")))>&>())))>>;
using gp_probe_115 = gocpp_probe_internal_c<std::decay_t<decltype((::std::string("hello world")))>>;
using gp_probe_117 = gocpp_probe_internal_c<std::decay_t<decltype((::std::deque<int64_t>{}))>>;
using gp_probe_118 = gocpp_probe_internal_c<std::decay_t<decltype((::std::list<int64_t>{3, 1, 2}))>>;
using gp_probe_119 = gocpp_probe_internal_c<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>>;
using gp_probe_120 = gocpp_probe_internal_c<std::decay_t<decltype((::std::array<int, 4>{4, 3, 2, 1}))>>;
using gp_probe_122 = gocpp_probe_internal_c<std::decay_t<typename gocpp_probe_internal_pr::range_t<std::remove_cvref_t<decltype((std::declval<std::decay_t<decltype((::std::list<int64_t>{3, 1, 2}))>&>()))>>::val>>;
using gp_probe_123 = gocpp_probe_internal_c<std::decay_t<typename gocpp_probe_internal_pr::range_t<std::remove_cvref_t<decltype((std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()))>>::val>>;
using gp_probe_124 = gocpp_probe_internal_c<std::decay_t<typename gocpp_probe_internal_pr::range_t<std::remove_cvref_t<decltype((std::declval<std::decay_t<decltype((::std::array<int, 4>{4, 3, 2, 1}))>&>()))>>::val>>;
using gp_probe_125 = gocpp_probe_internal_c<std::decay_t<std::tuple_element_t<0, std::remove_cvref_t<decltype((::std::minmax(17, 5)))>>>>;
using gp_probe_126 = gocpp_probe_internal_c<std::decay_t<std::tuple_element_t<1, std::remove_cvref_t<decltype((::std::minmax(17, 5)))>>>>;
using gp_probe_127 = gocpp_probe_internal_c<decltype(((std::declval<std::decay_t<decltype((::std::deque<int64_t>{}))>&>())[1]))>;
using gp_probe_128 = gocpp_probe_internal_c<decltype(((std::declval<std::decay_t<decltype((::std::array<int, 4>{4, 3, 2, 1}))>&>())[2]))>;
using gp_probe_132 = gocpp_probe_internal_c<::std::deque<int64_t>>;
using gp_probe_138 = gocpp_probe_internal_c<::std::list<int64_t>>;
using gp_probe_139 = gocpp_probe_internal_c<::std::priority_queue<int64_t>>;
using gp_probe_140 = gocpp_probe_internal_c<::std::queue<::std::string>>;
using gp_probe_141 = gocpp_probe_internal_c<::std::set<int64_t>>;
using gp_probe_142 = gocpp_probe_internal_c<::std::stack<int64_t>>;
using gp_probe_143 = gocpp_probe_internal_c<decltype((::std::tuple_size_v<::std::tuple<int64_t, int64_t, int64_t>>))>;
using gp_probe_144 = gocpp_probe_internal_c<::std::array<int, 4>>;
using gp_probe_146 = gocpp_probe_internal_c<::std::pair<int64_t, int64_t>>;
using gp_probe_147 = gocpp_probe_internal_c<::std::tuple<int64_t, int64_t, int64_t>>;
using gp_probe_148 = gocpp_probe_internal_c<decltype((::std::deque<int64_t>{}))>;
using gp_probe_149 = gocpp_probe_internal_c<decltype((::std::list<int64_t>{3, 1, 2}))>;
using gp_probe_150 = gocpp_probe_internal_c<decltype((::std::set<int64_t>{5, 1, 3}))>;
using gp_probe_151 = gocpp_probe_internal_c<decltype((::std::array<int, 4>{4, 3, 2, 1}))>;
using gp_probe_157 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).insert(3)))>&>()).second))>;
using gp_probe_158 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).insert(4)))>&>()).first))>;
using gp_probe_159 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).insert(4)))>&>()).second))>;
using gp_probe_198 = gocpp_probe_internal_c<decltype(((*gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).lower_bound(2))))>;
using gp_probe_199 = gocpp_probe_internal_c<decltype(((*gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::set<int64_t>{5, 1, 3}))>&>()).insert(4)))>&>()).first)))>;
using gp_probe_216 = gocpp_probe_internal_c<::std::string>;
using gp_probe_218 = gocpp_probe_internal_c<::std::stringstream>;
