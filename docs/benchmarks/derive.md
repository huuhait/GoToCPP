# `#Derive` benchmarks (step 10.10)

The derived `Equal`, `Clone` and `String` against methods written by hand in the same program, at `-release`
(`gocpp run -release`), g++ 15 and clang++ 21, best of three rounds (the rounds differ by less than 15%),
milliseconds for 2 million calls (`Rec.*`: `Equal` and `Clone` of a struct with strings, slices, a map, an
array of structs and a pointer) or 100 million calls (`Flat.*`: a struct of seven numbers, `Level.String`:
an enum of six names with two of eight values out of range, the hand-written version using `strconv.Itoa`).
The hand-written methods are the code a Go programmer writes (`==` on comparable structs, `copy`, loops).

| | g++ derived | g++ hand | clang++ derived | clang++ hand |
|---|---:|---:|---:|---:|
| `Rec.Equal` (2M) | 234 | 252 | 152 | 153 |
| `Rec.Clone` (1M) | 213 | 213 | 211 | 214 |
| `Flat.Equal` (100M) | 39 | 38 | 32 | 33 |
| `Flat.Clone` (100M) | 35 | 35 | 33 | 33 |
| `Level.String` (100M) | 514 | 1087 | 380 | 390 |

The derived methods are at the speed of the hand-written ones. (The first version called the derived `Equal` of a
field of a comparable type and was 30% slower than `==` on clang++; a field whose type has a derived `Equal` and
holds only values is compared with `==` now.) The program is `tests/run`-style code: a struct `Rec` with
`#Derive(Equal, Clone)` and `EqualH`/`CloneH` written out, driven by a `time.Now()` loop with a rotating input so that
the compiler cannot fold the calls.

## Editor and `gocpp check`

The language server and `gocpp check`/`vet` do not lower the program, so they do not need the bodies: with
`check.Config.NoDerivedBodies` the generators run for their errors and the package is type-checked once (ADR-080).
Per keystroke (didChange to publishDiagnostics, median of 15, a loaded machine, so the noise is 15%):

| package | before (checked twice) | now | hand-written |
|---|---:|---:|---:|
| 705 lines, 50 types, 100 derived methods | 243 ms | 25 ms | 90 ms (3155 lines, the same methods written out) |
| 3155 lines, one derived type | 171-197 ms | 92 ms | 90 ms |

The generators themselves cost about a third of one `go/types` pass for the 50-type package (memoized method sets and
C-type tests: the first version asked the same questions again and again).

Compile time of `gocpp build`/`run`/`emit`: a package that derives is type-checked twice (the second check sees the generated bodies). For the
`tests/run/derive_equal.gp` program (eleven derived `Equal` methods, 140 lines) `gocpp check` takes about the same as
the same program with the methods written out, plus one more `go/types` pass over the package (tens of milliseconds).
