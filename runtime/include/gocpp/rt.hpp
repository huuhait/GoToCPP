// gocpp/rt.hpp: the gocpp runtime. Generated code includes this header and nothing else of
// the runtime: #include <gocpp/rt.hpp>.
//
// Everything is in namespace gp. The runtime's .cpp files (runtime/src/*.cpp and the OS layer
// in runtime/src/os/<os>/) are compiled with the program and linked into it.
#pragma once

// First, before any system header: at -O0, this turns off the _FORTIFY_SOURCE that a Nix
// compiler wrapper adds (see the file).
#include <gocpp/nofortify.h>

// GP_TRACE_FRAME is in the lambda of a func literal that a go or defer statement runs (internal/
// lower deferValue): in a g++ -release -trace build (GOCPP_TRACE, ADR-136) it keeps the
// lambda out of line. g++ inlines such a lambda into the runtime's code that runs it, and its -g1
// line tables name an inlined lambda without a linkage name, so the trace would have no frame for
// the func literal (main.main.func1); out of line, the symbol table names it. clang's line tables
// have the linkage names (-fdebug-info-for-profiling), and a debug build inlines nothing.
#if defined(GOCPP_TRACE) && defined(__GNUC__) && !defined(__clang__)
#define GP_TRACE_FRAME __attribute__((noinline))
#else
#define GP_TRACE_FRAME
#endif

// GP_NOINLINE is Go's //go:noinline on a function (internal/lower/decl.go funcBuild).
#define GP_NOINLINE __attribute__((noinline))

// GP_RECURSIVE and GP_RECURSION_MARK() are on the functions that can call themselves (directly or
// through other functions of their package: internal/lower/recursion.go), so that a recursion
// without end overflows the stack in an optimized build too, as Go's does ("fatal error: stack
// overflow"), instead of looping or returning. GP_RECURSIVE keeps the function's calls real calls:
// no tail call made a jump, no recursion made a loop (clang's disable_tail_calls; g++'s
// -fno-optimize-sibling-calls for this function only). GP_RECURSION_MARK() is an empty asm
// volatile, which clang must treat as a side effect: without one, C++'s forward-progress rule lets
// it assume that a recursion with no side effect returns, and drop it. It costs no instruction;
// g++ does not drop such a recursion, so it gets nothing. A debug build (-O0) makes no tail call.
// (GNU spellings: they may stand among the decl-specifiers, after inline, which [[...]] may not.)
#if defined(__OPTIMIZE__) && defined(__clang__)
#define GP_RECURSIVE __attribute__((disable_tail_calls))
#define GP_RECURSION_MARK() __asm__ volatile("")
#elif defined(__OPTIMIZE__) && defined(__GNUC__)
#define GP_RECURSIVE __attribute__((optimize("no-optimize-sibling-calls")))
#define GP_RECURSION_MARK() ((void)0)
#else
#define GP_RECURSIVE
#define GP_RECURSION_MARK() ((void)0)
#endif

#include <gocpp/checks.hpp>
#include <gocpp/string.hpp>
#include <gocpp/ptr.hpp>
#include <gocpp/bounds.hpp>
#include <gocpp/slice.hpp>
#include <gocpp/array.hpp>
#include <gocpp/hash.hpp>
#include <gocpp/complex.hpp>
#include <gocpp/map.hpp>
#include <gocpp/func.hpp>
#include <gocpp/lazy.hpp>
#include <gocpp/arith.hpp>
#include <gocpp/newtype.hpp>
#include <gocpp/type.hpp>
#include <gocpp/iface.hpp>
#include <gocpp/ifaceconv.hpp>
#include <gocpp/error.hpp>
#include <gocpp/errors.hpp>
#include <gocpp/hresult.hpp>
#include <gocpp/iter.hpp>
#include <gocpp/maps.hpp>
#include <gocpp/reflect.hpp>
#include <gocpp/math.hpp>
#include <gocpp/bytealg.hpp>
#include <gocpp/shims.hpp>
#include <gocpp/minmax.hpp>
#include <gocpp/print.hpp>
#include <gocpp/panic.hpp>
#include <gocpp/defer.hpp>
#include <gocpp/rangefunc.hpp>
#include <gocpp/result.hpp>
#include <gocpp/fmt.hpp>
#include <gocpp/main.hpp>
#include <gocpp/unsafe.hpp>
#include <gocpp/interop.hpp>
#include <gocpp/callback.hpp>
#include <gocpp/goroutine.hpp>
#include <gocpp/chan.hpp>
#include <gocpp/select.hpp>
#include <gocpp/runtimepkg.hpp>
#include <gocpp/time.hpp>
#include <gocpp/syscall.hpp>
#include <gocpp/testing.hpp>

#include <gocpp/sync.hpp>
#include <gocpp/atomic.hpp>

// The standard headers that generated code uses directly: std::tuple for multiple results,
// INT64_C and the fixed-width integer types.
#include <cstdint>
#include <tuple>

// Last: the static_asserts of the sizes and alignments that go/types gets for gocpp's types
// (generated from the table in internal/check/sizes.go).
#include <gocpp/sizes.hpp>
