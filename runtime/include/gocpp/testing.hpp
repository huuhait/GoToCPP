// gocpp/testing.hpp: the native functions of the standard package testing and of internal/testdeps
// (Phase 09 step 9.9; ADR-056).
//
// Go's testing package is written in .gp (stdlib/testing): the tests, the subtests, the output
// rules of `go test`, the benchmarks and the examples. What it needs from the machine is here,
// named by //gocpp:native lines (ADR-027, internal/load/native.go), because the packages that Go's
// testing imports (sync, time, os, runtime, flag, strings, ...) are declaration stubs until
// the later steps of Phase 09:
//
//     //gocpp:native gp::testing::now
//     func now() int64                          // the call is ::gp::testing::now()
//
// Types are the C++ types of the Go parameters: int is int64_t, a string is gp::string, several
// results are a std::tuple.
//
//   - A test binary has no `time` and no `sync` yet, so the clock, the one lock of the package's
//     state (lock, unlock: a recursive mutex, never held while user code runs or a goroutine
//     waits), the timeout alarm and the list of the running tests are here.
//   - frames is what runtime.Caller and runtime.Callers give Go's testing: the stack of the
//     calling goroutine with the Go name, the .gp file and the line of each frame. It reads the
//     program's own symbol table and DWARF line table, as the trace of a panic does
//     (gocpp/trace.hpp).
//   - capture_start and capture_stop redirect file descriptor 1 to a temporary file, for the
//     examples (Go swaps os.Stdout for a pipe).
//   - lookup_env, set_env and unset_env are os.LookupEnv, Setenv and Unsetenv for T.Setenv.
//
// The C++ of this header is in src/testing*.cpp, which only a program that imports testing is
// linked with (rtfiles.TestingSources). This header is for stdlib/testing and
// stdlib/internal/testdeps; generated code of a program that doesn't import testing never calls
// these functions.
#pragma once

#include <gocpp/slice.hpp>
#include <gocpp/string.hpp>

#include <cstdint>
#include <string>
#include <tuple>

namespace gp::testing {

// now is a monotonic clock in nanoseconds (the start is arbitrary).
int64_t now() noexcept;

// alloc_counting(+1) turns the counting of allocations on, alloc_counting(-1) takes the request
// back (counting is on while some user wants it). alloc_stats returns the number of allocations and
// the bytes allocated since the program started, counted while it was on: operator new calls in
// this program (see testing.cpp for what that is), not Go's heap.
void alloc_counting(int64_t delta) noexcept;
std::tuple<int64_t, int64_t> alloc_stats() noexcept;

// sleep_ns blocks the calling thread for ns nanoseconds.
void sleep_ns(int64_t ns);

// lock and unlock are the one recursive mutex that guards the state of the tests (the T and B
// values, the output buffers). User code never runs under it.
void lock() noexcept;
void unlock() noexcept;

// args returns the program's arguments without the program name (os.Args[1:]).
gp::slice<gp::string> args();

// program_name is the name the program was started with (os.Args[0]).
gp::string program_name();

// ncpu is the number of CPUs the process may run on (runtime.GOMAXPROCS(0) in Go).
int64_t ncpu() noexcept;

// goos and goarch name the target, for the labels of the benchmarks ("goos: linux").
gp::string goos();
gp::string goarch();

// cpu_name is the model name of the processor, for the "cpu:" line of the benchmarks (empty when
// the system does not say).
gp::string cpu_name();

// frames describes the stack of the calling goroutine, innermost frame first, one frame per line
// "function<TAB>file<TAB>line<LF>": the Go name of the function ("example.com/m/a.TestX",
// "testing.(*T).Run"), the .gp file of the call (the full name) and its line. The runtime's own
// frames are left out, as in a panic's trace. max limits the number of frames (0: 64). The
// result is empty when the program has no debug information.
gp::string frames(int64_t max);

// capture_start redirects the standard output (file descriptor 1) to a temporary file;
// capture_stop restores it and returns what was written. They don't nest.
void capture_start();
gp::string capture_stop();

// lookup_env, set_env and unset_env: os.LookupEnv, os.Setenv and os.Unsetenv. set_env and unset_env
// return an error text, "" when it worked.
std::tuple<gp::string, bool> lookup_env(gp::string key);
gp::string set_env(gp::string key, gp::string value);
gp::string unset_env(gp::string key);

// make_temp_dir creates a new folder in the system's temporary folder, whose name starts with
// prefix, and returns its path and an error text ("" when it worked). remove_all removes a file or
// a folder with all it holds and returns an error text.
std::tuple<gp::string, gp::string> make_temp_dir(gp::string prefix);
gp::string remove_all(gp::string path);

// make_dir creates the folder path (mode 0777, like os.Mkdir) and returns an error text.
gp::string make_dir(gp::string path);

// getwd and chdir are os.Getwd and os.Chdir: the path and an error text, or an error text.
std::tuple<gp::string, gp::string> getwd();
gp::string chdir(gp::string dir);

// read_files lists the files (not the folders) in the folder dir, sorted by name, one name on
// each line; a folder that does not exist gives "" and no error. read_file returns the content
// of a file. Both return an error text, "" when it worked. They are for the seed corpus of the
// fuzz targets (testdata/fuzz).
std::tuple<gp::string, gp::string> read_files(gp::string dir);
std::tuple<gp::string, gp::string> read_file(gp::string path);

// parse_float parses a whole string as a floating-point number (a literal of a corpus file).
std::tuple<double, bool> parse_float(gp::string s);

// exit flushes C's standard streams and ends the process with the code, like os.Exit.
[[noreturn]] void exit(int64_t code) noexcept;

// alarm_wait waits for ns nanoseconds, or until alarm_cancel is called: it returns true when the
// time has passed (the test binary's -test.timeout), false when canceled. With ns <= 0 it
// returns false at once.
bool alarm_wait(int64_t ns);
void alarm_cancel();

// running_add and running_delete keep the names of the tests that are running, with their start
// times (now), for the message of a timeout; running_list returns them sorted by name, one per
// line "name<TAB>start" (start is the now() of running_add).
void running_add(gp::string name, int64_t start);
void running_delete(gp::string name);
gp::string running_list();

} // namespace gp::testing
