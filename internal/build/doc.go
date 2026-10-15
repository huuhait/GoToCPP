// Package build will be gocpp's build driver. It finds the C++ compiler
// (GOCPP_CXX, CXX, then clang++ or g++), runs the steps from .gp files to a
// program (transpile, compile, link) as an action graph, caches the results
// by content hash, compiles the .c/.cpp files next to the .gp files, and links
// the libraries a package asks for.
//
// Phase 01 starts it with the compiler finder (FindCXX), for the namespace
// probe. Phase 02 adds the compiler's version checks and arguments
// (cxx.go), the flags (flags.go), the embedded runtime, extracted once
// into the cache folder with its object files (runtime.go), one-package
// builds (compile.go: Builder) and the process groups of the compilers
// (proc_unix.go), which the namespace probe uses too. Phase 04 adds the
// flags of packages (pkgflags.go: the //gocpp: directives and pkg-config,
// checked by the go command's flag allow-list, security.go) and C/C++
// sources. Phase 06 adds the action graph, the cache, parallel builds,
// build modes, compile_commands.json and the first Windows builds, and
// Phase 14 finishes Windows.
package build
