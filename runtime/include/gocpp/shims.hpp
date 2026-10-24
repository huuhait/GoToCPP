// gocpp/shims.hpp: the native functions of the small internal packages that are gocpp's own
// (Phase 09 step 9.2): internal/godebug and runtime/cgo. Their .gp files call these through
// //gocpp:native (ADR-027).
#pragma once

#include <gocpp/iface.hpp>
#include <gocpp/string.hpp>

#include <cstdint>
#include <string_view>

namespace gp::godebug {

// env(): the value of the GODEBUG environment variable, "" when it is not set: the one the program
// started with, then the last one that package syscall set (env_changed). It never calls getenv.
gp::string env();

// generation(): a number that changes when the program sets or removes GODEBUG through package
// syscall (os.Setenv, Unsetenv, Clearenv: env_changed, after the change), so that a Setting that
// read env() at one generation can keep its value until the next (Go's runtime tells godebug about
// a new $GODEBUG the same way: runtime/env_posix.go, godebugNotify). A change that C or C++ code
// makes with setenv is not seen, as in Go.
std::uint64_t generation() noexcept;
void env_changed(std::string_view value); // "" for an unset variable

} // namespace gp::godebug

namespace gp::cgo {

// The handle table of runtime/cgo (cgo.Handle): handle_new stores v and returns its handle (never
// 0, counting up from 1), handle_value gives the value back, and handle_delete removes it. The last
// two panic with "runtime/cgo: misuse of an invalid Handle" for a handle that is not in the table;
// handle_new panics with "runtime/cgo: ran out of handle space" when the counter wraps.
uintptr_t handle_new(const gp::any& v);
gp::any handle_value(uintptr_t h);
void handle_delete(uintptr_t h);

} // namespace gp::cgo
