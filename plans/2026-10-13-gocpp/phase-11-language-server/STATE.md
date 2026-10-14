# Phase 11: Fast Language Server · STATE

**Status:** `[~]` in progress (only the TextMate grammar, job 11.10a, which needs nothing else; the v0 early track starts after Phase 01)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] Measured go/types re-check speed with cached deps: `strings` 3 ms, `encoding/json` 5 ms, `net/http` (28.5k lines) 29 ms parse+check (2026-10-13)
- [~] 11.1 LSP core (JSON-RPC stdio, initialize/shutdown, incremental sync, overlays; protocol types copied from gopls's generated code + generator) · *v0*
- [ ] 11.2 Snapshots + cancellation + debounce · *v0*
- [ ] 11.3 Memoized pipeline · *v0*
  - [ ] per-file parse+expand cache
  - [ ] per-package check cache
  - [ ] export-data summaries (memory + on-disk; deep `gcexportdata` first, shallow via vendored `gcimporter` if memory needs it; cache keyed by gocpp binary hash; side table for opaque type spellings)
  - [ ] API-hash short-circuit
  - [ ] LRU memory cap
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
