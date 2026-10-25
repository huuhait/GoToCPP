// gocpp/gc/rt.hpp: the runtime of the GC engine (phase-16 README §1.3, §2.2 and §2.4).
//
// The generated C++ of a .go package includes this header and nothing else of the runtime:
// #include <gocpp/gc/rt.hpp>. Everything is in namespace gp::gc, so that the GC engine's types
// (gp::gc::string) and the reference-counting engine's (gp::string) can live in one program
// without a clash.
//
// What it carries:
//   - heap.hpp: gp::gc::heap, the memory interface over a backend (src/gc/heap_malloc.cpp in
//     job 16.1c, bdwgc in 16.1d), new_, new_atomic_, roots and init; the shared runtime reaches
//     the collector through gocpp/gc/hooks.hpp, which it includes itself (job 16.1d);
//   - golayout.hpp: Go's layout of the engine's types and the struct-layout helpers;
//   - checks.hpp and panic.hpp: Go's bounds, nil and make/append checks and the placeholder
//     panic (step 16.6 replaces it with the shared one);
//   - string.hpp: Go's 16-byte string; arith.hpp: Go's shifts and division; print.hpp: print
//     and println;
//   - slice.hpp, array.hpp, func.hpp, map.hpp and chan.hpp: Go's values (job 16.1e);
//   - iface.hpp: Go's interfaces and the placeholder type descriptors (16.2/16.8).
// Goroutines, defers, channels' operations and the rest of step 16.5 are not here yet.
#pragma once

// First, before any system header: at -O0, this turns off the _FORTIFY_SOURCE that a Nix
// compiler wrapper adds (see the file).
#include <gocpp/nofortify.h>

#include <gocpp/gc/golayout.hpp>
#include <gocpp/gc/heap.hpp>
#include <gocpp/gc/panic.hpp>
#include <gocpp/gc/checks.hpp>
#include <gocpp/gc/arith.hpp>
#include <gocpp/gc/string.hpp>
#include <gocpp/gc/slice.hpp>
#include <gocpp/gc/array.hpp>
#include <gocpp/gc/func.hpp>
#include <gocpp/gc/map.hpp>
#include <gocpp/gc/chan.hpp>
#include <gocpp/gc/iface.hpp>
#include <gocpp/gc/print.hpp>

// Last: gc/sizes.hpp asserts every type above against Go's numbers. It includes this file,
// which is already in progress, so its include is a no-op and the assertions run after all the
// types are defined.
#include <gocpp/gc/sizes.hpp>

#include <cstdint>
