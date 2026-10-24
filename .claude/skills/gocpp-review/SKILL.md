---
name: gocpp-review
description: Independent review of a gocpp job branch - the short checklist of plans/2026-10-13-gocpp/WORKFLOW.md section 5 with the commands to run (diff, added-file hygiene, TODO claims, Go behavior via gp-repro, focused tests via gp-focused) and the review template. Use when you are the reviewer of a job.
---

# Reviewing a job

You review; you never edit the subject code (then nobody reviews your fix). Work in the job's
worktree, read-only except `.scratch/`.

## 1. Scope (2 minutes)

```sh
git log --oneline dev..HEAD
git diff --stat dev...HEAD
git diff --name-status --diff-filter=A dev...HEAD    # every ADDED file
git status --short                                   # uncommitted work = finding
```
Finding (should-fix or blocker) for: binaries, `.scratch` content, helper scripts (`edit*.py`,
`bisect*.sh`), logs, `*.out`, files over ~1 MB, edits outside the ticket's scope.

## 2. Read the whole diff against the ticket

`git diff dev...HEAD`. Every round, the whole change, not only the fixes. Check:
- **Go behavior**: matches `go run`, or a documented divergence. Prove it:
  `scripts/gp-repro .scratch/case.gp` on the edge cases the coder did not test (nil, empty, max
  values, panics, goroutines, both compilers with `-cxx clang++`).
- **Tests**: new behavior has run tests; error paths have `// ERROR` tests; goldens changed on purpose.
- **Generated C++ / runtime**: no UB (targeted `-asan` / `-race` on the new tests), readable,
  `#line` points at the right `.gp` lines; nothing outside `runtime/src/os/` calls the OS; Windows
  files still compile.
- **Plan**: matches the phase README/ADR; deviations reported, not hidden; docs updated.
- **TODO.md**: each claim the report says fixed is really fixed (repro it); a fixed claim the
  report forgot is a finding.
- **Quality**: gofmt/vet clean, `.clang-format` for C++, comments explain the non-obvious.

## 3. Run the tests yourself

`scripts/gp-focused -x` (and `-cxx clang++` for runtime/emitter changes); add what the change
needs. Report what ran, with results. Never `make ci` or full sweeps.

## 4. Review (WORKFLOW.md 3.3)

```
Review of job <name>, round N: changes requested | clean
1. [blocker] file:line: what is wrong, why, suggested fix.
2. [should-fix] ...
3. [nit] ...
Checked: <commands run and results>
```
Clean = no blocker and no should-fix left. Every issue you find, in scope, an older one in the
same area ("also on dev"), or one unrelated to the ticket, is a numbered finding for the coder to
fix fully in the next round, never a TODO.md claim (user, 2026-10-23). Mark a find "needs a
multi-day redesign" only when it really does; the manager then makes it its own job.
