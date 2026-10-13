# ADR-006: `int` is 64-bit, and arithmetic and runtime checks work like Go

Status: accepted (2026-10-13)

## Context

Go defines results that C++ leaves undefined or computes differently. Signed overflow wraps in
Go but is undefined behavior in C++. C++ converts `int8`, `int16`, `uint8` and `uint16` operands
to `int` before arithmetic. Division by zero, big shift counts and bad indexes are defined (or a
panic) in Go, but undefined in C++. Users expect Go's results, and the differential tests
compare with real Go. The user decided `int` is 64-bit
([Q2](../../plans/2026-10-13-gocpp/TODO.md#decisions-answered-2026-10-13)) and runtime checks
are on, like Go ([Q5](../../plans/2026-10-13-gocpp/TODO.md#decisions-answered-2026-10-13)).

## Decision

- `int` and `uint` are `int64_t` and `uint64_t`; v1 has only 64-bit targets (D9). `C.int` is
  there for C APIs.
- Every compile uses `-fwrapv`, so signed overflow wraps like Go.
- Results of `int8`, `int16`, `uint8` and `uint16` arithmetic are cast back to their type:
  `uint8_t(a + b)`.
- `/` and `%` call `gp::div` and `gp::rem`: they panic on zero, and `MIN / -1` gives `MIN`.
  Shifts call `gp::shl` and `gp::shr`: a count ≥ the width gives Go's result, and a negative
  count panics. Plain operators are used only when a constant makes them safe.
- A float → int conversion copies Go's results on amd64 (`gp::f2i<T>`).
- `-ffp-contract=off`: no fused multiply-add, so float results don't depend on the compiler.
- Runtime checks are on: index, slice bounds, nil pointer, nil map write, division, shift,
  `make` and slice → array, with Go's exact messages. An unrecovered panic exits with code 2.
- `gocpp build -nochecks` turns off only the bounds and nil checks. Division by zero and
  negative shifts still panic, because C++ has undefined behavior without these checks. Results
  that Go defines (a shift ≥ the width, `MIN / -1`) never change.

```go
var a, b uint8 = 200, 100
fmt.Println(a+b > 250) // Go prints false: 200+100 wraps to 44
```

C++ would add the two as `int` (300) and print `true`, so gocpp emits `uint8_t(a + b) > 250`.

## Alternatives

- **C++ `int`, 32 bits** (rejected): surprises Go users.
- **Undefined behavior on overflow** (rejected): C++'s default; results would differ from Go.

## Consequences

Good: arithmetic gives Go's results, and every rule has differential tests against `go run`.

Bad:
- The helpers and checks cost a little time; `-nochecks` (bounds and nil) is there for hot code.
- Go on arm64 saturates out-of-range float → int conversions. gocpp copies amd64 everywhere,
  so those cases run in the differential tests only on amd64.
- MSVC's own compiler can't be used: it has no `-fwrapv` ([ADR-021](021-platforms-linux-windows.md)).
- Values of C/C++ types follow C++ rules (D14).

## Evidence

- Spike with Go 1.25.7: `int64(f)` and `int32(f)` with `f := 1e300` give `-9223372036854775808`
  and `-2147483648` ([go-output-formats](../../spikes/2026-10-13/go-output-formats/)).
- Audit: the `uint8` example above prints `false` in Go and `true` in the direct C++;
  `-fwrapv` doesn't help, because nothing overflows in `int`.

See: [Phase 02 §2.3](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#23-expressions-the-important-rules), [§2.7](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#27-the-driver), [Phase 03 §2.9](../../plans/2026-10-13-gocpp/phase-03-core-language/README.md#29-runtime-checks-and-panics-gos-exact-messages), [Phase 07 §2.2](../../plans/2026-10-13-gocpp/phase-07-generics/README.md#22-operations-on-type-parameters), divergences [D9, D14](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#22-the-language-as-a-diff-against-go).
