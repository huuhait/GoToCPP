// gocpp/shims.hpp: the native functions of the small internal packages that are gocpp's own
// (Phase 09 step 9.2): internal/godebug and runtime/cgo. Their .gp files call these through
// //gocpp:native (ADR-027).
#pragma once

#include <gocpp/iface.hpp>
#include <gocpp/string.hpp>

#include <cstdint>

namespace gp::godebug {

// env(): the value of the GODEBUG environment variable now, "" when it is not set.
gp::string env();

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
