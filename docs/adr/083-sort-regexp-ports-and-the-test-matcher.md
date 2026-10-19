# ADR-083: `sort`, `regexp/syntax` and `regexp` are Go's own packages, and `gocpp test` matches `-run` with `regexp`

Status: accepted (2026-10-20)

## Context

Step 9.4c is the last slice of "port the tier-1 pure packages" (ADR-072 was `unicode`, `strconv`, `errors`; ADR-073 `io`, `strings`, `bytes`,
`bufio`, `context`; ADR-078 `cmp`, `slices`, `maps`, `iter`). Its prerequisites are merged: `slices` (7.10), `sync` with `sync.Pool` (8.7), the
reflect subset with `internal/reflectlite.Swapper` (9.3), `unicode` and `strconv` (9.4a), `strings` and `bytes` (9.4b). `regexp` was a declaration stub, `sort`
and `regexp/syntax` did not exist, and `gocpp test` matched the patterns of `-run`, `-skip`, `-list` and `-bench` with a translation of RE2
to `std::regex` (ADR-056, `runtime/src/testing_regex.cpp`) that rejected `\pL`, flags inside a pattern and ranges that end in a non-ASCII character.

What was not known beforehand: whether `regexp` (5000 lines that use `sync.Pool`, interfaces over `input`, `unicode` tables and many small
pointers) compiles and behaves like Go's, whether its internal tests (they name unexported functions and fields) can run, and how fast it is.

## Decision

1. **The three packages are `tools/portstd` output** (`go run ./tools/portstd -check` covers them): `sort` (5 files), `regexp/syntax` (8), `regexp` (4), 8040 lines. They compiled
   and ran, but two things leaked under reference counting, found by LeakSanitizer in the first `-asan` run of the sweeps (every `regexp.Compile` kept 1 to 5 KB: 110 MB for 30000
   compiles), and each has a patch rule with its reason (`tools/portstd/patches/regexp__syntax.rules`, `regexp.rules`; `PORT.md` of the packages lists them):
   - **A `syntax.Regexp` owned itself.** Go keeps short slices in the fields `Sub0 [1]*Regexp` and `Rune0 [2]rune` of the node (`re.Sub = re.Sub0[:1]`, 15 places in `parse.go` and
     `simplify.go`). In gocpp a slice of an array field holds a counted reference to the block that owns the array, which here is the node that holds the slice. The rules allocate those
     slices (`[]*Regexp{sub}`, `[]rune{r}`, `make([]rune, 0, 2)` where the callers test `!= nil`); the fields stay, as they are exported, and the trees are the same.
   - **`makeOnePass`'s `check` called itself through a variable** (`var check func(...)`, then `check = func(...) { ... check(...) }`): a closure that refers to itself is a cycle (D1), and it held
     the one-pass program, the queues and the rune tables. The rule sets `check = nil` when the function is done.
   `tools/stubgen` has `Real: true` for `regexp` (its declarations are still compared with Go's), `internal/check/gates.go` and `docs/spec/feature-matrix.md` have rows for all three, the
   table in `docs/guides/porting-blockers.md` says that step 9.4 is complete. `sort.Slice` swaps through `reflectlite.Swapper`, the machine and bit-state caches of `regexp` are
   `sync.Pool`s, the one-pass and backtracking matchers are Go's. `tests/run/regexp_noleak.gp` (Linux) compiles and drops 30000 regexps and fails when the resident memory grows
   by 8 MB (it grew by 110 MB without the rules, by 12 to 72 KB with them, and Go's own heap by 3 MB: a leak of 270 bytes a compile fails); `-asan` runs of every other test check the rest.
2. **`gocpp test` matches with package `regexp`.** `testing/internal/testdeps.TestDeps.MatchString` is Go's (`regexp.Compile` and `MatchString`, with a mutex
   because gocpp's `-test.parallel` can match from several goroutines). Go keeps one compiled pattern; gocpp's compile is 3 to 16 times slower and a `-run` pattern with `/` alternates
   between the patterns of its elements, so every subtest recompiled two regexps (5000 subtests: `-run '^TestMany$/^sub1$'` took 1.30 s against 0.145 s before the port and 0.03 s with Go). `testdeps` keeps up to 64 compiled
   patterns in a map (emptied when full; a pattern that does not compile is kept with its error). `tests/run/testdeps_match_cache.gp` counts the compiles (`testdeps.Compiles`); the harness's Go twin builds
   `testing/internal/testdeps` as `internal/testing_testdeps` (its one native, `exit`, is in `native.gp` and has a twin file) so that real Go runs the test too. Deleted on purpose: `runtime/src/testing_regex.cpp` (786 lines), the `std::regex`
   matcher in `runtime/src/testing.cpp`, `gp::testing::match_string` and `translate_regexp` in `testing.hpp`, and the `#include <regex>` of the runtime's
   testing sources. Semantics are Go's by construction: `-run`/`-skip`/`-bench`/`-list` patterns split at `/`, the same flags inside a pattern, Unicode
   classes, case folding and the same error texts. `tests/gotest` (`TestRegexpDialect`) lists test names with 145 patterns on the binary of `gocpp test -c` and
   on the one of `go test -c` and compares names, stderr and exit status; the eight patterns that used to be "not supported" and thirteen new ones (Unicode classes, flags in the middle, folding of non-ASCII) moved into the equal list.
3. **Go's own tests are converted as in ADR-072 and ADR-073** (a script, not kept; step 9.10 runs the originals with `gocpp test`): single-file programs
   with the 70-line harness (`testT`, `runTests`, a count of the `if` statements that the tests run, so that a test that returns early shows), expected output
   from real Go 1.25.14, checked again by `TestRunVsGo`:
   - `sort`: `tests/run/sort_gotest_{sort,search,structs}.gp` (28 `Test*` functions: Sort, Stable, Slice, the Bentley-McIlroy sorting-network and swap-limit tests,
     the adversary that counts comparisons, stability, NaN, `Find`, the exhaustive searches), `sort_goexamples*.gp` (the 19 examples of 6 files, run in order).
     Not run: `TestSearchWrappersDontAlloc` (an allocation count: `AllocsPerRun` measures nothing here), the benchmarks. `Heapsort` and `ReverseRange` of `export_test.go` are local copies of
     Go's two unexported functions; `math/rand/v2` is a splitmix generator, so Go and gocpp sort the same data.
   - `regexp/syntax`: `tests/run/regexpsyntax_gotest_{parse,prog,simplify}.gp` (11 of 12 tests: the dumps of the parse trees, the invalid patterns, the programs of the compile table,
     `Simplify`, `String`). Not run: `TestAppendRangeCollapse` (the unexported `appendRange`); `minFold` and `maxFold` of `TestFoldConstants` are Go's two constants copied.
   - `regexp`: `tests/run/regexp_goexamples.gp` (31 examples). Go's internal tests (`package regexp`: `all_test`, `find_test`, `exec_test`, `exec2_test`, `onepass_test`, **44
     tests**, including the Fowler POSIX suite, `TestRE2Search`, the one-pass and the backtrack tests, `TestDeepEqual`) name unexported functions and fields (`compile`,
     `longest`, `compileOnePass`, `mergeRuneSets`, `minInputLen`...), so they run in `tests/pkgs/regexp_internal`, a module that holds **a copy of the port**
     (`regexp/*.gp`, byte for byte the files of `stdlib/regexp`) beside the converted tests (`*_gotest.gp`, in package `regexp`) and the data files of Go (`testdata/*.dat`,
     `re2-search.txt`, read with `os`). `TestRegexpInternalCopy` (`tests/regexp_internal_test.go`) fails when a file of the copy differs from the port or when an unknown
     file is there, so the copy cannot drift. Not run: `TestRE2Exhaustive` (it is skipped under `testing.Short()`; its data is a 59 MB log in a bzip2 file and package
     `compress/bzip2` is not in gocpp); `filepath.Glob("testdata/*.dat")` is the list of the three files (`path/filepath` is not in gocpp yet); benchmarks. Under `-asan` (the harness's `GOCPP_FLAGS` reaches the program) `TestOnePassCutoff` returns early and
     counts its three checks: its `x{1,1000}y{1,1000}` recursion is about 2000 frames deep and clang++'s AddressSanitizer frames overflow the 8 MB stack of the main goroutine (goroutines are threads with a
     fixed stack; Go's grow); it runs in every other build, and a user whose patterns nest repeats that deep (`x{1,1000}y{1,1000}`, compile or match) sees the same overflow under `clang++ -asan`, on the main thread and in a goroutine (fine at `-O0`, with g++ and with `-release`; docs/guides/testing.md). `regexp_noleak` likewise does not read the resident size under `-asan` (the 256 MB quarantine of freed blocks hides it; LeakSanitizer reports a leak at exit).
4. **Differential sweeps** against real Go (one FNV hash per function, 64-bit; the error texts are inside the hashes or printed in full):
   - `sort_diff_ops`: 9 shapes of data (random, four distinct keys, sorted, reversed, sawtooth, organ pipe, equal, ...) at 113 sizes, through Sort, Stable, Slice,
     SliceStable, Reverse, Ints, Strings, Float64s (with NaN and -0), the Search family and `Find`; the order of equal keys after the unstable sorts, the number of `Less`
     and `Swap` calls and the panic texts of `Slice` are part of the hashes (`Sort` and `Slice` give the same hash, as in Go).
   - `regexp_diff_match`: 1500 random patterns and 300 anchored one-pass patterns (1676 valid, 124 invalid) (literals, classes, Unicode classes, anchors, groups, named groups, flags, lazy repeats, some invalid) on 8 random
     inputs each (multi-byte, newlines, invalid UTF-8): Match, Find, FindAll, Submatch, Replace with eleven templates, Split, Expand, `Longest`, bytes variants, and
     `MatchReader`/`FindReaderIndex` on readers that return one to three bytes.
   - `regexp_diff_errors`: 330 lines of error texts printed in full (219 invalid and borderline patterns with `syntax.ErrorCode` and `Expr`, the nesting limit of 1000
     at 999, 1000 and 1001, repeat counts and `expression too large`, 100000-byte patterns, the panic texts of `MustCompile`, `CompilePOSIX` rejections) and 4000 random
     mutations of them hashed with their error text.
   - `regexp_diff_unicode`: the range lists of `\p{X}`, `\P{X}`, `[^\p{X}]`, `\p{^X}` and the same with `(?i)` for every category, script, property and fold table of
     `unicode` (1093 parses of the five forms, two for the properties and the fold tables), the Perl and POSIX classes with seven flag sets, `(?i)` followed by each of 40000 code points and the orbit of `SimpleFold`, and 12000 random
     code points matched against 24 classes.
   - `regexpsyntax_diff_ops`: 300 random patterns parsed with eight flag sets (2100 trees, 300 parse errors): the tree, `Simplify`, `Equal`, the re-parse of `String()`, the program, `Prefix`, `StartCond`,
     `MatchRune`, `MatchRunePos`, `EmptyOpContext`.
   - `regexp_concurrent` (and `tests/race/ok_regexp_shared.gp`): 12 goroutines on shared `*Regexp`s (the pools of machines, bit states and one-pass machines, `Longest`
     set before sharing, `Copy`) must give the hashes of the same work done in order; `-race` reports nothing.
   Mutation checks (a one-line change in a port or in `testdeps`, built into a scratch `gocpp`; every one must make a test fail): `sort` insertion sort order, the ninther threshold, `Search`'s midpoint
   (`sort_gotest_*`, `sort_diff_ops`); `regexp/syntax` `maxHeight` 1001, `negateClass`, the folding loop and its first range, `Simplify` of `x{0}` (`regexpsyntax_*`, `regexp_diff_errors`,
   `regexp_diff_unicode`); `regexp` `Split`'s limit, the `$$` of `Expand`, the NFA's start test, the backtracker's visited bits, `onePassNext` (`regexp_goexamples`, `regexp_diff_match`, `regexp_internal`);
   `testdeps`' pattern cache (`TestGoTest/run_subtest_pattern`); a drifted byte of the copy of the port (`TestRegexpInternalCopy`). Some mutants first survived: the sweep had no anchored one-pass patterns (the 300 of `genOnePass` close that gap), the internal tests run on the copy of the port, not
   on `stdlib/` (their mutants are made in the copy), and two folding mutants were equivalent code or sat in a range that the sweeps hardly reach (the mutant in the main folding loop, which skips the
   orbit above U+1000, is killed by both unicode sweeps). A `Search` mutant that never ends is "killed" by the time limit.
5. **Test sizes are chosen for `-O0`** (the default build is `-O0 -g`, 60 to 120 times slower than Go on this code): the sweeps run 4 to 20 seconds each at `-O0`. They
   avoid the one Go-side cost that is not gocpp's: `Regexp.String` of a class that reaches into the folding range (`\D`, a negated class, the full range) asks
   `unicode.SimpleFold` for every rune of the range (10 ms a call in Go itself), so `regexpsyntax_diff_ops` picks those classes one time in 30 and `regexp_diff_unicode`
   hashes the ranges, not the text, of negated classes.

## Consequences

Measured on the shared machine (load 150 to 240), pinned to one core, best of three, g++ 15 and clang++ 21. Ratios are gocpp/Go; Go 1.25.14 times are in the first column.
The programs are `bench/rx.gp` and `sb.gp` of the job's scratch folder (the text is generated, 1 MB of words with e-mail addresses, numbers, accented and CJK words, 8 MB for the
prefix scans; 50000 access-log lines).

| `regexp` | Go | g++ -O2 | g++ -release | clang++ -O2 | clang++ -release | g++ -O0 |
|---|--:|--:|--:|--:|--:|--:|
| `MatchString` `[a-z]+@[a-z]+\.net`, no match (ns/byte) | 20.5 | 9.1x | 10.8x | 8.7x | 8.3x | 113x |
| `FindAllString` `\w+` (ns/byte) | 47.1 | 6.2x | 7.2x | 5.9x | 5.6x | 90x |
| `FindAllStringIndex` `needle\d+`, 8 MB (ns/byte) | 1.6 | 7.4x | 9.2x | 6.5x | 6.2x | 101x |
| `MatchString` rare literal, no match, 8 MB (ns/byte) | 0.10 | 0.9x | 0.9x | 0.9x | 0.9x | 3.9x |
| `ReplaceAllString` `$2:$1` (ns/byte) | 33.0 | 8.7x | 10.1x | 8.2x | 8.1x | 115x |
| `FindStringSubmatch`, access-log line (backtracker, ns/line) | 997 | 4.4x | 6.2x | 5.5x | 5.1x | 77x |
| `MatchString` one-pass `^\d+-\d+-\d+$` (ns/op) | 80.9 | 5.4x | 6.2x | 5.6x | 5.2x | 70x |
| `Split` `\s*[,;]\s*` (ns/byte) | 56.6 | 6.4x | 8.1x | 6.2x | 5.9x | 77x |
| `FindAllStringIndex` `(?i)quick\s+brown` (ns/byte) | 13.4 | 9.1x | 12.4x | 9.9x | 8.0x | 118x |
| `FindAllString` `\p{L}+` (ns/byte) | 43.6 | 6.2x | 7.8x | 5.8x | 5.7x | 72x |
| `FindAllStringIndex` alternation of 8 words (ns/byte) | 78.6 | 9.8x | 12.1x | 8.8x | 9.0x | 125x |
| `(a+)+b` on `a`^5000 (linear time; us/op) | 119 | 12.4x | 7.9x | 6.5x | 6.3x | 86x |
| `Compile` of 8 typical patterns (us/pattern) | 4.3 | 3.5x | 2.3x | 1.6x | 1.5x | 16x |

| `sort` (1M elements; ns/element) | Go | g++ -O2 | g++ -release | clang++ -O2 | clang++ -release |
|---|--:|--:|--:|--:|--:|
| `Ints` | 79 | 1.1x | 1.2x | 1.2x | 1.0x |
| `Slice` | 122 | 1.3x | 1.4x | 1.3x | 1.3x |
| `Sort` (`Interface`) | 144 | 1.3x | 1.4x | 1.3x | 1.3x |
| `Stable` (`Interface`) | 458 | 1.7x | 2.2x | 2.0x | 1.9x |
| `SliceStable` | 324 | 2.6x | 2.6x | 2.7x | 2.8x |
| `Strings` (200k) | 168 | 2.8x | 2.9x | 2.9x | 2.8x |
| `SearchInts`, `Search` (1M lookups) | 164, 149 | 1.0x, 1.2x | 1.1x, 1.1x | 1.2x, 1.0x | 1.0x, 1.0x |

- **`sort` is within 1.0x to 2.9x of Go at -O2**: it runs on `slices`' pdqsort and on interfaces. `-release` (`-O2 -flto`) is not faster than `-O2` here.
- **`regexp` is 4x to 12x slower than Go at -O2** (the memchr-driven scans of a rare literal are equal), 70x to 125x at `-O0`. A sampling profile of the
  `\w+`, `ReplaceAllString` and `MatchString` runs (g++ -O2, a SIGPROF sampler preloaded into the program) puts the time here: in the NFA (`machine.add`, `step`, `match`) about 33%
  in the code of the matcher itself and **about half in the counted pointers**: the destructors of `gp::ptr<queue>`, `<machine>`, `<thread>`, `<Inst>`, `<lazyFlag>` (about 25%), the
  `shared_ptr` control block (11%), the `append` and the destructors of slices of `*thread` and `entry` (about 8%); the backtracker and the one-pass runs spend 12% in
  `~ptr<bitState>`, 11% in the control block, 4% in `~slice` and 11% in the interface thunk of `input.step`. The program is single-threaded, so these are the non-atomic
  counts of libstdc++, the cost is the instruction count and the lost inlining. This is compiler and runtime work (Phase 15), so no patch rule tries to hide it. **What would
  help, in the order of the profile**: (1) a `*T` receiver, parameter or local that does not escape is passed and held without a count (borrowed: `m *machine`, `q *queue`,
  `t *thread`, `i *Inst` in `add`/`step` are never stored beyond the call); (2) the same for the slice headers that `append` and a re-slice copy (`q.dense = append(q.dense, ...)`);
  (3) devirtualizing a call through an interface whose dynamic type is known in the function (`i.step` on `inputString`), or inlining the `gp_thunk`; (4) PGO or `-O3` for the
  generated C++ (not measured); (5) a native `regexp` prefilter is not needed: literal-prefix scans already use `internal/bytealg`.
- **Compile time** (192 cores, `-O0 -g`, empty object cache for the first build). A program that prints with `fmt` alone builds cold in 2.8 s with g++ (37 s of CPU) and 2.2 s with
  clang++ (41 s). A program that imports `regexp` (one `MustCompile` and a `FindStringSubmatch`, `bench/hello_rx.gp`) needs `regexp`, `regexp/syntax`, `sort`, `slices`, `strconv`, `strings`,
  `bytes`, `unicode`, `sync` and `io` (8000 lines of ported Go and the `unicode` tables) on top: 4.6 s wall and 103 s of CPU with g++ (which is where `unicode` costs), 2.8 s and 95 s
  with clang++. Once those objects are cached, a change to the program takes 2.8 s with g++ and 1.7 s with clang++ (compile of `main`, link of a 24 MB binary with debug
  information; 19 MB with clang++), against 0.15 s for an unchanged program. Every test binary of `gocpp test` now links `regexp` (through `testdeps`, as in Go).
- `sort` and `regexp` allocate counted blocks the way the packages of ADR-073 do; the `regexp` caches hold machines in a `sync.Pool`, so a long run has no growth (the
  concurrent test makes 12 goroutines x 150 rounds on shared patterns without a report of the leak checker, `-asan`).
- `unicode/utf8`-style conversions in `regexp` (`inputString.step`, `utf8.DecodeRuneInString`) and `Regexp.String` for classes are as slow as the rest of the port at `-O0`; the
  sweeps are sized for it (decision 5).
- **Windows.** The 19 run tests of the three packages (`sort_*`, `regexp_*`, `regexpsyntax_*`) pass and `regexp_noleak` is skipped (Linux only) on a real Windows PC over ssh (`TestRunWindows`, `GOCPP_TEST_WINDOWS=all`), built with MinGW-w64; the
  module test `regexp_internal` has no Windows runner (`tests/pkgs` is not part of `TestRunWindows`).
- **Left for later:** `TestRE2Exhaustive` (needs `compress/bzip2` and the 59 MB log, hours at `-O0`), the benchmarks of Go's three packages (step 9.10 runs them with
  `gocpp test -bench`), the internal tests of `regexp` as plain `gocpp test` of the package (step 9.10 may replace the copy in `tests/pkgs/regexp_internal`), `regexp`'s speed (the list
  above), `testing` (`T.Context`, `-fuzz`... unchanged).
