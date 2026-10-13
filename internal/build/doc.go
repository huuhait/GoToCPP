// Package build will be gocpp's build driver. It finds the C++ compiler
// (GOCPP_CXX, CXX, then clang++ or g++), runs the steps from .gp files to a
// program (transpile, compile, link) as an action graph, caches the results
// by content hash, compiles the .c/.cpp files next to the .gp files, and links
// the libraries a package asks for.
//
// Phase 02 starts it (compiler detection and flags, the embedded runtime,
// one-package builds). Phase 04 adds C/C++ sources and link flags. Phase 06
// adds the action graph, the cache, parallel builds, build modes,
// compile_commands.json and the first Windows builds, and Phase 14 finishes
// Windows.
package build
