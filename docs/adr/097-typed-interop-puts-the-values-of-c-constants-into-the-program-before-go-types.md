# ADR-097: Typed interop puts the values of C/C++ constants into the program before `go/types`

Status: accepted (2026-10-20). Builds on [ADR-060](060-typed-interop-probes.md) (the constant probe),
[ADR-088](088-typed-interop-caches-every-compiler-answer-per-reference-under-the-headers-contents.md) (the cache) and
[ADR-087](087-typed-interop-code-generation.md) (the switch); the contract is
[docs/spec/typed-interop.md](../spec/typed-interop.md) §10.

## Context

Stock `go/types` cannot evaluate `cstdio.BUFSIZ`. The loader rewrites it to `C.BUFSIZ`, which `FakeImportC`
makes opaque, so `var buf [cstdio.BUFSIZ]byte` has no length, `const n = cmath.M_PI * 2` has no value and
`case cstdio.EOF:` is compared with nothing. Opaque mode answers with a phase gate ("C macros in constant
expressions are not supported yet (Phase 12)") and, in the lowering, "write the number". This is gap E1 of the
phase. A fork of `go/types` is the last resort of the plan (it would cut gocpp off from x/tools); the plan's
answer is to probe the constants first and substitute them in the syntax tree.

The probe exists since 12.2: `Ref.WantValue` makes the probe file ask for the `ConstantExpr` value of an
integer, the exact `bit_cast` bits of a double and the `StringLiteral` of a string; 12.7 caches each answer on disk
per reference under the contents of the headers. What was missing is the place where the answer goes in.

## Decision

1. **`check.Config.Constants`, a function the driver gives.** `internal/typedinterop` imports `internal/check`, so
   `check` cannot call the probe; it is given `ConstFunc(ctx, ConstQuery) (map[key]ConstAnswer, error)`.
   `typedinterop.Constants` is the implementation: it builds one reference per name (a bare path with
   `WantValue`), asks the kind probe, runs `probe.Runner.Probe` (so the 12.7 cache, batches and parallelism
   apply) and converts the results. The hidden switch `GOCPP_TYPED_INTEROP=1` sets it (`cmd/gocpp/typedlower.go`,
   one line in `check.go`); unset, `Constants` is nil and nothing changes. The mode flag and the default are 12.11.
2. **Where.** `Check` calls `substituteConstants` for each package right before its first `go/types` run. A
   syntactic walk (using the loader's `Rewrite.Uses`, which already says which selector chains are header and
   namespace names) collects the names in four places and only there: an **array length**, the value of a
   **const declaration**, a **case** of an expression switch and the count of a **shift** (`x << N`, `x <<= N`).
   Inside a place it looks through parentheses, `+ - ^ !`, binary operators, and the arguments of a conversion to
   a basic Go type and of `len cap min max real imag complex`; not into `unsafe.Sizeof` (the C++ object's
   size, not an int's), nor into the arguments of a call of a C/C++ function or of a Go function. All names of
   the package go to the function in **one query**; each answer replaces the node of the name in place, at the
   same position, by a literal: `8192`, `-1` (a unary minus on a literal), `'a'`, `true`, `"text"`, a
   floating-point literal. Everything else stays: a name outside these places is still `BUFSIZ` in the C++.
3. **Values.** An integer or enumerator is exact (`__int128` through `math/big`); a `char` constant (`'a'`, also
   `wchar_t`, `char16_t`, `char32_t`) is a rune literal; a `bool` is `true`/`false`; a string is a Go string literal
   of the exact bytes (adjacent literals joined, escapes decoded; a string with a byte that is not UTF-8 is
   refused, because clang's JSON dump replaces such a byte and the length of the array type tells); a float is the
   **exact double**: decimal when its exact expansion is short (`0.5`, `3.0`), else a hexadecimal literal
   (`0x1.999999999999ap-04`), because go/types computes constants exactly and `0.1` would be another number than the
   double that the macro is (`0.1 * 3` must be what the C++ double is, not `0.3`). NaN and infinities have no Go
   constant. An enumerator of an `enum class` is not an integer for C++ and is not substituted.
4. **A name that is not a constant.** When the compiler evaluated the name and it has no constant value (a macro
   with a side effect or a call, a function-like macro without its arguments, a mutable variable, a pointer or
   an object of a class, a type, NaN or infinity) and the place **needs** a constant (an array length, a const
   declaration), the result is an error at the `.gp` position with the reason ("array length cstdio.NEXT must be
   constant: the C++ compiler cannot evaluate it at compile time (non-type template argument is not a constant
   expression)"); the node becomes a `BadExpr` so that `go/types` and the gate say nothing more. In a case or a
   shift, Go allows a variable, so the name is simply left as it was. A **macro whose value is the place or the
   time of its expansion** (`__LINE__`, `__FILE__`, `__FILE_NAME__`, `__COUNTER__`, `__DATE__`, `__TIME__`,
   `__TIMESTAMP__`, `__BASE_FILE__`, `__INCLUDE_LEVEL__`, in its body or in the body of any macro it uses,
   from `clang -E -dM` over the probe's headers, cached with the probe's results) is refused the same way
   ("its value depends on where or when it is used (the macro expands to __LINE__)"): the probe would have
   given the line of its own temporary file, and the cache would have kept it. A name that clang knows nothing about
   (undefined, a header that does not compile, no probe answer) is left as it was and the existing checks
   report it: **unprobed means unchanged**.
5. **Opaque mode stays the arbiter.** Typed mode must not reject what opaque mode and the C++ compiler accept. A
   case or a shift count where Go's constant rules reject the value (`case cstdio.EOF:` in a switch on a `byte`,
   a negative count, a string macro against an integer tag, an integer macro in a tagless switch) is not an error
   of the program but of the substitution: the check takes the substitutions of that expression back and checks
   again (`constSubst.revert`), so the name is opaque as before. A shift's place is the **whole shift**
   (`1 << lib.N`, not only the count): the error of `var m int32 = 1 << lib.S31` is at the shifted expression.
   A duplicate case stays an error. An array length
   and a const declaration follow Go's rules (a negative length, `const b uint8 = lib.NEG`): opaque mode did not
   accept those programs at all.
6. **The words of the messages.** go/types prints the literal; the messages at a substituted place get the name the
   user wrote back (`invalid array length cstdio.EOF (untyped int constant -1)`), by a fix at the position of
   the place (`constSubst.fixAt`, next to the `#Derive` fix). A place with more than a name is fixed by the text of
   its nodes before and after the substitution (`5 - 5 - 1` becomes `5 - lib.A - 1`, not `lib.A - 5 - 1`).
7. **The guard.** The probe compiler is clang and the build compiler may be g++. A header that branches on
   `__clang__` (or on a compiler version) gives the build another value than the one that was put into the
   program, silently. So the first translation unit of the package (after its headers) asserts each name that
   was replaced: `static_assert(static_cast<long long>(VV) == 1LL, "gocpp: h.VV differs between the probe
   compiler (clang++) and this compiler (main.gp:8:11)")`, behind a `#line` of the use. An integer, character,
   bool or enumerator is compared as `long long` (`unsigned long long` above 2^63, not checked above 64 bits), a
   float as `static_cast<double>(NAME) == <the exact literal>`, a string as `std::string_view(NAME, sizeof(NAME) -
   1) == std::string_view("...", N)` (octal escapes). The spelling is the probe's own (`EOF`, `::kMax`,
   `::geo::deep::kN`, `(::kPt).x`; `ConstAnswer.Spelling`). A failure is reported by the C++ error mapper as an
   error of the program at the `.gp` line (`C++: static assertion failed: gocpp: ...`), not as a bug of gocpp. A
   name is asserted once however often it is used; the asserts cost one constant comparison each and nothing at
   run time (measured: 500 asserts add about 2 ms to a `-fsyntax-only` of 85 ms, with clang++ and g++). Headers already differ between the two compilers in names that identify them (`_PSTL_CLANG_VERSION`,
   `_GLIBCXX_CLANG`, `__GNUC__` 4 under clang and 15 under g++): a program that uses such a macro as a constant
   now fails to build with g++ instead of using clang's value.
8. **Cache and cost.** The answers are the 12.7 unit results: warm, a package costs the kind probe's cached answer and
   disk reads (60 ms for the test program on a loaded machine; the cold cost is one clang run of the batch,
   ~0.15 s of CPU plus the preamble). Nothing is asked for a package without a name in these places.
9. **Not for a cross build.** The value of a macro is the host's: `BUFSIZ` is 8192 with glibc and 512 with
   MinGW. For another `GOOS/GOARCH` the function answers nothing and the names stay opaque (the gate says so).
   With clang missing it answers nothing, silently (12.11 prints the one-time message).
10. **`Package.Consts`** lists the substitutions that stand (name as written, position, path, value as Go source,
   place, the probe's answer and spelling) for the guard, tools and tests. `gocpp expand` prints the user's own
   declarations byte for byte (ADR-091), so it shows the names, not the values; the values are in the C++
   (`gocpp emit`). The standard library's packages are not asked (`lp.Std`): they have no typed names.

## Consequences

- `[cstdio.BUFSIZ]byte`, `const n = cmath.M_PI * 2`, `case cstdio.EOF:`, `1 << lib.SHIFT` type-check with stock
  `go/types` in typed mode, and the C++ has the numbers (`gp::array<uint8_t, 8192>`, `case -1:`).
- **A substituted constant is a Go constant.** Arithmetic that Go does on it is exact (`lib.UINT_MAX + 1` is
  4294967296, not 0 as in C++) and an untyped constant takes the type of its context (`x := 1 << lib.N` is a Go
  `int`, where opaque mode left the C++ type). The macro's own expression is C++'s: `#define M (1UL << 31)` is
  2147483648.
- **The probe compiler is clang, the build's may be g++**: the guard (decision 7) makes a difference a build error.
- Not done: constexpr **function calls** (`lib.Square(3)`, `std.numeric_limits[int].max()`), variable-template
  instances (`std.numbers.pi_v[float64]`), `constexpr const char*` variables (only string literals are strings),
  the C-style `C.NAME` names of `import "C"`, a name in a type argument of a generic instance, and a macro in
  another package that this one imports (each package is asked about its own names).
