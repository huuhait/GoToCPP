# ADR-015: The language server copies gopls and has speed budgets in CI

Status: accepted (2026-10-13)

## Context

The goal is an editor that feels as fast as gopls does for Go. gopls is fast because of its
architecture: it never redoes work, never checks more than it must, and never waits for slow
things. gocpp's front-end is built from the same parts as gopls (the Go parser and `go/types`,
in one Go binary), so the same speed is possible.

## Decision

`gocpp lsp` follows the gopls architecture:
- **One front-end:** exactly the compiler's (parser fork, `go/types`, the macro and `!`
  rewrites). There is no second "editor parser" that could disagree with the compiler.
- **Snapshots and content-hash caches:** every edit makes a new immutable snapshot (work for old
  ones is cancelled), and typing in one file re-parses only that file.
- **Dependencies come from export-data summaries, never from source.** Editing a function body
  doesn't change a package's export data, so its dependents are not re-checked.
- **Priority:** the file under the cursor first. Diagnostics wait about 30 ms after the last
  keystroke; completion and hover answer at once.
- **C/C++ never blocks:** gocpp diagnostics never wait for a C++ compiler. The namespace probe
  runs in the background (cached per include set), and its `undefined: x` errors appear when it
  finishes. C/C++ completion and hover come from a background header index (Phase 12).
- One static binary, LSP protocol code copied from gopls (with its generator), memory caps (LRU).

**Speed is a contract.** Written p95 budgets, measured by replaying recorded editor sessions;
a regression over 10 % fails CI. Some of the budgets:

| Action | Budget (p95) |
|---|---|
| keystroke → updated **gocpp** diagnostics | < 100 ms |
| completion | < 50 ms |
| hover, go-to-definition | < 30 ms |
| open a 50k-line project → first diagnostics | < 300 ms warm, < 2 s cold |
| memory for a 50k-line project | < 400 MB |

On the same code (a real ~30k-line Go library copied as `.gp`), every row must be within 1.5× of
gopls. A first version (v0) needs only Phase 01; in the one-person order it is built right
after M1, and it grows with every phase.

## Alternatives

- **A separate "light" editor parser** (rejected): it could disagree with the compiler.
- **Features first, speed later** (rejected): budgets are exit criteria; no feature may break one.

## Consequences

Good: fast from the first version; a random-edit test checks that, once background work has
finished, the server's diagnostics equal a fresh `gocpp check -interop=opaque` (no stale
results).

Bad:
- `go/types` must stay stock, because export data comes from x/tools
  ([ADR-002](002-front-end-go-parser-fork.md)).
- The public `gcexportdata` writes "deep" export data, which grows with the import graph; gopls
  uses an internal "shallow" format. If memory goes over budget, we vendor x/tools' `gcimporter`.
- Export data stores opaque C types as `invalid type`; a side table keeps their spellings for hover.
- The benchmarks need reference projects; the ~30k-line Go library is not chosen yet.

## Evidence

- Spike 2026-10-13 (Go 1.25.7, median of 7 runs, dependencies from cached export data):
  re-checking `strings` (2,455 lines) took 0.8 ms to parse and 2.2 ms to type-check; `net/http`
  (28,514 lines) took 7.7 ms and 21.5 ms ([lsp-speed](../../spikes/2026-10-13/lsp-speed/)).

See: [Phase 11 §1](../../plans/2026-10-13-gocpp/phase-11-language-server/README.md#1-idea), [§2.1](../../plans/2026-10-13-gocpp/phase-11-language-server/README.md#21-the-speed-contract-p95-budgets), [§2.2](../../plans/2026-10-13-gocpp/phase-11-language-server/README.md#22-architecture-the-gopls-model), [§2.5](../../plans/2026-10-13-gocpp/phase-11-language-server/README.md#25-keeping-it-fast-measured-not-hoped), [§2.6](../../plans/2026-10-13-gocpp/phase-11-language-server/README.md#26-early-track-start-using-it-after-phase-01), [Phase 12 §2.9](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md#29-cc-in-the-language-server-without-slowing-it-down).
