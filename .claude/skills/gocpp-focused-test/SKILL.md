---
name: gocpp-focused-test
description: Pick and run only the tests a gocpp change needs (never make ci or full sweeps) - scripts/gp-focused maps the changed files to focused go test commands and runs them memory-capped through the dev shell with ccache. Use before every hand-over (coder) and to re-run a job's tests (reviewer, manager).
---

# Focused tests for a change

```sh
scripts/gp-focused                 # list the checks for the job's changes (git diff dev...HEAD + uncommitted)
scripts/gp-focused -x              # run them: in-devenv (ccache), MemoryMax=16G, timeout 20m each
scripts/gp-focused -x -cxx clang++ # the same with clang++ (do both when the runtime or the emitter changed)
scripts/gp-focused -x -mem 8G -timeout 10m tests/diff/foo.gp   # chosen files only
```

What it maps (see the script header for all rules): a changed Go package -> vet + its tests (heavy
`cmd/gocpp`, `tests`, `runtime`: only the Test functions of changed `_test.go` files);
`tests/<kind>/<name>.gp` -> that one subtest; `runtime/src|include/X` -> `TestCXX/<cxx>/O0/X` and
`tests/run/*X*`; `stdlib/<pkg>` -> the run tests importing it + `tools/stdgocount`;
`internal/lower|cppgen` -> also `TestGolden`.

Rules that keep it fast and honest:
- Never run `make ci`, `verify.sh`, `go test ./...`, or interop/phase/sanitizer sweeps. The user does
  the broad run. A targeted `-asan` run of your own repro is fine.
- `-x` runs with `-v` and reports `passed N, skipped M`; a command whose tests all skipped FAILS
  (phase gates: it sets `GOCPP_TEST_PHASE=10` like `make ci`; set it higher for phase 11-15 tests).
- The list is a start: add what the change really needs (e.g. `-run` on cmd/gocpp tests that cover
  a non-test file), and say in the report exactly which commands ran and their results.
- Runs go through `scripts/in-devenv` (ccache in front of every compiler, 100G cache). If you set
  `XDG_CACHE_HOME` for isolation, also set `CCACHE_DIR=$HOME/.cache/ccache`.
- `TestCXX` builds all ~35 runtime test programs even for one subtest; ccache makes the second run
  fast.
