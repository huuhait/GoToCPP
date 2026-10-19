# ADR-060: Typed interop asks clang with `decltype` probes in one file per package

Status: accepted (2026-10-18). Builds on the spike of 2026-10-13 in
[Phase 12](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md) §2.2; the contract is
[docs/spec/typed-interop.md](../spec/typed-interop.md).

## Context

Phase 12 needs the real C++ type of every C/C++ expression of a program, without modelling C++
(overloads, templates, ADL). Steps 12.1 and 12.2 had to choose how to ask clang, and what the answer
looks like for the steps that read it (12.3 records, 12.4 type mapper, 12.5 checker, 12.7 cache, 12.8
macro constants).

## Decision

1. **One probe file per package**, with the package's `#include`s, and for each reference one line
   `using gp_probe_N = gocpp_probe_internal_c<decltype((E))>;` (or `gocpp_probe_internal_c<T>` for a name that is a type). Clang runs once
   with `-fsyntax-only -Xclang -ast-dump=json -Xclang -ast-dump-filter=gp_probe_`; a probe that fails is
   absent from the dump and its error comes from the diagnostics, attributed to the probe line.
2. **`decltype((E))` with double parentheses**, so the answer carries the value category (`T&`, `T&&`,
   `T`), and **`gocpp_probe_internal_c<...>` around it**, so clang prints the canonical type (`std::basic_string<char>`).
   The bare alias prints a half-desugared type (`value_type &`).
3. **The collector writes expressions as C++ pieces, not as Go syntax** (`collect/ir.go`): the arguments
   that have a Go type are `std::declval<T&>()` (a variable, field, index, dereference) or
   `std::declval<T>()` (a temporary), exactly as the generated code passes them, so a function that takes
   `T&` refuses a temporary in the probe as it does in the build. Go types that C++ cannot name are
   **stand-ins** (`gocpp_probe_internal_pr::string`, `slice`, `array`, `map`) with the operations of the runtime's types;
   `string` converts to `std::string_view` only, as in Phase 04. Results can name them.
4. **A reference is canonical**: references with the same C++ meaning have the same key, are sorted by
   key, and get their ID from the sorted position, so the probe file and the table do not depend on the
   order or the comments of the source. A local variable declared from a C/C++ value is the `decay_t`
   of its value, inlined while short, else a named alias.
5. **Constants** are three kinds of probe: an integer through a template argument (`ConstantExpr.value`,
   `__int128`, read with `math/big`), a float as the exact bits of `bit_cast<uint64_t>((double)v)`, a
   string read from the AST's `StringLiteral`. A failing value probe is not an error of the reference.
6. **The dump is read as a stream** (`probe.ScanDecls`): one top-level object at a time, objects over a
   limit dropped except for their names, the whole dump limited, so memory does not grow with the dump.
7. **The cache key is the probe text, the compiler, the flags, the environment and the contents of every
   header the compiler read** (from the `-MD` file). `probe.Runner` memoizes by the part of the key that
   is known before the run and re-checks the headers' contents on a hit; 12.7 builds the disk cache on
   `PreKey`, `Key` and `DepsHash`.
8. **Names that cannot collide, results that cannot lie** (review fixes of 2026-10-19): the helpers
   are `gocpp_probe_internal_*` and an error on a helper line fails the run; a string constant
   is the initializer literal of a `const char *` probe only; a wide constant (`long double`,
   `unsigned __int128`) is exact or has no value; unnamed types lose their file position; a
   header package named like its namespace is looked up in the namespace first.
9. **clang++ is required** (`probe.ErrNoClang`); the probe never runs with g++, whose AST has no JSON
   dump. Step 12.11 falls back to opaque mode.

## Consequences

- One clang run answers a whole package (0.1 to 0.5 s with the headers of the zoo), and every probe is
  independent of the others (`-ferror-limit=0`).
- A probe mirrors the generated code only as far as the stand-ins and the value categories do. Where
  they differ from the build (a runtime type with a conversion the stand-in lacks), a probe fails that
  the build accepts: 12.5 treats a failed probe as an error only when it is an exact mirror, as the
  Phase 12 README says.
- The results are strings (`Result.Type`); 12.4 parses them or asks a second, structured probe for the
  records it needs. Nothing here depends on clang's JSON beyond `type`, `name`, `value` and `inner`
  of the probe declarations, which is what makes a tolerant reader possible across clang versions.
- ADR numbers 059 are taken twice by other jobs; this one is 060.
