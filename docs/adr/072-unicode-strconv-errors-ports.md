# ADR-072: `unicode`, `strconv` and `errors` are Go's own packages, with patches only where gocpp's costs differ

Status: accepted (2026-10-20)

## Context

Step 9.4a replaces the stub of `unicode` and `strconv` and the Phase 05 copy of `errors` with the ports that
`tools/portstd` writes from Go 1.25.14 (ADR-034). Three questions had no answer in the plan:

- `errors` uses `internal/reflectlite` in `Is` and `As`. The reflect subset of step 9.3 provides it (`internal/reflectlite`
  is an alias of `reflect`), and the port compiled and passed all 43 `errors_*` tests unchanged. But `errors.As` on a
  chain of five wrapped errors took 7.9 us, against 1.4 us for the three native functions that step 5.7 wrote over the
  type descriptors (`gocpp/errors.hpp`), and `errors.Is` 1.3 us against 0.8 us (-O0).
- `unicode`'s table searches were about 4 to 6 times slower than they need to be: Go's `is16` and `lookupCaseRange` take
  `&ranges[m]`, which in gocpp is a counted pointer (ADR-004; two atomic operations per step), and `IsLetter` on a CJK
  character took 638 to 813 ns.
- `strconv` has two variants of `index`: through `internal/bytealg` (the file `bytealg.go`) or, with the tag
  `compiler_bootstrap`, a plain loop. Step 9.1 had listed it with `compiler_bootstrap` (before `internal/bytealg`
  existed).

## Decision

1. **The three packages are Go's source, unchanged, except for the rules in `tools/portstd/patches`** (`PORT.md` of
   each package lists them, each with its reason). `strconv` needs none for correctness; `errors` and `unicode` have
   performance rules only, whose results are Go's.
2. **`errors` keeps the three natives of step 5.7** (`comparableType`, `checkTarget`, `assign` over the type descriptors, with
   Go's panic texts) through `patches/errors.rules` and `natives.txt`: the code is Go's, the three uses of `reflectlite` are
   calls of the natives. The port without the patch works too (the reflect subset is complete enough); the rule can go
   when `reflect` is as fast as the descriptors.
3. **`unicode` reads table entries by value** (`range_ := ranges[m]`, and `lookupCaseRange` returns `(CaseRange, bool)`):
   `IsLetter` on a CJK character about 4-5x faster (813 to 154 ns in the coder's run, 638 to 163 ns in the review's), `ToUpper` 4-6x (735 to 116 ns; 570 to 128 ns), `SimpleFold` 4-7x (666 to 96 ns; 333 to 85 ns) at -O0 (Go: 8, 8
   and 12 ns). A general fix (a cheaper `&s[i]` in the compiler) would make the rule unnecessary; it is a Phase 15 item.
4. **`strconv` takes Go's amd64 files** (`tags=purego`, no `compiler_bootstrap`): `index` calls the native
   `internal/bytealg.IndexByteString`. `strconv`'s `optimize` switch is unexported, so the slow path of `FormatFloat` and
   `ParseFloat` (Go's `TestFtoaRandom`, `TestAtofSlow`) is tested only through its natural uses (more than 19 digits,
   precision above 15).
5. **A gocpp file beside a port is declared**: `own <pkg> <file.gp>` in `tools/portstd/ports.txt` (`own errors wrapf.gp`),
   so the tool stops refusing the package and does not warn; a hand-written file that is not declared is still refused.
6. **Tests are Go's own test tables.** `tests/run/{strconv,unicode,errors}_gotest_*.gp` are Go's `_test.go` files
   (`atoi`, `atof`, `ftoa`, `itoa`, `quote`, `atob`, `atoc`, `ctoa`; `letter`, `digit`, `graphic`, `script`; `errors`, `join`,
   `wrap`) converted by a script (not kept: step 9.10 does it with `gocpp test`) into single-file programs: the `Test*` functions run through a 70-line harness in place of
   `testing` (step 9.9), `math/rand` is a splitmix generator, `testing.Short()` is true, and the benchmarks, `AllocsPerRun`
   and the tests that reach unexported names (`SetOptimize`, `ParseFloatPrefix`, `NewDecimal`) are dropped. The documented
   examples run as `*_goexamples.gp`. Every file prints `ok TestName` lines, so the expected output
   comes from real Go and `GOCPP_RUN_VS_GO=1` checks it again. `tests/run/{strconv,unicode,errors}_diff_*.gp` are sweeps
   (every code point, 400000 floats, 300000 integers, 3000 error trees) that print hashes from real Go. Step 9.10 does the
   same for the rest of the tier-1 packages with `gocpp test` instead of the harness.
7. **`unicode` cannot be dot-imported**: it exports `C` (the category), and gocpp reserves `C` for C's built-in types in a
   dot import. Go's `unicode` tests use `. "unicode"`; the converter qualifies the names (`unicode.IsLetter`). A real
   divergence from Go, in D19 of docs/spec/language.md.

## Consequences

- `errors`, `strconv`, `unicode` are as fast as gocpp's -O0 code allows: `strconv.Itoa` 738 ns, `FormatFloat(x, 'g', -1, 64)` 2.9 us,
  `ParseFloat` 650 ns against Go's 28, 66 and 71 ns (a release mode, not built yet, closes most of it: STATE of 9.1).
- The stub checklist of docs/guides/porting.md is exercised for three more packages: `TestRealPackagesMatchStub` now type-checks a real
  package against stubs of the packages it imports and real `.gp` for those without a stub (`internal/bytealg`, `math`, ...).
- `strings`, `bytes`, `io`, `bufio`, `sort`, `context`, `regexp` are the rest of step 9.4: they wait for `sync` (8.7) and `slices` (7.10).
