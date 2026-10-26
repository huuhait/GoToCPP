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
| v3 | coder 3 | ~/worktrees/GoToCPP/dev.stalehints-c3 · `job/dev.stalehints-c3` | manager re-ran: `go build` + `go vet` clean · comments only (same one trailing-comment code-line change as v1) · **v3 reframes the `testing.hpp` list instead of editing it**: it names the packages whose functions the natives *replace* ("sync, time, os, runtime, strconv, io, …") rather than the packages `testing` *imports*. That is the accurate framing: `gocpp`'s `stdlib/testing` imports `errors, flag, fmt, os, runtime/debug, time` (not sync or strings), and `stdlib/testing/PORT.md:11-22` lists natives replacing `sync`, `time`, `os`, `runtime`, `strconv`, `regexp` — so v3's sentence is true where a literal edit of the old list can always drift again | `d3423e13` |

**Where the three versions disagree (for the PM):** all three agree the four sites are stale and fix
them, but on `runtime/include/gocpp/testing.hpp` they take three different routes to a true sentence.
v1 keeps the original "the packages that Go's testing imports" framing and edits the list in place to
"sync, time, os, runtime, flag, strings, … are real packages now". v2 drops `flag` from that list and
keeps the rest. v3 abandons the imports framing and names the **replaced** packages instead. The
manager's reading: v1's list is not the list of imports either — `stdlib/testing/*.gp` imports only
`errors, flag, fmt, os, runtime/debug, time`, so "sync" and "strings" are in v1's list because they
are *replaced*, not imported, which is the category error v3 removes. v2's removal of `flag` is
right about `flag` but leaves the same wrong category. v3 is the only one whose claim is
checkable against `PORT.md` without re-deriving it. This is a PM decision, not the manager's.

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

**Status:** `[~]` in_progress · **Merging coder:** – · **Canonical worktree:** ~/worktrees/GoToCPP/dev.staleflag

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

**Status:** `[~]` in_progress · **Merging coder:** 3 · **Canonical worktree:** ~/worktrees/GoToCPP/dev.whynotes

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

### Rounds

| Round | Fresh reviewer # | Commit (state hash) | Manager test run | Verdict |
|---|---|---|---|---|
| R1 | – | – | – | – |

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| – | – | – | – | – | – | – | – | – |
