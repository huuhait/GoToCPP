# Phase 11: Fast Language Server (as fast as gopls)

> `gocpp lsp` gives you diagnostics, completion, hover, go-to-definition, rename and
> formatting **as fast as gopls does for Go**. Speed is a contract with written budgets, measured
> in CI, and compared side by side with gopls on the same code.

| | |
|---|---|
| Depends on | **Phase 01 only** for the early version (v0); grows with every phase; Phase 10 for `!`/macro support |
| Unlocks | comfortable daily work from very early on; Phase 12 plugs C/C++ knowledge into it |
| Milestone | **L0** when v0 ships (right after M1) and **M7** when the phase is complete |
| Size | large (about 6–8 weeks in total; the v0 part, about 2 weeks, is done early) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

gopls feels instant because of its **architecture**, not magic: it never redoes work, never
checks more than it must, and never waits for slow things. gocpp can reach the same speed for a
simple reason: **its front-end is built from the same parts as gopls**, the Go parser and
`go/types`, running in a single Go binary.

The raw engine is already fast enough. Measured on this machine (2026-10-13, Go 1.25.7, median of
7 runs) for re-checking one package after an edit, with dependencies loaded from cached export
data:

| Package | Lines | Parse | Type-check |
|---|---|---|---|
| `strings` | 2,455 | 0.8 ms | 2.2 ms |
| `encoding/json` | 4,303 | 1.2 ms | 3.9 ms |
| `go/types` | 23,812 | 7.2 ms | 22.4 ms |
| `net/http` | 28,514 | 7.7 ms | 21.5 ms |

So even a big package re-checks in about 30 ms. The job of this phase is to **keep it that
way**: never re-check dependencies from source on a keystroke, re-parse only the edited file, and
never let clang (C/C++ knowledge) sit on the hot path.

## 2. How it works

### 2.1 The speed contract (p95 budgets)

| Action | Budget (p95) |
|---|---|
| server start → `initialize` answered | < 100 ms |
| open a 50k-line project → first diagnostics | < 300 ms warm cache, < 2 s cold |
| keystroke → updated gocpp diagnostics | < 100 ms |
| completion | < 50 ms |
| hover, go-to-definition, signature help | < 30 ms |
| format a 1,000-line file | < 20 ms |
| find references / rename across the workspace | < 300 ms |
| memory, 50k-line project | < 400 MB |

**Reference projects** for the budgets:
1. A real Go library/CLI (about 30k lines) copied as `.gp`. Because gocpp *is* Go syntax, **the same
   code** can be opened in gopls (as `.go`) and in `gocpp lsp` (as `.gp`), which gives an honest
   side-by-side comparison. Goal: within 1.5× of gopls on every row.
2. An interop-heavy program (raylib game + std containers + sqlite).
3. The gocpp standard library itself.

### 2.2 Architecture (the gopls model)

```
 editor ──JSON-RPC (stdio)──► gocpp lsp
                                 │ every change → a new immutable Snapshot (overlays = unsaved files)
                                 ▼
               memoized pipeline, every step keyed by a content hash:
     file ──► parse + expand (macros, derive, !) ──► type-check package ──► diagnostics / index
                                                             ▲
                      dependency summaries (export data) ────┘  in memory + on-disk cache
                                 │
                                 └──► background: workspace symbol index; clang header index (Phase 12)
```

1. **One front-end.** The server uses exactly the compiler's front-end (parser fork, `go/types`,
   the expand/`!` rewrites). There's no second "editor parser" that could disagree with the
   compiler.
2. **Snapshots.** Each edit creates a new immutable snapshot. A request reads the snapshot that
   was current when it arrived. Work for older snapshots is **cancelled** (Go `context`).
3. **Memoize by content hash.** Parsed+expanded files, type-checked packages and diagnostics are
   cached by the hash of their inputs. Typing in one file re-parses only that file.
4. **Dependencies come from summaries, never source.** A type-checked package is summarized as
   export data, kept in memory and in an on-disk cache. gopls moved to this design in 2023 for
   speed and memory. Note the difference found in the audit: the public
   `golang.org/x/tools/go/gcexportdata` writes **deep** export data (it includes everything a
   package's API mentions, so it grows with the import graph), while gopls uses an internal
   **shallow** format. We start with the public API, measure memory against the budget, and if
   needed vendor x/tools' internal `gcimporter` (BSD license) for shallow export. The format isn't
   stable across builds, so the on-disk cache is keyed by the gocpp binary's hash. Export data
   records opaque C types as `invalid type`, so a side table keeps their original spellings
   (`*SDL.SDL_Window`) for hover across packages.
5. **API-hash short-circuit.** Editing a function body doesn't change the package's export data,
   so dependent packages are **not** re-checked (the same trick as the Phase 06 build).
6. **Priority order.** The file under the cursor first, then its package, then other open packages,
   then reverse dependencies (workspace diagnostics) in the background, in parallel on all cores.
7. **Debounce, don't delay.** Diagnostics wait about 30 ms after the last keystroke. Completion and
   hover answer immediately from the newest snapshot.
8. **C/C++ never blocks.** Opaque mode (Phase 01) means gocpp diagnostics never need clang. C/C++
   completion and hover come from a background header index (Phase 12) and appear when ready.
9. **One static binary.** No JVM or Node, so the process starts instantly. gopls's own protocol
   package is internal and can't be imported, and public Go LSP modules tend to lag behind LSP
   3.17. So we copy gopls's *generated* protocol code (`gopls/internal/protocol`, BSD license)
   together with its generator, which reads the official LSP metaModel and can be re-run for new
   LSP versions.
10. **Memory caps.** Type-checked packages that aren't open are dropped (LRU) and reloaded from
    their summaries when needed.

### 2.3 Features

| Feature | Notes |
|---|---|
| diagnostics | parse + type errors + gocpp rules, per file, as you type |
| completion | scopes, fields and methods, package members, keywords, snippets, auto-import |
| hover | type and doc comment; for `F!` it explains what happens on error *at this spot* (return / panic / exit 1); for a `!` function it shows the real signature (`func loadConfig(path string) (Config, error)`) |
| go to definition / type definition | `.gp` code, macro definitions (`#util.Dbg` → its function) |
| references, rename | gocpp names only (renaming C/C++ names is refused) |
| document/workspace symbols | background index |
| signature help, inlay hints | parameter names, the inferred types of `:=`, and a mark on every call whose error a `!` function handles for you |
| formatting, organize imports | `gocpp fmt` (from Phase 01); header imports grouped after gocpp imports |
| semantic tokens | `!` calls and `!` functions, `#macros`, header packages, namespaces and other C/C++ names get their own colors |
| code actions | "add `!`" where an error result is ignored · "make this a `!` function" · "expand `!` into `if err != nil`" · "expand macro" · "add missing import" |
| document links | a header import (`"<raylib.h>"`, `"./physics.h"`) opens the header file |
| C/C++ intelligence | added by Phase 12 (completion after a header package like `raylib.` or a namespace like `std.`, hover/definition into headers) |

### 2.4 Editors

- **VS Code extension** (`editors/vscode`): `.gp` language, a TextMate grammar (derived from an
  MIT-licensed Go grammar, plus header imports, `name!`, `func name!(`, `#Name`, `#body()`,
  `macro`), the LSP client,
  format on save, and `contributes.breakpoints` for the gocpp language (so Phase 13's debugging
  works without extra settings). `.gp` is also used by gnuplot; users can change that with
  `files.associations`.
- **tree-sitter grammar** (optional): a fork of `tree-sitter-go` with the gocpp syntax changes,
  for Neovim, Helix and Zed.
- **Setup docs** for Neovim, Helix, Zed and Emacs. Any LSP client works, since it's plain LSP over stdio.

### 2.5 Keeping it fast (measured, not hoped)

- **Benchmark harness:** replays recorded editor sessions (open project, type 200 characters,
  complete, hover, rename, save) against the reference projects and reports p50/p95 per request type.
- **CI gate:** a regression over 10 % on any budget row fails the build.
- **gopls side by side:** the same session against gopls on the `.go` copy of reference project 1,
  published as a table.
- **Built-in profiling:** `gocpp lsp -profile` (pprof CPU/heap), `-trace` (per-request timings),
  and a "slow request" log with snapshot details for bug reports.
- **Correctness check:** a differential test applies random edits and, once background work has
  finished, compares the server's diagnostics with a fresh `gocpp check -interop=opaque`. They must
  always agree (no stale results). Typed-interop diagnostics (Phase 12) are compared separately,
  after their background probes finish.

### 2.6 Early track: start using it after Phase 01

The language server needs only the front-end, not codegen. So **v0 ships right after Phase 01**
(diagnostics, hover, go-to-definition, completion, formatting). From then on you write gocpp in a
real editor while the compiler is still being built, and every later phase adds to it:
interop names (04), interfaces and embedding (05), multi-package workspaces (06), generics (07),
`!` and macros (10), and C/C++ completion (12).

## 3. Steps

1. **11.1 LSP core**: JSON-RPC over stdio, `initialize`/`shutdown`, incremental text sync, overlays,
   protocol types (copied from gopls, §2.2 item 9).
2. **11.2 Snapshots** + cancellation + debounce.
3. **11.3 Memoized pipeline**: per-file parse+expand cache, per-package check cache, export-data
   summaries (memory + disk), API-hash short-circuit, LRU memory cap.
4. **11.4 Diagnostics** (as you type).
5. **11.5 Hover, go-to-definition, signature help.**
6. **11.6 Completion** (scopes, members, package members, keywords, snippets, auto-import).
7. **11.7 Formatting + organize imports** (using `gocpp fmt` from Phase 01).
8. **11.8 References, rename, document/workspace symbols** (background index).
9. **11.9 Semantic tokens, inlay hints, document links, code actions.**
10. **11.10 VS Code extension** + tree-sitter grammar (optional) + editor setup docs.
    *The grammar part can start any time.*
11. **11.11 Benchmark harness + reference projects + CI gate + gopls side-by-side table.**
12. **11.12 Profiling, tracing, slow-request log; random-edit differential test.**
13. **11.13 `!` and macro support** (after Phase 10): hover explanations (incl. a `!` function's
    real signature), inlay hints on the calls a `!` function handles, macro go-to-definition, the
    `!` code actions, "expand macro" (also for wrap macros).

Steps 11.1–11.7 and 11.10 form the **v0 early track** (right after Phase 01).

## 4. Deliverables

- `gocpp lsp` with the features in 2.3 (C/C++ parts come from Phase 12)
- `editors/vscode` extension, optional tree-sitter grammar, editor setup docs
- LSP benchmark harness, reference projects, CI speed gate, gopls comparison table

## 5. Exit criteria

- Every budget in 2.1 is met (p95) on all three reference projects.
- On the shared reference project, every row is within 1.5× of gopls.
- The random-edit differential test passes: server diagnostics always equal
  `gocpp check -interop=opaque` once background work is done.
- (The C/C++ budget row and the "no request waits on clang" test belong to Phase 12, where clang
  first runs inside the server.)
- VS Code checklist passes: highlighting, live diagnostics, hover, go-to-definition, completion,
  rename, format on save, `!` and macro features.

## 6. Risks

| Risk | Mitigation |
|---|---|
| Big packages slow down every keystroke | Measured: 28.5k lines re-check in about 30 ms; only the edited package is re-checked, and dependencies come from summaries |
| Stale or wrong diagnostics from caching bugs | Immutable snapshots + content hashes + the random-edit differential test |
| clang makes the editor slow | C/C++ work is background-only and cached; budgets are measured with it on |
| Memory grows on large workspaces | Summaries instead of full type info for dependencies; LRU eviction |
| Feature creep before speed | Speed budgets are exit criteria; no feature lands if it breaks a budget |
