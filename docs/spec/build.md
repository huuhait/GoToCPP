# Building programs

> How `gocpp build` turns packages into a program, what it keeps, and when it does nothing. The
> design is in [ADR-008](../adr/008-build-driver.md), [ADR-025](../adr/025-split-layout.md) and
> [ADR-036](../adr/036-action-graph-and-up-to-date-check.md); the plan is
> [Phase 06](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md).

## What a build does

`gocpp build [-o out] [-p n] [-x] [-work] [-asan] [-nochecks] [packages]`:

1. **Load and check** the packages (the same errors as `gocpp check`; nothing is compiled when there are errors).
2. **Emit** the C++ of all packages into `<cache>/gen/<hash>` (`pkg.hpp`, one `.gp.cpp` per file with
   functions, `pkg_desc.cpp`, `pkg_init.cpp`; [ADR-025](../adr/025-split-layout.md)). A file is written
   only when its bytes change.
3. **Compile** the actions: the package's own C and C++ files, the sources of the libraries of modules
   (`cxx` recipes), and every generated unit, through the object cache. At most `-p n` run at the same
   time (`GOMAXPROCS` by default).
4. **Link** the objects with the runtime.

The actions start in the order of the packages, the user's own C and C++ files first, so `-p 1` is
deterministic. A compile that fails does not stop the others: the build shows every error it found,
grouped by package, in that order (after 20 failures no new compile starts, and the message says how
many were left out), and nothing is linked. The warnings of the compiles that worked are printed in the
same order whatever order they ended in. `^C` stops the compilers.

`-x` prints the commands, a line `# cache hit <unit>` for each compile the object cache answered,
and at the end the counts of what the build did:

```
# actions: load=1 check=5 emit=3 write=1 compile=1 hit=3 link=1 runtime=0
```

`load` is how many times the packages were loaded (0 for a build that was up to date), `check` and `emit` the
packages checked and emitted, `write` the generated files whose bytes changed, `compile` the runs of the
C++ compiler for units, C and C++ files and libraries, `hit` the answers of the object cache, `link` the
runs of the linker and `runtime` compiles of the runtime's own files.

`GOCPP_CPUPROFILE=file` writes a CPU profile of gocpp's own work (the front end, the cache lookups; not the
time in the C++ compiler) for `go tool pprof`.

`-work` keeps the work folder and prints its name; it uses no object cache (so the files in it have real
paths), and the build is not recorded for the up-to-date check.

## The build is up to date

After a successful `gocpp build`, the next one with the same arguments and environment first checks a
record of the last build (a *stamp*). When nothing it read has changed, it prints nothing, writes
nothing, touches nothing and exits with status 0, without loading the packages: milliseconds, whatever the
size of the program. The record has:

- what the loader read and listed (sources, `gocpp.mod`, `gocpp.sum`, folders: so a file that was added or
  removed counts), and the headers the packages import by a path;
- the folders that the patterns of a `cxx` recipe (`src src/*.c`) were expanded over, listed like the loader's
  folders: a new file that matches a pattern is a new source of the library (a removed or edited one is seen by
  the compile's own dependency list);
- for every compile, the source and the headers the compiler read (the `-MD` list, system headers too);
- the files the linker opened (libraries, start files) and the linker program, and pkg-config's answers;
- the compilers (their program files by content: the object cache's key and the on-disk detection record have the content hash too, so a compiler replaced in place with the same size and modification time is another compiler everywhere), gocpp itself and the environment variables the build reads
  (`GOCPP_*`, `CXX`, `CC`, `CPATH` and the other include paths, every `NIX_*` variable of the compiler and linker
  wrappers, `LD_RUN_PATH`, `LIBRARY_PATH`, `SOURCE_DATE_EPOCH`, `CCACHE_*`, `PATH`, `LDEMULATION`, `GNUTARGET`, `PKG_CONFIG*`);
- the program that was written (its content and its permission bits): a program that is missing or was changed, or
  made not executable, is built again.

`-x` says why a build is not up to date (`# not up to date: .../a.gp changed`) or not recorded
(`# not stamped: ...`). A build is **not recorded** when it printed a warning (warnings come with every
build), when a compile was not kept in the object cache, with `-work`, with a `...` pattern, when a
link flag may read a file the check cannot follow (`-Wl,--version-script=...`, `@file`, `-fuse-ld=`), and for
`gocpp run`. `GOCPP_NOUPTODATE=1` turns the check off.

What the check does not see (the object cache has the same limits): a system header shadowed by a new
file in an include folder that the compile did not name, a library that appears earlier on the linker's
search path, and a change that keeps a file's size, times and inode.

## What is kept, and where

The cache folder is `$XDG_CACHE_HOME/gocpp` (`~/.cache/gocpp` on Linux, `gocpp env` will print it). Every file is
written to a temporary name and renamed, and read back with its hash or its stat, so a build that is
killed, or two that run together, never leave something that a later build trusts. All of it can be
removed at any time; a build that runs while it is removed may fail with a message about a missing file
(run it again), never produce another program.

| Folder | What | Limit |
|---|---|---|
| `obj` | the object cache: objects of units and of C and C++ files, with the list of headers each read (`GOCPP_NOOBJCACHE`, `GOCPP_OBJCACHE_SIZE`) | 2 GiB, oldest first |
| `rt` | the runtime: sources, objects, precompiled headers | |
| `gen` | the generated C++ of each program that was built | folders not used for a week, when there are more than 100 |
| `stamp` | the records of finished builds | not used for 60 days |
| `cxxid` | what each C++ compiler is (its version line and family) | not used for 180 days |
| `probe` | the compiler's answers about headers | |
| `mod` | the module cache (downloads) | removed only by `gocpp clean -modcache` |

`gocpp clean -cache` removes everything but `mod` (a program that is being built keeps its folder in `gen`; a folder
that is a symbolic link is removed as a link, what it points at is never touched);
`-gen`, `-stamp` and `-modcache` remove one kind; `-n` prints what would be removed, with the sizes.

## Package summaries and the API hash

The record keeps, for each package, a hash of its sources and its **API hash**: the hash of `pkg.hpp`,
which holds everything a package that imports it can see: its types, prototypes, variables and the exported
constants with their values (and no `#line`, so a function body is not in it). `-x` prints them, and says
which packages changed since the last build of the program:

```
# api example.com/m/a 9e7a9b0a935a7e91
# sources changed: example.com/m/a
# api changed: example.com/m/a
```

A changed API recompiles the units of the packages that include the header; a changed body recompiles
the package's own unit and nothing else (the header is written again only when its bytes change).

## Libraries (`-buildmode`)

`gocpp build -buildmode=<mode>` makes a library instead of a program ([ADR-043](../adr/043-library-build-modes.md);
the guide is [libraries.md](../guides/libraries.md)). The mode is part of the up-to-date key, and every
unit is compiled with `-fPIC`, so a library never reuses the objects of a program (the object cache keeps both).

| Mode | Files | For |
|---|---|---|
| `exe` | the program (the default) | |
| `c-archive` | `libx.a` and `libx.h` | C programs; link with `-lstdc++ -pthread -lm -ldl` |
| `c-shared` | `libx.so` and `libx.h` | C programs, `dlopen`; every unit is `-fvisibility=hidden` |
| `cpp-lib` | a folder: `lib/libx.a`, `include/`, `lib/cmake/x/xConfig.cmake`, `lib/pkgconfig/x.pc` | C++ projects, with the native C++ API |

**Names.** For the C modes `-o libx.a` (or `libx.so`) names the library and the header is `libx.h` next to it; without
`-o`, `lib<program>.a`/`.so` in the current folder. For `cpp-lib` the patterns name the packages (several allowed, none
needs to be `main`), `-o` is the folder of the installed tree (default `<name>-cpp`) and the name is the package's, or the
module's last element for several packages.

**`//export Name`** in the doc comment of a function (no blank line between; the name is the function's own; not a
method, not generic, not `init`) makes it a C function. The C modes need at least one export and a `main` package with a
`main` function, as cgo does. The types follow cgo:

| Go | C |
|---|---|
| `int8`..`int64`, `uint8`..`uint64`, `int`, `uint`, `uintptr`, `float32`, `float64` | `GoInt8`..`GoUint64`, `GoInt`, `GoUint`, `GoUintptr`, `GoFloat32`, `GoFloat64` |
| `bool` | `GoUint8` |
| `unsafe.Pointer`, `*T` for a number or a `C` type, `*C.char` | `void*`, `T*`, `char*` |
| `C.int`, `C.long`, `C.size_t`, ... | `int`, `long`, `size_t` |
| `string` | `GoString {p, n}` |
| `[]T` for numbers or bool | `GoSlice {data, len, cap}` |
| several results | `struct Name_return {r0, r1, ...}` |

Not allowed (an error at the `.gp` position that says what to do instead): structs, arrays, maps, channels, funcs,
interfaces, `error`, complex numbers, a variadic function, a pointer to a gocpp struct, string or slice (reference counting
would free what C holds), a slice of anything but numbers or bool, a string behind a pointer.
Values are copied at the boundary: arguments are copies for the call; a `string` or slice result is `malloc` memory that
the caller frees with `free` (a string has a NUL after its last byte); a pointer result must point to C memory or a package
variable, or it is a fatal error.

**Start.** There is no load-time constructor. The first call of any exported function, on any thread, runs the package
initialization once in Go's order (thread-safe; other threads wait for it) and makes the calling thread a goroutine (its
record is freed when the thread ends). `os.Args` and the environment are the host's. gocpp installs no signal or terminate
handler. A panic that leaves an exported function (and `runtime.Goexit`) prints Go's message and trace and ends the process
with status 2; it never unwinds into C. A callback into C that calls back an export works (the nesting is plain C calls).

**c-shared isolation.** Each shared library has its own copy of the runtime and shares no symbol with another:
hidden visibility, `-fno-gnu-unique` (GCC), `--exclude-libs,ALL` and a version script that exports only the `//export`
names. Two libraries load in one process and unload with `dlclose` (`TestModes*` in cmd/gocpp). Unloading runs the
library's shutdown (`atexit`): it ends the idle goroutine threads, waits for them and forgets the adopted records. A library
with a goroutine that has not ended is not unloaded (it keeps a reference to itself from the first `go` statement until an exported
function returns with no goroutine left: `dlclose` is a no-op meanwhile; ADR-043); the end of the process does not wait for goroutines.

**cpp-lib.** The installed tree is a prefix: `include/gpkg/<import path>/pkg.hpp` for every package (with the layout of
[ADR-025](../adr/025-split-layout.md)), the runtime's `include/gocpp/`, `include/<name>.hpp` (namespace aliases for the
root packages and `gocpp_<name>::init()`, which runs the package initialization once: call it before any other call),
and the CMake and pkg-config files. With CMake:

```cmake
find_package(geo CONFIG REQUIRED)          # CMAKE_PREFIX_PATH=<the tree>
target_link_libraries(app PRIVATE gocpp::geo)
```

The target carries `-std=c++20 -fwrapv -fno-strict-aliasing -ffp-contract=off -fexceptions` (the consumer compiles gocpp's
inline code), the build's `-asan`/`-nochecks` flags, the public flags of packages whose headers include C or C++ headers,
`Threads::Threads -lm -ldl` and the packages' link flags. Use the compiler family that built the archive (CMake warns).
The stamp also records the header and the tree files, so a build with a missing or edited output builds again.
