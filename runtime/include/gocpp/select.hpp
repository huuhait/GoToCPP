// gocpp/select.hpp: Go's select statement (Phase 08, step 8.5).
//
// A select is lowered to a few named objects, one for each case, then one call that chooses and
// completes a case, then a C++ switch on the number of the case it returns:
//
// clang-format off
//     select {                                  {
//     case v := <-a:                                gp::sel_recv gp_s0{a};
//         use(v)                                    gp::sel_send gp_s1{b, f()};
//     case b <- f():                                gp::sel_recv gp_s2{c, gp::sel_drop};
//     case <-c:                                     switch (gp::select(gp_s0, gp_s1, gp_s2, gp::sel_default)) {
//     default:                                      case 0: { int64_t v = gp_s0.gp_take(); use(v); } break;
//     }                                             case 1: ... break;
//                                                   case 2: ... break;
//                                                   default: ... }
//                                               }
// clang-format on
//
//   - gp::sel_recv gp_s{c} is `case <-c` (and `case v := <-c`, `case v, ok = <-c`, ...): gp_s
//     holds the channel and the place for the value; when the case is chosen, gp_take() moves the
//     value out and gp_ok() says whether it is one (false: the channel was closed and the value is
//     the zero value). gp::sel_recv gp_s{c, gp::sel_drop} is the case whose value is not used
//     (`case <-c`, `case _ = <-c`, `case _, ok = <-c`): nothing is constructed.
//   - gp::sel_send gp_s{c, v} is `case c <- v`: it holds the value, which Go evaluates when the
//     select statement starts, whether or not the case is chosen.
//   - gp::sel_default is `default`. gp::select() with no case is `select {}`: it blocks for ever.
//   - The channel of a case is held by reference when it is a variable that nothing can change
//     during the statement (an lvalue the emitter knows is local), and by a copy of the handle
//     otherwise: the caller passes gp::chan<T>(expr). Go evaluates each channel expression once, in
//     source order, before the select communicates: the objects are declared in source order.
//   - gp::select returns the position of the case, in the order of the arguments, or the position
//     of the default when no case was ready. Which of several ready cases it takes is chosen at
//     random with equal probability (Go's pollorder).
//
// A nil channel is a case that is never ready. A closed channel is ready to receive (the zero
// value, ok false); a send on a closed channel panics ("send on closed channel") when it is chosen
// or reached, as in Go. A case with a send needs T to be movable; a receive from a closed channel
// needs T to have a zero value (T() constructs one), as for gp::recv2.
//
// The algorithm is Go's selectgo (runtime/select.go; src/select.cpp): the cases are polled in a
// random order while all the channels are locked, in the order of their addresses (so two selects
// on the same channels can't deadlock each other, and a channel in two cases is locked once); if
// none is ready and there is no default, the goroutine leaves one entry (a sudog) on each channel
// and parks. All the entries share one claim flag and one wake-up cell: whoever completes a case
// first takes the flag, so exactly one case happens. After the wake-up the goroutine locks the
// channels again and removes its other entries. Two selects (one that sends and one that receives
// on the same channel) can complete each other: the one that arrives second finds the entry of the
// first in the queue and takes its flag. The goroutine counts as blocked (gp::num_blocked) while
// it waits, "select" in the traceback.
//
// For up to 16 cases nothing is allocated.
#pragma once

#include <gocpp/chan.hpp>

#include <cstddef>
#include <cstdint>
#include <new>
#include <type_traits>
#include <utility>

namespace gp {

namespace detail {

// One case of a select, for select_run, which is not a template.
enum class sel_kind : uint8_t { recv, send, dflt };

struct sel_entry {
    chan_core* c = nullptr; // null for the default and for a nil channel (never ready)
    void* elem = nullptr;   // recv: where the value goes (null: dropped); send: the value to send
    bool* ok = nullptr;     // recv: whether a value was received (may be null)
    sel_kind kind = sel_kind::dflt;
};

// select_run chooses a case among cs[0..n) and completes it, as the top of this file says. It
// returns the position of the case (the default's when nothing was ready), or never returns
// (no case, all channels nil, no default). A send case that is completed has moved from its value;
// a receive case has constructed its value. It throws the panic of a send on a closed channel.
int select_run(sel_entry* cs, int n);

[[noreturn]] void select_forever();

// A case holds its channel by reference or by value: chan_ref<T>.
template <typename T>
class chan_ref {
public:
    chan_ref(const chan<T>& c) noexcept : gp_p(&c) {}
    chan_ref(chan<T>&& c) noexcept : gp_own(std::move(c)), gp_p(&gp_own) {}
    chan_ref(const chan_ref&) = delete;
    chan_ref& operator=(const chan_ref&) = delete;
    detail::chan_core* gp_core() const noexcept { return gp_p->gp_core(); }

private:
    chan<T> gp_own;
    const chan<T>* gp_p;
};

} // namespace detail

struct sel_drop_t {
    explicit sel_drop_t() = default;
};
inline constexpr sel_drop_t sel_drop{};

// The default case.
struct sel_default_t {
    detail::sel_entry gp_entry() const noexcept { return {}; }
    void gp_after(bool) const noexcept {}
};
inline constexpr sel_default_t sel_default{};

// sel_recv is `case <-c`: see the top.
template <typename T>
class sel_recv {
public:
    sel_recv(const chan<T>& c) noexcept : gp_c(c) {}
    sel_recv(chan<T>&& c) noexcept : gp_c(std::move(c)) {}
    sel_recv(const chan<T>& c, sel_drop_t) noexcept : gp_c(c), gp_drop(true) {}
    sel_recv(chan<T>&& c, sel_drop_t) noexcept : gp_c(std::move(c)), gp_drop(true) {}
    sel_recv(const sel_recv&) = delete;
    sel_recv& operator=(const sel_recv&) = delete;
    ~sel_recv() {
        if (gp_has) {
            gp_slot.drop();
        }
    }

    // The value, when this case was chosen (once).
    T gp_take() noexcept(std::is_nothrow_move_constructible_v<T>) {
        gp_has = false;
        return gp_slot.take();
    }
    // Whether the channel delivered a value (false: it is closed).
    bool gp_ok() const noexcept { return gp_okay; }

    detail::sel_entry gp_entry() noexcept {
        return {gp_c.gp_core(), gp_drop ? nullptr : gp_slot.addr(), &gp_okay,
                detail::sel_kind::recv};
    }
    void gp_after(bool chosen) noexcept { gp_has = chosen && !gp_drop; }

private:
    detail::chan_ref<T> gp_c;
    detail::elem_slot<T> gp_slot;
    bool gp_okay = false;
    bool gp_has = false;
    bool gp_drop = false;
};

template <typename T>
sel_recv(const chan<T>&) -> sel_recv<T>;
template <typename T>
sel_recv(chan<T>&&) -> sel_recv<T>;
template <typename T>
sel_recv(const chan<T>&, sel_drop_t) -> sel_recv<T>;
template <typename T>
sel_recv(chan<T>&&, sel_drop_t) -> sel_recv<T>;

// sel_send is `case c <- v`: see the top.
template <typename T>
class sel_send {
public:
    sel_send(const chan<T>& c, std::type_identity_t<T> v) : gp_c(c), gp_v(std::move(v)) {}
    sel_send(chan<T>&& c, std::type_identity_t<T> v) : gp_c(std::move(c)), gp_v(std::move(v)) {}
    sel_send(const sel_send&) = delete;
    sel_send& operator=(const sel_send&) = delete;

    detail::sel_entry gp_entry() noexcept {
        return {gp_c.gp_core(), &gp_v, nullptr, detail::sel_kind::send};
    }
    void gp_after(bool) noexcept {}

private:
    detail::chan_ref<T> gp_c;
    T gp_v;
};

template <typename T>
sel_send(const chan<T>&, std::type_identity_t<T>) -> sel_send<T>;
template <typename T>
sel_send(chan<T>&&, std::type_identity_t<T>) -> sel_send<T>;

// select_unreachable follows a select whose cases all jump away (a terminating statement in
// Go), where C++ can't see that the switch has no other value: it is never called.
[[noreturn]] void select_unreachable();

// select is the select statement over the cases (sel_recv, sel_send and at most one sel_default).
// With no case it is `select {}`.
[[noreturn]] inline void select() {
    detail::select_forever();
}

template <typename... Cs>
    requires(sizeof...(Cs) > 0)
int select(Cs&... cs) {
    detail::sel_entry entries[sizeof...(Cs)];
    int k = 0;
    ((entries[k++] = cs.gp_entry()), ...);
    int chosen = detail::select_run(entries, static_cast<int>(sizeof...(Cs)));
    k = 0;
    (cs.gp_after(k++ == chosen), ...);
    return chosen;
}

} // namespace gp
