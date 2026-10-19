# Random differential testing

`tools/difftest` finds bugs that no hand-written test thought of. It generates random Go
programs whose behavior the language fully defines, runs them with real Go and with gocpp, and
compares what they print and how they end. Real Go is the reference: any difference is a bug in
gocpp, a documented divergence ([docs/spec/language.md](../spec/language.md), D1 to D28), or a
bug in the generator, and the tool says which.

```
go run ./tools/difftest -n 10000 -seed 1 -batch 200 -j 3        # a campaign against gocpp
go run ./tools/difftest -mode gc -n 1000 -seed 1                # check the generator with real Go only
go run ./tools/difftest -print 17 -seed 1                       # print program 17 of seed 1
go run ./tools/difftest -bundle 200 -first 0 -seed 1 > main.go  # print a whole bundle
```

A campaign compiles a lot of C++: on a shared machine run it through the machine's slot scripts,
and size `-j` (bundles in flight, each a build of gocpp with `-p` compiles at the same time) to
the free cores: see "Speed and sizing" below.

## What is generated

A program is a set of top-level declarations with the prefix `pN_` and an entry function
`prog_N`, made of 2 to 5 scenarios (a program has 150 to 400 lines). The scenarios are written
to stay inside what the language specification defines: integer arithmetic of every width (wrap
around, shifts by counts of any size, division and remainder, `MinInt / -1`), conversions between
integer and float types, strings (UTF-8, invalid bytes, runes, slicing, conversions), slices
with aliasing (a model of length and capacity tells which `append` writes the shared array and
which allocates), arrays and structs with value semantics, maps (printed by `fmt`, which sorts
them, and looked up by key), pointers, methods with value and pointer receivers, interfaces,
embedding and promoted methods, dynamic dispatch, nil interfaces versus interfaces that hold a
nil pointer, type switches and assertions, closures with the Go 1.22 loop variables, `defer`
(order, argument evaluation, named results, `recover`, re-panic, a panic in a deferred call),
the run-time panics of the language (one operation, one recovered panic each), labels and `goto`,
`switch` with `fallthrough`, constants and `iota`, shadowing, multiple results and variadics,
method values and expressions, generic functions (the instances gocpp builds), goroutines with
channels and `select` (workers, pipelines, ping-pong, closed channels, only ever with one ready
case), and `fmt` with `%v %+v %#v %T` and many other verbs and flags (also invalid ones, whose
output the standard library defines). Since the fix round of step 15.9 also: complex numbers
(arithmetic, division by zero and by infinity, printing); the methods fmt looks for (`String`,
`GoString`, `Format` with every flag, `Error` next to `String`, a `String` that panics, nested
and unexported fields); map keys that are interfaces, floats (`NaN`, `-0`) and arrays and
structs, printed in the order fmt sorts them, and the run-time panics for unhashable and
uncomparable values; `min` and `max` with NaN and signed zeros, `clear`; float to integer
conversions (values whose truncation fits, also in `[2^31, 2^32)` for `uint32`, and, as a
separate scenario, values that do not fit, where gc's amd64 results are the reference and gocpp
copies them); invalid UTF-8 (overlong forms, surrogates, too large, truncated) through `range`,
`[]rune`, `utf8` and `fmt`; the builtin `print` and `println`, whose stderr is compared; and the
standard packages that gocpp ports (`stdlib/PORTED.txt`): `strings`, `strconv`, `unicode`,
`unicode/utf8`, `bytes`, `math` (the functions whose results the documentation fixes, not the
transcendental ones), `math/bits`, `errors`, `sync`, `time` (durations, dates in UTC and in
fixed zones, formatting, parsing; never the clock or the zone database). Not generated because
gocpp does not port them yet: `slices`, `maps`, `cmp`, `iter`, `sort`, `context` and `os`.

What a program never does: depend on map iteration order (or on the order of several NaN keys, or
of interface keys of different types, which fmt sorts by type address), on `cap` after an
`append` that grew the slice (except the `capgrowth` scenario, off unless asked for: see "Known
differences"), on addresses (no `%p`, no pointers inside printed values), on the scheduling of
goroutines, on floating-point formatting corners that depend on the implementation (floats are
generated from short binary fractions), on `unsafe.Sizeof` of strings, slices or interfaces,
on stack traces, on the inliner of gc (a `recover` helper carries `//go:noinline`, D28; no
expression has two operations that can panic, D22; no argument list reads a variable that
another argument changes, D25). Every generated expression is total: no operation in it can
panic, so which panic a program reports never depends on the order in which two operands are
evaluated. Panics are made by the scenarios that test them, one panic point each.

The generator is parameterized by the features that gocpp has *now*: `-phase N` takes the
features of the plan up to phase N (the default, 9, adds the ported standard packages to
goroutines, channels, `select` and generic functions; not yet generic types, range over func,
`slices`, `maps`, `os`),
`-features +name,-name` switches single features (`gen.AllFeatures()` lists them). When a new
phase lands, raise the default and add its scenarios to `tools/difftest/gen/scen_*.go`: a
scenario is a function `func(g *Gen)` registered with the features it needs.

## Bundles and builds

A build of gocpp takes seconds, so programs run in bundles: 100 to 200 programs in one
package, spread over 8 files (gocpp compiles the C++ of the files at the same time). Every
program is called from `main` under `try`, which prints a marker `#### N` (and the scenario names)
to stdout and, with `println`, to stderr, and recovers a panic that no scenario meant to make (that is reported as a generator bug, and the program is
dropped). Real Go builds and runs the same bundle, and the output of each program is compared
with the output of gocpp, stdout and the stderr of the builtin `println` separately. A program that makes gocpp crash or hang, or a bundle that does not
build, is found by its marker or by the file and line of the error; the other programs of the
bundle are run again without it. Programs that end with a fatal panic (a few percent) end with
a recovered, printed panic inside the bundle, and a sample of them (`-fatal-sample`) also runs
alone, where the exit code (2), what `println` printed before the panic, and the line that says
why it died (`panic: ...`, with addresses and the `[recovered]` note masked) are compared.

Output that differs only in addresses is not a difference of gocpp but a generator bug (the lint
in `gen.Lint` should have refused the program), and is recorded as such; only values that look
like the two implementations' addresses (Go's `0xc0` heap and low code addresses against gocpp's
position independent `0x55`/`0x7f` ones) are masked, in the whole output, so a wrong digit of an
ordinary hexadecimal number (`0xdeadbeef` against `0xdeadbeee`) is a finding.

The reference runs twice: a program whose output differs between two runs is a generator bug
and is dropped too. The campaign counts these (`generator bugs` in the summary): a healthy
generator has none, and the unit tests (`go test ./tools/difftest/...`) type-check 2000
programs and check gc against the generator for every scenario.

## Mismatches

For each mismatch, the tool:

1. makes a *signature*: the first differing pair of lines, or the first error line (the
   compiler's first error for generated C++ that did not compile; the first frames of a
   sanitizer report), with numbers and generated names masked, and puts equal signatures of the
   same scenario in one *cluster*: it reduces the first few (`-max-reduce`) of a cluster, and
   counts the others with it. A member of a cluster that was found known must still match the
   known entry (its `src` regexp) itself: a second kind of leak is not hidden by the first;
2. *minimizes* the program with the reducer (`tools/difftest/diff/reduce.go`, at most 100
   builds of gocpp: `-reduce-budget`): it deletes declarations and statements, unwraps `if`
   and `for` statements, simplifies literals and drops arguments, on the text at the positions
   of the syntax tree, and keeps a change only when real Go still builds and runs the program
   (twice, when it has goroutines) and gocpp still shows the same kind of mismatch (for a build
   failure, the same first error line). What is left is readable generated code;
3. *classifies* it with [tools/difftest/known.txt](../../tools/difftest/known.txt): entries that
   match the kind, a regexp on the signature and optionally on the minimized program, with a
   reason and a link to the document that explains the difference. A known difference is
   counted and not reported again;
4. otherwise writes a *finding* to `tools/difftest/findings/<cluster>-seedS-progN/` (ignored by
   git): `main.go` (the minimized program as a bundle, valid Go and valid gocpp: copy it to
   `main.gp`), `finding.md` with the signature, what Go prints and what gocpp prints.

The summary says how many programs and lines ran, how many gocpp builds, the mismatches by kind
(`output`, `exit`, `stderr`, `build`, `ice` for generated C++ that does not compile, `crash`,
`timeout`, `sanitizer`), the known ones by name, and the findings. The exit status is 3 when
there is a finding or a generator bug.

Fixing a finding: make a test from the minimized program in `tests/run` (or `tests/diff`, which
compares with real Go) with the output real Go prints, fix gocpp, and run the campaign again with
`-redo`: the state files of finished bundles say "done" also for a bundle that had a finding, and
`-redo` deletes those of the bundles with findings or generator bugs, so that they run again
(`known.txt` is for differences that are understood and stay). A build that fails because the
generated C++ did not compile, or lost its object files, is done once more before it counts
(`build_retries` in the counters): on a shared machine something cleaning `/tmp` took the
temporary folder of a build away twice in 16,000 programs.

## Campaigns

- State is kept in a folder (`-state`, by default under the work folder and named after the seed,
  the phase, the generator `Version` and a hash of the features, so that a changed generator
  never mixes its programs with the old ones): one small file per finished bundle. Running the
  same command again continues where it stopped, and a finished campaign does nothing. Counters of
  all runs are added up.
- Seeds and ids are stable: program `i` of seed `s` is the same program with the same features
  and scenario count, so a finding is reproduced with `-print i -seed s`.
- `-compilers g++,clang++` builds every bundle with the first compiler and every `-clang-every`
  bundle (default 4) with the others; `-release-every K` (default 5) builds every K-th bundle
  with `gocpp build -release` (optimized: the -O0 builds are the common case, the optimizer
  has its own bugs).
- `-asan` builds the gocpp programs with AddressSanitizer and UndefinedBehaviorSanitizer: any
  sanitizer report fails the program (`sanitizer` kind), also when it did not stop it.
- `-json file` writes the counters; `-redo` runs bundles with findings again; an interrupt
  (Ctrl-C, SIGTERM) stops the builds with SIGTERM, so that gocpp stops its compilers, and says
  "interrupted": the state is saved.
- Disk: the cache of gocpp holds the objects of the standard library (shared by all bundles, kept
  to `-objcache`, 3 GiB by default) and, for every build, the generated C++ and a stamp. A
  janitor removes those older than 45 minutes every 3 minutes, and the temporary folders of
  killed builds (`tmp/go-*`, `tmp/cpp-*`). The cache stays at a few GiB for any number of programs.
- `-p n` (default 8) is the number of compiles at the same time in one build, `-nice n` (default
  10) lowers the priority of the tool and of everything it starts.

## Speed and sizing

A bundle of 100 programs (about 300 lines each) takes gocpp about 30 s of wall time and about
115 CPU seconds with `-O0 -g0` (`-g0` is 17-19 % less than the default `-g`; the `-asan` builds keep
`-g` because the reports need file and line) with g++, more with clang++ and `-release`, plus
the run of the program and a build of real Go (about 7 s). `-j` is the number of bundles in
flight: take about `free cores / 5`. Measured on a 192-core machine that other jobs shared (step 15.9, fix round): 10,000 programs
(g++ on every bundle, clang++ on every fourth, `-release` on every fifth, 281 programs also run
alone) took 47 minutes of wall time with `-j 3 -p 8` (212 programs a minute) and 577 CPU
minutes, that is 3.5 CPU seconds a program. One million programs are therefore about 960 CPU
hours: about 24 hours on 40 free cores (`-j 8`), 4 days on 10. An earlier estimate of 50 hours
was for `-j 3` on a quiet machine and the first generator, without the standard packages, clang
and `-release`. Against real Go alone (`-mode gc`) the rate is 4,800 programs a minute with `-j 4`
(0.2 CPU seconds a program).

## Known differences

[known.txt](../../tools/difftest/known.txt) has the differences that are understood:

- `D1-self-referencing-closure-leak`: LeakSanitizer reports, at the end of an `-asan` run, a
  closure that calls itself through the variable that holds it (a reference cycle,
  [D1](../spec/language.md#d1-reference-counting-instead-of-a-garbage-collector)). The generator
  sets such a variable to nil after use, so a campaign does not make it, and a leak of any other
  kind is not matched by the entry (its `src` pattern asks for the closure).
- `cap-growth-size-classes`: `cap()` after `append` of pointer-containing or zero-size elements.
  gc rounds the new array up to a malloc size class: appending 33 strings one by one gives
  capacity 71 in Go and 64 in gocpp, `[]any` 71 and 64, `[][]int` 37 and 32, and `[]struct{}`
  keeps `cap == len` in Go but not in gocpp (8, 16, ...). The language promises only
  `cap >= len`, so the `capgrowth` scenario is **off unless asked for** (`-features +capgrowth`)
  and the entry keeps the differences from being reported as new. The runtime
  (`runtime/src/slice.cpp`) is to be changed to gc's size classes: tracked in
  [TODO.md](../../plans/2026-10-13-gocpp/TODO.md).

## Does it find bugs?

The unit tests prove the harness with fake backends (bundle splitting, crash attribution,
resuming, `-redo`, classification of known clusters, the address filter, the janitor, the stop
with SIGTERM, reduction). The power of the generator is measured with bugs injected into a
scratch copy of the compiler (`DIFFTEST_MUTANTS=1 go test -run SeededBugs ./tools/difftest/diff`,
about 8 minutes; `DIFFTEST_MUTANT=name` runs one): each mutant must be found within 2000
programs. A mutant that makes the generated C++ not compile counts as found, with a note. The
seven mutants and where they were found (step 15.9; bundles of 100 programs, seed 77):

| Mutant | Found in |
|---|---|
| `>>` of a negative signed value by a count of the width or more gives 0 instead of -1 | program 201 |
| `<<` by a count equal to the width is not 0 | program 201 |
| `append` allocates when the elements exactly fit the capacity | program 204 |
| `copy` of overlapping windows always copies forward | program 182 (the first generator missed it in 2000 programs; the slice model got explicit overlapping copies) |
| `uint32(f)` of a float in `[2^31, 2^32)` goes through `int32` | program 217 (a mutant of the reviewer; the first generator missed it: `floatconv`) |
| the UTF-8 decoder accepts the overlong `E0 80..9F` forms | program 205 (a mutant of the reviewer; `utf8bad` and the new invalid words) |
| the per-iteration copy of the loop variable of a 3-clause `for` is dropped (Go 1.22) | program 210 |

Campaigns of step 15.9 (the final generator, `Version` 2, phase 9 features; gocpp at the rebased
tree):

- against gocpp, g++ on every bundle, clang++ on every fourth and `-release` on every fifth:
  16,000 programs (4.4 million lines, 160 bundles of 100; 472 programs also run alone for the
  exit code and stderr of a fatal panic): 0 mismatches;
- against gocpp with `-asan` (g++): 2,000 programs (550,000 lines): 0 sanitizer reports;
- the generator against real Go only: 20,000 programs (5.5 million lines): all build, run, and
  print the same twice, 0 generator bugs;
- earlier generator versions (`Version` 1, before the fix round): 22,000 programs against gocpp
  and 3,600 under `-asan`, and 35,000 against real Go, also 0 mismatches.

So gocpp has no known difference from Go on the programs this generator makes, except the
`cap` growth of `capgrowth` (that scenario: 3 of 100 programs differ, as in `known.txt`). The
generator bugs of the first runs, now fixed: a slice length assumed after a shrink or in a loop,
a map read from a slice assumed non-nil, a string that doubles in nested loops, a function value
or pointer printed with `%v` (an address), a `recover` helper that gc inlines (D28), reduction in
a signed type (`int(uint64)` can be negative), a `map[any]` with keys of different types (fmt
sorts them by type address) and with several NaN keys (their order is the map's).

The exit criterion of the plan (at least 1,000,000 programs, every mismatch triaged) is a
nightly job: `go run ./tools/difftest -n 1000000 -j 8 -seed N` is resumable and adds up over
nights (sizing above).
