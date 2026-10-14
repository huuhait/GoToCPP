// gocpp/main.hpp: gp::run_main, the start and the end of a gocpp program.
//
// The generated C++ main is
//
//     int main(int argc, char** argv) {
//         return gp::run_main(argc, argv, nullptr, &gpkg_main::main);
//     }
#pragma once

#include <gocpp/string.hpp>

#include <vector>

namespace gp {

// run_main stores the program's arguments and environment, runs init (when it isn't null:
// the package initialization) and then main. When main returns, the program exits with code
// 0. When a panic reaches it, it prints "panic: <value>" and "goroutine 1 [running]:" to
// stderr and exits with code 2, like Go. Either way it ends the process through
// gp::os::exit, which flushes C stdio and runs no C++ static destructors, so run_main never
// returns; the int result is there for the shape of main.
[[noreturn]] int run_main(int argc, char** argv, void (*init)(), void (*main)());

// args are the program's arguments, the program name first (for os.Args, Phase 09), and envs
// is the environment at start as "KEY=value" strings (for os.Environ). Both are empty before
// run_main.
const std::vector<gp::string>& args() noexcept;
const std::vector<gp::string>& envs() noexcept;

} // namespace gp
