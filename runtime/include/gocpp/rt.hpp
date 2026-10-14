// gocpp/rt.hpp: the gocpp runtime. Generated code includes this header and nothing else of
// the runtime: #include <gocpp/rt.hpp>.
//
// Everything is in namespace gp. The runtime's .cpp files (runtime/src/*.cpp and the OS layer
// in runtime/src/os/<os>/) are compiled with the program and linked into it.
#pragma once

// First, before any system header: at -O0, this turns off the _FORTIFY_SOURCE that a Nix
// compiler wrapper adds (see the file).
#include <gocpp/nofortify.h>

#include <gocpp/checks.hpp>
#include <gocpp/string.hpp>
#include <gocpp/arith.hpp>
#include <gocpp/print.hpp>
#include <gocpp/panic.hpp>
#include <gocpp/fmtshim.hpp>
#include <gocpp/main.hpp>

// The standard headers that generated code uses directly: std::tuple for multiple results,
// INT64_C and the fixed-width integer types.
#include <cstdint>
#include <tuple>
