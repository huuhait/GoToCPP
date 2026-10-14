# runtime

The gocpp runtime: a C++20 library in `namespace gp`. Generated code includes one header,
`#include <gocpp/rt.hpp>`, and is linked with the runtime's `.cpp` files. The runtime is
embedded in the `gocpp` binary (`go:embed`, ADR-009), so there is nothing extra to install. The
`//go:embed` directive is in `files.go` (package `rtfiles`), and the driver imports that
package: a `//go:embed` pattern can't reach a parent folder, so the directive must be inside
`runtime/`.

This is runtime v0 (Phase 02): the API shape is final, and every later phase grows it.

| Folder or file | What is there |
|---|---|
| `include/gocpp/` | the headers (below). Embedded |
| `src/rt.cpp` | the parts that are not templates: strings, panics, the print and `fmt` formats, `run_main`. Standard C++ only. Embedded |
| `src/os/linux/` | the Linux OS layer: `os.cpp`. Phase 08 adds the Linux side of the thread layer `gp::os`. Embedded |
| `src/os/windows/` | the Windows OS layer (Phase 14; not there yet) |
| `files.go` | package `rtfiles`: `FS()` holds `include/` and `src/`; `Sources(goos)` lists the `.cpp` files a program is compiled with (`src/*.cpp` and `src/os/<goos>/*.cpp`) |
| `test/` | the C++ tests (not embedded): `unit.cpp`, `diff.cpp` and `fortify.cpp` |
| `testdata/diff/main.go` | the Go side of the differential test |
| `*_test.go` | the Go tests that build and run the C++ tests, and the checks below |
| `compile_flags.txt` | the flags for clangd, so the files open without false errors in an editor (the dev shell's clangd) |

## The headers

| Header | What it has |
|---|---|
| `rt.hpp` | includes everything below, `nofortify.h` first |
| `nofortify.h` | turns `_FORTIFY_SOURCE` off at `-O0` (below) |
| `checks.hpp` | the concepts `gp::go_int` and `gp::go_float`, and the panics of the runtime checks with Go's messages: `panic_divide`, `panic_shift`, `panic_index` |
| `string.hpp` | `gp::string` (owner, data pointer, length: 32 bytes), `"..."_gs` literals (static storage, no allocation), `+`, `+=`, `==`, `<=>`, `s[i]` with Go's bounds check, `gp::len(s)` |
| `arith.hpp` | `gp::div`, `gp::rem`, `gp::shl`, `gp::shr` and `gp::f2i<T>`, each returning the Go type |
| `print.hpp` | the builtins `gp::print` and `gp::println`: stderr, in Go's runtime number format (`+1.500000e+000`) |
| `panic.hpp` | `gp::panic_error` (the exception a panic throws) and `gp::panic(v)` |
| `fmtshim.hpp` | `gp::fmtshim::Print`, `Println` and `Printf` (verbs `%v %d %s %t %f`), a stand-in for `fmt` until Phase 05 and 09 |
| `main.hpp` | `gp::run_main(argc, argv, init, main)`, `gp::args()`, `gp::envs()` |
| `os.hpp` | the OS layer's interface (declarations only) |

What they do, in short:
- **Integers work like Go.** `gp::div` and `gp::rem` panic on a zero divisor, and `MIN / -1` is
  `MIN`. `gp::shl` and `gp::shr` give Go's results for a count of the width or more, and panic
  on a negative count. `gp::f2i<T>` converts a float like Go does on amd64, also out of range.
- **A panic is a C++ exception**, `gp::panic_error`. `gp::run_main` catches it, prints
  `panic: <value>`, a blank line and `goroutine 1 [running]:` to stderr, and exits with code 2.
  The stack trace comes in Phase 03.
- **Output order is Go's.** Nothing is buffered: each `print`, `println` and `fmtshim` call
  formats its text first and writes it with one `write` call, unless the OS takes only part of
  it (then the OS layer writes the rest). `run_main` ends the program through the OS layer,
  which flushes C stdio and exits at once (no C++ static destructors).
- **Writes behave like Go's.** A non-blocking stdout or stderr that is full makes the program
  wait (`poll`); it doesn't lose the rest. SIGPIPE works as in Go: at start the OS layer
  installs a handler that does nothing (not "ignore", so programs that the process starts get
  the default back), so a write to a broken pipe fails with EPIPE. Then a `fmtshim` write to
  stdout (Go's `os.Stdout`) ends the program with SIGPIPE, also when the parent left SIGPIPE
  ignored; the builtin `print` and `println` ignore the error and the program goes on; a
  write to another file gets the error (`gp::os::write_result::broken_pipe`, for Phase 09).

## Which parts are `.cpp` files

Templates and small functions are in the headers. The rest is in `src/rt.cpp`, so that the
formatting code isn't compiled again in every file of the program, and the OS layer is in
`src/os/<os>/`, so that no OS header reaches generated code. The driver compiles these files
with the program, with the same flags (`rtfiles.Sources`). Phase 06 compiles them once into
its cache.

## The OS rule

The runtime may reach the operating system **only** through `src/os/`: writing to a file
descriptor, exiting, the program's arguments and the environment (`gocpp/os.hpp`). Phase 08 adds
the thread layer `gp::os` there: starting a thread with a stack size, thread-local storage and
the stack-overflow hook. Everything else uses standard C++ (`std::chrono`, `<atomic>`,
`std::filesystem`), so the same code builds on Linux and Windows.

`TestNoOSHeadersOutsideOS` checks this in `make ci`: outside `src/os/`, every file of `runtime/`
(whatever its name, since C++ can include any file; only the Go files, `README.md` and
`compile_flags.txt` are left out) may include only the C++20 standard headers, the standard C
headers and gocpp's own headers. Anything else, such as `<unistd.h>`, `<sys/...>`,
`<pthread.h>` or `<windows.h>`, fails the test. It also reads `%:include`, continuation lines
and comments inside the directive.

A check of `#include` lines can't see everything: glibc's standard headers declare some POSIX
functions anyway (g++ defines `_GNU_SOURCE`, so `<cstdio>` declares `fileno`), and code can
declare an OS function by hand. So `TestWindowsSyntax` also compiles every `.cpp` file outside
`src/os/` for Windows with the dev shell's MinGW-w64 (`-fsyntax-only`), where those don't
exist. It is skipped when `x86_64-w64-mingw32-g++` isn't installed. Neither check reaches a
POSIX function that also exists on Windows (MinGW has `fileno`): reviews still matter.

## `nofortify.h`

A Nix shell's compiler wrapper turns on "hardening" by default. It defines `_FORTIFY_SOURCE`
after the user's flags, so `-U_FORTIFY_SOURCE` doesn't help, and every file built without
optimization (`-O0`) then gets a warning. `gocpp/nofortify.h` undefines `_FORTIFY_SOURCE` when
the compiler doesn't optimize (`__OPTIMIZE__` is not defined). `rt.hpp` and the runtime's
`.cpp` files include it before anything else, and the driver passes it with `-include` to the
user's own C/C++ files (Phase 04). The project's dev shell turns hardening off, so
`TestNoFortify` turns it on again with `NIX_HARDENING_ENABLE` (nixpkgs' default list) to check
this.

## Tests

`go test ./runtime` (also part of `make test` and `make ci`) runs:
- `TestCXX`: compiles `test/unit.cpp` and `test/diff.cpp` with the embedded runtime, with g++
  and clang++, each at `-O0`, at `-O2` and with `-fsanitize=address,undefined` (a sanitizer
  report fails the test). The flags are the driver's: `-std=c++20 -fwrapv -fno-strict-aliasing
  -ffp-contract=off -g`, plus `-Wall -Wextra -Werror`, and `-Qunused-arguments` for clang.
  - `unit.cpp` checks every helper: strings, `div`/`rem`/`shl`/`shr` for every integer type
    (`MIN / -1`, shifts by 0, width - 1, width, 200, a negative count), `f2i` at the
    boundaries and for NaN and ±Inf, the print and `fmt` formats, panics, `run_main`.
  - `diff.cpp` is the same program as `testdata/diff/main.go`. The test runs both, with each
    case, and compares the exit codes and stdout and stderr merged into one stream, up to Go's
    stack trace: number formats (1500 random floats too), integer tables, float → int (on
    amd64 only), strings, panics, the output order and 1 MiB writes. Without a `go` command
    it is skipped.
  - Without Go: the exact panic output and exit codes; on Linux, 1 MiB into a full
    non-blocking pipe (all of it must arrive) and into a broken pipe, with and without SIGPIPE
    ignored by the parent (`fmtshim::Print` must die of SIGPIPE, `println` must go on; these
    expectations are also checked against Go when it is there); and a leak, which
    LeakSanitizer must report in the asan mode.
    Every program ends with `_exit`, which skips ASan's leak check, so `gp::os::exit` runs it
    itself in an ASan build.
- `TestNoFortify`: the `nofortify.h` check above, with `-Werror`.
- `TestFSHasEveryFile`: the embedded files are exactly the files of `include/` and `src/`.

The tests that walk `runtime/` leave out names that start with `.` (editor and tool files: an
Emacs lock `.#rt.cpp`, clangd's `.cache/`; `//go:embed` leaves them out too) and symbolic links
that point nowhere.
- `TestSources`: `rtfiles.Sources` lists the right `.cpp` files.
- `TestSymbols`: the runtime's object files define no strong global symbol outside namespace
  `gp` (or the `gp_` prefix), checked with `nm`, so a user's C or C++ file can't clash with
  them (an `extern "C"` helper would be a global C name). Weak symbols, the standard library's
  inline functions and templates, are left out: the linker merges them.
- `TestNoOSHeadersOutsideOS` and `TestWindowsSyntax`: the OS rule; `TestCheckIncludes` tests
  the checker itself.
- `TestClangFormat`: the C++ files follow the repo's `.clang-format`
  (`clang-format --dry-run --Werror`). It runs with clang-format 21, the dev shell's, and is
  skipped with another version or none.

Like the test harness, `GOCPP_CXX=clang++` tests with one compiler only, and a compiler that
isn't installed (or that `build.LookCXX` can't use) is a skip, unless `GOCPP_REQUIRE_CXX`
names it (`make ci` sets `GOCPP_REQUIRE_CXX=g++,clang++`).
