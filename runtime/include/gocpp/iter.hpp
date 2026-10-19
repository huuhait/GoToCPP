// gocpp/iter.hpp: the coroutines of package iter (Phase 07 step 7.10). Go's iter.Pull and Pull2
// run the push iterator on a coroutine that the runtime provides (runtime.newcoro and
// runtime.coroswitch, linked into package iter with //go:linkname). gocpp's port of iter
// (stdlib/iter, written by tools/portstd) calls these two natives instead, through
// //gocpp:native (ADR-027), with the coroutine handle as a uintptr:
//
//     c := newcoro(func(c uintptr) { ... })       gp::iter::newcoro(f)
//     coroswitch(c)                               gp::iter::coroswitch(c)
//
// A goroutine is an OS thread (ADR-007), so the coroutine is a goroutine of its own that never
// runs at the same time as the goroutine that switches to it: newcoro starts it parked (it counts
// in runtime.NumGoroutine at once, as Go's does), coroswitch(c) hands control to the other side
// and parks the caller until the other side switches back (the same function, called from the
// coroutine, goes back to whoever switched in last, so the iterator may be driven from different
// goroutines one after the other). When f returns (or calls runtime.Goexit) the coroutine ends,
// the goroutine count goes down, and control goes back to the goroutine that switched in last, as
// Go's coroexit does. Both sides count as blocked ("coroutine") while they wait, so the deadlock
// detector sees a program in which every goroutine waits, as Go's does.
//
// Nothing here unwinds or recovers: a panic or Goexit in f is f's business (Pull's deferred
// function records it and runs it again on the caller's side). A panic that leaves f is a panic
// of a goroutine, which ends the program, as in Go. A coroutine that is never switched to again
// (its iterator was dropped without Pull's stop) waits for ever, as in Go, and its goroutine
// stays counted. Go says the same: "stop must be called".
#pragma once

#include <gocpp/func.hpp>

#include <cstdint>

namespace gp::iter {

// newcoro(f): a coroutine that will run f(handle) on its first coroswitch, where handle is the
// value that newcoro returned. The handle is valid until f returns; Pull never switches to a
// coroutine whose function has returned.
std::uint64_t newcoro(const gp::func<void(std::uint64_t)>& f);

// coroswitch(c): hand control to the other side of c and wait until it hands it back (or, when the
// other side's function returns, until it is over).
void coroswitch(std::uint64_t c);

} // namespace gp::iter
