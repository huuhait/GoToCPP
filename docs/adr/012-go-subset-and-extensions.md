# ADR-012: gocpp is Go 1.25 syntax plus a small, explicit set of extensions

Status: accepted (2026-10-13)

## Context

gocpp wants Go's syntax and style, and Go's tools with them (gofmt, `go/types`, `go/doc`, the
analyzers). It also needs C++ names, and it adds a few features Go doesn't have. Every
difference from Go must be visible and written down, so users and tests know what is Go and
what isn't.

## Decision

The language is "The Go Programming Language Specification (go1.25), except for the points
below", written down in `docs/spec/language.md`. `go/types` checks with `GoVersion: "go1.25"`.
New syntax (and one reserved name) is kept to a short, explicit list:

| Divergence | Example |
|---|---|
| D2 header imports | `import rl "<raylib.h>"` |
| D3 Go keywords after `.` | `m := std.map[std.string, int]{}` |
| D4 constant type arguments | `var buf std.array[byte, 256]` |
| D5 constructor calls | `v := std.vector[int](10, 0)` |
| D11 build directives | `//gocpp:pkg-config sdl2` |
| D12 `!` (must) | `data := os.ReadFile!(path)` |
| D13 macros | `#Assert(n > 0, "n must be positive")` |
| D18 `macro`, `lazy` | `macro` only at package level, `lazy` only in macro parameters |
| D19 `C` is reserved | `n := C.int(5)`; a declaration named `C` is an error |
| D20 dotted type names | `var d std.chrono.seconds` |

- The other divergences are about behavior, and they are listed too: reference counting (D1),
  the `reflect` subset (D6), goroutines as OS threads (D7), map order (D8), 64-bit only (D9),
  data races (D10), C/C++ values (D14), sizes (D15), `unsafe.Pointer` (D16), stdlib additions
  (D17), and `range` over C++ containers and C arrays (D21).
- A feature that is not built yet is rejected with "not supported yet (Phase NN)", never with
  broken C++. The same table generates `docs/spec/feature-matrix.md`.
- Never allowed in user code: `//go:linkname` and `plugin`.

## Alternatives

- **Inventing a lot of new syntax** (rejected): gofmt, `go/types`, the gopls design and Go's
  analyzers would stop working, and Go's standard library could not be copied.

## Consequences

Good:
- Stock `go/types` checks gocpp code with Go's rules, and code keeps gofmt's style.
- Programs that use only the Go-compatible part run under real `go run` too, so the real Go
  toolchain is the reference (the `tests/diff` differential tests).
- Go's standard library can be ported mechanically ([ADR-018](018-stdlib-ported-from-go.md)).

Bad:
- 100% Go compatibility is a non-goal: existing Go code and modules are not expected to build
  unchanged.
- New syntax costs parser patches (6 in all) and editor support.

## Evidence

- Spike 2026-10-13: stock `go/parser` accepts the header import paths, but rejects `std.map[...]`
  and `std.array[int, 5]` ([parsecheck](../../spikes/2026-10-13/frontend/parsecheck/)). The audit
  added dotted type names (`var d std.chrono.seconds`). These three shapes became the three
  Phase 01 parser patches; the spike's interop files, which avoid them, parse unchanged.

See: [root README §3](../../plans/2026-10-13-gocpp/README.md#3-goals-and-non-goals), [Phase 00 §2.2](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#22-the-language-as-a-diff-against-go), [§2.3](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#23-test-harness-design), [Phase 01 §2.7](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#27-gotypes-configuration), [§2.9](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#29-unsupported-feature-checker-phase-gates).
