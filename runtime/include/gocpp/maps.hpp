// gocpp/maps.hpp: the native function of the standard package maps (Phase 07 step 7.10). Go's
// maps.Clone calls the runtime's map copy through //go:linkname (maps.clone); the port
// (stdlib/maps, tools/portstd) declares it without a body and with //gocpp:native (ADR-027), and
// the call becomes
//
//     clone(m any) any                    gp::maps::clone(m)
//
// m holds a map (Clone has already returned for nil), of the type M that Clone asserts the result
// to: the copy has the same dynamic type. It is gp::map_clone (gocpp/map.hpp) through the map
// type's descriptor (type_ops::gp_map_clone, gocpp/typeops.hpp). A map type whose key or value
// can't be copied (a C++ value that is move-only) can't be cloned: a panic.
#pragma once

#include <gocpp/iface.hpp>

namespace gp::maps {

gp::any clone(const gp::any& m);

} // namespace gp::maps
