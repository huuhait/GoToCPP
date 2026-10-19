# ADR-036: The build's actions, the executor and the up-to-date check

Status: accepted (2026-10-18), refines [ADR-008](008-build-driver.md) and [ADR-025](025-split-layout.md)

## Context

ADR-008 promised an action graph (transpile, compile, link) with content-hashed keys and a no-op rebuild
of a four-package program in under 200 ms. Step 6.2 built the simple driver: every build loads and
type-checks all packages, writes the C++ of all of them (only the files whose bytes changed), asks the
object cache about every unit, and links. Measured on the dev machine (median of 10, a busy machine): about 320 to 340 ms
for four packages, 340 to 355 ms for 20, 655 to 667 ms for 100. Where the time goes for four packages: about 30 ms
to detect the compilers (two runs of the compiler), 25 ms to load, check and emit (65 ms for a hundred
packages), 30 ms for the object cache's answers, and about 200 ms for the link, which ran although nothing
had changed (under a Nix compiler wrapper the linker is a shell script that starts `realpath` for every
library folder). For a hundred packages the cache's check that no file hides a header was quadratic (every
unit asks about every header of the packages it imports in every include folder: 15000 system calls).

## Decision

**Actions and the executor (6.7).** The compiles of a build (the packages' own C and C++ files, the
sources of the libraries of modules, the generated units, and the runtime's files) are actions that one
executor runs, at most `-p N` at a time (`GOMAXPROCS` by default, Go's flag). Actions start in a fixed
order (the user's C and C++ files, the libraries, the generated units by package), so `-p 1` is
deterministic. A failure does not stop the others: all the errors that can be found are reported, in the
order of the actions, whatever order the compiles end in (go build does the same for independent
packages); after 20 failures nothing new starts and the message says how many actions were left out. What
a compile printed when it succeeded (its warnings) is written in the order of the actions too, as soon
as every action before it has finished. A canceled context (^C) stops the starting; the compilers die
with their process groups as before. Nothing is linked after a failure. The driver prints each failure
like a single one (a log `compile.log`, `compile-2.log`, ... for the internal ones) and exits with the
highest status.

**Counters.** `-x` ends a build with `# actions: load=1 check=5 emit=3 write=1 compile=1 hit=3 link=1
runtime=0`: how many times the front end ran (a build that is up to date has `load=0`), packages checked and
emitted, generated files whose bytes changed, compiler runs, answers of the object cache, links and runtime
compiles. The tests of the incremental builds assert these counts, never times.

**The up-to-date check (6.5, 6.6).** A successful `gocpp build` leaves a *stamp* in
`<cache>/stamp/<key>.json`; the next build with the same *key* reads it first and, if everything it
recorded is as it was, prints nothing, writes nothing and exits 0 without loading the packages.
- The **key** is a hash of the patterns, the build flags, the `-o` value, the folder, the target, gocpp
  itself (its path, size, time and version) and the environment that the build reads (`GOCPP_*`, `CXX`, `CC`,
  the include-path and Nix-wrapper variables of the object cache, every `NIX_*` variable, `LD_RUN_PATH`,
  `LIBRARY_PATH`, `SOURCE_DATE_EPOCH`, `CCACHE_*`, `PATH`, `LDEMULATION`, `GNUTARGET`, `PKG_CONFIG*`); a compiler is
  known by its program file's content hash (kept for its path, size, times, inode and change time), not by its
  stat alone: the stamp, the object cache's key and the detection record all use it. A file's record has its
  permission bits too (a `chmod` of the program is a change).
- The **trace**: the loader reads files only through a `FileSystem` (it already had the interface for
  the language server); `gocpp build` gives it a recorder, so the stamp has every file the loader read (with a
  hash), every folder it listed (with a hash of the names that matter: sources, headers, `gocpp.mod`, `gocpp.sum`,
  folders: not the program the build writes next to them) and every file it asked about.
- The **compiles**: for each unit the object cache's entry that answered it (the set of headers the compiler
  read, system ones too, and the files whose appearance would hide one) and the hash of the source. This
  is the `-MD` list of ADR-008; the stamp reuses the object cache's sets and its validation.
- The **link**: the link of a recorded build runs with `-Wl,-t`, which makes the linker print the files it opens
  (libraries `-lx` found, the compiler's start files, libstdc++, libc); they are recorded, and the line is
  taken out of the output. A link flag that makes the linker read a file the trace does not show (a version
  script, a response file, `-fuse-ld=`) makes the build unstampable. The linker program itself is recorded.
- **pkg-config**'s answers (it is run again when the stamp is checked) and the compilers (path, version, program file).
- The **program**: its stat, or its hash when its times are too new to trust.
A file is trusted when its size, modification time, inode and change time are as recorded and none of the times
is within two seconds of the moment it was read (git's racy rule); otherwise it is hashed, and a touched
file with the recorded content is fine (the stamp is renewed). `-x` says why a build is not up to date
(`# not up to date: ...`) or not stamped (`# not stamped: ...`); `GOCPP_NOUPTODATE=1` turns the check off.
A build is not stamped when it printed a warning (the checker's or a compiler's: they come with every build),
when a compile was not kept by the object cache, with `-work`, with a test hook, with a `...` pattern, and for
`gocpp run` (which builds into a folder it removes). A stamp that is cut short or damaged is no stamp.

**Package summaries (6.6).** Each stamp keeps for every package the hash of its sources and its **API hash**,
the hash of `pkg.hpp` (types, prototypes, variables, constants with their values; no `#line`, so a body is
not in it). `-x` prints them (`# api example.com/m/a <hash>`) and, against the stamp of the last build,
`# sources changed: ...` and `# api changed: ...`. The compile level needs no separate API check: `pkg.hpp` is a
header every dependent's compile read, so the object cache already recompiles dependents exactly when it
changed (a changed constant: yes; a changed body: no, because write-if-changed keeps the file).

**The compilers on disk.** The answer of the compiler detection (version line, family) is kept in
`<cache>/cxxid/` for the program file that gave it (real path, size, time) and used while the file is the
same: 60 to 100 ms for every gocpp command.

**Cache management.** The cache folder (`$XDG_CACHE_HOME/gocpp`): `obj` (object cache, 2 GiB, oldest first),
`rt` (runtime, precompiled headers), `gen` (generated C++ per program: trimmed after a week when there are
over 100), `stamp` (trimmed after 60 days; a stamp that is used is touched), `cxxid`, `probe`, `mod` (modules).
`gocpp clean -cache` removes all but `mod` (a program that is being built keeps its folder in `gen`),
`-gen`, `-stamp`, `-modcache`, with `-n` to see what and how much. Every file is written to a temporary
name and renamed; anything that does not read back (a stamp that is not JSON, a set that is gone from
the object cache) is a miss.

## Alternatives

- **Skip the type-check and the emit of unchanged packages** (summaries with export data). go/types
  can import export data, but gocpp's checker works on a rewritten tree (header and namespace uses), its facts
  (kinds of C/C++ names, promoted methods) are not in any export format, and the emitter asks the packages it
  imports for descriptors and the packages that import it for theirs, so a package's files depend on its
  dependents. A body change re-checks and re-emits the whole program today (25 ms for four packages, 65 ms
  for a hundred); the compiles and the link dominate. Revisit if the front end grows.
- **Hash every source file in every build** instead of trusting stats: simpler, but 100 packages' worth of
  reads and the headers' stats would still be needed; the stat rule is the object cache's own.
- **Resolve `-lx` by hand** (search the `-L` folders and the compiler's `-print-search-dirs`): wrong under a
  Nix wrapper, which adds `-L` where only the linker sees it. The linker's own trace is exact.
- **A build server / daemon**: out of scope; the stamp gives milliseconds without one.

## Consequences

Good: a build with nothing to do costs 27 ms for four packages, 32 ms for 20 and 95 ms for 100 (it stats the files it
recorded; the 100 are a chain, the worst shape for the check);
a one-body change costs the front end plus one compile and one link; every cache decision is visible with `-x`
and counted in the tests; several errors at once.

Bad / gaps:
- gocpp itself is known in the key by its path, size, time and version, not by its content; `fileContentID` reads
  the file after its stat (a file replaced in between gets the new stat's hash for the old content, until its
  next change); the detection record's content-hash file is trusted by its length (64 bytes) only.
- The directory listing counts every subfolder and symlink, so a new folder in a watched folder rebuilds once
  (never a stale program).
- The glob of a `cxx` recipe lists folders with the OS, not through the loader's file system: `resolveLibs` records
  each of them (`CxxLib.GlobDirs`) in the trace, so a new file that matches is seen (`TestUpToDateRecipeGlob`).
- A body change still re-runs the whole front end (check and emit of all packages: 25 ms for four, 65 ms for
  a hundred).
- The check does not see a file that the linker or compiler reads and the dependency list does not name, a
  header that is shadowed by a new file earlier on the include path outside the user's folders (the object
  cache's limit), a library that appears earlier on the linker's search path, or a system library changed with
  the same size and times.
- `gocpp run` builds into a folder it removes, so it never takes the up-to-date path (it still gets the object
  cache's hits).
- A build with warnings is never stamped, so a project with a warning in a C file always runs the whole
  front end.
