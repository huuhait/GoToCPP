# ADR-061: Go's `test/` folder is the conformance suite, read from the pinned tarball and classified by rule

Status: accepted (2026-10-18), step 15.10.

## Context

Phase 15 asks that at least 90 % of the applicable run-tests of Go's own `test/` folder pass at the end
of the project. The folder has about 2,600 programs with a first-line directive that says how Go's
runner treats each (`run`, `errorcheck`, `compile`, `rundir`, ...). Three questions had to be settled:
where the files come from, what "applicable" means, and how progress is tracked while most features are
still missing.

## Decision

1. **The files are read from the pinned Go source tarball** (`tools/gotest/PINS`, the same three lines as
   `tools/portstd/PINS`, shared cache `$XDG_CACHE_HOME/gocpp/portstd`), never copied into the repository:
   they are Go's tests under Go's license, and the local GOROOT of a Nix shell has no `test/` folder. The
   repository keeps the selection (`manifest.txt`), the expectations (`expect.txt`, `passing.txt`) and a
   snapshot of the results (`docs/conformance/go-test.md`).
2. **Applicable is decided by rules, with a reason code.** `tools/gotest/classify.go` reads the header, the
   build constraints, the imports and the qualified names of a test, and says `applicable` or
   `skip:<reason>`. A reason must be true of the test **before** it is run (it uses assembly, gc flags,
   runtime internals, a package gocpp does not plan, a documented deviation, ...). A test that merely fails
   is never skipped. Tests that Go's runner skips itself are counted apart from gocpp's own decisions.
   The one reason that touches the pass rate through a design decision is `spec-deviation`: a test that
   depends on a deviation documented in `docs/spec/language.md` (today D19, a declaration named `C`; D23,
   `println` of a slice or map). The report shows these counts, so the choice stays visible.
3. **A failure is a cluster, a cluster has an owner.** The first diagnostic of a failing test, with quoted
   text and numbers removed, is its cluster key; `clusters.txt` maps keys to the plan step that fixes them,
   and `triage` writes `expect.txt` and the `needs` column of the manifest from it. A test that passes
   leaves `expect.txt` by itself, so a step's progress is the number of tests that leave its column.
4. **The regression gate is a set, not a rate.** `passing.txt` lists the tests that passed in the committed
   snapshot; `make conformance-check` runs those and fails when one stops passing. The set only grows
   (`gate -update` refuses to drop tests without `-allow-shrink`). The gate is opt-in and heavy: it is
   not in `make ci`; the tool's own tests are.
5. **Errorcheck follows Go's matcher** (`// ERROR "re"` per line, `LINE+1`, continuation lines, `-0`), applied
   to the output of `gocpp check -e`, so that a mismatch means a real difference in gocpp's checker.

## Consequences

- Nothing of Go's tests is vendored; an offline machine needs the cached tarball (`gotest fetch` once).
- The manifest depends on GOOS/GOARCH like Go's runner: the committed file is linux/amd64.
- The wording of gocpp's diagnostics matters now: the `errorcheck` tests compare message text with gc's, so a
  syntax error from the forked go/parser (`expected ';', found ')'`) fails a test that expects gc's
  (`unexpected )`). The failure clusters of the report list these; changing the wording is a decision for
  the language owner, not for this tool.
- Moving to a new Go version means re-pinning (`tools/portstd -pin`), copying PINS, regenerating the
  manifest and reviewing its diff.
