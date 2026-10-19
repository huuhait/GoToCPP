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
#include <cstdio>
#include <stdio.h>
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
using gp_probe_0 = gocpp_probe_internal_c<decltype((EOF))>;
using gp_probe_0_v = gocpp_probe_internal_val<gocpp_probe_internal_pr::enc((EOF))>;
constexpr auto gp_probe_0_s = gocpp_probe_internal_pr::str_of((EOF));
using gp_probe_4 = gocpp_probe_internal_c<decltype((stdout))>;
using gp_probe_4_v = gocpp_probe_internal_val<gocpp_probe_internal_pr::enc((stdout))>;
constexpr auto gp_probe_4_s = gocpp_probe_internal_pr::str_of((stdout));
using gp_probe_5 = gocpp_probe_internal_c<int>;
using gp_probe_8 = gocpp_probe_internal_c<decltype((::fflush(stdout)))>;
using gp_probe_9 = gocpp_probe_internal_c<decltype((::printf("%d %.1f %s %c\012", 5, 2.5, "x", 'z')))>;
using gp_probe_10 = gocpp_probe_internal_c<decltype((::printf("%lld %llu\012", std::type_identity_t<long long>(1099511627776), std::type_identity_t<unsigned long long>(9223372036854775808ULL))))>;
using gp_probe_11 = gocpp_probe_internal_c<decltype((::printf("%lld\012", std::declval<std::decay_t<decltype((std::type_identity_t<long long>(1099511627776)))>&>())))>;
using gp_probe_12 = gocpp_probe_internal_c<decltype((::printf("%s\012", "hello")))>;
using gp_probe_13 = gocpp_probe_internal_c<decltype((::printf("EOF is %d\012", EOF)))>;
using gp_probe_14 = gocpp_probe_internal_c<decltype((::printf("printf wrote %d bytes\012", std::declval<int&>())))>;
using gp_probe_15 = gocpp_probe_internal_c<decltype((::puts("puts")))>;
using gp_probe_16 = gocpp_probe_internal_c<decltype((std::type_identity_t<long long>(1099511627776)))>;
using gp_probe_17 = gocpp_probe_internal_c<decltype((std::type_identity_t<unsigned long long>(9223372036854775808ULL)))>;
using gp_probe_18 = gocpp_probe_internal_c<std::decay_t<decltype((std::type_identity_t<long long>(1099511627776)))>>;
