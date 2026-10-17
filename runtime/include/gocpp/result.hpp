// gocpp/result.hpp: gp::result_slot<R>, the variable that holds a result of a function in the
// single-exit shape (Phase 04 step 4.8; Phase 05 §2.7 uses it for every result).
//
// A function with a defer statement stores the value of each `return` in a result variable and
// jumps to the exit, where the deferred calls run and the function returns the variable
// (defer.hpp). For a gocpp type the variable is `T gp_r0{};`. A C/C++ type can have no default
// constructor (a class whose only constructors take arguments), no assignment operator (a
// class with const or reference members) or be a reference, so the variable is a result_slot,
// whose storage is chosen where the C++ is compiled:
//
//     R is a reference, T& or T&&       a pointer to the object; gp_take() gives the reference back
//     R can be default-constructed      an R, zero-valued from the start as Go's result variable
//       and assigned                      is; gp_set(v) assigns it, gp_take() moves it out
//     any other R (no default           a buffer and a flag: gp_set(v) constructs the object
//       constructor, not assignable)      there, gp_take() moves it out and ends it
//
//     gp::result_slot<R> gp_r0;         // the result
//     ...
//     gp_r0 = value;  (gp_r0.gp_set(value))      return value; in the body
//     return gp_r0.gp_take();                     at the exit
//
// A result type that is neither movable nor a reference can't be returned from a variable at
// all (C++ returns it only from the expression that makes it), so it is a clear static_assert
// error in gp_set, at the .gp line of the return statement, in a function with defers. A result
// that was never set, in the third storage (a function that recovers a panic returns the zero
// values of its results in Go, which a type without a default constructor doesn't have), is a fatal
// error, not an access to a missing object. Like every runtime type, a result_slot has only
// operators and gp_-prefixed members.
#pragma once

#include <gocpp/panic.hpp>

#include <memory>
#include <new>
#include <string_view>
#include <type_traits>
#include <utility>

namespace gp {

namespace detail {

// slot_inline<R>: R is kept in the slot itself, as a zero value from the start.
template <typename R>
inline constexpr bool slot_inline =
    std::is_default_constructible_v<R> && std::is_move_assignable_v<R> && !std::is_const_v<R> &&
    !std::is_volatile_v<R>;

// unmovable_result ends the program where a result of a type that can be neither moved nor copied
// is taken. The static_assert in gp_set is the error that the program gets; this only gives
// gp_take something to return where no gp_set reports it (a function that always panics).
[[noreturn]] inline void unmovable_result() {
    fatal("a result of a type that can be neither moved nor copied was never set");
}

} // namespace detail

template <typename R, typename = void>
class result_slot;

// A reference.
template <typename R>
class result_slot<R, std::enable_if_t<std::is_reference_v<R>>> {
public:
    using gp_pointee = std::remove_reference_t<R>;

    result_slot() noexcept = default;
    result_slot(const result_slot&) = delete;
    result_slot& operator=(const result_slot&) = delete;

    // The result refers to v. A reference result that was never set is a fatal error.
    template <typename U>
    void gp_set(U&& v) noexcept {
        static_assert(std::is_convertible_v<U&&, R>,
                      "gocpp: the result doesn't convert to the reference type of the result");
        gp_p_ = std::addressof(static_cast<R>(std::forward<U>(v)));
    }
    template <typename U>
    result_slot& operator=(U&& v) noexcept {
        gp_set(std::forward<U>(v));
        return *this;
    }
    R gp_take() const {
        if (gp_p_ == nullptr) [[unlikely]] {
            detail::fatal("a result of a reference type was never set");
        }
        return static_cast<R>(*gp_p_);
    }

private:
    gp_pointee* gp_p_ = nullptr;
};

// A value that is kept in the slot, as a zero value from the start.
template <typename R>
class result_slot<R, std::enable_if_t<!std::is_reference_v<R> && detail::slot_inline<R>>> {
public:
    result_slot() = default;
    result_slot(const result_slot&) = delete;
    result_slot& operator=(const result_slot&) = delete;

    template <typename U>
    void gp_set(U&& v) {
        static_assert(std::is_move_constructible_v<R> || std::is_copy_constructible_v<R>,
                      "gocpp: the C++ type of this result can't be moved or copied, so a "
                      "function with defer can't return it (C++ returns such a type only from "
                      "the expression that makes it): remove the defer, or return a pointer");
        gp_v_ = R(std::forward<U>(v));
    }
    template <typename U>
    result_slot& operator=(U&& v) {
        gp_set(std::forward<U>(v));
        return *this;
    }
    R gp_take() {
        if constexpr (std::is_move_constructible_v<R> || std::is_copy_constructible_v<R>) {
            return std::move(gp_v_);
        } else {
            detail::unmovable_result(); // (gp_set has reported it)
        }
    }

private:
    R gp_v_{};
};

// Any other value: a buffer and a flag.
template <typename R>
class result_slot<R, std::enable_if_t<!std::is_reference_v<R> && !detail::slot_inline<R>>> {
public:
    result_slot() noexcept = default;
    result_slot(const result_slot&) = delete;
    result_slot& operator=(const result_slot&) = delete;
    ~result_slot() { gp_end(); }

    template <typename U>
    void gp_set(U&& v) {
        static_assert(std::is_move_constructible_v<R> || std::is_copy_constructible_v<R>,
                      "gocpp: the C++ type of this result can't be moved or copied, so a "
                      "function with defer can't return it (C++ returns such a type only from "
                      "the expression that makes it): remove the defer, or return a pointer");
        gp_end();
        ::new (static_cast<void*>(gp_buf_)) R(std::forward<U>(v));
        gp_has_ = true;
    }
    template <typename U>
    result_slot& operator=(U&& v) {
        gp_set(std::forward<U>(v));
        return *this;
    }
    R gp_take() {
        if constexpr (std::is_move_constructible_v<R> || std::is_copy_constructible_v<R>) {
            if (!gp_has_) [[unlikely]] {
                detail::fatal("a result of a type without a default constructor was never set");
            }
            R* p = std::launder(reinterpret_cast<R*>(gp_buf_));
            R out(std::move(*p));
            gp_end();
            return out;
        } else {
            detail::unmovable_result(); // (gp_set has reported it)
        }
    }

private:
    void gp_end() noexcept {
        if (gp_has_) {
            std::launder(reinterpret_cast<R*>(gp_buf_))->~R();
            gp_has_ = false;
        }
    }

    alignas(R) unsigned char gp_buf_[sizeof(R)];
    bool gp_has_ = false;
};

} // namespace gp
