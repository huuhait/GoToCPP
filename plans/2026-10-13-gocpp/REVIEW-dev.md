# dev REVIEW: jobs against `dev` that are not a phase step

> Maintained by the **manager only**, per [WORKFLOW.md](WORKFLOW.md) (v2: 3 coders → PM merge → review until one CLEAN). The manager is the manager. Coders, the PM and reviewers never edit this file.
> Legend: `[ ]` not_started · `[~]` in_progress · `[x]` completed.
> Finding lifecycle: `OPEN` → `FIXED-PENDING` / `REFUTED-PENDING` → `CLOSED-FIXED` / `CLOSED-REFUTED` / `REOPENED`.
> Every review covers correctness + spec parity, tests + edge cases + determinism, and simplicity + safety, scoped to the job.
> Performance-shaped jobs run three techleads before the coders (§3 step 0); their briefs are recorded under "Techlead research".

## Jobs

| Status | Job | Steps | Rounds | Clean | Completed |
|---|---|---|---|---|---|
| `[~]` | `dev.evalorder` — map key read before the appended value's argument | – | 0 / 1 | 0 | – |
| `[~]` | `dev.probecached` — `TestDeclsDeterministic` fails when the probe cache is on (TODO 137) | – | 0 / 1 | 0 | – |
| `[~]` | `dev.stalehints` — four stale comments in the compiler (TODO 145) | – | 0 / 1 | 0 | – |
| `[~]` | `dev.staleflag` — two LSP completion nits (TODO 184) | – | 0 / 1 | 0 | – |
| `[~]` | `dev.whynotes` — `emit -annotate` headline "captures by reference" for a boxed capture (TODO 144) | – | 0 / 1 | 0 | – |
| `[~]` | `dev.rtbugs` — nine Runtime claims in one batch (TODO 103-109, 120, 126 first sub-claim) | – | 0 / 1 | 0 | – |

**Worker-limit note (manager, 2026-10-26).** WORKFLOW.md §1 sets the limit at five active workers,
and a job's three build-phase coders count as three. The four jobs above were started together at
the user's explicit instruction ("start all at once"), so while their coder rounds run, active
workers are 13 (12 coders + `dev.evalorder`'s c2) — over the limit, knowingly, for this round only.
To keep the machine usable every coder was given `-cpus 6 -mem 6G` instead of the `-cpus 24 -mem 16G`
default (§`scripts/gp-focused:12-14` — unbounded parallel builds push the load past 1000 and time
out other jobs' tests). Their PM and review rounds are sequenced, not parallel, so the count drops
back under the limit before any of them merge.

**Manager error: worktree collisions, and how it was repaired (2026-10-26).** The first attempt to
spawn the four jobs' first coders failed on an unavailable model override (three dead agents, one
silent). The manager respawned them **without** noticing that it then created a second batch of
twelve worktrees on top, so three jobs ended up with **two live agents in one worktree**:
`dev.probecached` c1, `dev.stalehints` c1 and `dev.staleflag` c1. `dev.whynotes` did not collide
(its second agent died on the model error before touching the tree).

Damage and repair:

| Slot | What happened | Repair |
|---|---|---|
| `dev.probecached` c1 | Committed `bd1c5f2e`, which the surviving agent described as adopting "the existing uncommitted draft" left by the other agent — **not an independent version** | Worktree reset to `dev`, coder respawned clean. `bd1c5f2e` was manager-verified (green in both cache modes) but is not used as a v2 version |
| `dev.stalehints` c1 | Two agents edited the same four files; uncommitted work only. One agent noticed the collision itself ("a second process is concurrently editing and testing in this same worktree") | Worktree reset, coder respawned clean |
| `dev.staleflag` c1 | Committed `9a36a05c`; lost the race against the other agent and both edited the same two files | Worktree reset; `9a36a05c` kept as tag `contaminated-staleflag-c1`. Work was manager-verified (incl. a reproduced negative check) but is **not** counted as an independent version; coder respawned clean |
| `dev.staleflag` c3 | Stopped by mistake while cleaning up (it was in its own clean worktree, not a duplicate); it had uncommitted work | Worktree reset, coder respawned clean |

No worktree was shared at any point (all twelve are separate paths); the collision was two agents
per *worktree*, not per repo. Lesson for the next job: count the worktrees against the agents in the
same step, and when a spawn fails, resume the dead agent rather than adding new ones.

**Correction: `signal: killed` under load is the 6G scope cap, not the machine (2026-10-26).** The
manager first attributed the stalehints c2 `TestCXX/g++/O2/unit` `signal: killed` to machine load.
`journalctl --user -u run-p1461356-i5655052.scope` shows what actually happened: the scope was
**OOM-killed**, "Consumed 31min 7.181s CPU time over 5min 30.507s wall clock, **6G memory peak**,
5G memory swap peak". `scripts/gp-focused` runs each command under `systemd-run -p MemoryMax=$mem`,
so `-mem 6G` is a hard cap on a test whose C++ compile legitimately needs more, and the kernel
kills it. A second runtime `TestCXX` scope (`run-p1097039-i5290735.scope`) died the same way at 28s.
This is a **recurring artifact of running `./runtime`'s `TestCXX` under `-mem 6G`**, not a property
of any diff: no job in this batch may be marked FAILED on a killed `TestCXX`/`./runtime` test without
re-running that one test at `-mem 16G` on a quiet machine, or comparing against the same commit's
result on `dev`. Every `signal: killed` in this file must be read that way.

**Second correction: two processes ran bare `go test` outside any scope (2026-10-26).** While
checking the load, the manager found both read-only PMs running unthrottled suites
(`go test -count=1 -parallel 4 ./internal/typedinterop/probe` and `go test -count=1 -run '^TestDiff$' ./tests`)
as direct children of the session, with no `systemd-run` wrapper: 52 of the 324 live `cc1plus`
processes were theirs, and load reached 246 on 64 cores. Both were messaged to stop and to settle
questions with `scripts/gp-focused -x -cpus 6 -mem 6G` instead. Lesson: a worker's "read-only" does
not mean "no load" — a PM told to investigate a test failure will run it, so the ticket has to say
where its budget is. The four tickets tell coders the `-cpus 6 -mem 6G` rule but the PM briefs did
not.

---

## Job dev.evalorder: `m[s+"x"] = append(m[s+"x"], f())` reads the key before `f` (Go: after)

**Status:** `[~]` in_progress · **Merging coder:** – · **Canonical worktree:** ~/worktrees/GoToCPP/dev.evalorder

### Brief

> (The exact coder brief from WORKFLOW.md §7.1, filled in; the same text went to all three coders.)

- **Goal:**
  Go evaluates `m[k] = append(m[k], args...)` by reading the map expression and the key **first**
  only into temporaries when `k` is not a name/constant, and it reads the key **after** the
  append arguments when the key needs no conversion for the fast map calls. gocpp reads it before
  in a case where gc does not: `sameSafe` accepts a string `+` in `*ast.BinaryExpr`, but gc's
  `ir.SameSafeExpr` has **no `OADDSTR`** — a string `+` is lowered to `OADDSTR` before the walk, so
  gc returns false and does **not** take the `mapAppend` fast path. gocpp's fast path then reads
  `s+"x"` before `f()`, and `f` changing `s` sends the value to the wrong key.

  The job: make `sameSafe` match gc's `SameSafeExpr` operator set (a string `+` is not reusable —
  it allocates and it is `OADDSTR` to gc, so drop `token.ADD` for string operands), and check the
  whole operator/conversion set against `ir/expr.go:1040` rather than only the reported case.
  Anything else that `SameSafeExpr` rejects must be rejected too, since every acceptance of it
  lets gocpp reorder relative to Go.

- **Reference / plan:**
  - `internal/lower/maps.go:495` (`mapAppend`, the fast path and its doc comment) and
    `internal/lower/maps.go:656` (`sameSafe`, the operator switch at 703–711 and the conversion
    case at 712–718).
  - gc: `$(go env GOROOT)/src/cmd/compile/internal/ir/expr.go:1040` `SameSafeExpr`; `walk/assign.go:39`
    (`ir.SameSafeExpr(left, mapAppend.Args[0])`, the only caller that matters here);
    `typecheck/typecheck.go:322–334` (where `+` of strings becomes `OADDSTR`).
  - `plans/2026-10-13-gocpp/phase-03-core-language/README.md` §2.3 (Maps).
  - Existing regression program `tests/diff/maps_append_key.gp` — it covers the key-type rule
    (`wrappedKey`) but **not** a key that is an expression whose operands `f` changes, which is
    exactly the bug. Read it; it is the model for the new test.

- **Done when:**
  1. A new differential test program (under `tests/diff/`, run by `TestDiff`) that fails on `dev`
     and passes with the patch: `m[s+"x"] = append(m[s+"x"], f())` where `f` changes `s`, checked
     against real Go with `scripts/gp-repro`. Add the neighbouring forms in the same program —
     `int + int`, `uint32 + uint32`, `float64 + float64`. (Only the string `+` is a bug today, but
     the test must pin the whole rule so a later edit cannot re-open it.) Keep the file
     gofmt-clean and `// phase: 03`.
     **Correction (coder 3, verified by the manager):** a `tests/diff` program must **not** have a
     `// Output:` block — `diffFormat` (`tests/format.go:117`) allows only
     `diffLine|lsanLine|pendingLine|goBuildLine`, and `tests/format.go:179` requires `// Output:`
     only where it is allowed. The harness compares against real Go itself. An earlier version of
     this ticket asked for one; that was wrong.
  2. Whatever the correct answer turns out to be in each case must equal `go run` on this machine
     (get it with `scripts/gp-repro`, which diffs the two for you).
  3. `scripts/gp-focused -x` green, and green again with `-cxx clang++` if you touched anything
     the runtime sees (you should not need to).
  4. Prove the new test fails before the patch: put the pre-fix commit hash and the failing
     output in the handoff.

- **Scope:** `internal/lower/maps.go` (+ its `_test.go` if a unit test is the natural place for a
  `sameSafe` case), and the one new `tests/diff/*.gp` file. Do **not** touch `wrappedKey`,
  `mapShift`, `mapPre`/`mapSimple` emission, or the general assignment-order machinery in
  `internal/lower/order.go` — the ordering they implement is already right; only the "is this
  expression safe to re-read" predicate is wrong.

- **TODO.md claims to fix (quoted):**
  > - [ ] Order: `m[s+"x"] = append(m[s+"x"], f())` reads the key before f (Go after): gocpp's
  >   `sameSafe` accepts a string `+`, gc's SameSafeExpr has no OADDSTR, so gc does not use
  >   mapAppend there (dev.makeslice review, also on dev).

- **Test commands the manager re-runs:** `scripts/gp-focused -x` in the version's worktree, plus
  `scripts/gp-repro` on the new program against real Go.

### Techlead research (performance-shaped jobs only)

Not a performance job. This is a Go-conformance (evaluation order) bug; no techlead stage.

| Techlead | Baseline command + numbers | Layer | Root cause (or `HYPOTHESIS`) | Proposed change | Expected recovery |
|---|---|---|---|---|---|
| – | – | – | – | – | – |

### Versions

| Version | Coder | Worktree / branch | Manager test run | Commit (state hash) |
|---|---|---|---|---|
| v1 | coder 1 | ~/worktrees/GoToCPP/dev.evalorder-c1 · `job/dev.evalorder-c1` | `scripts/gp-focused -x` green: gofmt clean · vet OK · internal/lower 407 passed 0 skipped · TestGolden 41 passed 1 skipped · TestDiff/maps_append_key_op 3 passed | `e1717e08` (3 commits: `0e3185cb`, `de44b3df`, `e1717e08`) |
| v2 | coder 2 | ~/worktrees/GoToCPP/dev.evalorder-c2 · `job/dev.evalorder-c2` | **manager verified independently** on `c764cdfb`: `go build`/`go vet`/`gofmt -l` clean · `TestDiff/maps_append_key_expr` ok 16.189s · `go test ./internal/lower` ok 11.320s · **negative check reproduced by the manager**: reverting only `internal/lower/maps.go` to `dev` fails exactly 11 rows (string+string, constant+string, string+unchanging-string, string+itself, both no-op conversions, promoted and explicit field, explicit and promoted field, pointer→unsafe.Pointer, nil→*int, promoted through an embedded pointer); restored → green. Models gc's post-typecheck `SameSafeExpr` (`stripConvNop`/`convNopExpr` for OCONVNOP, the OADDSTR rule, OCONV only for `types.IsSimple`, `fieldPath` for the flattened ODOT/ODOTPTR chain) | `c764cdfb` (2 commits: `741c2633`, `c764cdfb`) |
| v3 | coder 3 | ~/worktrees/GoToCPP/dev.evalorder-c3 · `job/dev.evalorder-c3` | `scripts/gp-focused -x` green: gofmt clean · vet OK · internal/lower 405 passed 0 skipped · TestGolden 41 passed 1 skipped · TestDiff/maps_append_key_add 3 passed | `87d12c919c42` |

#### Manager ground truth (`.scratch/mgr/table_rigorous.gp`, `scripts/gp-repro`, each row
resetting its variable immediately before the statement)

| Row | shape | dev 19646648 | v3 87d12c91 | Go | state |
|---|---|---|---|---|---|
| r1 | `m[s+"x"] = append(m[s+"x"], f())` | BEFORE | AFTER | AFTER | fixed |
| r2 | `m[string(s)] = append(m[string(s)], f())` | AFTER | BEFORE | BEFORE | fixed |
| r3 | `m[string(s)] = append(m[s], f())` | AFTER | BEFORE | BEFORE | fixed |
| r4 | `m[s] = append(m[s], f())` | AFTER | AFTER | AFTER | ok |
| r5 | `m[i] = append(m[i], f())`, `map[any]`, int key | AFTER | AFTER | BEFORE | **open** |
| r6 | `m[i+1] = append(m[i+1], f())`, `map[int]` | BEFORE | BEFORE | BEFORE | ok |
| r7 | `m[u]`, `map[uint32]` | AFTER | AFTER | AFTER | ok |
| r8 | `m[f]`, `map[float64]` | AFTER | AFTER | AFTER | ok |

Note for whoever repeats this: an earlier manager table reported r5 as "same" because it reset
the variables inside `f` instead of before the statement, so `f` set them to the value they
already had. Coder 3 caught the discrepancy. Every row must reset first.

**Both versions reach the same four-row result.** The manager re-ran the table on v1
(`e1717e08`) and v3 (`87d12c91`): both fix r1, r2, r3 and both leave r5. Two independent
coders, two different implementations, same measured outcome — that agreement is the evidence
that r1–r3 are the real boundary and r5 is genuinely a different mechanism.

**v1's own regression, caught and fixed inside the job (verified by the manager).** Commit
`0e3185cb` tested "identical types" before the float guard, so it stripped `float64(f)` of a
`float64`. gc's `tcConv` promotes a same-kind float or complex conversion back to `OCONV` even
from the identical type, so gocpp then took the fast path where gc does not. Repro
(`hold := map[float64][]int{1: {1}, 5: {99}}; hold[float64(kf)] = append(hold[kf], setkf())`):

| tree | output | Go |
|---|---|---|
| dev `19646648` | `[99 7] [99]` | `[99 7] [99]` — correct |
| `0e3185cb` | `[1 7] [99]` | **regression** |
| `e1717e08` | `[99 7] [99]` | correct |

Confirmed by the manager on all three trees. `e1717e08` must be read with `0e3185cb` — the
intermediate commit is known-wrong on this one shape. Note the regression only shows when both
keys already exist in the map; a map with no prior entry hides it, which is why the diff test
seeds `hold` with entries for both keys.

**Scope decision (manager, 2026-10-26).** r5 is a divergence from gc, not a spec violation: the
Go spec leaves key-vs-value order unspecified ("evaluation order between the key and the value
is not specified"), but `docs/spec/cpp-mapping.md:350` promises gc's order where the spec is
open. Its fix is not in `sameSafe`: coder 3 implemented a `keyNop` gate (accept the fast path
when `AssignConv` inserts no node), measured it, and reverted it — the emitted C++ then built the
interface twice and it flipped `m[*p]` / `m[arr[i]]` on `map[any]` from correct to wrong. r5
belongs to the general map-store order machinery in `internal/lower/order.go`, which this ticket
puts out of scope, so **r5 becomes its own job** and gets a TODO.md line rather than being
claimed here.

### PM merge plan

**PM:** – · **Merging coder:** –

> (The PM's merge plan, copied unchanged.)

**Merge result:** – · **Commit (state hash):** – · **Departures from the plan (from the merge handoff):** –

### Rounds

| Round | Fresh reviewer # | Commit (state hash) | Manager test run | Verdict |
|---|---|---|---|---|
| R1 | – | – | – | – |

Verdicts: `FINDINGS (n)` · `CLEAN` · `VOID (reason)`. Every reviewer number is a new instance.

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| – | – | – | – | – | – | – | – | – |

- **Coder response:** `FIX` (the default: patch + failing-without test, named) · `REFUTE` (exception, **once per finding only**: evidence `file:line` + reproduction). The coder exists to shrink TODO.md, so refusing is rare.
- **Adjudication:** `CONFIRM-REFUTED` / `REJECT-REFUTED` / `FIX-VERIFIED (test fails pre-fix: yes)` / `FIX-REJECTED`, with the reviewer # and evidence. **Single pass: the reviewer's verdict is final in both directions; the manager closes a confirmed refutation without re-checking.**
- **State:** never set to `CLOSED-*` by the manager alone. `CLOSED-REFUTED` needs a coder refutation **and** a reviewer `CONFIRM-REFUTED`; `CLOSED-FIXED` needs a reviewer `FIX-VERIFIED`.

### Disputes

> None by design: `REFUTE` is single-shot and the reviewer adjudicates it once (WORKFLOW.md §6). This table exists only for a claim neither side could demonstrate, which goes to the user.

| Finding | Refutations rejected | Reproduction | Result | User decision |
|---|---|---|---|---|
| – | – | – | – | – |

### Out of scope

Reviewer notes about things outside this job. They do not block; each is filed where it belongs.

| Round | Note | Filed in |
|---|---|---|
| – | – | – |

### Completion

> Fill in **only** after a CLEAN round, with no finding outside `CLOSED-*`.

- [ ] CLEAN round: –, fresh reviewer –
- [ ] Commit (state hash) of that round: –
- [ ] Final manager test run on that hash: –
- [ ] Every finding `CLOSED-FIXED` / `CLOSED-REFUTED`: –
- [ ] Out-of-scope notes filed: –
- [ ] Squash-merged into dev (commit), TODO.md claims closed, step marked `[x]`: –
- Re-entered by: –

---

## Job dev.probecached: `TestDeclsDeterministic` fails whenever `GOCPP_PROBE_CACHE` is set

**Status:** `[~]` in_progress · **Merging coder:** – · **Canonical worktree:** ~/worktrees/GoToCPP/dev.probecached

### Brief

> The ticket is `.scratch/mgr/tickets/dev.probecached.md`; it was copied to the coders unchanged.

- **Goal:** `TestDeclsDeterministic` (`internal/typedinterop/probe/decls_more_test.go:13`) zeroes
  `Timing`, `Stats`, `Deps` and `Runs` at line 21 but not `Cached`, so with `GOCPP_PROBE_CACHE` set
  the second run marshals `"cached":true` and the byte comparison fails. Make the test pass with and
  without the cache, without weakening what it proves.
- **Done when:** the ticket's repro passes, the no-cache form still passes, the package passes,
  `scripts/gp-focused -x -cpus 6 -mem 6G` green, gofmt and vet clean, pre-fix output quoted.
- **Scope:** `internal/typedinterop/probe/decls_more_test.go` only. A test bug, not a product bug.
- **TODO.md claim to fix (quoted, line 137):**
  > - [ ] `TestDeclsDeterministic` (internal/typedinterop/probe/decls_more_test.go:13) fails whenever `GOCPP_PROBE_CACHE` is set: the second run comes from the cache (`"cached":true`) and the comparison keeps that field. Repro: `GOCPP_PROBE_CACHE=$(mktemp -d) go test -count=1 -run '^TestDeclsDeterministic$' ./internal/typedinterop/probe` (found by the dev.probecache review).

**Manager pre-check:** reproduced the failure on `dev` @ `1a1224be` before spawning; the claim is open.

### Versions

| Version | Coder | Worktree / branch | Manager test run | Commit (state hash) |
|---|---|---|---|---|
| v1 | coder 1 (respawned after a worktree collision) | ~/worktrees/GoToCPP/dev.probecached · `job/dev.probecached` | manager re-ran: cache `ok 0.996s` · no cache `ok 2.021s` · `-run '^TestDecls'` `ok 36.429s` · gofmt/vet clean · manager re-ran the ticket repro again: with a fresh `mktemp -d` cache `ok 1.260s`, without it `ok 2.082s` | `ab5a563e` |
| v2 | coder 2 | ~/worktrees/GoToCPP/dev.probecached-c2 · `job/dev.probecached-c2` | manager re-ran: cache mode `ok 2.786s` · no cache `ok 2.790s` · subtests `no_cache` PASS, `cache` PASS · gofmt/vet clean | `acb587d0` |
| v3 | coder 3 | ~/worktrees/GoToCPP/dev.probecached-c3 · `job/dev.probecached-c3` | manager re-ran: cache `ok 1.032s` · no cache `ok 2.195s` · `-run '^TestDecls'` with cache `ok 81.928s` · mutation check reported by the coder · **negative check reproduced by the manager**: reverting only `decls_more_test.go` to `dev` with `GOCPP_PROBE_CACHE` set fails, and the two marshalled tables differ in exactly one field — the second has `"cached":true` and the first does not — which is precisely the TODO claim | `62de3d7b` |

### Rounds

| Round | Fresh reviewer # | Commit (state hash) | Manager test run | Verdict |
|---|---|---|---|---|
| R1 | – | – | – | – |

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| – | – | – | – | – | – | – | – | – |

---

## Job dev.stalehints: four stale comments in the compiler

**Status:** `[~]` in_progress · **Merging coder:** – · **Canonical worktree:** ~/worktrees/GoToCPP/dev.stalehints

### Brief

> The ticket is `.scratch/mgr/tickets/dev.stalehints.md`; it was copied to the coders unchanged.

- **Goal:** make four stale comments true today. Comment accuracy only, no behaviour change.
  - `internal/lower/ops.go:567` says "Phase 03 adds string(r)" — `string(r)` is implemented
    (`internal/lower/ops.go:36`).
  - `internal/lower/expr.go:669` says "from Phase 03 on" — Phase 03 is long done.
  - `cmd/gocpp/build.go:1035` names only `GOCPP_TYPED_INTEROP=1`, now the legacy spelling;
    `GOCPP_INTEROP` / `-interop` win (`internal/typedinterop/mode.go:25,45-47,112`).
  - `runtime/include/gocpp/testing.hpp:7` lists `flag` among Phase 09 stubs; `stdlib/flag/flag.gp`
    exists and `stdlib/testing/testing.gp` imports it.
- **Done when:** each site reads correctly, old→new quoted with file:line evidence, gofmt/vet clean,
  `scripts/gp-focused -x -cpus 6 -mem 6G` green.
- **Scope:** those four files, comments only. No executable line changes.
- **TODO.md claim to fix (quoted, line 145):**
  > - [ ] Stale comments: internal/lower/ops.go:562 "Phase 03 adds string(r)", internal/lower/expr.go:644 "from Phase 03 on", cmd/gocpp/build.go:1016 "when GOCPP_TYPED_INTEROP=1", runtime/include/gocpp/testing.hpp:5-8 lists `flag` among stubs (was L199, L299, L292).

**Manager pre-check:** located all four sites on `dev` @ `1a1224be`; the line numbers in the TODO
line have drifted (the ticket carries the real ones). All four claims verified stale.

### Versions

| Version | Coder | Worktree / branch | Manager test run | Commit (state hash) |
|---|---|---|---|---|
| v1 | coder 1 | ~/worktrees/GoToCPP/dev.stalehints · `job/dev.stalehints` | manager re-ran: `go build` + `go vet` on `./cmd/gocpp ./internal/lower` clean · every added line is a `//` comment (the only code-line change is the trailing comment on `attachTyped`) · manager checked v1's cross-references: `methodCall` is `internal/lower/methods.go:447`, `cname` is `internal/lower/expr.go:251`, `callValue` is `internal/lower/closures.go:423`, and `typedinterop.ResolveMode` is the function that orders `-interop` → `GOCPP_INTEROP` → `GOCPP_TYPED_INTEROP` (`internal/typedinterop/mode.go:25,112`) | `c676f80c` |
| v2 | coder 2 | ~/worktrees/GoToCPP/dev.stalehints-c2 · `job/dev.stalehints-c2` | manager re-ran: `go build` OK · vet clean · `TestTestingNativesDefined`/`TestTestingNatives` ok 34.283s · `go test -count=1 ./internal/lower` ok 11.766s · manager verified every added line is a comment and `build.go:1035`'s statement is byte-identical · coder reported one `TestCXX/g++/O2/unit` `signal: killed`; the manager re-examined it and it is the 6G `MemoryMax` scope cap, not the diff (see the OOM note above) · manager checked every factual claim the new prose makes: `T.Setenv` does call `os.Setenv` (`stdlib/testing/testing.gp:884`), `lookup_env` has no Go caller but is used by `runtime/test/testing_natives_test.cpp:110`, and `stdlib/testing/PORT.md:13` does carry the native table the comment points at | `4ff62002` |
| v3 | coder 3 | ~/worktrees/GoToCPP/dev.stalehints-c3 · `job/dev.stalehints-c3` | manager re-ran: `go build` + `go vet` clean · comments only (same one trailing-comment code-line change as v1) · **v3 reframes the `testing.hpp` list instead of editing it**: it names the packages whose functions the natives *replace* ("sync, time, os, runtime, strconv, io, …") rather than the packages `testing` *imports*. Both framings are true of the new text — `stdlib/testing/PORT.md:11-22` lists natives replacing `sync`, `time`, `os`, `runtime`, `strconv`, `regexp`, `io` — so v3's sentence is accurate, though it changes the comment's subject | `d3423e13` |

**Where the three versions disagree (for the PM):** all three agree the four sites are stale and fix
them, but on `runtime/include/gocpp/testing.hpp` they take three different routes to a true sentence.
v1 keeps the original "the packages that Go's testing imports" framing and edits the claim in place
to "…are real packages now". v2 does the same but **drops `flag` from the list**. v3 keeps the list's
subject out entirely and names the packages the natives **replace** instead.

**Manager correction (2026-10-26).** The manager first wrote here that v1's list was a category error
because gocpp's `stdlib/testing` imports only `errors, flag, fmt, os, runtime/debug, time`. **That was
the manager's mistake**: the comment's subject is *Go's* `testing` package, not gocpp's. Go's own
`testing/testing.go` imports — among others — `sync`, `sync/atomic`, `time`, `os`, `runtime`,
`runtime/debug`, `flag`, `strings`, `strconv`, `io`, `bytes`, `context` (checked against the dev
shell's `$(go env GOROOT)/src/testing/testing.go`). So the original list "sync, time, os, runtime,
flag, strings, …" is **correct as a list of Go's testing imports**, and the only stale part of the
sentence is the claim that they "are declaration stubs until the later steps of Phase 09" — all six
are `Real: true` in `tools/stubgen/main.go` today.

Consequences for the PM: **v2's removal of `flag` is factually wrong** — Go's `testing` imports
`flag`. v1's minimal edit is correct. v3's reframing is also true (PORT.md backs it) but changes what
the comment is about; it is a legitimate style choice, not a correctness fix. The PM should decide
between v1 and v3 on style, and must not take v2's list.

### Rounds

| Round | Fresh reviewer # | Commit (state hash) | Manager test run | Verdict |
|---|---|---|---|---|
| R1 | – | – | – | – |

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| – | – | – | – | – | – | – | – | – |

---

## Job dev.staleflag: two LSP completion nits

**Status:** `[~]` in progress · **Merging coder:** 3 · **Canonical worktree:** ~/worktrees/GoToCPP/dev.staleflag

### Brief

> The ticket is `.scratch/mgr/tickets/dev.staleflag.md`; it was copied to the coders unchanged.

- **Goal:** resolve the two completion nits of TODO 184.
  - A stray blank line before `AdditionalTextEdits` (`internal/lsp/server/completion.go:155-159`).
  - `TestFixDangling` builds `cache.ParsedFile` without `Mode` (`internal/lsp/server/completion_test.go:928`).
  Both are reviewer shorthand: the coders must find the actual defect from the code, and may report
  a claim as cosmetic or stale with evidence instead of inventing a fix.
- **Done when:** each item fixed with a failing-without test (if behavioural) or refuted with
  file:line evidence; `go test ./internal/lsp/server` passes; gofmt/vet clean; focused green.
- **Scope:** `internal/lsp/server/completion.go`, `internal/lsp/server/completion_test.go`.
- **TODO.md claim to fix (quoted, line 184):**
  > - [ ] Completion nits: a stray blank line before `AdditionalTextEdits` (internal/lsp/server/completion.go:158); `TestFixDangling` builds `cache.ParsedFile` without `Mode` (was L182).

**Manager pre-check:** both sites located on `dev` @ `1a1224be`. Whether either is behavioural or
cosmetic is deliberately left to the coders to establish.

### Versions

| Version | Coder | Worktree / branch | Manager test run | Commit (state hash) |
|---|---|---|---|---|
| v1 | coder 1 (respawned after a worktree collision) | ~/worktrees/GoToCPP/dev.staleflag · `job/dev.staleflag` | manager re-ran `TestFixDangling` ok 0.032s; manager verified the commit touches only the two scoped files; coder proved item 1 is protocol-invisible by marshalling six completion queries with and without the patch (identical output) · coder's `./internal/lsp/server` 499.778s and focused 667.977s both green | `c65e0044` |
| v2 | coder 2 | ~/worktrees/GoToCPP/dev.staleflag-c2 · `job/dev.staleflag-c2` | **manager verified independently**: `TestFixDangling` ok 0.038s, gofmt clean; reverting only `Mode: cache.ParseFull` fails both new rows (`#M(1)` and the `#M(a)` top-level macro), restored green. v2 fixes the blank line by **replacing it with a comment** rather than deleting it, which also restores gofmt's alignment — a third reasonable reading of the same nit | `ad28e422` |
| v3 | coder 3 (respawned after being stopped by mistake) | ~/worktrees/GoToCPP/dev.staleflag-c3 · `job/dev.staleflag-c3` | **manager verified independently**: reverted only `Mode: cache.ParseFull` → `TestFixDangling` FAILS on the `macro` row (`macro m(){\n\tp._\nvar y = 1\n\tp.\n...` — the second `p.` is lost), restored → ok 0.032s. Commit touches only the two scoped files. Coder's `./internal/lsp/server` 654.678s (1215 passed) and focused green | `8129beef` |

**Both versions agree on what the two nits are, and the manager agrees with them:**
- The blank line is **cosmetic** — a struct-literal layout nit that breaks gofmt's field alignment
  and never reaches the protocol (`AdditionalTextEdits` is `json:"additionalTextEdits,omitempty"`,
  `internal/lsp/protocol/tsprotocol.go:912`; no value changes). No test can fail on it, which v1
  demonstrated by diffing marshalled completion output with and without the patch.
- The missing `Mode` is **behavioural**. `fixDangling` re-parses with `pf.Mode`
  (`internal/lsp/server/completion.go:253`); with mode 0 Phase 10 syntax is an error whose recovery
  swallows the second dangling selector. The right value is `cache.ParseFull`, which is what the
  feature-facing parse uses (`internal/lsp/cache/parse.go:31`, `completion.go:179`).

**Refuted out-of-scope lead (c3).** c3 flagged `internal/lsp/server/autoimport.go:593`
(`parser.ImportsOnly|parser.ParseComments`, no `Phase10Syntax`) as possibly mis-parsing a Phase 10
header. The manager tested it: `ImportsOnly` stops at the import header and never parses a body, so
Phase 10 syntax in the body cannot reach that parse. A probe on a file with both `F!(1)` in a body
and a `macro` declaration parses with `err=<nil>` under `ImportsOnly|ParseComments`, and identically
under `+Phase10Syntax`, while `ParseFull` errors on the body as expected. No TODO line filed.

**Out-of-scope note to carry to the merge:** `internal/lsp/server/repair_test.go:14-18` has the same
missing-`Mode` fixture shape. Both coders checked `repair.go` and found it never reads `pf.Mode`, so
it is harmless there; it stays out of this job.

### PM merge plan

**PM:** fresh read-only PM · **Merging coder:** 3

> Base: **v3**, merging coder **3**. v1 and v3 have **byte-identical** `completion.go`
> (blob `6dec15da`) and differ only in the test; v3's `macro` row is the load-bearing
> declaration-level row and the one the manager reproduced. One row swaps in from v1 and the test
> comment is trimmed.
>
> | Item or file | Take | Reason |
> |---|---|---|
> | item 1, blank line (`completion.go:155-160`) | v1 = v3 (identical) | Deleting it puts all five fields in one gofmt alignment run; v2's comment keeps the two-run split it was meant to remove |
> | item 1 evidence (cosmetic claim) | v1 | The only version that demonstrated protocol-invisibility by diffing marshalled output |
> | fixture parse + `Mode` field | v3 (same bytes as v1) + v1's one-line rationale | All three agree on the code; v1's "a file as `cache/parse.go:70-74` builds it" is the only version that explains why the fixture changed |
> | row, re-parse half | **v1's `F!(1)` row** | The only shape that fails when just `pf.Mode` is reverted; v3's `F!(y)` passes under exactly that revert |
> | row, declaration half | v3's `macro m()` row | Fails (`got=""`) if either half is reverted; pins both |
> | test comment | v3, trimmed | v3's names both constructs; trim it to describe the `F!(1)` row too |
> | `repair_test.go` | no change | Out of scope; `repair.go` never reads `pf.Mode` |
>
> Dropped: v2's replacement comment (keeps the two-run split), v2's three-line fixture reflow,
> v1's `#Name()` and v2's `#M(1)` rows (same half as `F!(1)`), v2's top-level `#M(a)` (same half as
> the macro row), and **v3's `F!(y)` row — it cannot fail without the patch on its own**.

**Manager verification of the PM's row analysis.** The PM's table is the sharpest technical claim in
this plan, so the manager re-measured it directly on v3 + the `F!(1)` row, changing one thing at a time:

| Revert | `F!(1)` row | `macro m()` row | `F!(y)` row |
|---|---|---|---|
| nothing (all patched) | pass | pass | pass |
| only `Mode: cache.ParseFull` in the struct | **FAIL** (second `p.` unfixed) | **FAIL** (`got=""`) | **passes — vacuous** |
| only the fixture's own `parser.ParseFile` mode | passes | **FAIL** (`got=""`) | – |

So the PM is right on both counts: `F!(1)` is needed to pin the struct half, and v3's own `F!(y)` row
pins nothing by itself.

**Manager correction to its own earlier note.** The manager wrote above that the blank line "breaks
gofmt's field alignment" as if the file were not gofmt-clean. The PM checked and the manager
confirmed: `gofmt -l internal/lsp/server/completion.go` is **empty on `dev`**, before and after all
three patches. The blank line is legal gofmt; it only splits the literal into two alignment runs.
The nit is readability, which strengthens the "cosmetic, no test can fail" conclusion rather than
weakening it.

> **Discarded version.** The collided `dev.staleflag` c1 produced `9a36a05c`. It was manager-verified
> — `TestFixDangling` green, and the manager independently reproduced the negative check (dropping
> `Mode` from the fixture makes the new `F!(g)` case fail, printing the un-repaired second `p.`) —
> but it shares a worktree with another agent's work, so it is **not** an independent v2 version and
> is not used. It is kept as tag `contaminated-staleflag-c1` for reference. Its finding is
> independently corroborated by v2 (`ad28e422`), which reached the same root cause through different
> test cases (`#M(...)` Phase 10 syntax rather than `F!(g)`).

### Rounds

| Round | Fresh reviewer # | Commit (state hash) | Manager test run | Verdict |
|---|---|---|---|---|
| R1 | – | – | – | – |

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| – | – | – | – | – | – | – | – | – |

---

## Job dev.whynotes: `emit -annotate` headline says "captures by reference" for a boxed capture

**Status:** `[~]` merged, in review · **Merging coder:** 3 · **Canonical worktree:** ~/worktrees/GoToCPP/dev.whynotes

### Brief

> The ticket is `.scratch/mgr/tickets/dev.whynotes.md`; it was copied to the coders unchanged.

- **Goal:** the headline of a local closure in `internal/lower/whynotes.go` `noteClosure` (~line 115)
  states unconditionally that the closure "captures by reference", while the per-variable notes
  printed below it correctly say "captures X by sharing its box" for a boxed capture. Make the
  headline accurate without duplicating box detection in a way that can drift.
- **Done when:** a golden/unit test pins the new headline, the test fails without the patch (pre-fix
  output quoted), `go test ./internal/lower ./cmd/gocpp` passes (or the named golden tests if
  `cmd/gocpp` is too slow), gofmt/vet clean, focused green including TestGolden.
- **Scope:** `internal/lower/whynotes.go` (+ test, + affected goldens under `cmd/gocpp/testdata/`).
  Annotation text only — not an emission change.
- **TODO.md claim to fix (quoted, line 144):**
  > - [ ] The headline note of `emit -annotate` says "captures by reference" for a closure whose captured variable is a boxed `[n]` capture (internal/lower/whynotes.go:115) (was L32).

**Manager pre-check:** read `noteClosure` on `dev` @ `1a1224be`; the headline/per-variable
contradiction is real. Two goldens exist and may be affected:
`cmd/gocpp/testdata/annotate/closures.golden`, `cmd/gocpp/testdata/explain/closures.golden`.

### Versions

| Version | Coder | Worktree / branch | Manager test run | Commit (state hash) |
|---|---|---|---|---|
| v1 | coder 1 | ~/worktrees/GoToCPP/dev.whynotes · `job/dev.whynotes` | manager re-ran: `TestClosureHeadline` ok · `TestAnnotate`/`TestExplain` goldens ok · `internal/explain` ok · negative check reproduced (old whynotes.go fails both new tests and both goldens) | `3a0d06bf` |
| v2 | coder 2 | ~/worktrees/GoToCPP/dev.whynotes-c2 · `job/dev.whynotes-c2` | manager re-ran: `TestClosureHeadline` ok · named goldens ok · negative check reproduced (old whynotes.go fails `byBox` and `mixed`) · reports a pre-existing `TestGuideExamples` failure, **confirmed by the manager on clean dev** | `424c78df` |
| v3 | coder 3 | ~/worktrees/GoToCPP/dev.whynotes-c3 · `job/dev.whynotes-c3` | manager re-ran: `TestLocalClosureHeadline` ok · named goldens ok 31.015s · `internal/explain` ok · negative check reproduced (old code fails the new test and both closures goldens) · **also edits `internal/explain/topics.go`**, which v1 and v2 declined | `2576f0e1` |

**PM merge plan (fresh read-only PM, copied unchanged):**

> **Recommended base: v3**, merging coder **3**. Fewest parts move: v3 already carries the neutral
> headline, the `topics.go` prose fix, the smallest example and the matching goldens; only the
> zero-capture branch and one test case are added.
>
> | Item or file | Take | Reason |
> |---|---|---|
> | `internal/lower/whynotes.go` | v3 + v2's zero-capture branch | v3's single-pass neutral headline on any boxed capture cannot go stale; graft v2's `case len(caps) == 0: " that captures nothing"` instead of the false "captures by reference" |
> | `internal/lower/whynotes_test.go` | v3 (`TestLocalClosureHeadline`) + two folds | v3 pins both branches and the unchanged per-variable notes; add v2's zero-capture shape and v2's exact-text assertions |
> | `internal/explain/topics.go` | v3 | The only version that fixed the topic text; without it `gocpp explain closures` contradicts its own example |
> | `internal/explain/examples/closures.gp` | v3 | Shortest idiomatic case, gofmt-clean, cannot trip the `[&` truth rule |
> | both `closures.golden` | v3, regenerated | v3's already match its example; regenerate with `-update-annotate` after the `whynotes.go` edit |
> | TODO line 160 | close in the merge commit | The line's claim is exactly what the JSON/test now pins |
>
> Conflicts: neutral headline over v2's four-way summary (the latter duplicates the per-variable
> text and would need rewriting for a new capture kind — the staleness the ticket warns against);
> v3's "noted where the variable is used" over v1's "noted below" (the note is at the ident's
> position, so v3's text is true in `-annotate` and `-json` alike); v3's `topics.go` edit kept as a
> deliberate scope call (WORKFLOW §5 puts an older issue in the same area in scope, and the job's
> own example turns the topic page into a self-contradiction); v2's zero-capture discovery kept,
> its headline strings dropped.
>
> Risks the merging coder must check: regenerate **both** goldens with the repo's mechanism; add the
> zero-capture case (`add := func(x int) int { return x + 1 }`) and assert "that captures nothing";
> check whether the non-local branch has the same zero-capture wrongness; never put a by-reference
> and a boxed capture in one example literal (the `[&` truth rule); per-variable notes must stay
> byte-identical; keep the `.gp` example gofmt-clean; `TestGuideExamples` fails on `dev`
> independently (TODO 153-156) and must not read as this job's regression.

**Manager check of the plan.** Every load-bearing claim re-verified before accepting:
- The zero-capture bug is real and v3 does **not** cover it: `gocpp emit -annotate` on
  `add := func(x int) int { return x + 1 }` prints "…no allocation) **that captures by reference**"
  while `gocpp emit` writes `auto add = []<int = 1>(int64_t x) …` — an empty capture list. v3's
  neutral branch fires only when `boxed` is true, so this case still hits the false by-reference
  wording. The PM's graft is necessary, not a nicety.
- `TestAnnotateTruth`'s rule `{msg: "~by reference: it stays on the C++ stack", need: []string{"[&"}}`
  is at `cmd/gocpp/annotate_test.go:469`, as quoted.
- v3's `TestLocalClosureHeadline` (`internal/lower/whynotes_test.go:84`) pins the boxed and unboxed
  branches and the unchanged per-variable notes, as described.

**Merge result:** coder 3 (`whynotes-v3` tagged at `2576f0e1`) · **Commit (state hash):** `d940d4b4` ·
**Departures from the plan:** one, and an extension rather than a departure — the PM asked the coder
to *decide* whether the non-local branch has the same zero-capture wrongness. It does, and the coder
fixed it: `noteClosure`'s non-local headline said "what it captures is copied into the lambda (a box
is shared)" for a literal with no captures. `internal/explain/examples/generics.gp:27-28` carry two
such literals, so the fix changed two goldens. Everything else followed the plan: v3 base, v2's
`case len(caps) == 0` grafted, v2's four-way switch dropped, `topics.go` kept.

**Manager verification of the merge (independent of the coder's handoff):**
- `TestLocalClosureHeadline` ok 0.029s; `TestAnnotateGolden|TestExplainTopicsGolden` ok 1.298s.
- **Negative check reproduced by the manager**: disabling only the `case len(caps) == 0` arm makes the
  test FAIL, printing exactly the false sentence —
  `func literal #2 is a local closure: … (no gp::func, no allocation) that captures by reference`
  — for a literal whose capture list is empty.
- The non-local fix is correct, checked against `dev` **before** the patch: `gocpp emit` on a
  package-level `var g = func(x int) int { return x + 1 }` writes
  `gp::func<int64_t(int64_t)>([]<int = 1>(…) …)` — an empty capture list wrapped in `gp::func`, so
  "it captures nothing, and … is wrapped in gp::func" is true and the dropped clause "whose copies
  share the captured state" was vacuous for that case.
- Per-variable notes unchanged: the golden diff has no changed `captures … by …` line.

### Rounds

| Round | Fresh reviewer # | Commit (state hash) | Manager test run | Verdict |
|---|---|---|---|---|
| R1 | – | – | – | – |

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| – | – | – | – | – | – | – | – | – |

## Job dev.rtbugs: nine Runtime claims, one batch

**Status:** `[~]` merging · **Merging coder:** 1 · **Canonical worktree:** ~/worktrees/GoToCPP/dev.rtbugs · `job/dev.rtbugs` (machine: the user's
local box; the five jobs above run on another machine and arrive here by `git pull`)

### Brief

> The ticket is `.scratch/mgr/tickets/dev.rtbugs.md` (main checkout); it was given to the three
> coders unchanged. User's instruction 2026-10-26: one batch job with the same three coders, not split.

- **Goal:** the nine TODO.md Runtime claims below behave like Go (`scripts/gp-repro`), or are shown
  stale with evidence; per claim a test that fails on `dev` @ `70890fb3`.
- **Claims (TODO.md line on `70890fb3`):** 103 `&cvar` dangles where the address is kept (ADR-139);
  104 ADR-139 follow-ups (`cpplocal` informational, `mayHoldPointers` unpinned, dead `escTop`/`top == nil`);
  105 goroutine stacks 8 MiB (ADR-104 option 2b); 106 children inherit the 1 GiB RLIMIT_STACK;
  107 `make([][3]struct{}, MaxInt)` out of memory; 108 `*(*[N]struct{})(s)` N > 64 reads past
  `zerobase`; 109 reflect `MakeSlice`/`Grow` on zero-size and huge slices; 120 `runtime.Callers`
  1 vs 3 and `ReadMemStats` `Mallocs`/`Frees`/pause fields; 126 (first sub-claim only) goroutine
  panic while `main` returns exits 2 (Go 0).
- **Constraints:** no thread before `main.main` (ADR-104 2026-10-23); goroutine-start, 10k/100k
  goroutine and overcommit guardrails measured for claim 3; ASan/TSan/Windows kept; default `-O0`.
- **Done when:** per-claim failing-on-dev tests; claim 3's `count(3000000, 0)` in a goroutine prints
  3000000 in debug and `-release`, g++ and clang++; `scripts/gp-focused -x` and `-cxx clang++` green;
  ADR-104, ADR-139, D7, D24 and guides updated.
- **Not performance-shaped** (bugs; claim 3's costs are guardrails), so no techlead stage.

### Versions

| Version | Coder | Worktree / branch | Manager test run | Commit (state hash) |
|---|---|---|---|---|
| v1 | coder 1 | ~/worktrees/GoToCPP/dev.rtbugs-c1 · `job/dev.rtbugs-c1` | manager re-ran `scripts/gp-focused -x -cpus 16 -mem 16G` (g++) and `-cxx clang++`: **both green**, 43 commands each. v1 also brings `gates.go` back in line with `docs/spec/feature-matrix.md`, so `TestFeatureMatrix` passes | `0742c552b610` |
| v2 | coder 2 | ~/worktrees/GoToCPP/dev.rtbugs-c2 · `job/dev.rtbugs-c2` | manager re-ran both: g++ 56 commands passed, 1 failed (`TestFeatureMatrix`, **also fails on clean dev 451d4f28**, manager-checked); clang++ 56 passed, 2 failed: `TestFeatureMatrix` (dev's) and `TestCXX/clang++/O0/deadlock` — `test_dump_churn` timed out after 269 s (`deadlock_test.cpp:127: the wait timed out`, exit 97, thousands of goroutines parked in `chan receive` at :572). Manager re-ran that test alone 3× on v2 (ok 49.5 / 60.0 / 132.2 s) and 3× on clean dev (ok 75.3 / 103.5 / 90.5 s): **not reproduced**; v1 and v3 passed it in their runs. Open question for the PM/reviewers, in the area v2 changed (stack budget, `stack_release`) | `98cf2f111d9e` |
| v3 | coder 3 | ~/worktrees/GoToCPP/dev.rtbugs-c3 · `job/dev.rtbugs-c3` | manager re-ran both: g++ 34 passed and clang++ 34 passed, each with 1 failure, `TestFeatureMatrix`, **which also fails on clean dev** (manager-checked) | `beac4b6f` |

### PM merge plan

> Copied unchanged from the PM (fresh read-only agent, 2026-10-26). Merging coder: 1 (base v1).

> PM MERGE PLAN — dev.rtbugs
>
> Recommended base: v1   (reason: v1 has the best runtime OS layer: claim 3 stacks, claim 4 RLIMIT_STACK, and the Callers frames. It is the only version green on both compilers. Everything that moves into it is compiler-side or a small runtime piece, and most of that can be taken as whole files. On a v3 base, v3's growing-stack machinery would have to be pulled out by hand from thread.cpp, goroutine.cpp, os.hpp, rt.cpp, runtimepkg and windows/thread.cpp. Those hunks are tangled with v3's claim-8 hunks, which makes that merge riskier.)
> Merging coder: 1
>
> ## What I checked myself and what I took from the handoffs
>
> I checked these myself. The binaries are under /home/huuhait/GoToCPP/.scratch/pm-rtbugs/: dev@70890fb3 was built from git archive, and v1, v2 and v3 were built from their worktrees. The comparison script is /home/huuhait/GoToCPP/.scratch/pm-rtbugs/cmp.sh. The probe programs are under .scratch/pm-rtbugs/p/.
> - **Claim 3.** count(3000000) in a goroutine prints 3000000 on v1, v2 and v3, in debug and -release; dev overflows.
> - **Claim 3 guardrails** (my numbers):
>   - 10k blocked goroutines, VmPTE: dev 52 MB, v1 77 MB, v2 97 MB, v3 81 MB. RSS is the same everywhere (154 MB).
>   - 100k blocked goroutines start on all versions. VmPTE: dev 472 MB, v1 675 MB, v2 733 MB, v3 733 MB.
>   - Short-goroutine start cost is the same on all versions (O0 0.84-0.90 us, release 0.57-0.59 us).
>   - 38,000 sequential goroutines that each recurse 0.2-4 MiB deep take dev 2.3-3.9 s, v1 2.5-2.7 s, v2 2.3-2.4 s and **v3 6.8-7.0 s, with 4.1 s of system time**. v3's tripwire costs about 110 us per goroutine on a reused thread.
>   - v1 under -race starts 10,000 blocked goroutines (1.0 s); under -asan, 5,000.
>   - After the deep goroutine ends, RSS is v1 100 MB, v2 14 MB, v3 8 MB, and Go 121 MB (Go also keeps the stack until a GC runs).
> - **Claim 4.** I read /proc/self/limits and called Getrlimit(RLIMIT_STACK) from a program. Go and v1 show soft 8388608; dev, v2 and v3 show 1073741824.
> - **Claim 9.** I used v2's tests/diff/goroutine_panic_main_returns.gp (GOMAXPROCS(1), main busy 2 ms), 30 runs each:
>
>   | Version | taskset 1 CPU | taskset 4 CPUs |
>   |---|---|---|
>   | Go | exit 0 30/30 | exit 0 29/30 |
>   | dev | exit 2 30/30 | exit 2 30/30 |
>   | v1 | exit 0 30/30 | **exit 2 30/30** |
>   | v2 | exit 0 30/30 | exit 0 30/30 |
>   | v3 | exit 2 30/30 | exit 2 30/30 |
>
>   So the claim is not stale (v3 is wrong), and v1's cpu_count()==1 test misses GOMAXPROCS=1 on a multi-CPU machine. With the bare `go func(){panic()}()` and nothing else, every version and Go exit 0.
> - **Claims 5/6/7, cross-runs.** I ran each version's tests on every version against Go:
>   - **v1 crashes with SIGSEGV** on v3's reflect_makeslice_zerosize.gp. The cause is `reflect.Swapper` / `sort.Slice` on `[][3]struct{}`: ASan shows a memcpy into the read-only zerobase at reflect.cpp:1814. dev, v2 and v3 are fine. This is a v1 regression.
>   - **v2 OOMs** on v1's slices_make_zerosize_empty.gp (`make([]W, MaxInt)`, where W mixes [3]struct{}, [2][0]string, struct{} and Z).
>   - v1 and v3 pass all three versions' zero-size tests, and give identical unsafe.Sizeof values (1 1 1 2 12/8).
>   - Claim 6: v3's tests/interop/zerosize_array_copy.gp segfaults on dev with clang++ and passes on v1, v2 and v3.
> - **Claim 8.**
>   - Callers: v1 and v2 match Go on all three versions' Callers tests. **v3 does not**: its runtime.main/goexit frames have no file/line, and runtime.Caller(n) returns none for them.
>   - MemStats: all three versions pass v2's and v3's tests.
>   - Allocation cost on a binary-trees-like program, 40×depth 16, pinned to one CPU: at -release all are about equal (94-102 ms). **At -O0, the default build, dev is 855-862 ms, v1 938 (+9.5%), v2 961 (+12.5%), v3 857-863 (±0).**
> - **Claim 1.** I ran each version's cvar_addr_calls with -asan on every version:
>   - **v2 has a heap-use-after-free** on the slice-element case (`gelem = &sl[1].n`, `gi = &sl[1].in.a`).
>   - v1 and v3 give identical results on each other's tests, apart from the raw-member policy.
>   - **v1's raw-member build error has a false positive:** `t := cac.Tracked{}; var h cac.Holder; h.p = &t; fmt.Println(h.p.inc())` is a valid program that dev compiles and runs, and v1 rejects it with "cannot keep the object alive". v3 compiles it and keeps nothing (alive 0).
>   - v3's keep_forever on `h.p = &t; gheld = h`, called 1000 times, leaves 1000 objects alive (Go keeps 1); v2 gets stack-use-after-return there.
>   - Extra keeping calls (a keeping helper, an interface method over `any`, a channel send, a generic, a closure): dev has a UAF; v1, v2 and v3 are all correct.
>
> Taken from the handoffs without checking:
> - Windows and Wine runs.
> - The overcommit=2 path (nobody has root).
> - The TSan unit tests.
> - The coders' own failing-on-dev lists, except the ones listed above.
> - v1's statement that its focused runs were green. The manager's runs confirm it.
>
> ## Per item / file
>
> | Item or file | Take | Reason (one line) |
> |---|---|---|
> | Claim 1 (&cvar call-keeps, &sl[1].a, raw member) | v3 | Same results as v1 on every cross-test; its keep_forever closes the raw-member case without v1's false-positive build error; v2 has a UAF on &sl[1].a and leaves the raw member open. |
> | internal/lower/argkeep.go (new), cbox.go, cunsafe.go, cvalues.go, expr.go, file.go, generics.go, tinfo.go, interop_test.go | v3 (whole files) | v1's changes in these files are all claims 1/2, so whole-file takes replace them cleanly; this includes the escTop/top==nil removal. |
> | runtime/include/gocpp/interop.hpp | v3, only the keep_forever hunk | The rest of v3's diff in this file is clang-format churn on unrelated lines. |
> | Claim 2 (cpplocal, mayHoldPointers pin, escTop) | v1 (internal/vet/cinterop.go, cpplocal.txtar) + v3 (lowering, comes with claim 1) | All three are equivalent here. |
> | internal/vet/corpus_test.go, phase10_test.go | v2 | knownFindings keyed "file:line analyzer" with a stale-entry check: the "per-line expectation" the claim asks for. |
> | Claim 3 (goroutine stacks, option 2b) | v1 | 512 MiB is exactly the largest stack a Go goroutine reaches; least extra page-table memory (+25 MB per 10k vs +45 for v2); simplest (glibc stacks + a size/16 fallback ladder in os::thread_start on Linux and Windows); caps to 8 MiB where reservations are charged. |
> | runtime/src/os/linux/thread.cpp, os.cpp, runtime/include/gocpp/os.hpp, runtime/src/os/windows/thread.cpp | v1 + v2's `stack_release` | v1 for the stacks and claim 4; add v2's `os::stack_release` (Linux madvise below the top 8 MiB, Windows no-op) so a reused idle thread does not keep up to 512 MiB that it touched once. |
> | runtime/src/goroutine.cpp, include/gocpp/goroutine.hpp | combine: v1, plus v2's stack_release hook in worker_main (≥1 ms, w->stack > 8 MiB), plus v2's `let_main_return` instead of v1's `yield_to_main` | Claim 9 measured above; drop v2's big_stack_bytes budget and its 1 GiB default. |
> | Claim 4 (children's RLIMIT_STACK) | v1 | Grows the main stack once at start, then restores the original soft limit, so every child (C system/popen/posix_spawn, any future os/exec) and Getrlimit see the shell's value, as in Go (verified). v2 and v3 only add hooks that no built path calls. |
> | runtime/src/runtimepkg.cpp/.hpp, rt.cpp, trace.cpp, frames.cpp | v1 | SetMaxStack, overflow report, GC pause natives, Callers frames (match Go; v3's lack file/line). |
> | Claim 5/6 (empty C++ classes) | v3 | Same sizes and test results as v1, but v1's reflect side crashes in Swapper/sort.Slice; v2 misses mixed empty structs (W). |
> | internal/check/emptyclass.go (new), sizes.go, sizes_test.go, internal/lower/typedecl.go, runtime/include/gocpp/array.hpp, typeops.hpp | v3 (whole files) | They must stay consistent with each other and with v3's reflect gp_empty flag. |
> | internal/check/gates.go, docs/spec/feature-matrix.md | v1 | v1 syncs the matrix with gates.go, which fixes TestFeatureMatrix (red on dev and on v2/v3). |
> | Claim 7 (reflect) | v3 | type_ops::gp_empty gates alloc_array, assign, copy_elems and Swapper in about 30 lines; v1's larger rewrite has the Swapper crash. |
> | runtime/src/reflect.cpp | v3 (whole file) | v1's changes there are all claim 7. |
> | runtime/include/gocpp/slice.hpp | v1's hunk only if clang++ needs it with v3's array | It was for v1's own specialization (-Wdivision-by-zero). |
> | Claim 8 Callers | v1 (frames.cpp, trace.cpp) | Matches Go on all three tests. |
> | Claim 8 ReadMemStats counting | combine: v1's runtime/src/alloc.cpp + testing_alloc.cpp, with v3's -O0 technique | Keep v1's sanitizer branch (malloc hooks, sanitizer operator new left alone), its new-handler loop and the TSan fix; port v3's plain words + `__atomic` builtins + `[[gnu::always_inline]]` bump path to remove v1's +9.5% at -O0. |
> | stdlib/runtime/runtime.gp, stdlib/runtime/PORT.md | v1 | Mallocs/Frees/HeapObjects/pause fields are already done on the base. |
> | Claim 9 | v2 | `let_main_return` keyed on gomaxprocs(0)==1 matches Go on 1 and 4 CPUs; main's return path is unchanged and no thread is added. |
> | tests/diff/goroutine_panic_main_returns.gp, goroutine_panic_main_blocks.gp | v2 | Fail on dev (30/30 exit 2), pass on v2. |
> | tests/run/stack_goroutine_deep.gp, runtime_callers_bottom.gp, slices_make_zerosize_empty.gp | v1 | Fail on dev; pass on the parts kept. |
> | tests/run/release_stack_goroutine_deep.gp | v2 | Pins "-release prints 3000000" (Done when); passes on v1, fails on dev; fix its comment from "1 GiB" to 512 MiB. |
> | tests/run/reflect_makeslice_zerosize.gp | combine: v3's file + v1's cases it lacks (Value.Grow past limit, Set into element) | Same file name; v3's has the Swapper case. |
> | tests/run/slices_make_zerosize_fields.gp, runtime_memstats_counts.gp | v3 | Fail on dev. |
> | tests/interop/zerosize_array_copy.gp/.hpp | v3 | The only real failing-without test for claim 6 (clang++ SIGSEGV on dev). |
> | tests/interop/cvar_addr_calls.gp/.hpp | combine: v3's file + v1's `look` read-only and maps/slices cases, with a merged .hpp | Tracked needs both inc() and peek/Rec. |
> | New regression test (merging coder writes it) | — | `h.p = &t; fmt.Println(h.p.inc())` compiles and leaves 0 alive: guards keep_forever's trigger. |
> | runtime/test/* and runtime/*_test.go | v1, plus v2's test_stack_release from 98cf2f11 (the fixed version) | Drop v1's go-panic-main-returns case (it tests v1's cpu_count behaviour). |
> | runtime/README.md, runtime/files.go, json/filepath tests calling SetMaxStack(8<<20), stack_* test comment edits | v1 | — |
> | tools/portstd/gotests/std/io__fs.py, tests/stdgo/manifest/io__fs.json, tools/portstd/patches/*.rules, json/filepath PORT.md | v1 | Manifest and generator stay consistent. |
> | docs ADR-104 | v1 + one line for stack_release | — |
> | docs ADR-139, interop.md | v3 | They describe the claim-1 policy that is taken (keep_forever; C-function keeps stay cgo's rule). |
> | language.md D7, D24 | v1 | — |
> | language.md D15 | v3 | v1 and v3 give the same Sizeof values. |
> | cpp-mapping.md | combine: v1 runtime/Callers text + v3 empty-class text | — |
> | guides (cpp-developers, stdlib, stdlib-tests, release-candidate, windows-testing, vet) and ADR README | v1 | Adjust vet.md if v2's per-line keys change its wording. |
>
> ## Conflicts and resolution
>
> - **Goroutine stack design.** v1 (512 MiB glibc reservation + fallback ladder) wins over v2 (1 GiB + address-space budget) and v3 (own mmap that grows via a MADV_GUARD tripwire or a SIGSEGV + mprotect, with joinable threads reaped). v1 has fewer page tables than v2, and is far simpler and ~2.6× faster than v3 on repeated mid-depth goroutines. Take only v2's stack_release on top.
> - **Default size, 512 MiB vs 1 GiB.** 512 MiB (v1). It is the largest power of two under Go's 1e9 limit and costs half the page-table overhead of 1 GiB.
> - **SetMaxStack semantics.** v1 (applies to goroutines that start after the call). v3's "applies to running goroutines too" exists only because of its growing stacks, which are dropped.
> - **Claim 4 approach.** v1 (grow, then restore the soft limit) wins over v2 (new syscall.Exec + exec_stack_limit hook) and v3 (exec_limits hook used only by a test shell). v1 alone covers every child path and Getrlimit.
> - **Raw C member that leaves.** v3's keep_forever wins over v1's build error and v2's "left open". v1's predicate rejects valid code that dev ran correctly; v2 still dangles. Document the per-call leak (Go keeps one) in ADR-139/interop.md.
> - **Call-keeps analysis.** v3 (argkeep.go) is behaviourally equal to v1 on every test, and keep_forever needs v3's escapedAddr-based trigger. v2's fixpoint is dropped because of the &sl[1].a UAF.
> - **Claim 9, fix vs stale.** Fixed, with v2's gomaxprocs-based let_main_return. v3's "stale" is refuted by my 30-run table; v1's cpu_count check fails under GOMAXPROCS(1) on more than one CPU.
> - **Zero-size representation.** v3 (emptyclass.go + gp_empty) wins over v1 (ZeroEmpty + reflect rewrite with the Swapper crash) and v2 (coarse identity keys that OOM on W).
> - **alloc.cpp.** v1's structure plus v3's -O0 fast path. v3's file as-is defines operator new under ASan/TSan as well (ASan's own operator new and its new/delete-mismatch checks are lost) and throws bad_alloc without calling the new handler. v2's is +12.5% at -O0.
> - **Callers frames.** v1. v2 is equivalent; v3 is incomplete.
> - **knownFindings.** v2's per-line keys with the stale check, rather than v1/v3's plain removal of the three whole-file entries.
> - **Same-name test files** (cvar_addr_calls, reflect_makeslice_zerosize). Combine the cases into one file each, as in the table.
>
> ## Dropped and why
>
> - **v3 runtime growing stacks** (thread.cpp "growing stacks", stack_trim, set_stack_limit, the thread_start max_stack parameter, use_protect_stacks, the related goroutine.cpp/rt.cpp/os.hpp/windows changes, tests/overflow/deep_goroutine*.gp, the overflow_goroutine.gp edit). Measured 2.6-3× slower with 4 s of system time on repeated 0.2-4 MiB goroutines, plus large complexity. A kernel write into the unprobed tripwire page or a not-yet-committed page would return EFAULT, not grow the stack; my 38k-depth os.Stat sweep did not reproduce this, but the hazard exists by design.
> - **v2's 1 GiB default and big_stack_bytes/stack_budget.** More page tables; v1's ladder already handles an exhausted address space.
> - **v2's syscall.Exec** (posix_file.cpp sys_exec, syscall.hpp, syscall_linux.gp, tests/diff/syscall_exec_stack_limit*.gp, exec_stack_limit/exec_failed_stack_limit). A new API is not needed once v1 restores the limit for every child.
> - **v3's exec_limits/forget_start_limits.** No built caller.
> - **v1's yield_to_main** and the goroutine_test case go-panic-main-returns. Replaced by v2's let_main_return.
> - **v3's claim 9 "stale" report.** Refuted.
> - **v1's raw-member error** (the cunsafe.go noteStores leavingAddr/heldBeyond branch) and tests/interop/cvar_addr_error_raw_member.gp. False positive on valid code.
> - **v1's claim-1 analysis** in cbox.go (callArgs/keptParams/calleeFunc/heldBeyond). Superseded by v3's equivalent analysis.
> - **v1's internal/check ZeroEmpty, typedecl.go, array.hpp and reflect.cpp changes.** Superseded by v3's; Swapper crash.
> - **v2's alloc.cpp/alloc.hpp/alloc_new.cpp, its ifaceconv_test changes, and v2's frames/trace changes.** The v1 equivalents are kept.
> - **v3's alloc.cpp as a file** (only its technique is ported) and **v3's frames.cpp** (incomplete Callers).
> - **v3's interop.hpp clang-format churn** on value_eq, key_ok and map_key_check.
> - **v2's ckeep.go** and its cbox.go/expr.go changes. UAF on &sl[1].a.
>
> ## Risks the merging coder must check
>
> - **Lowering whole-file takes from v3 into the v1 base.** Re-run, with both compilers: TestExternLowering (v1 needed a fix there, 467d174c), TestCUnsafe and TestGolden/interop_unsafe (v2 tripped these with a keeps rule), TestTypedRun/TestTypedTwin (v3's typed-mode handling), TestCAddrOwnership and TestInterop cvar_* / unsafe_*. Check that funcGen fields such as `keptForever` come with v3's expr.go/file.go.
> - **Claims 5-7 from v3 on the v1 base.** Run v1's slices_make_zerosize_empty.gp and v1's reflect cases on the merge; they passed on v3 in my runs. Also check that v1's earlier fix ("reflect Set into a zero-size element wrote to zerobase") is covered by v3's assign no_data gate. Drop v1's slice.hpp hunk unless clang++ needs it.
> - **alloc.cpp -O0 port.** Re-measure /home/huuhait/GoToCPP/.scratch/pm-rtbugs/p/alloc/main.gp (`taskset -c 9`, -O0 and -release) against dev's 855-862 ms; the goal is within about 2%. The +9.5% is most likely v1's std::atomic member calls in operator new at -O0; I could not profile (no perf), so confirm the cause. Keep the sanitizer hooks, the new-handler loop and the TSan fix (0742c552).
> - **v1's main-stack growth (claim 4).**
>   - Under overcommit 0 it adds 1 GiB to Committed_AS per process.
>   - mlockall(MCL_CURRENT) would populate the grown 1 GiB; nobody measured this.
>   - Under overcommit=2, a finite RLIMIT_AS or valgrind, growth is refused, the raised limit stays and children inherit 1 GiB. D7 must say so.
>   - overcommit=2 is untested by every coder.
> - **keep_forever.** It leaks one object per execution of `h.p = &x; g = h` (1000 calls → 1000 alive). Document this. Check that LeakSanitizer does not report the kept list in sanitizer runs of tests/interop.
> - **stack_release.**
>   - Use v2's version from 98cf2f11 (the clang -O2 / TSan flake fixed by touching every page).
>   - It must never run on the main thread.
>   - The ≥1 ms clock reads happen on every goroutine end; re-check the short-goroutine cost against v1 using .scratch/pm-rtbugs/p/spawn.
> - **let_main_return.** It reads gomaxprocs(0) on the panic path only. Re-run the deadlock TestCXX under clang++; the manager saw a test_dump_churn timeout on v2 that it could not reproduce.
> - **Release test comment.** Fix release_stack_goroutine_deep.gp's comment (1 GiB → 512 MiB); deep goroutine tests in tests/run fail under -race, as dev's existing stack_main_deep does. Follow whatever handles stack_main_deep in the sanitizer sweeps.
> - **Per-line knownFindings keys** (v2) must match the merged corpus lines; the stale-key check makes this fail loudly if they don't.
> - **Windows.** Cross-build and run the goroutine/thread/winos/array unit tests under Wine (v1's ladder with 512 MiB, v2's stack_release no-op).
> - **gp-focused.** Run `scripts/gp-focused -x` and `-cxx clang++`, adding -run targets for TestFeatureMatrix, TestOpaqueCorpus/TestMustCorpus, TestDiff (the two new files), TestStackOverflow and TestWindowsRuntime.
>
> ## Versions I could not use
>
> - None was unusable as a whole. v2 is red only on TestFeatureMatrix (also red on dev) plus a clang++ deadlock timeout the manager could not reproduce; parts of v2 are taken. v3's runtime stack layer and its claim-9 "stale" verdict are not used, for the reasons above.

### Merge

- Coder 1 committed the merge `6b29cce0` (18:17) and was running focused tests when the user stopped
  it to change the CPU cap (user, 2026-10-26: "make it cap 32 threads plz"; every run is now
  `-cpus 32 -mem 32G`, extra checks share the 32). Its g++ run reached command 44 of ~51 with no
  failure. A stopped agent cannot be resumed, so on the user's "go" a **fresh merging coder** took over
  at `6b29cce0` with the PM plan, the three handoffs and the plan's risk list; it does not redo the merge.
- Manager error: while stopping its own focused run to stay within the cap, `pkill -f` matched the
  manager's own shell; an orphaned `go test ./internal/check` was then killed by literal pid. No file
  was touched.

### Rounds

| Round | Fresh reviewer # | Commit (state hash) | Manager test run | Verdict |
|---|---|---|---|---|
| R1 | – | – | – | – |

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| – | – | – | – | – | – | – | – | – |
