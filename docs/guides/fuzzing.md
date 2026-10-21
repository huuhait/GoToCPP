# Fuzzing gocpp

The compiler is fuzzed end to end: random text goes through the parser, the formatter, the
checker, the lowering and the emitter, and some of the C++ that comes out is compiled. A fuzz
target looks for four kinds of failure: a **panic**, a **hang** (a run of more than 10 s), a
**memory blowup** (more than 3 GB of live heap), and a result that is **wrong in a way the
compiler can tell itself** (see the table below). It does not run the programs it makes
(`tools/difftest` compares what programs print with Go's output; this is about robustness).

## What is fuzzed

| target | package | what it checks |
|---|---|---|
| `FuzzParse` | `tools/fuzz` | the parser, in five modes (also the `!`, macro and `lazy` syntax of Phase 10): no panic, every position inside the file, the printer prints what parses |
| `FuzzFormat` | `tools/fuzz` | `gocpp fmt`: what it makes parses, is the same program (the syntax tree without positions, comments and parentheses is equal), and formatting it again changes nothing |
| `FuzzCheck` | `tools/fuzz` | load and check a one-file program: no panic, and **no `internal error` diagnostic on a file without syntax errors** |
| `FuzzLower` | `tools/fuzz` | a program that checks cleanly lowers and emits: **the emitter never refuses a program the checker accepts** (a refusal that the user can understand, `cppgen.Error` with `User` set, is fine) |
| `FuzzEmit` | `tools/fuzz` | the same, and: the second emit is byte for byte the first, every `#line` names a file of the program and a line in it, and one program in N is **compiled** (below) |
| `FuzzGen` | `tools/fuzz` | the program generator (below) turns the engine's bytes into a valid program: the checker must accept it, and then all of `FuzzEmit` |
| `FuzzLSP` | `tools/fuzz` | the language server, over pipes in the same process: an input is a program and a script of edits (`didChange` with ranges, full text), requests at a position (hover, definition, completion, references, rename, formatting, semantic tokens, ...), cancels, saves, closes and opens; no panic, every request answered (a deadlock is a hang), no internal-error answer, exit code 0 after `shutdown` |
| `FuzzCppErrors` | `cmd/gocpp` | the mapper of g++, clang++, ld and lld errors to `.gp` lines, on compiler-like text and on any text: no panic, no hang, one answer for one text |
| `FuzzParse`, `FuzzParseRecipe`, `FuzzParseVendor` | `internal/load/gocppmod` | `gocpp.mod`, its recipes and `vendor/modules.txt`: no panic, `Format` is idempotent (campaign names `GoMod`, `GoModRecipe`, `GoModVendor`) |

The older targets of the parser, the printer and `internal/modfetch` stay where they are.

A finding is never a "the compiler rejects this program" (it should: most inputs are not
programs). It is the compiler crashing, hanging, contradicting itself (the checker accepts what
the emitter refuses; the formatter changes a program), or producing C++ that does not compile.

### Seeds

Every target starts from the repository's own programs: the `.gp` files of `tests/`,
`examples/` and `stdlib/` (those under 24 KB), the programs of the generator, and for the
parser targets a set of Phase 10 syntax. A plain `go test` runs a sample of the seeds of the slow
targets (every 4th to 8th) so that `go test ./tools/fuzz` takes 30 to 45 seconds on a loaded
machine; the engine, and `GOCPP_FUZZ_ALL_SEEDS=1`, use all of them.

## The sampled C++ compile

The emitter's output for a program that checks cleanly must compile. Compiling takes about
0.3 s per unit even with the precompiled runtime header, a thousand times the rest of the
pipeline, so `FuzzEmit` and `FuzzGen` compile **one program in N** with **both g++ and clang++**,
with `-fsyntax-only` and the flags of `gocpp build` (`build.Flags`, `build.GeneratedFlags`), and
a program is a finding when either compiler prints an error **or a warning** (the generated
code is warning-free by design). The program is picked by a hash of its bytes, so a given input
is always or never compiled; once one compile has failed, the process compiles everything, so that
the engine's minimizer can tell whether a smaller input still fails.

```
GOCPP_FUZZ_COMPILE_EVERY=40 go test -fuzz '^FuzzEmit$' -fuzztime 20m ./tools/fuzz
```

`GOCPP_FUZZ_COMPILE_EVERY` is 0 (off) for plain `go test`; the campaign runner sets 40 for
`Emit` and `Gen`. The runtime's headers and the precompiled `rt.hpp` for each compiler are kept
in `GOCPP_FUZZ_CACHE` (default: a folder in the system's temporary folder), written under a
name of their own and renamed into place: g++ crashes on a half-written `.gch`.
`TestCompileStageSeesBrokenEmitter` (`tools/fuzz/fuzzkit`) breaks the emitted C++ three ways and
checks that the stage reports each with both compilers.

## The generator

Byte mutations break the syntax or the types of a program almost at once, so the checker's
and the emitter's deeper paths are hard to reach with them. `tools/fuzz/gen` makes **valid,
well-typed programs** from a seed or from bytes (`gen.FromSeed`, `gen.FromBytes`): named types,
structs with embedding, methods, interfaces and their implementations, function values and
closures, `defer`, `panic` and `recover`, generic functions (a fixed set of templates, called
with explicit and with inferred type arguments), maps, slices, arrays, pointers, channels,
`go`, `select`, `switch` (also `fallthrough`), labelled loops, `for range` over integers, slices,
maps and strings. It avoids what would make a program invalid, not what makes it unusual:
constant expressions that overflow or divide by zero, unused variables and imports, and
unreachable-code mistakes are all kept out by construction.

`Options.Extras` (on in `gen.Default()`) adds the shapes that the Go test suite's bug clusters
are made of: multi-value forwarding `f(g())` (into functions, variadic functions, methods,
closures, `defer`, `go`, `fmt.Println` and `return`), `const` blocks with `iota` (untyped, of named
types, and `1 << (3 * iota)`), shifts by constant and by variable counts (`x << n`, `T(1 << n)`,
`x <<= n`), rune literals and conversions between runes, strings and `[]rune`, three-index slices
of slices and arrays, named slice, map and element types (`type MyBytes []MyByte`), method
expressions (`T.m`, `(*T).m`, `I.m`), anonymous structs (with assignment, comparison, conversion to
a named struct), struct tags and `goto`. `Options.Gaps` also turns on constructs that the emitter
does not lower yet (conversions between a string and a slice of a named byte type), which are
listed in `tools/fuzz/gaps.go`; it is off, so that a gap does not hide everything else in the
program. `TestGeneratorCoversConstructs` requires every construct in at least 10 of 300 programs,
`TestGeneratedGoCompiles` builds programs with the real Go compiler (`GOCPP_GEN_VET=n`), and
`TestSeededBugs` puts a defect into a program for each construct and requires the check to report it.

The generator is tested against the real `go/types`: `TestProgramsTypeCheck` generates 2000
programs and requires that all of them type-check (`GOCPP_GEN_N=50000` for a bigger sweep), and
`TestGenerated` (`tools/fuzz`) runs 60 of them (`GOCPP_GEN_N`) through the whole gocpp pipeline. `FuzzGen`
turns the engine's bytes into programs, so coverage-guided mutation steers the generator's choices.

Features that the compiler does not have yet are options of the generator that are off: generic
types (`Options.GenericTypes`) and range over functions (`Options.RangeFunc`).

## Running a campaign

```
make fuzz-short                         every target, a minute each (FUZZTIME=2m to change it)
make fuzz-long TARGET=Check,Emit TIME=20m   (TIME defaults to 20m, TARGET to all)
make fuzz-seeds                         only the seeds, as go test runs them
go run ./tools/fuzz/campaign -list      the targets
```

`tools/fuzz/campaign` runs `go test -fuzz` for each target in a process group of its own, at
priority 19, with at most 4 workers, and kills the group when it ends or is interrupted (the
child also gets `PR_SET_PDEATHSIG`). The time of a target is the fuzzing time you give plus, for
the pipeline targets (Parse, Format, Check, Lower, Emit), 30 to 90 seconds: the engine first runs
every seed (2400 to 2800 programs, 30 to 50 seconds on a loaded machine), and that counts as
fuzzing time, so a bare minute would be spent before the first mutation. When the engine finds a
failing input the runner:

1. reads the failure for a **signature**: its first line (the panic message, `HANG`, the
   `t.Fatalf` text) with numbers removed, and the first three stack frames in gocpp's code. The
   Go fuzzing engine does not show what a worker printed when it died, so for "fuzzing process
   hung or terminated unexpectedly" (a hang or a memory blowup that `fuzzkit.Guard` ended, a fatal
   error) the runner runs the input again alone as a test, and takes the text (`fuzz: HANG:
   FuzzCheck runs for more than 10s`, the stacks) from that run; two hangs in different places are
   two findings. An input that passes alone is a worker that was killed from outside (the memory
   limit of the machine), and is dropped;
2. moves the input from `testdata/fuzz/<FuzzName>/` (where the engine wrote it, and where it
   would stop the next run at once) to `fuzz-findings/<Target>/<signature>-<hash>`;
3. adds the finding to `fuzz-findings.md` (a table of runs, then each finding with the start
   of the failure's output); a second input with the same signature is dropped and counted;
4. lets the target run on for the rest of its time (at most 20 different findings per target).

`fuzz-findings/` and `fuzz-findings.md` are not committed (`.gitignore`), and a second campaign
knows the signatures of the first one's. Logs are in `fuzz-findings/logs/` (only those of runs that
failed or crashed: a clean run's log is removed); `fuzz-findings/runs.tsv` has one line per run. A
worker that the engine killed can leave its folder `gocpp-fuzz-gen-*` in the temporary folder and
every change of the runtime's headers leaves a precompiled header in the cache: the first compile of
a process removes what is older than an hour and not the current runtime's.

The engine saves interesting inputs of a run in Go's fuzz cache (`$GOCACHE/fuzz`), which the next
run starts from; `go clean -fuzzcache` clears it. Execs are slow for these targets (checking and
emitting a program takes 10 to 50 ms, a language-server run about 25 ms), so a 20 minute run does
tens of thousands of programs, not millions, and `-fuzzminimizetime` is 20 s (the engine's default
of a minute stalls the counters of a slow target).

Knobs (environment): `GOCPP_FUZZ_HANG` (seconds, 10), `GOCPP_FUZZ_MEM` (megabytes of live heap,
3000), `GOCPP_FUZZ_PHASE` (the phase whose gates the checker uses; default 10, the phase `make ci`
tests up to), `GOCPP_FUZZ_COMPILE_EVERY`, `GOCPP_FUZZ_CACHE`, `GOCPP_FUZZ_ALL_SEEDS`,
`GOCPP_GEN_N`.

## What to do with a finding

1. Run it: copy the input file to `tools/fuzz/testdata/fuzz/<FuzzName>/<name>` (or the testdata of
   the target's package) and `go test -run '^<FuzzName>$/<name>' ./tools/fuzz`. It fails the
   same way, and it is a normal test from now on.
2. Fix the bug where it is (a panic in the checker is the checker's bug even if the input is
   absurd). If the behavior is **deliberate** and not yet implemented, add the emitter's message
   to `knownGaps` in `tools/fuzz/gaps.go` (with the TODO or STATE.md entry that lists it) so that
   campaigns find new things; never silence a finding without a listed reason.
3. Keep the file in `testdata/fuzz/` as the regression seed: every `go test` runs it.

Nothing the fuzzers do touches the disk beyond temporary folders (the C++ compile writes the
emitted files to a new folder under the system's temporary folder and removes it), and nothing
runs a generated program.

## Campaign results (Phase 15 step 15.8, 2026-10-18/19)

Round 1 (5 to 8 minutes per target, the first generator) and round 2 (the generator with
`Options.Extras`, below), on a loaded shared machine; the long runs of the plan (20 minutes per
target) are for a quiet one. The runs of round 2, as `fuzz-findings/runs.tsv` has them:

| target | fuzzing time | execs | notes |
|---|---|---|---|
| Parse | 3m32s | 2.6 M | |
| Format | 5m31s | (3.4 k interesting) | 2 findings (harness, below) |
| Check | 6m32s | 1.79 M | |
| Lower | 6m32s + 6m32s | 0.83 M + 1.72 M | 1 finding (go/types panic, below) |
| Emit | 6m32s | 1.17 M | |
| Gen | 5m14s + 10m03s | 5.9 k + 5.5 k programs | one in 40 compiled with g++ and clang++; 1 unreproduced worker death |
| LSP | 5m06s | 134 k scripts | |
| CppErrors | 2m06s | 327 k | |
| gocpp.mod, recipes, vendor | 2m each | 11.9 M, 10.9 M, 13.6 M | |

Before the campaigns, `TestGenerated` ran 4000 programs of the extended generator through check
and emit (no failure) and 300 of them through the C++ compile of both compilers (one failure, the
variadic bug below). What was found:

| finding | status |
|---|---|
| **a call of a variadic function with no argument for the variadic parameter did not convert its other arguments to their interface parameters**: `func f(p any, q ...int)` called as `f(x)` with `x float64` emitted `f(x, gp::slice<int64_t>())` and g++ refused it ("could not convert 'x' from 'double' to 'gp::any'"); found by the generator's sampled compile | **fixed** (`internal/lower/named.go`, argCount), `internal/cppgen/testdata/emit/variadic_iface_noextra.gp` |
| a shift as the operand of a comparison, `a << 1 < b` with a named integer type, made clang warn (-Woverloaded-shift-op-parentheses) in the generated C++ | **fixed** (the operand gets parentheses, `internal/lower/ops.go`), `testdata/emit/shift_compare.gp` |
| go/types of Go 1.25 panics ("nil underlying") on `type Deep Deep.A.A0`; `gocpp check` and the language server crashed | **fixed in gocpp** (`internal/check`, safeTypesCheck: an error, "internal error: the type checker failed on this package"), `TestGoTypesPanicIsAnError`; the bug is upstream's, listed in `knownInternal` |
| gocpp.mod: `Format` not idempotent for a backslash followed by a CR in the middle of a line (the TODO.md item "FuzzParse fails on `\\\r//`") | **fixed** (the loader reports "unexpected input character"), `TestBackslashBeforeCR` |
| the generator looped forever on a source of bytes that was used up | **fixed** (`TestFromBytesEnds`) |
| the sampled compile: clang refused the precompiled header because a second worker rewrote its header file | **fixed** in the kit |
| a conversion between a string and a slice of a named byte type (`type MB byte; []MB(s)`, `string([]MB{1})`): "not supported yet: the conversion from string to []MB" (`[]byte` and `[]rune`, and named rune slices, work) | **open**, in `gaps.go`; the generator writes it only with `Options.Gaps` |
| `func main()` without a body: the checker accepted it, the emitter said "not supported yet: a function without a body" | **fixed** by ADR-128: the checker says "missing function body (...)"; the entry in `gaps.go` stays for the old message |
| a call into a standard package that is not ported yet: "not supported yet: the call of strings.ToLower (gocpp lowers only ...)" while the checker accepts | **open**, the packages are listed in `gaps.go` (`unsafe` and the program's own packages are not: they stay findings) |
| `gocpp fmt` on `//\n//go:build\npackage A`, on `func A(){ ;}` and on `macro A(){;}` is not idempotent in the first pass | **gofmt's own behavior** (the same with the real gofmt; macro uses its printer); the target compares against go/format and collapses the `{  }` of an empty body |
| number literals (`0XFF`, `1E5`, `0123i`), a repeated import spec, a `;` statement, `func()()`, an implicit empty statement change under gofmt | harness: `fuzzkit.Dump` ignores them |

The engine reported a few "worker terminated unexpectedly while minimizing" failures that pass when
run alone (a worker killed from outside, or a program that took more than 10 s of a loaded
machine): the runner re-runs such an input and drops it when it passes, and counts it in `runs.tsv`.
