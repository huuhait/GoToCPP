# Phase 11: Fast Language Server · STATE

**Status:** `[~]` in progress (v0 track: 11.1, 11.2, 11.3a done; 11.10a grammar done)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] Measured go/types re-check speed with cached deps: `strings` 3 ms, `encoding/json` 5 ms, `net/http` (28.5k lines) 29 ms parse+check (2026-10-13)
- [x] 11.1 LSP core (JSON-RPC stdio, initialize/shutdown, incremental sync, overlays; protocol types copied from gopls's generated code + generator) · *v0*
- [x] 11.2 Snapshots + cancellation + debounce · *v0*
- [~] 11.3 Memoized pipeline (11.3a done; 11.3b: disk summaries + opaque spellings) · *v0*
  - [x] per-file parse+expand cache
  - [x] per-package check cache
  - [ ] export-data summaries (memory + on-disk; deep `gcexportdata` first, shallow via vendored `gcimporter` if memory needs it; cache keyed by gocpp binary hash; side table for opaque type spellings)
  - [x] API-hash short-circuit
  - [x] LRU memory cap
- [ ] 11.4 Diagnostics as you type · *v0*
- [ ] 11.5 Hover, go-to-definition, signature help · *v0*
- [ ] 11.6 Completion (scopes, members, keywords, snippets, auto-import) · *v0*
- [ ] 11.7 Formatting + organize imports (`gocpp fmt`) · *v0*
- [ ] 11.8 References, rename, document/workspace symbols
- [ ] 11.9 Semantic tokens, inlay hints, document links, code actions
- [~] 11.10 VS Code extension (+ optional tree-sitter, editor docs) · *v0*
  - [x] 11.10a the `.gp` language + TextMate grammar (`editors/vscode`, `make vscode-test`)
  - [ ] the LSP client, format on save, `contributes.breakpoints` (after the language server v0)
  - [ ] tree-sitter grammar (optional), editor setup docs
- [ ] 11.11 Benchmark harness + 3 reference projects + CI gate + gopls side-by-side table
- [ ] 11.12 Profiling/tracing/slow-request log + random-edit differential test
- [ ] 11.13 `!` and macro support (after Phase 10): hovers, inlay hints for handled calls, code actions, expand macro

## Exit criteria

- [ ] All p95 budgets met on the 3 reference projects
- [ ] Within 1.5× of gopls on the shared project
- [ ] Random-edit differential test passes (vs `gocpp check -interop=opaque`)
- [ ] VS Code checklist passes (incl. `!` and macro features)

## Decisions / notes

- 2026-10-13: Speed is a contract (budgets in README §2.1, enforced in CI). The architecture copies
  gopls: snapshots, content-hash memoization, export-data summaries, cancellation, background work.
- 2026-10-13 (grammar job 11.10a): in the TextMate grammar, a predeclared name after a dot is a
  name too, not only a keyword (`std.string`, `s.append()`, `C.int`; a selector is never a builtin
  in Go either). The grammar also fixes an upstream go-syntax bug: a function declaration whose
  result is `func()`, a generic type or `interface{…}`/`struct{…}` never ended (0 new leaks over
  the stdlib, leaking files 902 → 564).
- 2026-10-13: Job 11.10a (the TextMate grammar) committed, clean after 2 review rounds; VS Code
  1.109's own token scopes match the test tokenizer on 8 files; Node.js joined the dev shell.
- 2026-10-14 (manager, 11.1 review): line ends follow gopls's rule, not the LSP spec's: only `\n` ends a
  line (a `\r` before it belongs to the line end; a lone `\r` is a character), so the server, gopls,
  go/token and Neovim agree. For 11.2: Serve handles one message at a time today; 11.2 needs a read
  loop that applies notifications in order, requests in goroutines with a context per ID, and
  RequestCancelled (-32800) / ContentModified (-32801) replies. For 11.4: convert positions through
  byte offsets (`Mapper.Position(offset)`), and skip diagnostics for an overlay marked broken after a
  failed didChange. For 11.11: one didChange with 100 changes on a 1 MiB file costs ≈ 16 ms on an idle machine
  (the text and line index are rebuilt per change, like gopls; one change ≈ 0.17 ms).
- 2026-10-14: Job 11.1 (LSP core: `gocpp lsp`, internal/lsp/protocol copied from gopls v0.21.1 with
  COPIED.md and update.sh, JSON-RPC framing, the lifecycle, incremental text sync, overlays) committed,
  clean after 2 review rounds. Process start → initialize answered ≈ 1.5 ms. A failed didChange marks
  the overlay broken and tells the user once (window/showMessage).
- 2026-10-14 (manager, 11.2 review): gocpp runs requests truly concurrently, unlike gopls (whose
  jsonrpc2.AsyncHandler serializes requests except the ones that call jsonrpc2.Async). The order
  guarantee comes from taking each request's snapshot on the read loop. Consequence for 11.3: the
  caches must be concurrency-safe anyway (11.4's background diagnostics run beside requests):
  immutable computed values, a ctx-aware per-key promise/singleflight keyed by content hash, a short
  mutex per map never held while computing, and a semaphore (≈ GOMAXPROCS) around type-checks.
  Nothing is written to the client after `exit` (like gopls).
- 2026-10-14 (11.2 review): an `exit` that arrives right after `shutdown` first waits (at most
  shutdownWait + 1 s) for the answers in flight, so a client that sends initialize/shutdown/exit at once
  still gets shutdown's answer (allowed by the spec; a compliant client waits for the answer anyway).
  `gocpp lsp` ignores SIGPIPE, so a client that closes our stdout gives a write error, not a signal death.
- 2026-10-14: Job 11.2 (snapshots, cancellation, debounce: internal/lsp/cache Session/Snapshot with
  reference counts and a State hook for 11.3; protocol.Conn with notifications in order and requests
  concurrent, a context per ID, $/cancelRequest, ContentModified; internal/lsp/debounce with a fake
  clock) committed, clean after 3 review rounds; stress-tested with -race -count=200 on 1–8 CPUs.
- 2026-10-14 (manager, 11.3 review): deviation recorded: every package check re-parses all of the
  package's files from source (Check changes ASTs), not only the edited file (README §2.2 item 3,
  ADR-015); it costs ≈ 8% of an edit's CPU, within budget; a parse hook in internal/load plus AST
  cloning is an option for 11.11. Follow-up job 11.3b: the on-disk summary cache (keyed by the gocpp
  binary's hash, with a trim policy: the disk is small) and the side table of opaque type spellings
  (which must also feed the API hash); the "< 300 ms warm open" budget depends on it. The project now
  depends on golang.org/x/tools (v0.44.0, BSD-3) for go/gcexportdata, as ADR-001/002/015 planned.
- 2026-10-14 (manager, 11.3 round 2): known difference for the 11.12 random-edit differential test: a
  dependency's position-less error (no .gp files, only test files, build constraints, two packages,
  import cycles) is reported at every importing spec of every importer (like gopls's "could not
  import"); `gocpp check` reports it once, at the first import. The loader reads through
  `load.Config.FS` (the pipeline's snapshot view; every read of a metadata load is recorded and part of
  the check key).
- 2026-10-14: Job 11.3a (the memoized pipeline: internal/lsp/memoize, internal/lsp/cache parse/metadata/
  check caches keyed by content and recorded reads, deps from gcexportdata, API-hash short-circuit, cost
  caps in estimated memory, open packages kept; load.Config.FS/Overlay/Deps, check.Config.Deps)
  committed, clean after 3 review rounds. A one-line edit in a 10k-line package re-checks in ≈ 26–33 ms;
  diagnostics equal `gocpp check` on 476 packages except the known import-error difference.
