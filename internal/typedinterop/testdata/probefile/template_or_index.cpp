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
#include "./cvals2.h"
#include <limits>
#include <memory>
#include <string>
#include <tuple>
#include <type_traits>
#include <utility>
#include <variant>
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
using gp_probe_0 = gocpp_probe_internal_c<decltype((::arr))>;
using gp_probe_2 = gocpp_probe_internal_c<decltype((::table))>;
using gp_probe_4 = gocpp_probe_internal_c<double>;
using gp_probe_5 = gocpp_probe_internal_c<int>;
using gp_probe_6 = gocpp_probe_internal_c<long>;
using gp_probe_7 = gocpp_probe_internal_c<short>;
using gp_probe_8 = gocpp_probe_internal_c<unsigned int>;
using gp_probe_9 = gocpp_probe_internal_c<decltype(((std::declval<std::decay_t<decltype((::std::make_unique<::std::string>("hello")))>&>() != nullptr)))>;
using gp_probe_10 = gocpp_probe_internal_c<decltype(((::std::get<0>(std::declval<std::decay_t<decltype((::std::make_tuple(int(1), 2.5, ::std::string("three"))))>&>()) + 10)))>;
using gp_probe_11 = gocpp_probe_internal_c<decltype((((((*std::declval<std::decay_t<decltype((::std::make_unique<::std::string>("hello")))>&>())))[0] == 'h')))>;
using gp_probe_12 = gocpp_probe_internal_c<decltype((int(1)))>;
using gp_probe_13 = gocpp_probe_internal_c<decltype((bool(::std::is_same_v<int, int>)))>;
using gp_probe_14 = gocpp_probe_internal_c<decltype((bool(::std::is_same_v<int, long>)))>;
using gp_probe_15 = gocpp_probe_internal_c<decltype((bool(::std::numeric_limits<unsigned int>::is_signed)))>;
using gp_probe_16 = gocpp_probe_internal_c<decltype((double(::std::get<1>(std::declval<std::decay_t<decltype((::std::make_tuple(int(1), 2.5, ::std::string("three"))))>&>()))))>;
using gp_probe_17 = gocpp_probe_internal_c<decltype((int64_t(std::declval<std::decay_t<decltype(((::std::get<0>(std::declval<std::decay_t<decltype((::std::make_tuple(int(1), 2.5, ::std::string("three"))))>&>()) + 10)))>&>())))>;
using gp_probe_18 = gocpp_probe_internal_c<decltype((int64_t(::times<3>(4))))>;
using gp_probe_19 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::make_shared<int>(41)))>&>()).use_count())))>;
using gp_probe_20 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(std::declval<::std::variant<int, ::std::string>&>()).index())))>;
using gp_probe_21 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(::std::get<1>(std::declval<::std::variant<int, ::std::string>&>())).size())))>;
using gp_probe_22 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(::std::get<2>(std::declval<std::decay_t<decltype((::std::make_tuple(int(1), 2.5, ::std::string("three"))))>&>())).size())))>;
using gp_probe_23 = gocpp_probe_internal_c<decltype((int64_t(::std::numeric_limits<short>::max())))>;
using gp_probe_23_alt = gocpp_probe_internal_c<decltype((int64_t(std::type_identity_t<typename ::std::numeric_limits<short>::max>())))>;
using gp_probe_24 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref((::arr)).size())))>;
using gp_probe_25 = gocpp_probe_internal_c<decltype((int64_t(gocpp_probe_internal_pr::deref(((*std::declval<std::decay_t<decltype((::std::make_unique<::std::string>("hello")))>&>()))).size())))>;
using gp_probe_26 = gocpp_probe_internal_c<decltype((int64_t((::arr)[0])))>;
using gp_probe_27 = gocpp_probe_internal_c<decltype((int64_t(::size_of_v<double>)))>;
using gp_probe_28 = gocpp_probe_internal_c<decltype((int64_t(::size_of_v<int>)))>;
using gp_probe_29 = gocpp_probe_internal_c<decltype((int64_t((::table)[0])))>;
using gp_probe_30 = gocpp_probe_internal_c<decltype((int64_t((::table)[1])))>;
using gp_probe_31 = gocpp_probe_internal_c<decltype((int64_t(((::arr))[2])))>;
using gp_probe_32 = gocpp_probe_internal_c<decltype((int64_t(((::table))[3])))>;
using gp_probe_33 = gocpp_probe_internal_c<decltype((int64_t((*std::declval<std::decay_t<decltype((::std::make_shared<int>(41)))>&>()))))>;
using gp_probe_34 = gocpp_probe_internal_c<decltype((::times<3>(4)))>;
using gp_probe_35 = gocpp_probe_internal_c<decltype((::std::get<0>(std::declval<std::decay_t<decltype((::std::make_tuple(int(1), 2.5, ::std::string("three"))))>&>())))>;
using gp_probe_36 = gocpp_probe_internal_c<decltype((::std::get<1>(std::declval<std::decay_t<decltype((::std::make_tuple(int(1), 2.5, ::std::string("three"))))>&>())))>;
using gp_probe_37 = gocpp_probe_internal_c<decltype((::std::get<1>(std::declval<::std::variant<int, ::std::string>&>())))>;
using gp_probe_38 = gocpp_probe_internal_c<decltype((::std::get<2>(std::declval<std::decay_t<decltype((::std::make_tuple(int(1), 2.5, ::std::string("three"))))>&>())))>;
using gp_probe_39 = gocpp_probe_internal_c<decltype((::std::make_shared<int>(41)))>;
using gp_probe_40 = gocpp_probe_internal_c<decltype((::std::make_unique<::std::string>("hello")))>;
using gp_probe_41 = gocpp_probe_internal_c<decltype((::std::variant<int, ::std::string>(::std::in_place_index<1>, "in place")))>;
using gp_probe_42 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::std::make_shared<int>(41)))>&>()).use_count()))>;
using gp_probe_43 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<::std::variant<int, ::std::string>&>()).index()))>;
using gp_probe_44 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(::std::get<1>(std::declval<::std::variant<int, ::std::string>&>())).size()))>;
using gp_probe_45 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(::std::get<2>(std::declval<std::decay_t<decltype((::std::make_tuple(int(1), 2.5, ::std::string("three"))))>&>())).size()))>;
using gp_probe_46 = gocpp_probe_internal_c<decltype((::std::numeric_limits<short>::max()))>;
using gp_probe_46_alt = gocpp_probe_internal_c<decltype((std::type_identity_t<typename ::std::numeric_limits<short>::max>()))>;
using gp_probe_47 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref((::arr)).size()))>;
using gp_probe_48 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(((*std::declval<std::decay_t<decltype((::std::make_unique<::std::string>("hello")))>&>()))).append("!")))>;
using gp_probe_49 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(((*std::declval<std::decay_t<decltype((::std::make_unique<::std::string>("hello")))>&>()))).size()))>;
using gp_probe_50 = gocpp_probe_internal_c<decltype((::std::make_tuple(int(1), 2.5, ::std::string("three"))))>;
using gp_probe_51 = gocpp_probe_internal_c<decltype((::std::string("three")))>;
using gp_probe_52 = gocpp_probe_internal_c<std::decay_t<decltype(((::std::get<0>(std::declval<std::decay_t<decltype((::std::make_tuple(int(1), 2.5, ::std::string("three"))))>&>()) + 10)))>>;
using gp_probe_53 = gocpp_probe_internal_c<std::decay_t<decltype((::std::make_shared<int>(41)))>>;
using gp_probe_54 = gocpp_probe_internal_c<std::decay_t<decltype((::std::make_unique<::std::string>("hello")))>>;
using gp_probe_55 = gocpp_probe_internal_c<std::decay_t<decltype((::std::make_tuple(int(1), 2.5, ::std::string("three"))))>>;
using gp_probe_56 = gocpp_probe_internal_c<decltype(((::arr)[0]))>;
using gp_probe_57 = gocpp_probe_internal_c<decltype((::size_of_v<double>))>;
using gp_probe_58 = gocpp_probe_internal_c<decltype((::size_of_v<int>))>;
using gp_probe_59 = gocpp_probe_internal_c<decltype(((::table)[0]))>;
using gp_probe_60 = gocpp_probe_internal_c<decltype(((::table)[1]))>;
using gp_probe_62 = gocpp_probe_internal_c<decltype((((::arr))[2]))>;
using gp_probe_63 = gocpp_probe_internal_c<decltype((((::table))[3]))>;
using gp_probe_64 = gocpp_probe_internal_c<decltype(((((*std::declval<std::decay_t<decltype((::std::make_unique<::std::string>("hello")))>&>())))[0]))>;
using gp_probe_68 = gocpp_probe_internal_c<decltype((::std::in_place_index<1>))>;
using gp_probe_71 = gocpp_probe_internal_c<::std::numeric_limits<short>>;
using gp_probe_72 = gocpp_probe_internal_c<::std::numeric_limits<unsigned int>>;
using gp_probe_73 = gocpp_probe_internal_c<decltype((::std::is_same_v<int, int>))>;
using gp_probe_74 = gocpp_probe_internal_c<decltype((::std::is_same_v<int, long>))>;
using gp_probe_75 = gocpp_probe_internal_c<::std::variant<int, ::std::string>>;
using gp_probe_76 = gocpp_probe_internal_c<decltype(((::arr)))>;
using gp_probe_77 = gocpp_probe_internal_c<decltype(((::table)))>;
using gp_probe_78 = gocpp_probe_internal_c<decltype((((*std::declval<std::decay_t<decltype((::std::make_unique<::std::string>("hello")))>&>()))))>;
using gp_probe_84 = gocpp_probe_internal_c<decltype((::std::numeric_limits<unsigned int>::is_signed))>;
using gp_probe_84_alt = gocpp_probe_internal_c<typename ::std::numeric_limits<unsigned int>::is_signed>;
using gp_probe_88 = gocpp_probe_internal_c<decltype(((*std::declval<std::decay_t<decltype((::std::make_shared<int>(41)))>&>())))>;
using gp_probe_89 = gocpp_probe_internal_c<decltype(((*std::declval<std::decay_t<decltype((::std::make_unique<::std::string>("hello")))>&>())))>;
using gp_probe_97 = gocpp_probe_internal_c<::std::string>;
