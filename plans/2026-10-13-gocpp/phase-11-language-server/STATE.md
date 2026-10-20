# Phase 11: Fast Language Server · STATE

**Status:** `[~]` in progress (v0 track done 2026-10-14: 11.1–11.7, 11.10a/b; the optional tree-sitter grammar is left; 11.10a grammar done; 11.8, 11.9, 11.11, 11.12 merged since; open: 11.13, the 1.5x-of-gopls criterion, the VS Code checklist)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-20

## Steps

- [x] Measured go/types re-check speed with cached deps: `strings` 3 ms, `encoding/json` 5 ms, `net/http` (28.5k lines) 29 ms parse+check (2026-10-13)
- [x] 11.1 LSP core (JSON-RPC stdio, initialize/shutdown, incremental sync, overlays; protocol types copied from gopls's generated code + generator) · *v0*
- [x] 11.2 Snapshots + cancellation + debounce · *v0*
- [x] 11.3 Memoized pipeline (11.3a + 11.3b) · *v0*
  - [x] per-file parse+expand cache
  - [x] per-package check cache
  - [x] export-data summaries (memory + on-disk; deep `gcexportdata` first, shallow via vendored `gcimporter` if memory needs it; cache keyed by gocpp binary hash; side table for opaque type spellings)
  - [x] API-hash short-circuit
  - [x] LRU memory cap
- [x] 11.4 Diagnostics as you type · *v0*
- [x] 11.5 Hover, go-to-definition, signature help · *v0*
- [x] 11.6 Completion (scopes, members, keywords, snippets, auto-import) · *v0*
- [x] 11.7 Formatting + organize imports (`gocpp fmt`) · *v0*
- [x] 11.8 References, rename, document/workspace symbols
- [x] 11.9 Semantic tokens, inlay hints, document links, code actions
- [~] 11.10 VS Code extension (+ optional tree-sitter, editor docs) · *v0*
  - [x] 11.10a the `.gp` language + TextMate grammar (`editors/vscode`, `make vscode-test`)
  - [x] the LSP client, format on save, `contributes.breakpoints` (job 11.10b)
  - [x] editor setup docs (docs/guides/editors.md, job 11.10b)
  - [ ] tree-sitter grammar (optional)
- [x] 11.11 Benchmark harness + 3 reference projects + CI gate + gopls side-by-side table
- [x] 11.12 Profiling/tracing/slow-request log + random-edit differential test
- [ ] 11.13 `!` and macro support (after Phase 10): hovers, inlay hints for handled calls, code actions, expand macro

## Exit criteria

- [ ] Revalidate all p95 budgets on the final integrated reference projects. The 2026-10-18 table is historical; the 2026-10-20 round-6 integration smoke covers only small/medium opens and four scenarios, not all projects or combined 11.13 behavior.
- [ ] Within 1.5× of gopls on the shared project
- [x] Random-edit differential test passes (vs `gocpp check -interop=opaque`)
- [ ] VS Code checklist passes (incl. `!` and macro features)

## Decisions / notes

- 2026-10-20: Normal-loader parallel parsing follow-up integrated after clean four-file
  review at `541db0ab`. File selection, hooks/overlays and build constraints stay serial;
  workers parse captured bytes in private FileSets, then commit ASTs/errors/positions in
  source order. Shared position reservation and panic/join behavior have deterministic tests.
  Manager focused checks passed: build/vet, loader and checker/cache position tests under
  race, multi-package GCC/Clang builds, deterministic emission, compiler line directives,
  and the native-fmt tuple case on both compilers. No full sweep or new benchmark was run
  by the manager. The coder's matched small/medium three-scenario evidence showed 6.6–13.3%
  lower p95; medium resident memory increased, so no CPU/memory reduction is claimed.
  Type checking and rename remain optimization targets; gopls/all-project acceptance is open.

- 2026-10-20: Performance round 6 (ADR-112) integrated after clean whole-net review at
  `051eb2bf` and manager focused checks on top of native fmt. All six review findings are
  fixed: mixed dot/qualified rename validation, idle-GC generations, configuration provenance,
  dropped replies, idle-only warming, and merged-report gates. Manager checks passed for
  bounded checker equivalence, parallel-parser helper, cache invalidation/revert, rename,
  idle GC/warming, benchmark/CLI safeguards, build/vet, and a native-fmt tuple regression.
  A one-run pinned small/medium smoke covered fresh typing, imported fresh typing,
  completion-after-edit and rename (plus opens), with complete samples and zero failures;
  each selected row met its budget. This is not all-project or gopls acceptance. Its first
  attempt never started because the shared timing-lock wait expired; only the timing step
  was retried, not the already-passing tests.
  Historical pre-fix comparisons showed typing/rename/memory improvements, but the
  beat-gopls-every-row target remains unmet. At that round-6 checkpoint ordinary loading
  was still serial; the normal-loader follow-up above addresses it. Comparison tools still omit diagnostic ranges
  and metadata and use dependent-diagnostic settling sleeps; equivalence evidence is bounded.
  Step 11.13 is correctness-clean at `2192a725` but remains unmerged during focused guarded
  hover optimization: its earlier big-package hover/typing measurements miss their targets.
  No full CI, broad sweep, new Windows run or full benchmark was run for this integration.

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
- 2026-10-14: Job 11.4 (diagnostics as you type: server/diagnostics.go debounced per-package runs that publish
  per file, API-hash fan-out to importers, stale-result protection, didChangeWatchedFiles with dynamic
  registration, cache/probe.go background namespace probe with Provisional/Stale packages and at most 2
  probes at once, Session.Recheck, relatedInformation only when the client supports it) committed, clean
  after 2 review rounds. Keystroke p50 56 ms / p95 61 ms on a quiet machine (81/94 ms at load 14). Known
  differences for the 11.12 random-edit differential test (editor behavior, like gopls): every syntax error
  is shown, not only the first per line; an importer of a broken package shows its own errors too; an open
  `_test.gp` in a normal package gets the package's placeless diagnostics at its package clause (until test
  variants exist). The "change lock" is relaxed: requests take their snapshot on the read loop, and
  DidOpen/DidChange/DidClose/Invalidate are serialized by Session's change mutex. Left for 11.11: `under`
  in Clone is O(changed × handles); a probe started while waiting for the semaphore can be one round behind.
- 2026-10-14: Job 11.10b (editors/vscode: plain-JS client on vscode-languageclient 9.0.1 over stdio, settings
  gocpp.path/lsp.args/lsp.env/trace.server, restart command, Restricted Mode (no server until the folder is
  trusted), vscode-go's format-on-save and organize-imports defaults, contributes.breakpoints, node tests incl.
  an end-to-end start through the real createMessageTransports; docs/guides/editors.md for Neovim (tested
  headless, 0.11.6), Helix and Emacs (not tested yet), Zed (not supported yet)) committed, clean after 2 review
  rounds. Licenses: languageclient 10.x is skipped because it pulls BlueOak-licensed minimatch 10; 9.0.1 pulls
  ISC packages (semver, minimatch 5), accepted as permissive. Not tested inside a real VS Code window (the
  agents may not start a GUI): the user can try `code --extensionDevelopmentPath=$PWD/editors/vscode`.
- 2026-10-14: Job 11.3b (internal/lsp/cache/filecache: a disk cache per gocpp binary hash, atomic writes,
  checked entries, a trim that touches only its own files (5 days, 1 GiB, other binaries' folders 1 day);
  disk.go: dependency summaries from disk; opaque.go: the side table of C/C++ type spellings, in the API
  hash, `(*Package).OpaqueType` for hover) committed, clean after 2 review rounds. 50k-line workspace:
  cold open 43–108 ms (heap 45 MB), warm 8–18 ms (heap 4 MB). Deviation from the 11.3 note: the compiler's
  identity is not in the key; each summary stores the probe answers its check used, and they are asked
  again in the background at load (provisional until then; stale and re-checked if they differ), which also
  catches changed headers. Shallow export data was not needed. GOCPP_LSP_CACHE must be absolute (or `off`).
- 2026-10-14: Job 11.5 (hover in gopls's format incl. builtins, type switch variables and modest C/C++ hovers;
  definition/typeDefinition across files and packages and into header files; signature help; check.Config.Partial
  so broken packages still get types) committed, clean after 2 review rounds. p95 under 1 ms each (budget 30 ms);
  Partial adds ~15 ms to a keystroke that leaves the package broken (within 100 ms). Export-data positions are
  line-only and can be old after an API-preserving edit, so features re-find the declaration by name and kind
  (README §2.2 item 4 should say this). Left for later: definition into the embedded stdlib (no file on disk),
  answers in open _test.gp files (no test variants), build-constraint-excluded files in package lists
  (TODO(Phase 11)), `-I` folders from //gocpp:cflags (Phase 04). Follow-up: hover should use 11.3b's
  `(*Package).OpaqueType` so opaque types of imports stop showing as `invalid type` (merged separately).
- 2026-10-14: Job 11.7 (textDocument/formatting with gocpp fmt and a minimal line diff with a cost cap;
  Organize Imports as source.organizeImports: goimports-style sorting, headers after gocpp imports and never
  removed, unused gocpp imports removed from the type check, no action while the package is provisional or
  when code shares a line with an import declaration; a safety net that parses the result and compares the
  code outside the imports and the imports themselves) committed, clean after 4 review rounds. Benchmarks
  measured on a quiet machine in round 1 only (format 1,000 lines p95 1.6–5.3 ms; organize 0.3 ms); the
  later rounds ran at load 5–48: re-measure in 11.11's harness.
- 2026-10-14: Job 11.6 (completion: scope names ranked like gopls by depth and expected type, selector members
  with gopls's addressability rules, struct literal fields, keywords, call snippets with gopls's defaults,
  auto-import from an index of the embedded stdlib and the module (edits made by 11.7's organizer), lazy docs
  via completionItem/resolve, gopls's fix for dangling selectors through Snapshot.PackageWithFile (a private
  check of the fixed text, never cached for diagnostics); hover/signature help show C/C++ types of imports
  as written) committed, clean after 2 review rounds. Cached p95 2.7 ms, after an edit p95 32 ms (budget
  50 ms). The language server v0 track is done (M1+v0). Deviations (completer.go header): keywords at a
  blank statement, prefix filter (no fuzzy matching), docs through resolve, one dangling-selector pass.
- 2026-10-15 (Phase 04 job 4.4, accepted by the manager): internal/check rewrites each comparison of opaque
  (C/C++) values to a hidden `gp_cgoh.Cmp(x == y)` so it is a Go bool (like cgo). Measured on a 3000-line package
  that imports a header (fresh load, fake probe): with 6000 such comparisons Check takes ~165 ms vs ~125 ms
  before (+30%, still 2 check rounds); without C comparisons no change (~119 vs ~110 ms, noise); the real
  examples (cpp-std, raylib, sqlite) show no measurable difference. A `bool(p == nil)` shape can need one more
  round. Such a C-heavy 3000-line package was already above the 100 ms keystroke budget before; 11.11's
  benchmarks should include a C-heavy file, and incremental re-checking (Phase 11) is where this cost goes away.

- 2026-10-17: Jobs 11.8 and 11.9 merged after three review rounds (clean confirmation).
  Decisions: (1) rename follows a method through every interface of the module it satisfies (the family; narrowing to gopls's
  use-tied unification is a TODO.md line); (2) rename is refused when the declaring package or an importer is broken, when the
  result would not compile (the tests of unedited packages are re-checked too), or when a name clashes; (3) a renamed method that
  now implements fmt.Stringer, error, io.Reader/Writer/Closer or sort.Interface gets a warning (silent behaviour change); (4) the
  fixtures of the LSP tests are header-less and run with a missing GOCPP_CXX (a clang probe killed at server exit leaves the nix
  wrapper's cc-params file in TMPDIR; waiting for probes at shutdown is an optional TODO); (5) go-to-definition into module headers
  (job 6.1b) needs one early return in Pipeline.HeaderFile; limits for vendor/replace are in TODO.md.
