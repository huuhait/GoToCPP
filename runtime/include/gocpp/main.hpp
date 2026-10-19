// gocpp/main.hpp: gp::run_main, the start and the end of a gocpp program.
//
// The generated C++ main is
//
//     int main(int argc, char** argv) {
//         return gp::run_main(argc, argv, nullptr, &gpkg_main::main);
//     }
#pragma once

#include <gocpp/error.hpp>
#include <gocpp/string.hpp>

#include <vector>

namespace gp {

// run_main stores the program's arguments and environment, runs init (when it isn't null:
// the package initialization) and then main. When main returns, the program exits with code
// 0. When a panic reaches it, it prints "panic: <value>" and "goroutine 1 [running]:" to
// stderr and exits with code 2, like Go. Either way it ends the process through
// gp::os::exit, which flushes C stdio and runs no C++ static destructors, so run_main never
// returns; the int result is there for the shape of main. It also installs the new handler
// gp::detail::out_of_memory (panic.hpp): an allocation that fails is Go's fatal error
// "runtime: out of memory" (exit code 2, no deferred calls), not a std::bad_alloc; the
// terminate handler gp::detail::on_terminate (panic.hpp): a C++ exception that nothing catches
// ends the program like an unrecovered panic, before anything unwinds; gp::os::start catches a
// stack overflow of the main thread and of every thread that gp::os::thread_start starts, with
// the hook that run_main sets, which prints Go's lines (Linux); and the calling thread becomes
// the main goroutine, with the id 1 (gocpp/goroutine.hpp), which a panic reports. A panic in
// another goroutine ends the program the same way (gp::detail::exit_with_panic).
[[noreturn]] int run_main(int argc, char** argv, void (*init)(), void (*main)());

// args are the program's arguments, the program name first (for os.Args, Phase 09), and envs
// is the environment at start as "KEY=value" strings (for os.Environ). Both are empty before
// run_main.
const std::vector<gp::string>& args() noexcept;
const std::vector<gp::string>& envs() noexcept;

namespace detail {

// load_process_info sets args() and envs() from the running process, for a library that no
// run_main started (gp::lib::enter, gocpp/lib.hpp): the host program's arguments and environment.
void load_process_info();

} // namespace detail

// must_exit(err) is the end of func main!() and func init!() (Phase 10, step 10.5; the generated
// main calls it with the error that the must function returned). A nil error does nothing. Else
// it writes "<program>: <error>" to stderr, <program> being the base name of the program's
// first argument, and ends the program with the code 1 (gp::os::exit: C stdio is flushed, no
// C++ destructor runs). main's deferred calls have run by then: they ran when the function
// returned the error.
void must_exit(const gp::error& err);

} // namespace gp
