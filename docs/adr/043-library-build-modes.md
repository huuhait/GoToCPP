# ADR-043: Library build modes: c-archive, c-shared and cpp-lib

Status: accepted (2026-10-18), implements [Phase 06 README §2.7](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md),
builds on [ADR-025](025-split-layout.md) and [ADR-036](036-action-graph-and-up-to-date-check.md)

## Context

`gocpp build` made programs only. Step 6.10 adds `-buildmode=c-archive|c-shared|cpp-lib`, so that C
and C++ programs call gocpp code. Questions: how does a library start without `main()`, what may cross
the C boundary, how do two gocpp shared libraries live in one process, and what does a C++ project
get.

## Decision

**One pipeline, other glue.** The packages are lowered and compiled as for a program (`cppgen.Config.Library`:
the last package of the program defines `gp_init_all` and no package defines `main()`), with `-fPIC` for
every unit, also the runtime's (the flags are compiler arguments, so the runtime's objects, the precompiled
header and the object cache keep a copy for each mode, and the stamp key has the mode). A static mode ends in a
deterministic archive (`ar rcsD`, `gcc-ar`/`llvm-ar` when found) instead of a link.

**`//export` (c-archive, c-shared).** The directive must be in the doc comment of a top-level, non-generic
function with the function's own name (cgo's rule). `internal/libgen` checks cgo's type rules with errors at
the `.gp` position: numbers, `bool` (as `GoUint8`), `unsafe.Pointer` (`void*`), pointers to numbers and to
`C.*` types, `C.int` & co, `string` (`GoString`), slices of numbers or bool (`GoSlice`), several results
(a struct `Name_return {r0, r1, ...}`). No struct, array, map, chan, func, interface, error or pointer to
gocpp-layout types (reference counting would free what C holds). The header is cgo-compatible (`GoInt`,
`_GoString_`, `GoSlice`...). Values are copied at the boundary: a string or slice argument is a copy made for the
call; a string or slice result is `malloc` memory that C frees (a string has a NUL after the last byte). A
pointer result must not be kept alive only by a reference count (fatal error otherwise).

**Lazy start.** Every exported function is an `extern "C"` function (generated `gocpp_export.cpp`) that
calls `gp::lib::guard`: `enter` (once, thread-safe, `std::call_once`) runs `gp_init_all` in Go's order and
registers `shutdown` with `atexit`; the calling thread becomes a goroutine; the body runs inside a try block
whose handlers end the program like an unrecovered Go panic (message, trace, exit status 2): nothing unwinds
into a C frame. No signal handler and no terminate handler is installed (they belong to the host). A call
made from inside init goes through at once.

**Adopted threads.** The goroutine record of a thread that no `go` statement started was never freed. It is
now released by a thread-exit hook (`os::tls_on_thread_exit`, a pthread key destructor), so a library called
from a thread per request holds nothing for threads that are gone; `shutdown` ends the idle goroutine threads,
waits for them and forgets the remaining records and the hook before a shared library is unloaded.

**c-shared isolation.** Two gocpp libraries in one process each carry their own copy of the runtime
(static linking), and share nothing: `-fvisibility=hidden -fvisibility-inlines-hidden` for every unit,
`-fno-gnu-unique` (GCC would make the inline variables of the runtime's headers `GNU_UNIQUE`, which merge across
libraries and block `dlclose`), `-Wl,--exclude-libs,ALL` and a linker version script that lists the exports
as the only global symbols (without it the weak symbols of the C++ library's templates stay visible
and bind across libraries). `nm -D` shows only the exports. `dlopen`/`dlclose` of two libraries,
several times, works.

**cpp-lib.** The packages the patterns name (any packages, no `main`) become `lib<name>.a` plus an
installed tree under `-o` (default `<name>-cpp`): `include/gpkg/<path>/pkg.hpp` of every package, the
runtime's headers, `include/<name>.hpp` (short namespace aliases and `gocpp_<name>::init()`, which runs the
package initialization once), `lib/cmake/<name>/<name>Config.cmake` (an imported target `gocpp::<name>` that
carries `-std=c++20 -fwrapv -fno-strict-aliasing -ffp-contract=off -fexceptions`, the build's `-DGOCPP_NOCHECKS`
and sanitizer flags, the public flags of packages whose headers include C or C++ headers, the threads library
and the packages' link flags) and `lib/pkgconfig/<name>.pc`. The C++ API is the native one (`gp::string`,
structs, methods). A consumer must use the compiler family that built the archive (the CMake file warns).

**Stamp.** `-buildmode` is part of the key. The other files a library mode writes (the header, the
tree) are recorded in the stamp (`Stamp.Extra`), so a build whose header or tree file is missing or changed is
rebuilt.

## Consequences

- A hello library is about 7.5 MB as a static archive and 3.6 MB as a shared library (debug flags, runtime
  included); `-O`/stripping come with Phase 15's release flags.
- The C modes need a `main` package with `func main() {}` as in cgo.
- The Windows `.dll` is Phase 14.
- **Unloading with goroutines (amended after review).** A goroutine's thread runs code of the library, so
  `dlclose` must never unmap a library that has a goroutine that has not ended (running, blocked or sleeping;
  `running_goroutines()` in the runtime counts the goroutines the `go` statements started). A destructor that
  runs during `dlclose` cannot refuse the unload (pinning the library from it is too late: glibc unmaps right
  after), so the library holds one extra reference to itself while such a goroutine exists: the first `go`
  statement calls a hook (`set_go_hook`) that does `dlopen(own path, RTLD_NOLOAD)` (`os::library_ref`, not
  for a library linked into the program). With the reference the host's `dlclose` returns 0 and does nothing:
  no destructor, no stall, no output, the code stays mapped and the goroutines go on, like Go's c-shared
  libraries, which can't be unloaded at all. Every exported function gives the reference back when it returns (when it began with no goroutine it first waits, up to 250 ms, for the ones it started to end: the wait is bounded by time, because on a busy machine the goroutine threads may not run for a while; a goroutine that does not end in time keeps the library pinned until a later call)
  while no goroutine runs (`gp::lib::detail::leave`), so a library whose goroutines have ended is unloaded by
  the host's next `dlclose`. The reference is given back from the host's own call (the host still holds its
  reference, so the library can't go away under the thread that gives it back); a goroutine that ends after
  the host's last call keeps the library mapped until the process ends (a leak that is safe). A thread that
  calls an exported function of a library that the host already closed is the host's error, as it is for any
  `dlclose`. If the last call returns while a goroutine of an earlier call is still finishing, the reference
  stays until the process ends (safe, a leak). `end_threads` ends with a 2 ms grace sleep for a thread
  that is still returning from its function: a thread that is slower than that when the library is unloaded
  is a residual risk. With no goroutine, `dlclose` runs `shutdown` (`atexit` of the DSO): it ends the idle goroutine
  threads, waits for them and forgets the adopted records.
- **End of the process.** `shutdown` also runs at `exit()`. It finds a running goroutine only then (a pinned
  library is never closed), and the goroutines die with the process, so it waits for the idle threads only for
  20 ms, not the 3 s it waits when it unloads. A host whose `main` returns with a blocked goroutine exits at once.
  (`TestModesUnloadLive`: busy, blocked, many, released, finishing during `dlclose`, two libraries with one
  live, exit with goroutines alive.)

## Known gaps (6.10 lows)

- A slice parameter is a copy: what Go writes into a `[]byte` parameter does not reach the C buffer (cgo
  passes the memory itself). Return a slice, or take a pointer and a length.
- `GoString{NULL, n}` with `n > 0` is a C error that the library does not check: it reaches `std::string_view`
  and UBSan reports it.
- A foreign thread that runs a `go` statement while the package initialization runs (a package `init` that
  waits for a C thread that calls an exported function) may wait for `call_once`; the exported functions that
  `init` reaches on its own thread go through.
- In a cpp-lib, forgetting `gocpp_x::init()` leaves package variables unset (zero values, and no `init`
  functions run): the header says to call it first, nothing checks.
- An export can't have the name of a symbol of the C library: the build reads the symbols of the compiler's
  libc, libm, libpthread and libdl with `nm -D` (when `nm` and the files are there) and rejects them at the
  `.gp` position, with a list of the common names as a fallback (`//export mmap64` is caught this way, the
  name is in no header). A name that a header declares differently and that is no libc symbol makes the glue
  fail to compile; the build then says so (not "a bug in gocpp") and shows the compiler's errors.
