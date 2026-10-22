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

### A work folder that is removed

The work folder is a new folder in `TMPDIR` (`gocpp-build<N>`), with two small marker files in it (`.gocpp-work`
and `gocpp-work`: one name with a dot and one without, so that a cleaner that runs `rm -rf folder/*` is seen as well).
Nothing in gocpp removes another build's work folder, but other things on a machine do: a cleaner of `/tmp`,
a script that clears `TMPDIR`, a tool that shares the temporary folder of another session. A build that finds
its folder gone (or emptied: a marker is missing, or the program it linked is not there) makes the folder
again, with its parent if that is gone too, says so on standard error
(`gocpp build: the work folder … was removed while the build ran (another process or a cleanup of the temporary
folder deleted it; GOCPP_WORKLOST=fail makes this an error): building again`), starts with a clean record and clean
counts, and builds again, three more times at most; then it stops with the message that something else keeps
deleting it. It is never reported as an internal error of the C++ (`generated C++ did not compile`), and a work
folder that is there is never taken for a lost one.

`GOCPP_WORKLOST=fail` turns the self-healing off: the build stops at once with status 4 and the message, so that a
real bug of gocpp (a path that deletes what it should not) is not hidden behind a second try. The tests and
campaigns of the repository (`tests/`, `gocpp test` of the standard library's runner, `tools/gobench`,
`tools/difftest`, the fuzz campaigns, `tools/buildstress` without its cleaner) set it; users keep the default.

A removal in the milliseconds between the check of the program after the link and the start of the program
(`gocpp run`) or its move to `-o` (`gocpp build`) is not seen and not retried. `tools/buildstress`
([tests/README.md](../../tests/README.md#concurrent-builds)) makes all of this happen on purpose.

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

## Compiler launchers and external caches

A compiler command can already include a launcher; no separate `GOCPP_CCACHE` switch is needed:

```sh
CXX='ccache g++' gocpp build .
GOCPP_CXX='sccache clang++' gocpp build .
```

For a host build, `GOCPP_CXX` wins over `CXX`. With neither set, a non-Windows host tries
`clang++` and `g++`; a Windows host follows `GOCPP_TOOLCHAIN` and the
[MinGW-w64 selection rules](#gocppexe-on-a-windows-host). For a cross build, use `GOCPP_CXX` to
name the target's compiler explicitly; the host's `CXX` is ignored. A chosen command that fails
is an error, not a request to fall back to an uncached compiler. The launcher must be installed
separately and support the compiler command that follows it; gocpp neither installs it nor
configures a remote cache.

Commands are split into words, **not run through a shell**. Quote a whole word to retain spaces:
`GOCPP_CXX='"/opt/cache tools/ccache" "/opt/compiler tools/g++" -m64'`. Quotes have no escapes;
`$HOME`, `~`, pipes and shell substitutions are not expanded. The launcher receives its arguments
first, followed by gocpp's flags. Compiler detection (`--version`, `-dM -E`) goes through the same
command, so the family and version are the underlying compiler's, not the cache program's. For
package `.c` files the matching C compiler is substituted (`ccache g++` becomes `ccache gcc`),
with the other arguments retained; set `GOCPP_CC` explicitly if those arguments are C++-only.
Long commands put gocpp's arguments in a response file, leaving the launcher and its compiler
argument outside it. `-x` still shows the original command. The external tool must support the
compiler's response-file syntax.

**Two independent cache layers.** Keep gocpp's object cache and up-to-date check enabled normally:
when they answer, the external cache is not called. A gocpp `# cache hit` or `hit=` count is only
an **internal** hit. To measure an external cache, bypass the internal object cache and up-to-date
check and read the external tool's own statistics. For example, with ccache already installed,
in a disposable shell session from a small project's folder:

```sh
scratch=$(mktemp -d)
export XDG_CACHE_HOME="$scratch/xdg"
export CCACHE_DIR="$scratch/ccache"
export GOCPP_CXX='ccache g++'
export GOCPP_NOOBJCACHE=1 GOCPP_NOUPTODATE=1
ccache --zero-stats
gocpp build -x -o "$scratch/app" .
ccache --show-stats
gocpp build -x -o "$scratch/app" .
ccache --show-stats
# Edit a small source or a header, build again, then read the statistics again.
```

The first build has a cold runtime cache too; later builds can reuse runtime objects and the
runtime PCH even with `GOCPP_NOOBJCACHE=1`. Compare **changes in external hit/miss counts**, not
just elapsed times. `gocpp build -a` also forces runtime compiles but still allows an external
cache to answer. `GOCPP_NOPCH=1` is useful for a separate no-PCH comparison; it is not a default
recommendation. PCH reuse has compiler- and cache-specific requirements. In particular, do not
silently enable ccache's PCH sloppiness options just to obtain a hit. Work folders change between
builds, and debug paths, generated paths and PCHs can prevent external hits. Do not turn off
path hashing or other correctness checks without understanding the external tool's rules.

**Identity limit.** gocpp keys its caches by the launcher path and program file, command arguments,
and detected compiler family/version. It does not watch an underlying executable named only in
those arguments: replacing that compiler in place (or changing what its name resolves to in
`PATH`) can leave cached detection and objects unchanged. Prefer an absolute, versioned compiler
path. After a toolchain change behind a launcher, start a new gocpp process and use a fresh gocpp
cache or `gocpp clean -cache`; clearing only ccache/sccache does not clear gocpp's answers. Arbitrary
wrappers that choose a compiler from their own configuration have the same limit.

The launcher protocol is covered by deterministic fake-launcher tests. Actual hit rates and
speedups depend on the installed cache, compiler, flags and workload; those tests are not evidence
of a real ccache/sccache hit. See also [compile-time advice](../guides/generics-compile-time.md).

## Build flags, tags and the user's own compiler flags

`gocpp build` and `gocpp run` take these flags (`gocpp build -h` has the text; `-n`, `-emit-llvm`
and `-o` are for `build` only; `-compdb` is also a flag of `run`, `test` and `debug`):

| Flag | Meaning |
|---|---|
| `-o file-or-folder` | where the program goes |
| `-p n` | at most n compiles at a time (default `GOMAXPROCS`) |
| `-x`, `-v`, `-work` | print the commands, print the packages, keep the work folder |
| `-n` | print the commands and run none (the generated C++ is written; no object, no program, nothing recorded) |
| `-a` | compile everything again: no object cache, no runtime cache, no up-to-date check; the new objects are stored |
| `-tags a,b` | more build tags (comma list; the last `-tags` is the list), for `//go:build` lines and the file names of `.gp`, `.c` and `.cc/.cpp/.cxx` files; also in `check`, `emit` and `list` |
| `-debug` / `-release` / `-race` | `-O0 -g` (what a build is by default) / `-O2 -flto -g0`, no debug information (see [Program size](#program-size)) / ThreadSanitizer, on linux with Go's race report, exit code 66 and `GORACE` ([the race detector](../guides/race.md)); on darwin ThreadSanitizer's own text. `-debug` with `-release`, `-race` with `-release` or `-asan`, and `-race` for windows are errors |
| `-strip` | drop the symbol table and the debug information from the program (the link's `-s`; on macOS `-Wl,-S -Wl,-x`). With any mode but `-debug`, `-race` and `-asan` (errors: their tools need the symbols); `-release -strip` is the small build |
| `-asan`, `-nochecks`, `-cpperrors` | as before |
| `-gp:m`, `-gp:m=2` | print the decisions of the escape analysis in the form of `go build -gcflags=-m` (`moved to heap: x`, `&T{} escapes to heap`, `&x does not escape`, `leaking param: p`, `v borrows xs[i]`); `=2` says why. Only the program's own packages, not the standard library. On `build`, `run` and `emit` ([ADR-114](../adr/114-escape-analysis-keeps-non-escaping-objects-on-the-stack.md)) |
| `-gp:noescape` | turn the escape analysis off: every variable whose address is taken is boxed and every `&T{...}` and `new(T)` allocates (to find out whether a bug is the analysis's) |
| `-cflags`, `-cxxflags`, `-ldflags` | the user's flags: see below |
| `-emit-llvm[=ll\|bc]`, `-compdb[=path]` | files for other tools: see below |

### Program size

A build without a flag is a debug build on purpose: `-O0 -g` is what gdb, lldb and VS Code need (source
lines of the `.gp` files through `#line`, the values of variables, `gocpp debug`), and what a panic's trace reads its
function names and `file.gp:line` from. The cost is size: the debug information is most of the file, and a Windows
program also carries the whole COFF symbol table. Measured with g++ 15.3 on Linux, `-os windows` with the dev shell's
MinGW-w64 (the programs are `examples/hello`, `examples/sqlite`, and the Dear ImGui demos of `demos/`):

| program | no flag | `-strip` | `-release` | `-release -strip` |
|---|---|---|---|---|
| hello, linux | 5.6 MB | 585 KB | 375 KB | 228 KB |
| sqlite example, linux | 6.2 MB | 679 KB | 429 KB | 261 KB |
| hello, windows | 13.3 MB | 2.0 MB | 3.4 MB | 0.44 MB |
| `demos/imgui` (OpenGL), windows | 17.8 MB | 3.3 MB | 5.0 MB | 2.5 MB |
| `demos/imgui-dx11`, windows | 63.4 MB | 5.9 MB | 7.4 MB | 2.4 MB |

Where the bytes of the Windows hello world are (13.3 MB): 6.7 MB of `.debug_*` sections (5.0 MB of it `.debug_info`),
4.6 MB of COFF symbols (the long mangled names are in the string table), and 2.0 MB of program (1.5 MB of code at `-O0`).
The `-release -strip` file is 0.44 MB; the code is libstdc++ and the runtime. The runtime formats floats with its own
code (`runtime/src/ftoa.cpp`, a port of Go's `strconv`) and keeps its one `std::ostringstream` in its own object
(`runtime/src/fmtstream.cpp`), so that no locale facet (`time_get`, `money_get`, about 0.75 MB) is linked into a program
that does not print through a stream; `runtime/locale_test.go` and `tests/windows_size_test.go` guard both. Only the
`-release -strip` column of the hello and Direct3D 11 rows was measured again after this change; the other columns
and the OpenGL row are from before it. On Linux the program links
libstdc++ and libc dynamically, so its file holds only the runtime and the program. Every link already uses
`--gc-sections` with `-ffunction-sections -fdata-sections` (a program links only what it reaches), and a Windows
program links statically (`-static`), so the `.exe` needs no DLLs.

- **`-release`** is `-O2 -flto` and `-g0` (at the compiles and at the link: with `-flto` the compiler writes the debug
  information at the link from the `-g` of the link line, so `-g0` there too). The symbol table stays, so a panic
  still prints `main.main()` and the other Go function names, but no file and line (`?:0`; gdb has no source lines). To
  keep the debug information of an optimized program, for `perf` or a debugger: `-release -cflags=-g -ldflags=-g`
  (later flags win; the program is then as large as with `-O2` and `-g`: 5.2 MB for the Linux hello world).
- **`-strip`** drops the symbol table and every `.debug_*` section, also the runtime's gdb printers (`.debug_gdb_scripts`,
  84 KB). A panic prints `?()` with `pc=0x...` addresses (`addr2line` on an unstripped
  build of the same program with the same flags names them). It is `-ldflags=-s` with a check: a stripped debug or
  sanitizer build is an error.
- Both flags are part of the key of the up-to-date check (a build with another flag links again), and `-release`'s
  flags are part of the keys of the object cache and of the runtime library (`-strip` changes the link only).
- `-Os` was tried (154 KB for the Linux hello world, against 224 KB) and is not the default: g++ 15.3 for MinGW
  fails with an internal compiler error in `-Os -flto` on the Windows hello world and the dx11 demo. Use it with `-cflags=-Os -ldflags=-Os` on Linux
  if size matters more than speed.

**The user's flags are trusted.** A package's `//gocpp:cflags` line goes through the allow-list of Phase 04; the
flags the user gives are not checked, as `CGO_CFLAGS` are not. They are layered, a later flag winning where
the last one wins:

| | Compile (C and C++) | Compile (C++ only) | Link |
|---|---|---|---|
| 1. the mode | `-fsanitize=thread` (linux: and `-DGOCPP_RACE_MODE=1`), `-O2 -flto -g0` | | `-fsanitize=thread` (linux: and `-Wl,--export-dynamic-symbol=__tsan_default_options`), `-O2 -flto -g0`, `-s` for `-strip` |
| 2. the environment | `GOCPP_CFLAGS` | `GOCPP_CXXFLAGS` | `GOCPP_LDFLAGS` |
| 3. the build flags | `-cflags` | `-cxxflags` | `-ldflags` |

All of them go to every compile of the program: the generated C++, the package's C and C++ files, the
libraries of modules and gocpp's runtime. In a compile the order is the mode, the environment, the build flags, the
package's flags (`//gocpp:cflags`, `pkg-config`), then gocpp's language flags last (so that no flag turns off
`-fexceptions`, `-fwrapv` and the like). At the link they come after the package's link flags. A value is split like
`CC` is: spaces separate the words, and a quote keeps spaces only when it **starts a word** (Go's `quoted.Split`):
`-cxxflags="-DFOO='a b'"` is the two words `-DFOO='a` and `b'`; quote the whole word, `-cxxflags="'-DFOO=a b'"`
(the same in `GOCPP_CFLAGS`, `GOCPP_CXXFLAGS`, `GOCPP_LDFLAGS`). A flag may be given twice and adds.

The files that these flags make the compiler read are in the cache keys by their **content**: the plugins
(`-fplugin=`, `-fpass-plugin=`, `-Xclang -load -Xclang`), response files (`@file`) and the files of `-include` and
`-imacros`. They are in the key of the runtime's objects always, and in the key of the program's objects when the
object cache takes the compile: a compile with `@file`, `-fplugin`, `-Xclang` or the like is never kept in the object
cache (the cache can't see what such a flag reads), so those units are compiled in every build and the build is never
"up to date" (`-fpass-plugin=` and `-include` are cached, keyed by content). Relative names in the flags
(`-I`, `-L`, `--sysroot=`, `-isystem`, `-iquote`, `-idirafter`, `-Wl,-Map`, plugins, `@file`, `-include`, `-imacros`,
and a library or object file that exists under the current folder) are made absolute from the folder gocpp runs in,
because the compiles run in a private work folder; `-Wl,-rpath` is left as written. A `-include` or `-imacros` name that
is not a file under the current folder is looked up like `#include "name"` in the absolute folders of `-iquote`, `-I`,
`-isystem` and `-idirafter` (`-Iinc -include hh.h` with `hh.h` only in `inc/` is tracked by content too). The limits: the files that a response
file names in turn, and the files a plugin itself reads, are not tracked (`gocpp build -a` compiles all again); the
content hash of a file is remembered by its name, size and time for the life of the process (the language server).

**All of this is in the key of the up-to-date check**, and the flags are in the key of every object in the
object cache (they are on the compile's command line): a changed `-cxxflags`, `-tags`, mode, or `GOCPP_CXXFLAGS`
compiles what depends on it and links again, and the build with the old flags is found again when the flags
come back. A build with `-n`, `-a`, `-emit-llvm` or `-compdb` is not recorded (it has files that the check does
not look at); the `compile_commands.json` that a debug build writes by itself does not count (see below).

**Compiler plugins.** The user's flags may load code into the compiler: LLVM pass plugins with clang
(`-cxxflags=-fpass-plugin=/path/p.so`) and front-end plugins (`-fplugin=/path/p.so` with g++ or clang,
`-fplugin-arg-p-key=value`, clang's `-Xclang -load -Xclang p.so`). A package can never load one: the
allow-list refuses `-fplugin` in a `//gocpp:` line (`GOCPP_CFLAGS_ALLOW` stays the escape hatch). A relative
path with a folder in it is made absolute from the current folder (the compiles run in a work folder); a bare name is
the compiler's to look up. The plugin file is part of the object cache's key by the hash of its content (and of the
up-to-date key by its size and time), so a plugin that is built again compiles again. The probes that read
your C and C++ headers (the editor's and `gocpp check`'s) do not load plugins: they are meant for the program's
compiles. See [guides/llvm-tools.md](../guides/llvm-tools.md).

**`-emit-llvm`** (clang only: g++ gets an error that says so) also writes the LLVM IR of every translation unit
of the program, the generated C++, the package's own C and C++ files and the libraries' (not the runtime's), to
`<program>.llvm/` next to the program, in the text form (`.ll`) or with `-emit-llvm=bc` as bitcode. The files are
`gen/gpkg/<import path>/<file>.gp.ll`, `src/<package>/<file>.ll` and `lib/<module>/<file>.ll`. The IR comes from a second
run of the compiler with the flags of the object's compile (plugins included), `-c` turned into `-S -emit-llvm`. The folder
is emptied at the next build with the flag, only if it has the marker file `.gocpp-llvm` that gocpp writes (or is empty):
gocpp does not remove what it did not make.

### Paths with a double quote or non-ASCII letters

The project, the cache (`XDG_CACHE_HOME`, where the runtime is built) and `TMPDIR` can be in folders with spaces,
non-ASCII letters (`~/проект`, `C:\Users\Zoë`) and, on Linux and macOS, a double quote. Both compilers have bugs with
such paths, and gocpp works around them:

- Nix's clang wrapper writes its arguments to a response file with bash's `printf %q`, which in the `C` locale
  (the one gocpp runs compilers in, for the same messages everywhere) turns a non-ASCII path into `$'\320...'`, which clang
  then reads as another file name. gocpp sets `NIX_CC_USE_RESPONSE_FILE=0` for its compiler runs (unless you set a
  non-empty value yourself); other compilers ignore it.
- g++ writes `# 70 "<path>" 1` into its assembly for every inline `asm` statement without escaping a double quote in the
  path, and the assembler fails. For a g++ compile that has a path with a double quote in it (the source, a folder of
  `-I`, `-isystem`, `-iquote`, `-idirafter` or `--sysroot`, a `-include` or `-imacros` file; a quote in `-D` does not
  count), gocpp compiles to assembly (`-S`), escapes the quotes of those lines and assembles it (`-x assembler`, with
  the `-m...`, `-Wa,...` and `-Xassembler` flags), so `-x` shows one command, run as two steps. The limit: with `-flto`
  (`-release`, `-lto`) that unit is compiled **without LTO** (the marker would be written at the link, out of
  reach): it is not optimized together with the other units, and the LTO link takes it as a plain object.
- clang writes a forced include as `#include "<path>"` into its `<built-in>` buffer without escaping a double quote, so for
  a cache folder with a quote the runtime's `gocpp/nofortify.h` is included by name through `-iquote` of the runtime's
  include folder.

## Files for other tools

**`compile_commands.json`.** A debug build (the default, `-debug`, `-race`) of `gocpp build`, `run`, `test` or `debug`
writes it by itself (ADR-044, amended 2026-10-19), so that clangd understands the generated C++ you step into; a
`-release` build or `-n` writes it only with `-compdb`, and `-compdb=false` turns it off. `gocpp build -compdb` (with a
program) or `gocpp export compdb` (nothing is built) writes
the JSON compilation database that clangd and others read: one entry for every translation unit with the exact
command (`arguments`), `directory` and `file`. The units are the generated C++ of every package, the package's own C and C++
files, the sources of the libraries of modules, and the files of gocpp's runtime. The paths are stable: the generated
files are in `gen/<hash>` of the cache folder (the same folder for every build of the program), the runtime in `rt/`,
and the others in their own folders, so a tool can open them after the build; `-o` and the object file are left out
(the objects are in the work folder, which is gone). The entries are sorted by file, and the file is replaced
only when its content changes. Without a value `-compdb` writes to the generated folder and to the module's folder
(the one with `gocpp.mod`; else the package's folder); with a value, to that file or folder. What a debug build writes
by itself is the same, except that the module's copy is only `gocpp build`'s for the host, of a program in a module
(`run`, `test`, `debug`, a cross-compile and a loose `.gp` file write the generated folder's copy only; with several
programs in a module it holds the last one built), that nothing is written when there is no cache folder (the build is
in a folder that is removed), that it never replaces a symbolic link or a file that gocpp did not write (it says so;
`-compdb` does replace a regular file, and a link by a file: it never writes through a link), and that a file that cannot be written is a warning,
not an error. `-compdb=false` leaves an old file as it is. An up-to-date build leaves the files as they are. clangd with g++ or
under Nix wants `--query-driver=<compiler>` so that it finds the compiler's own include folders.

**`gocpp export cmake [-o folder] [build flags] packages`** writes a folder (default `<program>-cmake`) with a
`CMakeLists.txt`, `gen/` (a copy of the generated files) and `runtime/` (gocpp's runtime), which builds the program
without gocpp: `cmake -S folder -B build -DCMAKE_CXX_COMPILER=<compiler> && cmake --build build`. The flags of every
file are exactly those of the build that the export is made from, so the file checks that the compiler is of the same
family (clang or g++). The limits: the project's own C and C++ files, their headers, the libraries of modules and the
link libraries are named by their absolute paths on the machine that made the export; the generated C++ names the `.gp`
files and the headers of modules by absolute paths too (in `#line` and `#include` lines); the file is for the compiler,
the options and the system of this build, and it has to be made again when the `.gp` files change. A folder that exists,
is not empty and has no `CMakeLists.txt` that gocpp wrote is never touched.

## `gocpp list` and `gocpp env`

`gocpp list [-deps] [-e] [-json | -f template] [-tags list] [packages]` prints the packages that the arguments name, like `go list`: import
paths, or each package as a JSON object or through a `text/template` (functions `join` and `json`). The fields are those
of `go list` that mean something here (`Dir`, `ImportPath`, `Name`, `Standard`, `DepOnly`, `Module`, `Imports`, `Deps`,
`Incomplete`, `Error`) with `GpFiles`, `IgnoredGpFiles` and `TestGpFiles` for the `.gp` files, `CFiles` and `CXXFiles`, and
`CImports` (the C and C++ headers that the files import, as written), `CFlags`, `LDFlags`, `Link` and `PkgConfig` (the directives). Empty
fields are left out of the JSON, lists are sorted, and `-deps` puts every package after the ones it imports. `-e` keeps going
with the error in the package's `Error` field and exits with status 0, like `go list -e`. The patterns `std` and `all` are not
supported.

`gocpp env [-json] [name ...]` prints `NAME='value'` lines (sorted; `set NAME=value` on Windows) for every variable below
with the value in use, then the values gocpp finds out: `GOCPPCACHE`, `GOCPPRUNTIME`, `GOCPPMOD`, `GOCPPCXX`, `GOCPPCXXVERSION`,
`GOCPPCC`, `GOCPPVERSION`, and for a Windows target `GOCPPCXXSOURCE` (where the compiler came from). With names it prints their values only. `GOCPP_MODAUTH` is shown as `(set)`. A test keeps the list
complete: it fails when the Go code names a `GOCPP_` variable that `gocpp env` does not list.

| Variable | Meaning |
|---|---|
| `CXX`, `GOCPP_CXX`, `GOCPP_CC` | the compilers (`GOCPP_CXX`, else `CXX`, else clang++, else g++) |
| `GOCPP_CFLAGS`, `GOCPP_CXXFLAGS`, `GOCPP_LDFLAGS` | the user's trusted flags (above) |
| `GOCPP_CFLAGS_ALLOW`, `GOCPP_CFLAGS_DISALLOW`, `GOCPP_LDFLAGS_ALLOW`, `GOCPP_LDFLAGS_DISALLOW` | the allow-list for the flags of `//gocpp:` lines |
| `GOCPP_NOOBJCACHE`, `GOCPP_OBJCACHE_SIZE`, `GOCPP_NOPCH`, `GOCPP_NOUPTODATE`, `GOCPP_NOPROBECACHE`, `GOCPP_PROBE_CACHE` | the caches |
| `GOCPP_MODCACHE`, `GOCPP_MODFLAGS`, `GOCPP_MODPROXY`, `GOCPP_MODURL`, `GOCPP_MODAUTH`, `GOCPP_MODCACERT`, `GOCPP_MODTIMEOUT`, `GOCPP_OFFLINE` | modules |
| `GOCPP_LSP_CACHE`, `GOCPP_LSP_CACHE_AGE`, `GOCPP_LSP_CACHE_SIZE` | the language server's cache |
| `GOCPP_GATE_PHASE`, `GOCPP_CPUPROFILE` | development |
| `GOCPP_COLOR`, `GOCPP_EXCERPT` | `always` or `never`: the diagnostics' color and source excerpt (the `-color` and `-excerpt` flags win) |
| `GOCPP_TOOLCHAIN`, `GOCPP_TOOLCHAIN_URL`, `GOCPP_NO_MANIFEST` | the Windows compiler's source (below), and the application manifest of a Windows program (below) |
| `GOCPP_CLANG`, `GOCPP_PROBE_TIMEOUT`, `GOCPP_PROBE_JOBS`, `GOCPP_PROBE_BATCH`, `GOCPP_PROBE_PCH` | the clang++, the time limit of a run, the number of runs at the same time, the size of a batch and the precompiled header of the library headers of the typed interop probes (ADR-088, ADR-117) |
| `GOOS`, `GOARCH`, `PKG_CONFIG`, `XDG_CACHE_HOME` | the target, pkg-config, the cache folder |

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

## Cross-compilation

> [ADR-055](../adr/055-cross-compilation-targets-and-toolchain.md); the how-to is
> [guides/windows-cross.md](../guides/windows-cross.md).

The **target** of `check`, `build`, `run` and `emit` is a GOOS/GOARCH pair: `-os` and `-arch`, else
`GOCPP_OS` and `GOCPP_ARCH`, else `GOOS` and `GOARCH`, else the machine. `check` and `emit` take any pair
that `go tool dist list` knows; `build` and `run` take `linux/amd64`, `linux/arm64` and `windows/amd64`
(`gocpp build: building for darwin/arm64 is not supported (targets: ...)` for another).

What a target changes:

| | host | windows/amd64 | linux/arm64 (best effort) |
|---|---|---|---|
| compiler | `GOCPP_CXX`, `CXX`, clang++, g++ | `GOCPP_CXX` (on a Windows host also `CXX`), else by `GOCPP_TOOLCHAIN`: `x86_64-w64-mingw32-clang++` or `-g++` in PATH; on a Windows host also a plain `clang++`/`g++` that is MinGW (`-dumpmachine`) and the installs of MSYS2, Chocolatey, Scoop and WinGet's WinLibs; else the pinned llvm-mingw | `GOCPP_CXX`, else `aarch64-linux-gnu-g++` |
| compile flags | common | common, `-D_WIN32_WINNT=0x0A00` (Windows 10 API), and `-Wa,-mbig-obj` for GCC (a COFF object holds 32767 sections, which a program with many generic instances goes over) | common, and `--target=aarch64-linux-gnu` for a clang that does not name its target |
| link flags | common | `-static`, `-Wl,--stack,8388608` (the main thread has Linux's 8 MiB), `-pthread` for GCC (needed with a posix thread model, harmless with the mcf model of the dev shell's MinGW; 14.1's `.drectve` asks for the same 8 MiB stack, the two agree and survive `--gc-sections`: `TestWindowsDriverFlags`) | common |
| program name | `name` | `name.exe` (`-o prog` writes `prog.exe`; a name with an extension keeps it) | `name` |
| libraries | `libx.a`, `libx.so` | `libx.a`; `-buildmode=c-shared` writes `x.dll`, the import library `x.dll.a` and its copy `x.lib`, and `x.h` | as host |
| build constraints | the host's | `windows`, `amd64`, `_windows.go` | `linux`, `arm64`, `unix` |
| keys | as before | the target is in the compiler's identity (`CompilerID`), so the stamp, the object cache and the runtime's objects of two targets never meet | same |

The compiler for another target is not `CXX` (a Nix shell sets `CXX` for the host). The libraries of
`//gocpp:pkg-config` are asked for as usual, with the target's `PKG_CONFIG_*` of the environment.

**`GOCPP_TOOLCHAIN`** chooses where the Windows compiler comes from: `auto` (default: PATH, then the
pinned toolchain), `local` (PATH and, on Windows, the usual MinGW-w64 folders; never a download), `pinned` (the pinned toolchain, whatever is in
PATH). The pinned toolchain is llvm-mingw 20261006 (UCRT). Each host's archive has its size and SHA-256
in the binary; the download (`gocpp build` and `gocpp run` only, never `check` or the language server)
checks both before it unpacks anything, and a host with no hash fails closed. It goes to
`<cache>/toolchain/llvm-mingw-<release>-<host>/` through a temporary folder and one rename under a lock
file, read-only. `GOCPP_TOOLCHAIN_URL` is a mirror (an https folder URL with the archives under their
names). A redirect must stay on https (it may change host), a download that receives nothing for 60
seconds stops, and `GOCPP_OFFLINE=1` never downloads (a cached toolchain is used). The default of
`GOCPP_TOOLCHAIN` is the constant `toolchain.DefaultMode` (`auto` today: the download happens without
asking; `local` would make it opt-in); `gocpp env` shows it. `gocpp clean -cache` leaves the toolchain alone; `gocpp clean -toolchain` removes it.

**`GOCPP_EXEC`** (words, like `GOCPP_CXX`) is the program that `gocpp run` starts the program with, as
`go run -exec` does: `GOCPP_EXEC=wine gocpp run -os windows .`. A program for another target without it is
an error that says so before anything is built.

### gocpp.exe on a Windows host

> [ADR-081](../adr/081-windows-host-build-driver.md). Tested on a real Windows 11 PC over ssh (`tests/windows_native_test.go`).

On Windows `gocpp build` and `gocpp run` build for `windows/amd64` with a MinGW-w64 compiler, found in this order: `GOCPP_CXX`; `CXX`;
(`GOCPP_TOOLCHAIN` auto or local) `x86_64-w64-mingw32-clang++` and `-g++` in PATH, then `clang++` and `g++` in PATH **whose
`-dumpmachine` is x86-64 MinGW** (Visual Studio's LLVM, MSYS2's `msys` environment and Cygwin are not), then the folders of MSYS2
(`ucrt64`, `clang64`, `mingw64` under `MSYS2_ROOT` or `C:\msys64`), Chocolatey, Scoop, WinGet's WinLibs and `C:\mingw64`,
`C:\w64devkit`; (auto, or pinned) the pinned llvm-mingw, a zip of 190 MB that is downloaded once into
`%LocalAppData%\gocpp\toolchain\llvm-mingw-<release>-windows-amd64\` (SHA-256 and size in the binary; `GOCPP_OFFLINE` stops it;
a virus scanner holding a file for a moment is waited out). A message lists what was looked at and how to install a compiler.
`gocpp env GOCPPCXX GOCPPCXXSOURCE` says what a build would use and where it came from.

| | |
|---|---|
| cache, temporary files | `%LocalAppData%\gocpp` (`gocpp env GOCPPCACHE`), `%TEMP%` |
| program | `name.exe`; `-o prog` writes `prog.exe` (unlike `go build -o prog`, which writes the name as given; a name with an extension keeps it); linked `-static` (no DLLs next to it), main stack 8 MiB; replacing a read-only `.exe` works, a running one is an error that says so |
| application manifest | linked into every program: `longPathAware` (paths over 260 characters with the plain Win32 and C calls, when the system allows), `activeCodePage` UTF-8 (the narrow functions and `fopen` read a Go string as it is), `asInvoker`, Windows 10 (the UTF-8 code page needs 1903 or later). On by default because without it `GetACP()` is 1252 and a non-ASCII argument, variable or file name is damaged on its way to C code. Go's linker writes none. `GOCPP_NO_MANIFEST=1` leaves it out. With GCC the object replaces MinGW's `default-manifest.o` (a `-B` folder that must have no space: the cache, its short name, the work folder or the private folder `%ProgramData%\gocpp-paths-<SID>`); if there is none, a warning says the program has no manifest |
| libraries | `-buildmode=c-shared`: `x.dll` exporting the `//export` functions (`__declspec(dllexport)`) and nothing else, `x.dll.a` and `x.lib` (import libraries), `x.h`; `-o mylib` writes `mylib.dll` (`.so` is refused: a DLL is `.dll`). `c-archive`, `cpp-lib`: `.a` as on Linux |
| long commands | a compile or link line over 30,000 characters (CreateProcess takes 32,767) is passed as a response file `@cmd-N.rsp` |
| processes | each compiler runs in a job object (kill on close): canceling a build, or killing gocpp, ends the compiler's whole tree |
| files | locks are `LockFileEx` (two builds at once share one download and one runtime library); renames into the cache are repeated for a few seconds on *Access is denied* / sharing violations |
| paths the compilers read | MinGW-w64's compilers read their command line in the ANSI code page: `LOCALAPPDATA`, `TEMP`, `TMP` with a character outside it (and `TEMP`, `TMP` of more than 200 characters) are replaced at the start by the 8.3 short name or, for a name that has none (`tmp_Ł`), by a **private** folder `%ProgramData%\gocpp-paths-<SID>\...` (protected ACL: the user, SYSTEM and Administrators; an existing folder is used only if the user owns it and no one else has access to it; a folder that fails the check is refused). The program that `gocpp run` or `gocpp test` starts gets the original values back; a user's own C/C++ files, headers and `#include` paths get their short names; programs and libraries are linked under an ASCII name and renamed (ADR-081, decision 13). Spaces are fine everywhere. A user's own file below a non-ANSI folder on a volume without 8.3 names is the one case that is not solved |
| paths | the working folder and absolute patterns are spelled as the file system stores them, drive letter in capitals; diagnostics print `.\x.gp:3:4` or the absolute path; `#line` directives escape backslashes; CRLF in `.gp` files and `gocpp.mod` is read like LF |
| `compile_commands.json` | the `arguments` form, with Windows paths (no shell quoting needed) |

Known limits: typed-interop probes and `gocpp doc` call the host's `clang++`/`g++` and not this rule yet (step 14.6); a package's own C or C++ file below a path of over 260 characters may not open in a compiler that is not long-path
aware (the generated C++ is in the cache and is fine); an argument that starts with `\\?\` is not interpreted; the console window
(typing at a prompt) is step 14.7's checklist; `-race` does not exist for Windows.

### What works on Windows

The Windows OS layer (`runtime/src/os/windows`) is step 14.1's ([ADR-053](../adr/053-windows-os-layer-on-createthread.md);
`runtime/README.md` has its tests, on Wine and on a real PC over ssh). The cross-compilation tests of this step
build programs with the driver and run them under Wine:

| works (tested under Wine) | does not yet |
|---|---|
| `fmt.Println` and `println` to console, pipe and file; UTF-8 output | UCRT (the dev shell's MinGW-w64 is msvcrt; a real PC covers UCRT) |
| arguments (Go's command line rules) and environment | arm64 Windows |
| `os.Exit`, `panic` (exit 2, Go's message and frames on stderr), a C `exit` | |
| goroutines on threads, channels, `select`, `sync` | |
| a stack overflow (message and exit 2), the 8 MiB main stack, big programs | |
| `defer`, `recover`, maps, strings, closures, generics | |
| package `os` and `syscall` (step 14.3, [ADR-076](../adr/076-os-on-windows-over-a-generic-syscall.md)): files, directories, `RemoveAll`, environment, `Stat`, hard and symbolic links, pipes, `Chmod`, long paths, `Hostname`, `Executable`, `FindProcess` | `os/exec` and `StartProcess`, `os/signal`, `net`; `time.Local` (UTC) and zone names |

**Package `os` on Windows (step 14.3).** `syscall`, `internal/syscall/windows`, `internal/poll`, `internal/filepathlite` and `os` are Go's own
Windows files, cut to what a program needs and compiled with the program; the only natives are four (a call of a function by address,
`LoadLibrary`, `GetProcAddress`). So the errors, their texts (`The system cannot find the file specified.`), the permission bits
(`Chmod` is the read-only attribute), `\\?\` long paths, `SameFile`, reparse points, case-insensitive environment names and the sharing rules of
Windows are Go's. `tests/run/os_win_*.gp` (`//go:build windows`) carry the output of **Go 1.25.14 built for Windows and run on the real PC** as
their expectation; `tests/windows_sample.txt` also has the 43 `os_*` tests whose Linux output is Windows' too, and `tests/windows_skip.txt`
says why the others are left out. Run them with
`GOCPP_TEST_TARGET=windows GOCPP_WINDOWS_SSH=user@host go test ./tests -run TestRunWindows`.

Result of `GOCPP_TEST_WINDOWS=all` (2026-10-18, dev shell: MinGW-w64 msvcrt, Wine): 827 OS-independent run
tests built for windows/amd64 and run, 826 passed on the first run; the one failure was Wine's own
`wine client error: sendmsg: Socket operation on non-socket` on stderr of a program that ends with
goroutines still blocked (the harness drops exactly that line now). Left out: 5 tests with a
`//go:build linux` line, and 2 whose expected output has the frames of a panic's trace
(`tests/windows_skip.txt`; 14.1 reads the frames now, see the rerun below). On a real PC (`GOCPP_WINDOWS_SSH`) the 42-test sample and `tests/testdata/windows` pass.
