# The runtime library, the precompiled header and the link step

How `gocpp build` gets from compiled units to a program: the runtime as a cached static library
(Phase 06 step 6.8), the precompiled header of the runtime's headers, and the link command with its
order, deduplication and link-time optimization (step 6.9). The decisions are in
[ADR-057](../adr/057-runtime-library-and-link-line.md).

## The runtime library

The runtime's sources (`runtime/src`, embedded in the `gocpp` binary and extracted once into the
cache) are compiled once for a key and archived into one file:

```
<cache>/rt/<version>-<hash>/lib/<key>/libgocpprt.a
```

Every program with the same key links that file. The key is a hash of

- the compiler: path, arguments, family, version and the identity of its program file;
- the flags: the debug flags, the sanitizer (`-asan`), `-nochecks`, `-flto` (`-lto`), and
  `GOCPP_CXXFLAGS` and the like;
- the runtime's sources, and the environment that the compiler reads (`CPATH`,
  `CPLUS_INCLUDE_PATH`, `NIX_CFLAGS_COMPILE*`, ...);
- the archiver: its name, arguments and program file.

So a sanitizer build, a `-nochecks` build, an LTO build, another compiler and another archiver each have
a library of their own, and none of them is built twice. A change of a runtime source changes the
name of the runtime folder (it is a hash of the embedded files), so there is nothing to invalidate.

**Building.** One process at a time builds a key: it takes an advisory lock (`lib/<key>.lock`), and the
others wait for it and use its library. A lock that can't be taken (a file system without locks), or a
wait of ten minutes, makes a process build the library itself; the result is renamed into place, so
that costs only time. The compiles and the archive run in a temporary folder (`lib/.tmp-*`), and the
finished library is renamed to its name: a build that is killed leaves no library, and a leftover
folder is removed after an hour. A library is checked before it is used (a regular file that starts with
the archive magic); a damaged one is rebuilt.

**Without a library.** With no cache folder, with `GOCPP_NORTLIB=1`, or when there is no archiver
(and no `-lto`), the runtime's object files are linked one by one, as before (`obj/<key>/` in the
cache, or the work folder).

**The runtime has no static initializer that a library could drop**, because a library member is taken only for a symbol that
something wants, so a constructor of a global in an object that nobody references would never run.
The runtime's state is constant-initialized or lazy, and the programs' initialization is called by
`gp_init_all()`. Two objects have an initializer (`math.o`: `has_fma`, `has_sse41`; `timer.o`: the timer
heap), for data that only their own symbols reach, so any use of them takes the object and runs it. A test
(`TestRuntimeObjectsHaveNoStaticInit`) reads the objects: a new initializer in another object fails it.

## The archiver

The library is made by the archiver that belongs to the compiler (`GOCPP_AR`, a command with
arguments, replaces the search):

| Build | Compiler | Archiver |
|---|---|---|
| no LTO | any | the `ar` next to the compiler (with the compiler's target prefix, `x86_64-w64-mingw32-ar`), else in `PATH` |
| `-lto` | g++ | `gcc-ar` (or `gcc-ar-<major>`), else `ar --plugin <liblto_plugin.so>` (the file `g++ -print-file-name=` gives) |
| `-lto` | clang++ | `llvm-ar` (or `llvm-ar-<major>`), else `ar --plugin <LLVMgold.so>` (`-print-file-name`, or the `-plugin` argument of the link command that `clang -flto -###` prints: a Nix wrapper adds it only there) |

When the compiler's name has a prefix (`x86_64-w64-mingw32-g++`, or a link called `my-g++`) and the prefixed
tool is not found, the unprefixed one is the next try (`ar`, `gcc-ar`), with LTO as without.

The operation is `rcsD` (`rcs` on macOS): replace, create, write the symbol index, and zero the times and
owners so that the library is the same file whenever it is made.

A plain GNU `ar` does not read LTO objects: it prints "plugin needed to handle lto object", exits with
status 0 and writes an archive without an index, and the link fails later. That is why `-lto` never falls
back to a plain `ar`; when no archiver is found it stops with

```
-lto needs an archiver that reads the LTO objects of g++ (a plain ar writes a library without an
index: "plugin needed to handle lto object"); looked for gcc-ar, gcc-ar-15, ar with liblto_plugin.so
next to /usr/bin/g++ and in PATH: install gcc-ar, or set GOCPP_AR
```

## Link-time optimization

`gocpp build -lto` (also `run` and `test`) adds `-flto=auto` (g++: parallel link-time compiles) or
`-flto=thin` (clang++: ThinLTO) to the compiles of the runtime and of the generated C++ and to the
link. It does not change `-O`: combine it with `GOCPP_CXXFLAGS=-O2`. The C and C++ files of the
packages and the libraries of modules compile as before, and the linker takes their ordinary
objects next to the LTO ones. The flag is part of every key, including the precompiled header's.

## The precompiled header

The headers that every generated unit includes (`gocpp/rt.hpp` and what it includes) are compiled
once into a precompiled header per key (compiler, flags, sanitizer, `-flto`): clang `-include-pch
rt.hpp.pch`, g++ `rt.hpp.gch` found next to `rt.hpp`. It is used for a plain compile of a generated unit
only: with flags of the user or of the environment, a response file or a config file in the compiler's
arguments, or `GOCPP_NOPCH=1`, there is none, because those flags change what the header means. It is
built in the background while the runtime library compiles (the first build) and the compiles go on
without it until it is ready.

**A header never changes what a program means.** The list of files it was made from is saved next to it
with their hashes (g++ does not list them for a compile that uses it, so the object cache needs the
list). Once for each build the files are compared with the list; a header that was edited in place makes the
header stale: it is removed and rebuilt, and compiles go on without it. (clang refuses such a header
itself, g++ would go on with the text of the old one: the check is what makes both the same.) A compiler
that refuses or dies on a header gets the compile again without it; the header is removed if that works.
Tests: `TestPrecompiledHeaderInvalidatedByEdit` (edits `rt.hpp`: the stale header is not used and the next build
sees the edit), `TestPrecompiledHeaderKeyedByFlags`.

## The link command

One function (`linkArgs`) makes it, in this order:

| Part | Content |
|---|---|
| driver flags | the debug flags, the sanitizer, the flags of the target (Windows: `-static`, `-Wl,--stack,8388608`, and `-pthread` for g++), `-flto=...` (once, also when `GOCPP_LDFLAGS` has `-flto`); then `-Wl,--gc-sections` (not on macOS) |
| objects | the generated units in the order of the packages (the loader's order: dependencies first) and of the files, then the objects of the packages' own C and C++ files, then the objects of the modules' libraries; each file once |
| runtime | `libgocpprt.a` (or its object files), after everything that uses it |
| system | the link flags of the packages (`//gocpp:` directives, pkg-config), the packages that import the others first, deduplicated |
| user | `GOCPP_LDFLAGS`, as given |
| output | `-o out` |

A static library is read once from front to back, so what needs a library comes before it. The
deduplication (`dedupLinkFlags`) keeps what that order needs:

- `-lname` and `-l name`: once, the **last** one stays (it is behind more of its users);
- `-Ldir`: once, the **first** one stays (the search order);
- a file named by its path (`x.o`, `libx.a`): once, the last one;
- `-pthread`, `-rdynamic`, `-static`, `-framework X`, `-Wl,--as-needed` and the like: once, the first one
  (a toggle back, `--as-needed ... --no-as-needed ... --as-needed`, stays);
- a library under another linker state (`-Wl,-Bstatic`, `--as-needed`, `--push-state`) is another library;
- a line with `-Wl,--start-group` or `--whole-archive` keeps its libraries as they are (only the folders and
  the idempotent flags are deduplicated);
- every other flag, and a flag with an argument (`-arch arm64`, `-Xlinker x`), as it comes.

The command is the same for the same program: the order never depends on which compile finished first
(`TestLinkCommandReproducible`; `TestLinkArgsGolden` has the exact text for each target). `gocpp build -x`
prints it with the work folder as `$WORK`.

**Response files.** A command longer than 28000 bytes for a Windows target (32 K is its limit), or
120000 elsewhere, puts the objects, the runtime and the flags in `link.rsp` in the work folder (kept with
`-work`): `@link.rsp` goes on the command line, between the driver flags and `-o`. The plan that `build -n`
and `export` print keeps the files and flags themselves (the work folder is gone by then).

## Measurements

Measured 2026-10-18 on the 128-core build machine, **not quiet**: the load average was 120 to 530
(other jobs), so wall-clock seconds are inflated and noisy, and each cell is one run. The second
number of a cell is CPU time (user plus system of the whole process tree), which load disturbs much
less; read the CPU numbers. "before" is the build of `dev` at 90b779f (the runtime as its object files, the same
precompiled header), "after" is this step. Programs: `hello` (one package), `chain20` (a module of 20
packages in a chain), `stdheavy` (`<vector>`, `<map>`, `<string>`, `<algorithm>`, `<filesystem>` from `.gp`).
Cells are wall / CPU seconds. cold: empty cache (the runtime, the header and the program);
no-op: the same build again (this version of the driver links every time: the up-to-date check of
6.5 skips that); changed: one `.gp` file edited, runtime and header warm.

| compiler | program | cold before | cold after | no-op before | no-op after | changed before | changed after |
|---|---|---|---|---|---|---|---|
| g++ 15 | hello | 6.65 / 46.8 | 5.88 / 46.5 | 0.39 / 0.32 | 0.39 / 0.31 | 1.01 / 1.35 | 0.81 / 1.29 |
| g++ 15 | chain20 | 6.26 / 57.9 | 5.69 / 55.2 | 0.46 / 0.44 | 0.62 / 0.64 | 1.12 / 1.75 | 1.44 / 1.97 |
| g++ 15 | stdheavy | 7.64 / 51.0 | 8.16 / 50.9 | 0.53 / 0.52 | 0.40 / 0.35 | 2.87 / 3.86 | 2.25 / 4.00 |
| clang++ 21 | hello | 4.88 / 46.5 | 4.58 / 46.4 | 0.39 / 0.41 | 0.40 / 0.43 | 0.88 / 1.40 | 0.77 / 1.24 |
| clang++ 21 | chain20 | 5.41 / 57.0 | 5.65 / 64.2 | 0.44 / 0.64 | 0.48 / 0.66 | 1.30 / 2.22 | 1.15 / 1.89 |
| clang++ 21 | stdheavy | 7.17 / 52.4 | 7.27 / 49.6 | 0.48 / 0.51 | 0.45 / 0.47 | 1.88 / 3.56 | 1.57 / 3.34 |

**Re-measured after the rebase onto the merged executor and stamp (2026-10-20, load 180 to 400, so only the
shape counts).** Wall / CPU seconds; `dev` is the build without this step, `go build` is the same program
written as Go (own cache, cold means an empty `GOCACHE`):

| program | step | g++ dev | g++ here | clang++ dev | clang++ here | go build |
|---|---|---|---|---|---|---|
| hello | cold | 4.2 / 41 | 4.9 / 59 (3.2 / 29 .. 7.1 / 51 in 3 repeats) | 2.6 / 32 | 6.5 / 52 | 5.2 / 12 |
| hello | no-op | 0.03 | 0.02 | 0.05 | 0.14 | 0.05 |
| hello | one file changed | 0.8 / 1.1 | 0.5 / 0.7 | 0.9 / 1.0 | 0.9 / 1.0 | 0.28 / 0.26 |
| 20 packages | cold | 5.2 / 86 | 3.1 / 55 | 8.8 / 60 | 6.0 / 61 | 4.9 / 13 |
| 20 packages | no-op | 0.03 | 0.04 | 0.03 | 0.12 | 0.04 |
| 20 packages | one file changed | 1.0 / 1.4 | 0.5 / 0.9 | 1.1 / 1.4 | 0.9 / 1.4 | 0.15 / 0.65 |

The cold runs vary by a factor of two between repeats on this loaded machine (the same build took 29 to
51 s of CPU), so cold hello is a tie. The no-op build is the up-to-date check of 6.5 (it does not reach the
link). The changed build, which links, is the same or a little faster. `go build` links in a fraction of
the time and compiles Go, not C++: the gap is the C++ compiler's, not the link step's.

**The library is neutral for speed**, as expected: the same objects are compiled and the linker reads
the same code (the CPU of the no-op and changed builds is within the noise). What it gives is the
key, the lock, one input for the stamp and the up-to-date check, and a shorter link line and trace.

**The precompiled header wins and stays.** The same builds with `GOCPP_NOPCH=1` (after), CPU seconds:

| compiler | program | cold with / without header | changed with / without header |
|---|---|---|---|
| g++ | hello | 46.5 / 52.7 | 1.29 / 3.4 |
| g++ | chain20 | 55.2 / 76.6 | 1.97 / 4.5 |
| g++ | stdheavy | 50.9 / 48.5 | 4.00 / 7.1 |
| clang++ | hello | 46.4 / 51.5 | 1.24 / 4.4 |
| clang++ | chain20 | 64.2 / 83.9 | 1.89 / 4.3 |
| clang++ | stdheavy | 49.6 / 51.5 | 3.34 / 6.9 |

The header halves the CPU of an incremental build (every unit includes the runtime's headers) and
saves about a quarter of the cold build of a many-package program; on a cold hello or a std-heavy
program it is a wash (its own build is paid once, in the background).

**LTO** (`-lto`, no `-O` flag, so the cost is the link itself), changed build, wall / CPU: g++ hello
4.4 / 4.9 (without LTO 0.8 / 1.3), chain20 4.8 / 5.5, stdheavy 6.7 / 8.4; clang++ (ThinLTO) hello 1.6 / 2.8,
chain20 1.9 / 3.6, stdheavy 2.4 / 4.8. The link with `-flto=auto` costs about 3.5 s of CPU on
a hello world; ThinLTO about 1.5 s. A cold LTO build is as fast as a plain one in CPU for clang++
(43 / 56 / 54) and about 10 % more for g++ (51 / 79 / 55). `-lto` is for release builds.

**The Nix compiler wrapper and the link.** The linker wrapper of the Nix dev shell runs `realpath` once for
each `-L` folder (and for those of the wrapper's own flags), a process each. Linking a hello world object, median of
5, wall (CPU is the same): 0 folders 0.19 s, 10 folders 0.20 s, 40 folders 0.30 s, 160 folders 0.71 s
(g++; clang++ is 0.19, 0.24, 0.35, 0.72), about 3.3 ms per folder; 160 copies of one folder cost the
same as 160 different ones (0.72 s) and one folder 0.17 s. So the deduplication of `-L` (the first one
stays) is the mitigation: the packages of a module that all list the same pkg-config folder cost
one `realpath`, and a program of many packages with the same `-L` flags links as fast as one
package. A response file does not help (the wrapper reads the arguments after expanding it).


## Environment

| Variable | Effect |
|---|---|
| `GOCPP_AR` | the archiver for the runtime library, a command (`llvm-ar-21`, `ar --plugin /x/LLVMgold.so`); replaces the search |
| `GOCPP_NORTLIB=1` | no runtime library: link the object files |
| `GOCPP_NOPCH=1` | no precompiled header |
