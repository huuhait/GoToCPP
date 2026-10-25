// gocpp/gc/rt.hpp: the runtime of the GC engine (phase-16 README §1.3, §2.2 and §2.4).
//
// The generated C++ of a .go package includes this header and nothing else of the runtime:
// #include <gocpp/gc/rt.hpp>. Everything is in namespace gp::gc, so that the GC engine's types
// (gp::gc::string) and the reference-counting engine's (gp::string) can live in one program
// without a clash.
//
// This is the walking skeleton of job 16.1c: heap.hpp declares the small gp::gc::heap interface
// over a malloc-backed backend (src/gc/heap_malloc.cpp), string.hpp Go's 16-byte string, and
// print.hpp the builtins print and println. bdwgc (job 16.1d), Go-layout structs, slices, maps,
// interfaces and goroutines (16.5, 16.8) are not here yet.
#pragma once

// First, before any system header: at -O0, this turns off the _FORTIFY_SOURCE that a Nix
// compiler wrapper adds (see the file).
#include <gocpp/nofortify.h>

#include <gocpp/gc/heap.hpp>
#include <gocpp/gc/arith.hpp>
#include <gocpp/gc/string.hpp>
#include <gocpp/gc/print.hpp>

#include <cstdint>
