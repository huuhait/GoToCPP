# ADR-088: Typed interop caches every compiler answer on disk, per reference, under the contents of the headers

Status: accepted (2026-10-19). Builds on [ADR-060](060-typed-interop-probes.md) (probes and their
keys) and [ADR-074](074-typed-interop-declarations-and-foreign-types.md) (declaration runs); the
kind probe's cache of Phase 06 (`internal/interop/diskcache.go`) is the model for the file format
and the rules about racy files. The contract is [docs/spec/typed-interop.md](../spec/typed-interop.md) §9.

## Context

A typed run asks clang three kinds of questions: what each name is (the kind probe, cached on disk
since Phase 06), what type each expression has (one probe file per package, 0.1 to 0.5 s), and what the
classes, enums and functions look like (one run per distinct type name, 0.1 to 1 s each; SDL2 needs
51 of them, 33 s of CPU). Before this step only the first was cached across processes. Every
`gocpp check`, every restart of the language server and every edit of one line asked all of it
again, though almost nothing had changed. Four things had to be decided: what the unit of caching
is when the compiler answers a whole file at once, what the key is, how several processes share
the folder, and how many compilers may run.

## Decision

1. **The unit is one reference, not one probe file.** A probe file is a *preamble* (the includes
   and the helpers) and one *unit* per reference (its probe lines, and the local-variable aliases
   they use, each with the aliases that one uses). A result depends on the preamble, the compiler, the
   flags, the environment and the contents of the preamble's headers, and on nothing else: no other
   unit, no position in the file. So a result is stored under `(key of the preamble, hash of the
   headers' contents, hash of the unit's text)`, where the unit's text has its reference number taken out (the
   numbers are positions in the sorted keys and move when a reference is added). Two programs with the
   same includes share results.
2. **Batching must not leak.** The units that are not cached go to the compiler in *batches*: probe files
   with the same preamble and only those units. A batch's lines have other numbers than the full
   file's, so a result is *encoded* (`encodeUnit`) with the places in the probe file that its messages
   name expressed as "the preamble's line N", "the unit's own line i" or "the alias A the unit uses", and
   *decoded* (`decodeUnit`) into the lines of the full file. Every result passes through that encoding
   on its way into the table, also on a cold run, so a table made by one big batch, by many small ones, one by
   one or from the disk is the same table (`TestCacheBatchIndependence`, `TestRealClangBatchesEqualOneRun`).
   Two rules of ADR-060 changed for this: an error of an alias line is no longer the declaring reference's error
   (the alias is declared once, before the first line that uses it, so whose error it was depended on the
   neighbours; the reference fails on its own line for the same reason), and a message that points into
   another reference's line has no line (0).
3. **The key.** `probe.PreKey` (version `gocpp-probe-v4`) has the text, the compiler (path, arguments, family,
   target OS and architecture, version line, identity of the program file), the flags in order, the package
   folder (spelled the same way on Windows: slashes, lower case), `MaxDecl`, `MaxJSON` and `build.KeyEnv`
   (`CPATH`, `CPLUS_INCLUDE_PATH`, `SDKROOT`, the Nix wrappers' variables, ...). The disk key adds gocpp's own
   identity (version and executable, as the kind probe's keys have it: the probe code decides the text and
   the reading of the answers). The headers are not in the key: they are the *variants* of the preamble's
   context entry, each a list of files with the hash of their contents. An entry is used when the files
   have the contents they had (`DepsHash`, sha256 of every file, a hundred files read at once; a size and
   time shortcut inside one process also compares the inode change time on Linux, which no program can
   set back), so a touch keeps the entries and an edit with the old size and time loses them. Up to four
   variants are kept: going back to a branch finds the old results. A list older than a week is not used (see
   the limits below). The list is the compiler's own
   (`-MD`), plus the places where a header would hide the one that was found: the package folder and the
   `-iquote`/`-I`/`-isystem`/`-idirafter` and `CPATH` folders (listed whether they exist or not, since one
   made later hides), every compiler folder before the one a header was found in, and the folder of every
   file that has a quoted include (looked up there first) joined with the name as it was spelled (`../foo.h`
   keeps its `..`), taken from the include directives that `clang -E -dI` keeps in its output with the line
   markers' files (not from the text of the headers, which no regular expression reads completely: continued
   lines, comments, digraphs, macros for the name; one run per preamble, kept for as long as the headers are the same,
   and the same output gives the headers that a missing include was looked for under). The candidate paths are never cleaned (a `..` after a symbolic link is the parent of the link's target, which
   only the system knows), and the question is asked once per preamble, not per declaration run. The package folder (`-iquote`,
   added by the probe) is the first folder of the order. When the output is not complete (stopped at the time limit,
   cut mid-line) the folder of every non-system header read is watched for every
   name a header was found by;
   plus, when a header was not found (also inside another header: the names come from the same `-dI` run, as spelled), every
   place where it would be: the negative answer is cached and a header that appears there invalidates it.
   The toolchain (`-E -v`: the search path, the target, the GCC installation) is in the key and is asked
   once an hour.
4. **Declaration runs** are cached as whole parts under their text's `PreKey` with the same variants. The
   parts are written as JSON; the structured types (`ctype.Type`) are written as text and read back with
   the parser, or as JSON when the text does not parse to the same type. A refusal ("the declaration
   is larger than the limit": a run that was killed at 32 MB) is cached too; the killed compiler writes no
   dependency file, so a preprocessor-only run (`-E -MD`) finds the headers it depends on.
5. **Files.** `<user cache>/gocpp/probe/typed-v1/{c,u,d}/ab/<sha256>.json`, a first line with the checksum of the rest.
   Written to a temporary name in the folder and renamed. A file that is cut off, garbled, too big or of another
   format is a miss, counted, and written again. A header that was modified (its modification time, or its change time, which a program cannot set back)
   within two seconds before the compiler started or at any time after it (the contents the compiler read are not
   the ones that are hashed afterwards) keeps its results nowhere, not in memory either. A
   Runner keeps what it made or read in memory (256 MB), the disk is the second layer. Reads refresh the
   modification time once in ten minutes; `Prune` removes entries older than 90 days, then the least recently
   used while there are more than 100,000 entries or 256 MB, at the first write of a process and every 512
   writes. A folder that cannot be written (read-only home, full disk) stops the writes for a minute and
   the process goes on with its memory. `GOCPP_PROBE_CACHE=<abs folder>` moves it (also the kind probe's),
   `off` (or `GOCPP_NOPROBECACHE=1`) turns it off; a test binary uses it only when the variable is set. Results that
   depend on the host's CPU (`-march=native`) stay in memory. `gocpp clean -cache` removes it with the rest;
   `gocpp __typed-probe -cache-stats` and `-cache-clean` show and empty it.
6. **Processes share by locks.** Before a compiler runs for a set of units or a declaration part, the process
   makes a lock file (`O_EXCL`) named after the key; another process (or another Runner) that wants the same
   answer waits, then looks in the cache again instead of running. A lock older than the run's time limit plus
   15 s, or whose process (on this machine) is gone, is a killed process's and is taken over; the wait ends with the context or at that limit and the waiter runs
   anyway. In a process, a flight per key does the same for goroutines.
7. **One bound for the whole process.** At most `Jobs()` compilers run at the same time: `GOCPP_PROBE_JOBS`,
   else `min(8, NumCPU/2)`; a counting semaphore every run takes (probe batches, declaration runs, the
   auxiliary runs), whatever call or round of `Analyze` they belong to. A batch has at least 400 units
   (`GOCPP_PROBE_BATCH`) and there are at most `Jobs()` batches: the preamble is most of a run (0.3 s of CPU for
   `<vector>`, `<map>`, `<string>`, `<algorithm>`, against 0.4 ms for a probe of a member), so two batches cost twice
   the CPU for the same wall time below about a thousand probes. Cancelling the context kills each
   compiler's process group (a child of the compiler dies with it) and frees the slot and the lock.
8. **The overload sets of the called functions** are asked while the probe runs, not after: they depend on the
   references and the kinds, not on the probe's answers (`startFuncDecls`).

## Alternatives

- *One entry per probe file* (the memo's way). Rejected: one new reference or one changed literal made
  the file another file, and the whole run again; the point of the step is that it does not.
- *One compiler run per reference*, which makes the independence trivial. Rejected: each run parses the
  headers, 85% of a run for a program of 200 references.
- *Hash the headers' modification times.* Rejected by the plan (§2.6) and by experience: an editor that
  restores the time, a checkout that sets it to the commit's, a build that copies with `-p`. The contents are
  hashed; the stat only shortcuts inside a process, with the change time where the system has it.
- *Per-reference dependency lists* (a result depends on the headers of the declarations it names, from
  the dump's locations). Rejected: macros and earlier headers change how later ones parse, and the
  compiler's list is the only complete one. A header edit invalidates every result of the contexts that
  include it, which is one batch and the declaration runs.
- *A precompiled header of the preamble* to make the one run after an edit cheap (0.3 s to about 0.05 s). Not
  done here: it needs its own cache entry (tens of MB) and a second set of flags; left to the language server's
  step (12.13), which wants it for completion too.
- *A lock directory with a daemon.* Rejected: lock files with the stale rule are enough for tools that run
  seconds, and nothing has to be running.

A lock file is removed with a retry (a second): on Windows a waiter that reads it keeps it from being deleted for a
moment, and the lock that stayed made every waiter of the process wait the full limit; one that cannot be removed is
counted (`LockLeaks`) and is taken over later as a stale one.

## Consequences

- A warm run asks the compiler for nothing (but the toolchain, `-E -v`, once an hour; the kind probe's own cache expires after an hour too) and
  costs 5 to 30 ms for the probes of a program of 20 to 240 references. A changed literal costs one run of
  one unit (plus the units whose local variables it feeds). A new C name also asks the kind probe and the
  function overload run (three invocations in all, two of them in parallel).
- The dependencies are the preamble's, not the reference's: an edit of any header the package includes loses
  all the results of the package. The next run is one batch and the declaration runs, in parallel.
- A header that `__has_include` asks for and that appears later, and a change of the toolchain within the hour
  (the toolchain is asked once an hour), are not seen, as in the kind probe's cache (which expires after an hour).
  Here a header list is used for a week at most (`maxVariantAge`). A Runner with no cache folder does not ask
  for the toolchain, so there a header that hides one that a header includes is not seen.
- Results of an old gocpp are never read (the key has the executable): a dev build starts cold. A release
  keeps its cache until the user installs another build.
- A Windows build keys the compiler's target and the folder spelled the Windows way; the process group is
  not killed on Windows (the clang driver runs the compiler in its own process there; `exec.CommandContext`
  kills it).

## Evidence

- Tests of the cache with a fake compiler that counts its runs and writes real dependency files: hit,
  miss, corrupt files of six kinds, truncation, the key's components one by one (`TestCacheKeyComponents`: flags, order
  of `-I`, version, arguments, target, folder, includes, string conversion, environment, size limits,
  contents of a header, the compiler program, the kinds), edits with the old size and time, a touch,
  going back to old contents, a missing header in the package folder, in an `-I` folder and in the compiler's
  folder, racy headers, four goroutines and four real processes asking the same question (one run), a
  stale lock, pruning by count, size and age and in order of use, a read-only folder, cancellation with the
  children of the compiler, the global bound across overlapping calls, the same table from one batch, many
  batches and any scheduling.
- With clang: cold, warm, header edit and a new reference on probes and on declarations; the refusal on disk; the
  zoo (23 programs, with declarations) prints the same bytes cold and warm (`TestCacheColdWarmZoo`), the
  checker's 29 zoo programs say the same cold and warm with no compiler run (`TestCacheColdAndWarmGiveTheSameDiagnostics`).
- Benchmarks (`gocpp __typed-probe -decls -stats`, clang 21 in the dev shell on a 96-core machine with a load
  of about 30; "runs" counts every compiler invocation of the probes, the kinds and the declarations, counted
  by a wrapper around clang++; "probe" is the time inside the probe layer and "kinds" that of the kind probe;
  the rest of the wall time is the check of the program, 70 to 120 ms). The cold row is what every run cost before
  this step (except the kind probe, which was cached since Phase 06). The edit rows change one line:
  a literal in an existing call (`engine`, `stdmix`) or a new call to a C function (the others, marked +name).

  | program (references) | run | wall | runs | CPU of the compilers | probe + kinds |
  |---|---|---|---|---|---|
  | engine (local header, 30) | cold | 1,297 ms | 7 | 1,840 ms | 304 + 417 ms |
  | | warm | 77 ms | 0 | 0 | 6.2 + 0.9 ms |
  | | one line | 392 ms | 1 | 300 ms | 321 + 1 ms |
  | | header edited | 1,285 ms | 7 | 1,850 ms | 327 + 403 ms |
  | std-heavy (vector, map, string; 237) | cold | 1,580 ms | 6 | 2,260 ms | 404 + 231 ms |
  | | warm | 99 ms | 0 | 0 | 7.4 + 0.8 ms |
  | | one line | 428 ms | 1 | 320 ms | 346 + 1 ms |
  | | header edited | 1,561 ms | 6 | 2,060 ms | 428 + 241 ms |
  | sqlite3 (41) | cold | 1,086 ms | 6 | 1,510 ms | 326 + 33 ms |
  | | warm | 84 ms | 0 | 0 | 6.2 + 0.3 ms |
  | | one line +name | 484 ms | 3 | 680 ms | 351 + 34 ms |
  | | header edited | 1,200 ms | 6 | 1,650 ms | 375 + 30 ms |
  | zlib (33) | cold | 1,411 ms | 14 | 3,890 ms | 323 + 30 ms |
  | | warm | 102 ms | 0 | 0 | 6.1 + 0.6 ms |
  | | one line +name | 511 ms | 3 | 670 ms | 348 + 37 ms |
  | | header edited | 1,443 ms | 14 | 3,680 ms | 312 + 33 ms |
  | SDL2 (33) | cold | 4,694 ms | 53 | 24,590 ms | 478 + 257 ms |
  | | warm | 133 ms | 0 | 0 | 8.7 + 2.3 ms |
  | | one line +name | 1,332 ms | 3 | 1,200 ms | 507 + 272 ms |
  | | header edited | 5,977 ms | 53 | 26,480 ms | 573 + 277 ms |
  | raylib (20) | cold | 847 ms | 6 | 1,600 ms | 379 + 36 ms |
  | | warm | 80 ms | 0 | 0 | 5.5 + 0.2 ms |
  | | one line +name | 419 ms | 3 | 570 ms | 297 + 29 ms |
  | | header edited | 734 ms | 6 | 1,370 ms | 307 + 28 ms |

  A warm run asks the compiler for nothing and the probe layer costs 5 to 10 ms (the Go benchmark
  `BenchmarkWarmProbe` measures 11 ms for the 237 references, kinds, collection and generation included). An edit of a
  literal in an existing reference runs one compiler (the unit's batch); a new C name also asks the kind probe
  (Phase 06's cache merges the new path) and the overload run of its function, which goes beside the probe: three
  invocations, two of them at the same time. An edit of a header loses every result of the packages that include
  it and costs a cold run.
- Mutation checks: 63 mutants of the key (each of its components), the header lists (contents, change time,
  missing files, the search path, the week), the unit key and hash, the places in the file and the codec,
  the checksum, the lock and the flight, the bound and the splitting, cancellation, pruning, the
  read-only fallback and the declarations: all killed after the tests that were missing were added; three are
  equivalent (the compiler family is always clang in the key because a probe refuses anything else; "off" in
  `GOCPP_PROBE_CACHE` is also a name that is not absolute; the hash of a missing file is a word where a real
  hash is hexadecimal).
- Windows (the real PC, not Wine): the package's tests, with the test binary as the fake compiler (the PC has MinGW
  g++ and no clang++, so the tests that need clang skip): the key components, the lock files, the atomic rename over an
  existing file, pruning, the depfile and the diagnostics with CRLF and drive letters, the Windows spelling of folders
  (`TestWindowsOutputsOfTheCompiler` runs the real `ParseDeps` there).

## See

- Phase 12 README §2.6 and §7; `internal/typedinterop/probe/{cache,units,declcache,limiter,aux}.go`.
