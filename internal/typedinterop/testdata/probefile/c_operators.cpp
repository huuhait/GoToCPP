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
#include <cmath>
#include <vector>
namespace gocpp_probe_internal_pr {
struct string {
  string() = default;
  template <size_t N> string(const char (&)[N]) noexcept;  // a literal copies (Phase 04, step 4.9)
  explicit string(std::string_view) noexcept;  // string(x) of a std::string or a std::string_view copies
  operator std::string_view() const noexcept;
  std::strong_ordering operator<=>(const string&) const noexcept;
  bool operator==(const string&) const noexcept;
  size_t size() const noexcept;
  const char* data() const noexcept;
  const uint8_t& operator[](size_t) const noexcept;
};
template <class T> struct slice {
  using value_type = T;
  slice() = default;
  slice(std::initializer_list<T>) noexcept;
  operator std::span<T>() const noexcept;
  T* begin() const noexcept;
  T* end() const noexcept;
  T* data() const noexcept;
  size_t size() const noexcept;
  T& operator[](size_t) const noexcept;
};
template <class T, size_t N> struct array {
  using value_type = T;
  array() = default;
  array(std::initializer_list<T>) noexcept;
  T* begin() const noexcept;
  T* end() const noexcept;
  T* data() const noexcept;
  constexpr size_t size() const noexcept { return N; }
  T& operator[](size_t) const noexcept;
};
template <class K, class V> struct map {
  using key_type = K;
  using mapped_type = V;
  map() = default;
  map(std::initializer_list<std::pair<const K, V>>) noexcept;
  size_t size() const noexcept;
  V& operator[](const K&) const noexcept;
};
template <class T> constexpr decltype(auto) deref(T&& t) {
  if constexpr (std::is_pointer_v<std::remove_cvref_t<T>>) return (*t);
  else return std::forward<T>(t);
}
using std::begin;  // begin(c) finds std::begin and the begin of c's own namespace (ADL)
template <class C, class = void> struct range_t {
  using key = int64_t;
  using val = std::decay_t<decltype(*begin(std::declval<C&>()))>;
};
template <class C> struct range_t<C, std::void_t<typename C::mapped_type>> {
  using key = typename C::key_type;
  using val = typename C::mapped_type;
};
void not_exact();  // not constexpr: calling it makes the constant evaluation fail
template <class T> concept arith = std::is_arithmetic_v<T> || std::is_enum_v<T> ||
    std::is_same_v<T, __int128> || std::is_same_v<T, unsigned __int128>;
template <class T> requires arith<std::remove_cvref_t<T>> constexpr __int128 enc(const T& v) {
  using U = std::remove_cvref_t<T>;
  if constexpr (std::is_floating_point_v<U>) {
    // A float is read as the bits of a double: a wider type (long double) must round-trip,
    // else the evaluation fails (no constant) instead of giving a rounded value.
    if constexpr (sizeof(U) > sizeof(double)) { if (!((U)(double)v == v)) not_exact(); }
    return (__int128)std::bit_cast<uint64_t>((double)v);
  } else if constexpr (std::is_same_v<U, unsigned __int128>) {
    if (v > (unsigned __int128)std::numeric_limits<__int128>::max()) not_exact();  // above 2^127-1: not exact in __int128
    return (__int128)v;
  } else return (__int128)v;
}
template <size_t N> constexpr const char* str_of(const char (&a)[N]) { return a; }
template <class T> constexpr int str_of(const T&) { return 0; }
}
template <auto V> struct gocpp_probe_internal_val {};
template <class T> struct gocpp_probe_internal_c {};
using gp_probe_2 = gocpp_probe_internal_c<int>;
using gp_probe_3 = gocpp_probe_internal_c<decltype(((std::declval<int&>() != 7)))>;
using gp_probe_4 = gocpp_probe_internal_c<decltype(((std::declval<std::decay_t<decltype(((std::declval<int&>() * 3)))>&>() != 0)))>;
using gp_probe_5 = gocpp_probe_internal_c<decltype(((std::declval<std::decay_t<decltype(((std::declval<int&>() * 3)))>&>() % 4)))>;
using gp_probe_6 = gocpp_probe_internal_c<decltype(((std::declval<bool&>() && gocpp_probe_internal_pr::deref(std::declval<::std::vector<int>&>()).empty())))>;
using gp_probe_7 = gocpp_probe_internal_c<decltype(((std::declval<int&>() & ~3)))>;
using gp_probe_8 = gocpp_probe_internal_c<decltype(((std::declval<int&>() & 3)))>;
using gp_probe_9 = gocpp_probe_internal_c<decltype(((std::declval<int&>() * 3)))>;
using gp_probe_10 = gocpp_probe_internal_c<decltype(((std::declval<std::decay_t<decltype(((::sqrt(2.0) * 2)))>&>() * 1000)))>;
using gp_probe_11 = gocpp_probe_internal_c<decltype(((::sqrt(2.0) * 2)))>;
using gp_probe_12 = gocpp_probe_internal_c<decltype(((std::declval<int&>() + 1)))>;
using gp_probe_13 = gocpp_probe_internal_c<decltype(((::floor(std::declval<std::decay_t<decltype(((::sqrt(2.0) * 2)))>&>()) + 0.5)))>;
using gp_probe_14 = gocpp_probe_internal_c<decltype(((std::declval<std::decay_t<decltype(((std::declval<int&>() * 3)))>&>() - std::declval<int&>())))>;
using gp_probe_15 = gocpp_probe_internal_c<decltype(((std::declval<std::decay_t<decltype(((std::declval<int&>() * 3)))>&>() / 2)))>;
using gp_probe_16 = gocpp_probe_internal_c<decltype(((std::declval<int&>() << std::declval<int64_t&>())))>;
using gp_probe_17 = gocpp_probe_internal_c<decltype(((std::declval<int&>() << 2)))>;
using gp_probe_18 = gocpp_probe_internal_c<decltype(((std::declval<int&>() <= 7)))>;
using gp_probe_19 = gocpp_probe_internal_c<decltype(((std::declval<int&>() < std::declval<std::decay_t<decltype(((std::declval<int&>() * 3)))>&>())))>;
using gp_probe_20 = gocpp_probe_internal_c<decltype(((std::declval<int&>() == 7)))>;
using gp_probe_21 = gocpp_probe_internal_c<decltype(((std::declval<std::decay_t<decltype(((std::declval<int&>() * 3)))>&>() >= 22)))>;
using gp_probe_22 = gocpp_probe_internal_c<decltype(((std::declval<int&>() >> std::declval<int64_t&>())))>;
using gp_probe_23 = gocpp_probe_internal_c<decltype(((std::declval<std::decay_t<decltype(((std::declval<int&>() * 3)))>&>() >> 1)))>;
using gp_probe_24 = gocpp_probe_internal_c<decltype(((std::declval<int&>() > 3)))>;
using gp_probe_25 = gocpp_probe_internal_c<decltype(((std::declval<std::decay_t<decltype(((std::declval<int&>() * 3)))>&>() > 20)))>;
using gp_probe_26 = gocpp_probe_internal_c<decltype(((gocpp_probe_internal_pr::deref(std::declval<::std::vector<int>&>()).size() > 0)))>;
using gp_probe_27 = gocpp_probe_internal_c<decltype(((std::declval<int&>() ^ 5)))>;
using gp_probe_28 = gocpp_probe_internal_c<decltype(((std::declval<int&>() | 8)))>;
using gp_probe_29 = gocpp_probe_internal_c<decltype((((!std::declval<std::decay_t<decltype(((!gocpp_probe_internal_pr::deref(std::declval<::std::vector<int>&>()).empty())))>&>()) || std::declval<bool>())))>;
using gp_probe_30 = gocpp_probe_internal_c<decltype((::floor(std::declval<std::decay_t<decltype(((::sqrt(2.0) * 2)))>&>())))>;
using gp_probe_31 = gocpp_probe_internal_c<decltype((::sqrt(2.0)))>;
using gp_probe_32 = gocpp_probe_internal_c<decltype((int(std::declval<int64_t&>())))>;
using gp_probe_33 = gocpp_probe_internal_c<decltype((int64_t(std::declval<int&>())))>;
using gp_probe_34 = gocpp_probe_internal_c<decltype((int64_t((std::declval<std::decay_t<decltype(((std::declval<int&>() * 3)))>&>() % 4))))>;
using gp_probe_35 = gocpp_probe_internal_c<decltype((int64_t((std::declval<int&>() & ~3))))>;
using gp_probe_36 = gocpp_probe_internal_c<decltype((int64_t((std::declval<int&>() & 3))))>;
using gp_probe_37 = gocpp_probe_internal_c<decltype((int64_t((std::declval<std::decay_t<decltype(((::sqrt(2.0) * 2)))>&>() * 1000))))>;
using gp_probe_38 = gocpp_probe_internal_c<decltype((int64_t((std::declval<int&>() + 1))))>;
using gp_probe_39 = gocpp_probe_internal_c<decltype((int64_t((::floor(std::declval<std::decay_t<decltype(((::sqrt(2.0) * 2)))>&>()) + 0.5))))>;
using gp_probe_40 = gocpp_probe_internal_c<decltype((int64_t((std::declval<std::decay_t<decltype(((std::declval<int&>() * 3)))>&>() - std::declval<int&>()))))>;
using gp_probe_41 = gocpp_probe_internal_c<decltype((int64_t((std::declval<std::decay_t<decltype(((std::declval<int&>() * 3)))>&>() / 2))))>;
using gp_probe_42 = gocpp_probe_internal_c<decltype((int64_t((std::declval<int&>() << std::declval<int64_t&>()))))>;
using gp_probe_43 = gocpp_probe_internal_c<decltype((int64_t((std::declval<int&>() << 2))))>;
using gp_probe_44 = gocpp_probe_internal_c<decltype((int64_t((std::declval<int&>() >> std::declval<int64_t&>()))))>;
using gp_probe_45 = gocpp_probe_internal_c<decltype((int64_t((std::declval<std::decay_t<decltype(((std::declval<int&>() * 3)))>&>() >> 1))))>;
using gp_probe_46 = gocpp_probe_internal_c<decltype((int64_t((std::declval<int&>() ^ 5))))>;
using gp_probe_47 = gocpp_probe_internal_c<decltype((int64_t((std::declval<int&>() | 8))))>;
using gp_probe_48 = gocpp_probe_internal_c<decltype((int64_t(::std::max(std::declval<int64_t&>(), std::declval<int64_t>()))))>;
using gp_probe_49 = gocpp_probe_internal_c<decltype((int64_t(::std::min(int(std::declval<int64_t&>()), std::declval<int&>()))))>;
using gp_probe_50 = gocpp_probe_internal_c<decltype((int64_t((+std::declval<int&>()))))>;
using gp_probe_51 = gocpp_probe_internal_c<decltype((int64_t((-std::declval<int&>()))))>;
using gp_probe_52 = gocpp_probe_internal_c<decltype((int64_t((~std::declval<int&>()))))>;
using gp_probe_53 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::vector<int>&>()).empty()))>;
using gp_probe_54 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::vector<int>&>()).pop_back()))>;
using gp_probe_55 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::vector<int>&>()).push_back(std::declval<int&>())))>;
using gp_probe_56 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::vector<int>&>()).size()))>;
using gp_probe_57 = gocpp_probe_internal_c<decltype((::std::max(std::declval<int64_t&>(), std::declval<int64_t>())))>;
using gp_probe_58 = gocpp_probe_internal_c<decltype((::std::min(int(std::declval<int64_t&>()), std::declval<int&>())))>;
using gp_probe_59 = gocpp_probe_internal_c<std::decay_t<decltype(((std::declval<int&>() * 3)))>>;
using gp_probe_60 = gocpp_probe_internal_c<std::decay_t<decltype(((::sqrt(2.0) * 2)))>>;
using gp_probe_61 = gocpp_probe_internal_c<std::decay_t<decltype(((!gocpp_probe_internal_pr::deref(std::declval<::std::vector<int>&>()).empty())))>>;
using gp_probe_62 = gocpp_probe_internal_c<::std::vector<int>>;
using gp_probe_70 = gocpp_probe_internal_c<decltype(((!std::declval<std::decay_t<decltype(((!gocpp_probe_internal_pr::deref(std::declval<::std::vector<int>&>()).empty())))>&>())))>;
using gp_probe_71 = gocpp_probe_internal_c<decltype(((!gocpp_probe_internal_pr::deref(std::declval<::std::vector<int>&>()).empty())))>;
using gp_probe_72 = gocpp_probe_internal_c<decltype(((+std::declval<int&>())))>;
using gp_probe_73 = gocpp_probe_internal_c<decltype(((-std::declval<int&>())))>;
using gp_probe_74 = gocpp_probe_internal_c<decltype(((~std::declval<int&>())))>;
