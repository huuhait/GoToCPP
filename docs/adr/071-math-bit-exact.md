# ADR-071: `math` gives Go's results on amd64, bit for bit

Status: accepted (2026-10-18)

## Context

Go programs print and compare floating-point results, and gocpp's tests compare programs with real Go. Go's
`math` is not "the best result for each input": its functions are pure-Go algorithms (Sin, Pow, Gamma, ...),
and on amd64 ten of them run assembly or a compiler intrinsic instead (`Floor`, `Ceil`, `Trunc`, `RoundToEven`,
`Sqrt`, `Exp`, `Log`, `Hypot`, `Max`, `Min`; `FMA` is an instruction), with results that are **not** always those of the pure-Go code
and not those of glibc's `libm`. Checked with real Go 1.25 on amd64:

- `Log` of a subnormal number is wrong: `Log(5e-324)` is `-709.0895657128241` (the true value is `-744.44`),
  because `log_amd64.s` takes the exponent from the bits of `x` without normalizing a subnormal `x`. The
  pure-Go `log` gives `-744.44`.
- `Exp` is Shibata's algorithm with **FMA instructions when the CPU has AVX and FMA** and without them
  otherwise (`exp_amd64.go`: `useFMA`), so Go's own results depend on the CPU; `GODEBUG=cpu.fma=off` selects
  the other path. Both differ from the pure-Go `exp` in the last bit for many inputs.
- `FMA` is the instruction `VFMADD231SD` (a runtime check), and `RoundToEven`, `Floor`, `Ceil` and `Trunc`
  are `ROUNDSD`: they make a signaling NaN quiet, which Go's pure-Go code does not.
- gc on amd64 (`GOAMD64=v1`, the default) never fuses `a*b + c`; gc on arm64, ppc64 and s390x does.

Floating-point contraction in the C++ compiler is the other half: GCC's default in GNU mode is
`-ffp-contract=fast`, which would fuse `a*b + c` in the generated code and in every native function.

## Decision

1. **The contract is Go's results on amd64 with `GOAMD64=v1` (or `v2`), bit for bit**, for every exported
   function of `math`, on every platform gocpp builds for (like `gp::f2i`, which copies amd64's float to
   integer conversions: ADR-006). On arm64 Go runs other assembly for `Exp`, `Floor`, `Modf` and others and
   fuses expressions; gocpp does not follow it.
2. **`math` is Go's own source** ported by `tools/portstd` for `GOARCH=generic`, so every function is the
   pure-Go code, and `tools/portstd/patches/math.rules` replaces the pieces that Go runs differently on
   amd64, each with a reason:
   - `Exp` and `Log` are natives (`runtime/src/math.cpp`): a transcription of `exp_amd64.s` (both paths) and
     `log_amd64.s`, operation by operation, with their quirks. `Log` of a subnormal number is as wrong as in
     Go. Which path `Exp` takes is Go's rule: `AVX && FMA` of the CPU (`__builtin_cpu_supports`), and
     `GODEBUG=cpu.fma=off`, `cpu.avx=off` and `cpu.all=off` are read like `internal/cpu` reads them.
   - `Floor`, `Ceil`, `Trunc`, `Sqrt`, `Abs`, `RoundToEven`, `FMA` and the four `Float*bits` functions are
     inline natives (`gocpp/math.hpp`): they are exact in IEEE 754, so the bits are the same, and they are
     one instruction each. `Floor`, `Ceil`, `Trunc` and `RoundToEven` are `roundsd` itself when the CPU has
     SSE4.1 (as in Go), else SSE2 code with the same bits for every input (a signaling NaN included);
     `Sqrt` is `sqrtsd`; `FMA` is the instruction Go issues (`vfmadd231sd`, same operand roles) when the CPU
     has it, so that two NaN operands give the same NaN, else `std::fma`; the bit casts are `std::bit_cast`.
     The public functions (`math.Floor`) are the natives, with no call in between, so the program inlines them.
   - Everything else (the trigonometric, hyperbolic, `Pow`, `Gamma`, `Erf`, Bessel, `Mod`, `Remainder`,
     `Frexp`, `Ldexp`, ...) stays Go's code, never `libm`: glibc's results differ in the last bit from Go's.
3. **No contraction anywhere:** every compile of generated code and of the runtime has `-ffp-contract=off`
   and `-fno-fast-math` (ADR-006, `internal/build/flags.go`: already so; this ADR records the reason). `a*b + c`
   is two roundings, as in gc on amd64; the only fused operations are `math.FMA` and the explicit `std::fma` of
   `exp`'s FMA path. A test where fusing changes the result guards it (`math_fma_contraction_*`); it only bites when the compiler may emit FMA
   instructions (arm64, or `GOCPP_CXXFLAGS="-mfma -mavx2"` on x86-64: at the x86-64 baseline there is no FMA instruction to
   fuse into), so the whole `math_*` set is also run with `-O2 -mfma -mavx2`, and with `-ffp-contract=fast` put back in a scratch
   copy it fails (a mutation check).
4. **NaN payloads are in the contract for the functions of `math` and out of it for operators.** A function of
   `math` returns the same NaN bits as Go (the sweep tests hash raw bits, signaling NaNs and payloads
   included). The NaN that `a + b`, `a * b`, `min(a, b)`, `max(a, b)` or a conversion gives when an operand is a
   NaN (which of two NaN operands wins, the sign of `max`'s NaN) is what the instruction order of the compiler
   gives, in Go and in C++, and differs between gc, g++ and clang++: the tests that cover those operators
   compare results with every NaN as one value (`cw`).
5. **The tests are differential:** `tests/run/math_*.gp` (generated by `tools/mathcorpus`, with the expected
   output taken from real Go) evaluate every exported function on about 100,000 inputs each (special values,
   neighbours of them, random bit patterns, random values of several ranges) and compare a hash of the result
   bits and 200 sampled results. A bit that differs is a bug in gocpp, never "within tolerance".

## Alternatives

- **`libm` for the transcendental functions** (rejected): faster to write, and glibc is more accurate, but the
  last bit differs from Go's in a few percent of the inputs, so a program prints another number than under
  `go run`, and a ported test with exact expectations fails.
- **Port the pure-Go `exp` and `log`** (rejected): what the portstd tool does by default. Correct for
  `Log` of subnormals, but not Go's result on amd64: `Exp` differs in the last bit for many inputs and `Log`
  for subnormals. "Go's results" means what `go run` prints on the machines most programs run on.
- **Fix Go's `Log` of subnormals** (rejected for now): it is a known Go bug; a program that depends on it is
  rare, and the same one-line patch (`@replace` in `math.rules`) can turn it into the correct value later.
  The decision is the manager's: the contract is "same as Go", and this keeps it simple to state.
- **Always use the FMA path of `Exp`** (rejected): deterministic across machines, but different from Go on a
  CPU without FMA. Such CPUs are rare; honoring `GODEBUG=cpu.fma=off` also lets the tests cover both paths.
- **`-ffp-contract=on`** (rejected): contracts inside one expression only, but `a*b + c` is one expression.

## Consequences

Good: a program computes the same numbers as under `go run` on amd64, including `%v` output (the float
printing is ported `strconv`); the differential tests are the proof, and they are cheap to rerun
(`python3 tools/mathcorpus/run_all.py cats1,cats2,cats3` rewrites them from real Go).

Bad:
- gocpp reproduces a Go bug (`Log` of subnormals) and a CPU dependence (`Exp`'s FMA path).
- `Exp` and `Log` are C++ transcriptions of assembly: a new Go version that changes the assembly needs the
  transcription updated; the tests of the pinned version (and `make portstd-check`) show when.
- The native functions are exact, so they stay bit-identical to Go only while Go's amd64 code does the same
  IEEE operation: the sweep tests include signaling NaNs and NaN payloads for that reason.
- A call of `math.Floor`, `Ceil` or `Trunc` through a **function value** with a signaling NaN gives a different NaN than
  Go: gc compiles a direct call to `ROUNDSD`, which quiets the NaN (gocpp does the same, always), but Go's assembly
  body, which a function value reaches, returns the signaling NaN as it is (also with `GODEBUG=cpu.all=off` or
  `cpu.sse41=off`). The bits of a NaN are outside the contract of operators anyway (point 4); this is the one case
  where a `math` function differs, and no test covers it.
- Go decides `FMA` and the rounding intrinsics by the CPU only (`x86HasFMA`, `x86HasSSE41`), not by `GODEBUG`; gocpp
  does the same (`has_fma`, `has_sse41`). `GODEBUG=cpu.fma=off` changes only `Exp`'s path, as in Go.
- The expected outputs of the `math_*` tests were taken from Go on a CPU with AVX and FMA, so `Exp` took its FMA path:
  the tests that use `Exp` (directly or through `Pow`, `Sinh`, ...) would fail on a CPU without AVX or FMA, where Go
  and gocpp both take the other path and print other last bits. The unit test `runtime/test/math_test.cpp` has the
  tables of both paths and picks the one the CPU takes.
- The unit test shows one mutant that survives on purpose: `Log` with `HSqrt2 > f1` instead of `>=`. At the single
  input where the two differ (a mantissa equal to sqrt(2)/2) both branches give the same bits (checked in Go for seven
  exponents), so the mutant is equivalent.
- Go on arm64 prints other numbers for `Exp` and some others: the tests are differential on amd64 only
  (`//go:build amd64` is not needed: a test machine that is not amd64 compares with its own Go and may differ).

## Evidence

- Real Go on amd64 (1.25.7 in the dev shell, with the 1.25.14 sources the port reads; `math` did not change between them): `Log(5e-324) = -709.0895657128241`; `Exp` differs between
  `GODEBUG=cpu.fma=off` and the default for the same input.
- `tests/run/math_*_sweep.gp`, `math_exp_*`, `math_log_*` (bit-identical over 100k inputs each), and
  `runtime/test/math_test.cpp` (both paths of `Exp`, `Log`, `FMA` with NaN operands, against tables from Go).

See: [Phase 09 §2.1, step 9.5](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md), [ADR-006](006-numbers-and-checks.md),
[ADR-018](018-stdlib-ported-from-go.md), [ADR-034](034-portstd-generic-arch-and-rewritten-constraints.md).
