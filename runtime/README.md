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
| `checks.hpp` | the concepts `gp::go_int` and `gp::go_float`, and the panics of the runtime checks with Go's messages: `panic_divide`, `panic_shift`, `panic_index`, `panic_nil` |
| `string.hpp` | `gp::string` (owner, data pointer, length: 32 bytes), `"..."_gs` literals (static storage, no allocation), `+`, `+=`, `==`, `<=>`, `s[i]` with Go's bounds check, `gp::len(s)` |
| `ptr.hpp` | Go's pointers: `gp::ptr<T>` (`*T`: a `std::shared_ptr` with nil checks, 16 bytes), `gp::new_<T>` (`new(T)`, `&T{...}`, boxes), the aliasing constructors for pointers into objects (`&p.f`, `&s[i]`), `gp::static_ptr` (`&global`), `gp::ptr_cast` (`(*Q)(p)`), and `gp::weak_ptr<T>` with `gp::weak_make` and `gp::weak_value` (`weak.Pointer[T]`, `weak.Make`, `Value`) |
| `arith.hpp` | `gp::div`, `gp::rem`, `gp::shl`, `gp::shr` and `gp::f2i<T>`, each returning the Go type |
| `print.hpp` | the builtins `gp::print` and `gp::println`: stderr, in Go's runtime number format (`+1.500000e+000`) |
| `panic.hpp` | `gp::panic_error` (the exception a panic throws) and `gp::panic(v)` |
| `fmtshim.hpp` | `gp::fmtshim::Print`, `Println` and `Printf` (verbs `%v %d %s %t %f`), a stand-in for `fmt` until Phase 05 and 09 |
| `main.hpp` | `gp::run_main(argc, argv, init, main)`, `gp::args()`, `gp::envs()` |
| `os.hpp` | the OS layer's interface (declarations only) |

The headers include only the standard headers they need. Every macro of a header that
`rt.hpp` includes is in every generated file, and a Go name must not be one of them.

The emitter renames such Go names, from the list in `internal/cppgen/cxx_macros.txt`. So a
header that starts to include a new standard header adds macros to that list: `TestMacroList`
fails, and `go test ./internal/cppgen -run TestMacroList -update` adds them.

What they do, in short:
- **Integers work like Go.** `gp::div` and `gp::rem` panic on a zero divisor, and `MIN / -1` is
  `MIN`. `gp::shl` and `gp::shr` give Go's results for a count of the width or more, and panic
  on a negative count. `gp::f2i<T>` converts a float like Go does on amd64, also out of range.
- **A panic is a C++ exception**, `gp::panic_error`. `gp::run_main` catches it, prints
  `panic: <value>`, a blank line and `goroutine 1 [running]:` to stderr, and exits with code 2.
  The stack trace comes in Phase 03. A nil pointer dereference also sets the goroutine's
  signal (a thread-local value, like Go's `g.sig`), and then the report prints Go's signal
  line after the panic text (after the whole chain of panics, from step 3.13 on):
  `[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x...]`. A recover that
  stops the panic clears it (Phase 05). gocpp's `addr` is always `0x0`. Go's is the faulting
  address: `0x0` for a field at offset 0, a field at 4096 or more, or a whole value, and the
  field's offset for any access, read or write, to a field at a smaller nonzero offset.
- **Memory is reference counted** (ADR-004). Every heap block is owned by a `std::shared_ptr`;
  a pointer into a block (`&p.f`, `&s[i]`) shares the block's owner, so it can't dangle, and a
  pointer to a package-level variable shares a static owner that is never freed (so a weak
  pointer to a global never expires). A non-nil pointer always has an owner (the aliasing
  constructors check it, unless `GOCPP_NOCHECKS` is defined, gocpp's own switch for the
  `-nochecks` build of step 3.14; not `NDEBUG`, which a user's file may define). A cycle is never freed unless a weak
  pointer breaks it. After the last strong pointer to a `gp::new_` value goes, the value is
  destroyed, but its memory stays until the last weak pointer to it goes too (one allocation
  holds the value and the counts).
  The counts are atomic, so pointers can be copied on several threads; one pointer variable
  that one thread writes while another reads it is a data race, as in Go. A copy costs one
  atomic increment and dropping it one decrement; a move, `*p`, `p->f`, `==` and a nil check
  cost none (counted in the `-O2` code of both compilers; the numbers are in `ptr.hpp`).
- **Freeing never recurses deeply.** Freeing the head of a list frees the next node from its
  destructor, and so on. `gp::detail::release`, which `~ptr` calls, counts that nesting per
  thread, and past a depth of 128 it puts the owner on a thread-local list that the outermost
  release frees in a loop, so a list of millions of nodes is freed without running out of
  stack.
  Every runtime type that holds an owner releases it through it too (slices and maps in steps
  3.5 and 3.7, then closure cells, func values and interfaces). The limit is small because a
  goroutine is a thread, whose stack may be small: freeing a deep chain works on a stack of
  96 KB at `-O0`, 32 KB at `-O2` and 192 KB with ASan (measured with `ulimit -s`; the numbers
  are in `ptr.hpp`).
- **No plain-named members.** Every type that holds a Go value (`gp::string`, `gp::ptr`,
  `gp::weak_ptr`, and the slices, maps and others to come) has only constructors, operators
  and `gp_`-prefixed members, public or private, and everything else is a free function
  (`gp::len(s)`, `gp::weak_value(w)`). A Go method or field can then have any name (`get`,
  `len`, `Value`) without clashing with a runtime member, also when a named type is built on a
  runtime type (Phase 03 §2.1, CONTRIBUTING §5). `TestNoPlainMembers` checks every class in
  namespace `gp` (but not in a `detail` namespace) with clang's AST dump, and `unit.cpp` checks
  a list of likely names (the members of `std::shared_ptr` and `std::string`, and common Go
  method names) with `static_assert`s.
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
    boundaries and for NaN and ±Inf, the print and `fmt` formats, panics, `run_main`, and the
    pointers: nil and `==`, the nil panics (`*p`, `p->f`, `&p->f`, a method through a nil
    pointer) and the goroutine's signal line, ownership (a box outlives its function, an interior
    pointer keeps its whole block), copy and move counts, pointers to globals, weak pointers
    (nil after the last pointer goes, equality after that, a cycle broken with a weak pointer
    and one broken by hand, so LeakSanitizer stays quiet in the asan mode), hashing,
    `ptr_cast`, a recursive type, freeing lists of 200,000 nodes and trees 200,000 nodes deep
    on one side, each of which overflows the stack at `-O0` without `detail::release`
    (nothing is left pending afterwards; `GOCPP_TEST_BIG=1 go test ./runtime` also runs them
    with 2,000,000 nodes, which takes more time and memory than `make ci` should), and the "no
    plain-named members" rule for a list of names.
  - `diff.cpp` is the same program as `testdata/diff/main.go`. The test runs both, with each
    case, and compares the exit codes and stdout and stderr merged into one stream, up to Go's
    stack trace: number formats (1500 random floats too), integer tables, float → int (on
    amd64 only), strings, pointers (with `weak`), panics (also the nil pointer ones, whose
    signal line is compared with the `pc` left out), the output order and 1 MiB writes.
    Without a `go` command it is skipped.
  - Without Go: the exact panic output and exit codes; on Linux, 1 MiB into a full
    non-blocking pipe (all of it must arrive) and into a broken pipe, with and without SIGPIPE
    ignored by the parent (`fmtshim::Print` must die of SIGPIPE, `println` must go on; these
    expectations are also checked against Go when it is there); a pointer without an owner,
    which must end the program with a fatal error; and a leak, which LeakSanitizer must
    report in the asan mode.
    Every program ends with `_exit`, which skips ASan's leak check, so `gp::os::exit` runs it
    itself in an ASan build.
- `TestNoFortify`: the `nofortify.h` check above, with `-Werror`.
- `TestFSHasEveryFile`: the embedded files are exactly the files of `include/` and `src/`.

The tests that walk `runtime/` leave out names that start with `.` (editor and tool files: an
Emacs lock `.#rt.cpp`, clangd's `.cache/`; `//go:embed` leaves them out too) and symbolic links
that point nowhere.
- `TestSources`: `rtfiles.Sources` lists the right `.cpp` files.
- `TestNoPlainMembers`: the "no plain-named members" rule, on every class in namespace `gp`
  (clang++ `-ast-dump=json`); `TestNoPlainMembersFinds` tests the check on a header with every
  kind of member. Skipped without clang++.
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
