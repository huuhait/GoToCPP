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
| `[~]` | `dev.probecached` — `TestDeclsDeterministic` fails when the probe cache is on (TODO 137) | – | 2 / 3 | 0 | – |
| `[~]` | `dev.stalehints` — four stale comments in the compiler (TODO 145) | – | 1 / 3 | 0 | – |
| `[x]` | `dev.staleflag` — two LSP completion nits (TODO 184) | – | 3 / 3 | 1 (R3) | `57f3fa45` |
| `[~]` | `dev.whynotes` — `emit -annotate` headline "captures by reference" for a boxed capture (TODO 144) | – | 1 / 3 | 0 | – |

**Worker-limit note (manager, 2026-10-26).** WORKFLOW.md §1 sets the limit at five active workers,
and a job's three build-phase coders count as three. The four jobs above were started together at
the user's explicit instruction ("start all at once"), so while their coder rounds run, active
workers are 13 (12 coders + `dev.evalorder`'s c2) — over the limit, knowingly, for this round only.
To keep the machine usable every coder was given `-cpus 6 -mem 6G` instead of the `-cpus 24 -mem 16G`
default (§`scripts/gp-focused:12-14` — unbounded parallel builds push the load past 1000 and time
out other jobs' tests). Their PM and review rounds are sequenced, not parallel, so the count drops
back under the limit before any of them merge.

**Worktree cleanup (manager, 2026-10-26, after the merges).** The three build-phase coders' worktrees
are no longer needed once each job's merge commit exists and its version hashes are in this file, so
they were removed to reclaim the disk and keep `git worktree list` readable. Before removing anything
the manager checked each tree held no work that only it had (`git status --short` showed only
`?? .scratch/` in all five) and **tagged every version commit** so none can be lost:
`probecached-v2`=`acb587d0`, `probecached-v3`=`62de3d7b`, `stalehints-v1`=`c676f80c`,
`stalehints-v2`=`4ff62002`, `stalehints-v3`=`d3423e13` (plus the existing `whynotes-v1/v2/v3/merged`
and `evalorder-v1/v2/v3`). Removed: `dev.probecached-c2`, `dev.probecached-c3`, `dev.stalehints-c2`,
`dev.stalehints-c3`, `dev.whynotes-c2`; branches `job/dev.{probecached,stalehints,whynotes}-c{2,3}`
and the dead `job/dev.evalorder-c{1,3}`. What remains is exactly the four canonical job worktrees,
one branch each, which are what the in-flight review rounds are reviewing.

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

**Status:** `[~]` in_progress · **Merging coder:** 2 · **Canonical worktree:** ~/worktrees/GoToCPP/dev.evalorder

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

**PM:** fresh read-only PM · **Merging coder:** 2

> **Base: v2** (`c764cdfb`), merging coder **2**. Fewest parts move: v2's `internal/lower/maps.go`
> can be taken whole. v1 needs its ~150-line key-conversion layer *removed* before it is even
> correct, and v3 needs v2's conversion/field work added on top.
>
> | Item or file | Take | Reason |
> |---|---|---|
> | `maps.go` `sameSafe` + `mapAppend` doc | **v2** | All of gc's post-typecheck `SameSafeExpr`; the only version with the promoted-field chain and the wider OCONVNOP set |
> | `maps.go` helpers (`stripConvNop`, `convNopExpr`, `rounds`, `isUnnamed`, `isNillable`, `fieldPath`, `derefType`) | **v2** | Every one is reached by a row of its 11-row negative check; none is speculative |
> | `maps.go` `CallExpr` case (OCONV → `types.IsSimple`) | **v2** | Equivalent to v1's/v3's larger helpers, and smaller |
> | v1's `sameKey`/`keyInner`/`numericConv`/`assignNop`/`namedType`/`nopStrip`/`noCodeConv` | **drop** | Rejecting the fast path on `AssignConv` keys **regresses** `map[any]` field/index keys (measured below) and belongs in `order.go`, which the ticket excludes |
> | `maps_test.go` (`TestMapAppendShape`) | **v3** | The only *unit* pin on the fast-path decision; verified to fail on `dev` and pass on v2's implementation, so it tests behaviour not one implementation |
> | v1's `maps_test.go` (`TestNoCodeConv`/`TestIsSimple`/`TestNamedType`) | **drop** | They call functions that do not exist in v2 (`go vet`: `undefined: noCodeConv`) |
> | `tests/diff` file | **v2's `maps_append_key_expr.gp`**, plus rows from v1/v3 | Largest row set (11), but see the gap below |
> | Rows to **add** | combine | v2's file has **no `string(...)` conversion at all**, so r2/r3 — two of the three bugs the ticket names — are uncovered. Add `m[string(s)] = append(m[string(s)], f())`, `m[string(s)] = append(m[s], f())`, the float-split row, `int(MyInt)`, `S(s)`, `1+si`, `(*int)(p)` |
> | v1's / v3's separate diff programs | **drop** | Their rows fold into the single merged file; near-duplicate programs cost a C++ run each for no new coverage |
>
> **v1's `0e3185cb` must not be used** (wrong on the float-split shape, already recorded above).

**Manager verification of the PM's decisive claim — v1 regresses.** The PM's headline finding is that
v1 changes behaviour that is already correct on `dev`, so v1 cannot be the base. The manager re-ran
the PM's own probe (`probe22.gp`: a pre-seeded `map[any]` with a field key, an index key and a
deref/name key) through `scripts/gp-repro` on all four trees:

| Tree | field key | index key | deref/name key | vs Go |
|---|---|---|---|---|
| `dev` `19646648` | `[1 7] [99]` ✓ | `[1 7] [99]` ✓ | `[1] [99 7]` ✗ | 1 row differs (divergence (a)) |
| **v1 `e1717e08`** | **`[1] [99 7]` ✗** | **`[1] [99 7]` ✗** | `[1] [99 7]` ✗ | **3 rows differ — two new** |
| v2 `c764cdfb` | `[1 7] [99]` ✓ | `[1 7] [99]` ✓ | `[1] [99 7]` ✗ | 1 row differs — same as `dev` |
| v3 `87d12c91` | `[1 7] [99]` ✓ | `[1 7] [99]` ✓ | `[1] [99 7]` ✗ | 1 row differs — same as `dev` |

**Confirmed, independently of the PM.** v1 is the only version that breaks rows `dev` gets right, so
it is out on correctness, not on taste. v2 and v3 are both byte-equivalent to `dev` on this probe.

**This is the same defect the manager already recorded** (scope decision above): coder 3 built a
`keyNop` gate, measured that the emitted C++ then built the interface twice and flipped `m[*p]` /
`m[arr[i]]` on `map[any]` from correct to wrong, and reverted it. v1 shipped the same idea. The PM
reached the same conclusion from a different direction, which is good corroboration — and it means the
"drop v1's layer" decision rests on a measurement the job already made once.

**Manager verification of the test-coverage gap.** `grep 'string(' ` and `grep 'ks['` over v2's
`tests/diff/maps_append_key_expr.gp` (135 lines) return **nothing**: no string-conversion row exists,
so r2 and r3 would be unpinned if v2's file were taken as is. The PM's added rows are necessary, not
padding.

**§7.3 process note — done right this time.** Unlike staleflag and whynotes, the rename was performed
*before* dispatching the merging coder: `~/worktrees/GoToCPP/dev.evalorder` is on `job/dev.evalorder`,
one worktree for the job, no `-c2`/`-c3` paths left behind. Recorded because the same omission cost
two jobs a fix-up round.

### Merge result

**Merging coder 2 · `job/dev.evalorder` · three commits on base `c764cdfb`:** `60e56f38` (the merge),
`f817313d` (a comment naming a file that does not exist), `023cea66` (the named-int row).

| Item | Outcome | Manager check |
|---|---|---|
| `internal/lower/maps.go` | **v2 taken whole**, unchanged | `git diff --quiet c764cdfb HEAD -- internal/lower/maps.go` → identical. v1's `keyNop`/`sameKey` layer is not in it. |
| `internal/lower/maps_test.go` | v3's `TestMapAppendShape` + `lowerSrc` verbatim | Passes here (0.012s); `TestMapAppendShape` FAILs on four cases when `maps.go` is `dev`'s (`string add`, `string add right`, `conversion both sides`, `conversion one side`) — the two directions of the same rule |
| `tests/diff/maps_append_key_expr.gp` | v2's 11 rows + 12 added = **23 rows** | 16 of 23 differ from real Go when `maps.go` is reverted to `dev`; the 7 that still pass are exactly the rows the ticket says were already right |
| v1's `maps_append_key_op.gp`, v3's `maps_append_key_add.gp` | dropped as files, rows folded in | The plan's call; the merged file carries `constant + int` from v3 and the one-side `string(s)` and float-split rows from v1 |

**The plan's named-int row was vacuous and the coder caught it — manager confirmed.** The plan said
"`int(MyInt)`". Written as `mi[int(myi)] = append(mi[int(myi)], change())` both sides are one source
node, so gc strips both conversions, sees one ONAME twice and takes the fast path **on dev too**: the
row passed pre-fix and pinned nothing. The committed shape is
`mm[MyInt(myi)] = append(mm[myi], change())` — a conversion compared against a plain name. The
manager reproduced the negative check by reverting only `internal/lower/maps.go` to `dev`; both named
rows print `0 1` against Go's `1 0`. `maps.go` restored, `git diff --quiet c764cdfb HEAD` → identical
again, `TestMapAppendShape` green.

**Rows the plan asked for, and what covers them** — no gap left, and nothing added speculatively:

| Plan row | In the merged file as | Fails on `dev`? |
|---|---|---|
| `m[string(s)] = append(m[string(s)], f())` | `string conversion on both sides` | yes |
| `m[string(s)] = append(m[s], f())` | `string conversion on the left` | yes |
| float-split, both keys pre-seeded | `float conversion split`: `hold := map[float64][]int{1: {1}, 5: {99}}` | yes |
| `int(MyInt)` | `named int conversion`, reshaped (above) | yes |
| `S(s)` | `named string conversion` | yes |
| `1+si` | `constant + int` | yes |
| `(*int)(p)` | `pointer to the same pointer type`, `pp` reassigned in the argument | yes |

**Gofmt note.** The merging coder ran `gofmt -w` on the existing var block (`x, y int` / `kp = &x`).
That realignment is unrelated to the ticket but the file was already un-gofmt'd on `c764cdfb`, so the
harness rule (gofmt clean) requires it; recorded so a reviewer does not read it as scope creep.

**Out-of-scope divergences (unchanged by this merge, both recorded earlier):** (a) `map[any]` with a
non-interface key prints `map[5:[7]]` where Go prints `map[1:[7]]`; (b) the local-stack slow-key copy
order. r5 (`internal/lower/order.go`) is excluded by the ticket and untouched.

### Rounds

| Round | Fresh reviewer # | Commit (state hash) | Manager test run | Verdict |
|---|---|---|---|---|
| R1 | – | `023cea66` | – | – |

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

**Status:** `[~]` in_progress · **Merging coder:** 2 · **Canonical worktree:** ~/worktrees/GoToCPP/dev.probecached

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

### PM merge plan

**PM:** fresh read-only PM · **Merging coder:** 2

> Base: **v2**, merging coder **2**. v1 and v3 make the same one-line change (`tab.Cached = false`
> at line 21) and leave the test exercised only by a human who sets `GOCPP_PROBE_CACHE`; v2
> restructures the test into `no cache` / `cache` subtests that set the folder themselves and
> **assert** which of the two runs is cached. The zeroing alone is not a regression test in the
> default environment.
>
> Merge target is the canonical worktree `~/worktrees/GoToCPP/dev.probecached` on
> `job/dev.probecached`, which currently holds **v1**. Adopt v2's `decls_more_test.go` wholesale:
> no line of v1's version may survive beside it (v1's single combined comment and its
> un-parameterised body are both replaced).
>
> | Item | Take | Reason |
> |---|---|---|
> | test structure | **v2** | Only version whose fix is a regression test in a default `go test ./...`; v1/v3's assertion is vacuous cold-vs-cold |
> | `tab.Cached = false` | v2 (all three have it) | The TODO claim itself |
> | `t.Setenv(EnvNoProbeCache, "")` | **v2** | Load-bearing: `tsupport.Main` (`tsupport.go:33-35`) sets `GOCPP_NOPROBECACHE=1` whenever `GOCPP_PROBE_CACHE` is empty, so without clearing it the `cache` subtest runs with the cache off and passes vacuously |
> | `if want := tc.cached && i == 1; tab.Cached != want` | v2 | Fails without the fix; names which run must be cached |
> | v3's comment | **dropped** | Its justification is at best ambiguous: it says "the folder is part of the key", and `run.go:53-55` states the **cache** folder (`cfg.CacheDir`) is not part of any key — only `cfg.Dir`, the package folder, is (`run.go:370-371`) |
> | v1's comment | dropped | Folded into v2's per-subtest comments |
>
> Dropped: v1's and v3's combined rationale comments (v2's two short comments make the same
> points where they apply), and v3's `TestDeclsMemo` cross-reference (already true in the file).

**Manager verification of the PM's decisive claims.** All four were re-checked on `dev`:

- `tsupport.go:33-35` does set `GOCPP_NOPROBECACHE=1` when `GOCPP_PROBE_CACHE` is empty — so
  without v2's `t.Setenv(EnvNoProbeCache, "")` the `cache` subtest is vacuous. **Confirmed.**
- No CI or build path sets either variable: `grep -rn 'GOCPP_PROBE_CACHE\|GOCPP_NOPROBECACHE'`
  over `Makefile`, `.github/`, `scripts/`, `.envrc`, `flake.nix`, `devenv.nix` returns **nothing**.
  So under `make ci` the cache is off and v1/v3's fix is never exercised. **Confirmed** — this is
  what decides the base.
- `EnvProbeCache` / `EnvNoProbeCache` are the names v2 uses (`cache.go:56,59`). **Confirmed.**
- `t.Setenv` cannot race with the package's one `t.Parallel()` (`cache_includers_test.go:126`): it
  is a subtest of a different, non-parallel top-level test. **Confirmed.**

**Correction to the PM's wording.** The PM called v3's comment "factually wrong". It is not flatly
wrong — read as `cfg.Dir` (the per-test package folder, `run.go:370-371`) its parenthetical is true.
It is *misleading*, because the sentence's subject is the disk cache, whose folder `run.go:53-55`
explicitly excludes from the key. That is still reason enough to prefer v2's comments, which
describe the mechanism without the parenthetical, but the merge note should not accuse v3 of a
falsehood it did not state.

**Base-hygiene warning for the merging coder.** v2's branch is built on `1a1224be`, which is **not**
an ancestor of `dev` (`70890fb3`). Its `git diff dev --stat` therefore shows 390 deleted lines in
`plans/2026-10-13-gocpp/REVIEW-dev.md` and 44 in `TODO.md` — those are the *manager's* later
commits appearing as reversals, not coder edits (v2's one commit, `acb587d0`, touches only
`decls_more_test.go`). **Take the file, never `git merge` the branch**, or the job record and TODO
are reverted.

**Also noted by the PM, and true:** the PM's own full-package run of v2 hit `go test`'s 10-minute
default timeout at 606s while the machine was at load ~246. That is the load, not a red version —
the manager's own v2 runs above finished in 2.8s.

### Merge result

**Commit `2154f0e2`** on `job/dev.probecached`, in the canonical worktree (parent `ab5a563e`, v1).
One file, 38 insertions / 20 deletions: `internal/typedinterop/probe/decls_more_test.go`, byte-for-byte
identical to v2's file in the c2 worktree (`diff` empty). No line of v1's comment or body survives.
The branch changes **only** that file since its base (`git diff $(git merge-base HEAD dev) HEAD
--name-only`); the `plans/` entries visible in `git diff dev` remain the stale-base artifact.

**Manager re-ran and extended the negative check.** The coder's check (restore `dev`'s body, run with
a cache folder, observe the two JSON lines differ) is right, but the manager wanted to know *which
half* is load-bearing, since the merged test has both an assertion and a zeroing. Three runs:

| Change from the merged file | Result |
|---|---|
| delete the whole `if want := tc.cached && i == 1; …` block | **passes** (2.6s) |
| keep everything, drop only `tab.Cached` from the zeroing line | **fails in `TestDeclsDeterministic/cache`**, `two runs differ` |
| restore merged | passes (2.6s) |

So the **zeroing** is what fixes the defect; the assertion does not by itself catch it. The
assertion's value is different and still real: it is what makes the failure land in the `cache`
subtest rather than in a cold-vs-cold comparison — and the second row above shows the `cache`
subtest genuinely turns the cache on (it is the subtest that fails). With v1's shape — no subtests,
no folder — the same revert cannot fail at all under `make ci`, which is exactly the PM's argument
for v2 and is now measured rather than argued.

Also reproduced: exactly **one** `"cached":true` in the subtest's output, so it is the second run
only — the TODO 137 defect precisely.

### Rounds

| Round | Fresh reviewer # | Commit (state hash) | Manager test run | Verdict |
|---|---|---|---|---|
| R1 | fresh reviewer | `2154f0e2` | manager: merged file with cache `ok 2.585s`, no env `ok 2.547s`; three-way revert matrix above (assertion-only removal passes; zeroing removal fails in the `cache` subtest); gofmt clean | **FINDINGS (2 MINOR, 1 NIT)** |
| R2 | fresh reviewer | `ac9c8975` (fix) | manager on the fix: `NIX_CFLAGS_COMPILE="-O2 -march=native"` → `ok 3.901s`, normal → `ok 3.006s`; load-bearing re-proved by substituting `"off"` (`run 2: cached false, want true`) | **FINDINGS (2 MINOR, 1 NIT)** — P1 `FIX-VERIFIED` independently |

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| P1 | R1 | MINOR | `internal/typedinterop/probe/decls_more_test.go:20,37` | `NIX_CFLAGS_COMPILE="-O2 -march=native" go test -run '^TestDeclsDeterministic$' ./internal/typedinterop/probe` → `run 2: cached false, want true`. `diskFor` (`units.go:500`) refuses to write the folder when `build.NativeArch` matches `cfg.Flags`, `cfg.CXX.Args` **or `build.KeyEnv()`** — the last is the environment, which the test cannot control. The new assertion is therefore environment-dependent. | WORKFLOW §8 "every new test fails without the change", read with "a test that cannot hold in a supported environment is a test defect" | TBD | **CONFIRMED — manager reproduced.** Same command fails on `2154f0e2`, passes with the variable unset, and the pre-fix test passes in both. Default here has `NIX_CFLAGS_COMPILE` **empty**, so `make ci` is not affected; the defect is real but latent. FIX: make the expectation follow `diskFor`'s rule instead of assuming a writable folder, and keep the `cache` subtest failing if the folder silently stops being used in a normal environment (the reviewer verified that assertion is load-bearing both ways). | FIX (`ac9c8975`) | **FIX-VERIFIED by the manager, independently.** The fix hoists `hostCPU := build.NativeArch(cfg.Flags...) \|\| build.NativeArch(cfg.CXX.Args...) \|\| build.NativeArch(build.KeyEnv()...)` above the table and makes the expectation `tc.cached && !hostCPU && i == 1`. Manager ran both environments on `ac9c8975`: with `NIX_CFLAGS_COMPILE="-O2 -march=native"` → `ok 3.901s`; without → `ok 3.006s`. Load-bearing re-proved by the manager, not taken on trust: substituting `{"cache", "off", true}` gives `decls_more_test.go:47: run 2: cached false, want true`; restored, `git status` clean. Per the rule below, `CLOSED-FIXED` needs a **fresh reviewer's** `FIX-VERIFIED`, so this stayed `FIXED-PENDING` until R2 reported. R2 then `FIX-VERIFIED` it independently: `ac9c8975` passes in default (`ok 2.526s`), native (`ok 3.430s`) and the ticket repro (`ok 2.557s`); instrumented, default gives `hostCPU=false, run2 cached=true` and native `hostCPU=true, run2 cached=false`, matching `diskFor`; R2 re-proved load-bearing with the folder at `"off"`. | `CLOSED-FIXED` |
| P2 | R1 | MINOR | `internal/typedinterop/probe/cache_clang_test.go:72` | `NIX_CFLAGS_COMPILE="-O2 -march=native" go test -run '^TestRealClangCacheColdWarm$'` → `warm: 1 runs, {Units:6 Hits:0 Ran:6}`. Same root cause, older test. | WORKFLOW §8 / reviewer scope "an older issue in the same area must be reported" | — | **CONFIRMED pre-existing — manager reproduced on `dev` (`9b3511f1`), so this job neither caused nor can close it.** Out of scope; filed as a TODO line. | OUT-OF-SCOPE |
| P3 | R1 | NIT | `.scratch/probecached/{dev.go.bak,merged.go.bak}` | Two hand-placed copies of the test file in the worktree, untracked. | WORKFLOW §5.2 hygiene | — | **FIXED by the manager** — both removed; `git status --short` is `?? .scratch/` only. Nothing to hand to a coder. | CLOSED |
| F1 | R2 | MINOR | `internal/typedinterop/probe/decls_more_test.go:21-23` | The `ac9c8975` comment and commit message say "the environment can ask for that (build.KeyEnv) **and a test can't take it back**". That is false, and the fix pays for it with real coverage: in a native environment `hostCPU` is true, so the `cache` subtest asserts nothing and is byte-identical to `no_cache`. A test can scrub it — `t.Setenv(name, "")` for each `build.KeyEnv()` entry with `build.NativeArch`, before the runs — and then asserts `tc.cached && i == 1` unconditionally. | Ticket "Done when … without weakening what it proves"; WORKFLOW §8 "every new test fails without the change"; §5.1 MINOR | TBD | **CONFIRMED — manager proved it with its own test, not by reading the reviewer's.** `zz_f1_test.go` (overlay, outside the worktree) under `NIX_CFLAGS_COMPILE="-O2 -march=native"`: it scrubs every `NativeArch` `KeyEnv` entry, then `build.NativeArch(build.KeyEnv()...)` is **false** and the two runs give `run 1: cached=false`, `run 2: cached=true` — the assertion stays live and the test passes (`ok 0.933s`). So the premise in the comment is wrong and the unconditional form is reachable in exactly the environment P1 was filed about. FIX: scrub the native `KeyEnv` entries in the subtest, drop `hostCPU` from the expectation. | FIX |
| F2 | R2 | MINOR | `internal/typedinterop/probe/decls_more_test.go:22,24` | The copied predicate restates two of `diskFor`'s three refusal reasons and omits `units.go:497`'s `selfID() == ""`, while the comment claims "the rows take their expectation from the same rule". With `dir` set and `selfID()==""`, `diskFor` returns `("", false)`, both runs are cold, and the test fails with the misleading `run 2: cached false, want true`. | WORKFLOW §5.1 MINOR / §8 comment accuracy | TBD | **CONFIRMED — manager reproduced with an overlay stubbing `selfID`.** `zz_f2_test.go` sets `selfID = func() string { return "" }`, `hostCPU=false`, and both runs report `cached=false` while the test's own rule wants `true` for run 2 — it would fail on a third reason it never consults. Note the reviewer's own drift experiment cuts the other way and is worth keeping: deleting `diskFor`'s `NativeArch` line makes the current test fail loudly, so the copy is not a silent second source of truth in that direction; calling `diskFor` directly instead was tested and is worse (tautological). FIX: fixed by F1's shape — with the scrub and an unconditional expectation there is no copied predicate left to be unfaithful. | FIX |
| F3 | R2 | NIT | `.scratch/review/` in the canonical worktree | 22 MB of prior-round reviewer residue (`.go` copies, ~30 `overlay_*.json`, `deps-*.txt`), untracked. | WORKFLOW §5.2 hygiene | — | **FIXED by the manager** — `rm -rf .scratch/review` in `~/worktrees/GoToCPP/dev.probecached` (a literal path, as §5.2 requires); `.scratch` is now empty and `git status --short` prints nothing. | CLOSED |

**Manager notes on P1's reach.** `NIX_CFLAGS_COMPILE` is empty in this shell and nothing in `Makefile`,
`.github/`, `scripts/`, `.envrc`, `flake.nix` or `devenv.nix` sets `-march=native` (re-checked), so no
CI path is red today. It is still worth fixing rather than documenting: the assertion's premise is
"a folder was given, so run 2 is served from it", and `diskFor` has refusal reasons the test does not
consult. A future Nix dev shell that adds `-march=native` turns `make test` red with a message that
blames the cache.

**R2's finding against the manager's own P1 fix (recorded as such).** The P1 fix I merged reasoned
that the environment can force `hostCPU` and "a test can't take it back", and so made the expectation
`tc.cached && !hostCPU && i == 1`. R2 showed that sentence is false: a test *can* scrub `build.KeyEnv`
with `t.Setenv(name, "")`. I did not take that on trust — I wrote the scrub myself as an overlay test
outside the worktree and measured `KeyEnv` native before, empty after, `run 2: cached=true`. The
difference matters for what the test proves: under `-march=native` my version silently tests nothing
about the cache, which is the very coverage the PM plan and TODO 137 rest on. The same round found
the predicate was also not a faithful copy of `diskFor` (F2, `selfID() == ""`), so the two findings
point the same way: scrub the environment, assert unconditionally, keep no second copy of the rule.
Both are FIX, not REFUTE.

---

## Job dev.stalehints: four stale comments in the compiler

**Status:** `[~]` in_progress · **Merging coder:** 1 · **Canonical worktree:** ~/worktrees/GoToCPP/dev.stalehints

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

**Second manager correction (2026-10-26), after the PM's plan.** The paragraph above is right about
`flag` but draws too wide a conclusion: it says "v1's minimal edit is correct". **It is not.** v1's
edit keeps the blanket clause "(sync, time, os, runtime, flag, strings, …) **are real packages now**",
and that universal is false for two packages that Go's `testing` imports:

- `fmt` — `tools/stubgen/main.go:126` has **no `Real: true`** and `stdlib/fmt/PORT.md:3` says
  "**Not ported.**" (decision 9.7 = A, ADR-100: native engine kept).
- `runtime/trace` — imported by Go's `testing` (confirmed with `go list -f '{{join .Imports "\n"}}'
  testing`), and **`stdlib/runtime/trace` does not exist at all**.

So of the version that keeps the imports framing, both v1 and v2 state something untrue; the PM is
right that v3's reframing ("the packages whose functions these natives replace") is the one that
avoids the false universal instead of patching around it. The manager had verified only that the
listed six are real (`sync` 133, `time` 122, `os` 136, `runtime` 115, `flag` 138, `strconv` 129,
`io` 121 — all `Real: true`) and generalised from that to the whole sentence; the ellipsis "…" was
the hole. **Adjudication: v3 wins `testing.hpp`.** The list `flag` correction still stands as far as
it goes — v2 must not drop `flag` — but it is not sufficient to save v1.

**Manager verification of the PM's other two novel claims.** Both were re-checked and hold:

- **`lookup_env` bullet is stale in v1.** `runtime/include/gocpp/testing.hpp:25` says
  "lookup_env, set_env and unset_env are os.LookupEnv, Setenv and Unsetenv for T.Setenv", but
  `stdlib/testing/native.gp:11-12` records that "T.Setenv uses os since step 9.10", and every
  remaining caller of `lookup_env` is a C++ native test (`runtime/test/testing_natives_test.cpp:110,
  227, 331, 594, 704`). v2 and v3 both fix it; see `native.gp:11-13` "have no caller".
- **"when the interop mode is typed" is incomplete.** `internal/typedinterop/mode.go:52` is
  `DefaultMode = Auto`, and `mode.go:190` returns `Decision{On: true}` for auto when clang++ is
  usable. So typed code also runs under the default `auto` with clang++; the comment must say
  "typed, or auto with clang++".

**Manager verification of the PM's `expr.go` finding.** `internal/lower/expr.go:549-553`: a call whose
signature is `Variadic()` (line 550) or whose single argument is a tuple (line 552) is given its
callee **without** reaching `funcCallName`. So the doc's "Every call by the name of a function must
go through here" — which v1 keeps in a new wording — is untrue of exactly the shapes this file
handles, which is the same overclaim the ticket exists to remove. v2's wording is the accurate one.

**Manager note on the PM's base choice.** The PM picks **v1** as base because two of the four files
are final as written there. That is sound, but combined with the adjudications above the merged
result is: `ops.go` v1 · `expr.go` v2 · `build.go` v1 + the auto/clang++ wording fix ·
`testing.hpp` v3. Only `ops.go` survives unchanged from the base, so the merging coder should treat
this as a four-file assembly, not a light patch.

### PM merge plan (the PM's amended plan, recorded)

**PM:** fresh read-only PM (amended once after the manager's `flag` correction) · **Merging coder:** 1

> Base: **v1**, merging coder **1**. v1 is already in the canonical worktree
> `~/worktrees/GoToCPP/dev.stalehints` on `job/dev.stalehints`, so no rename is needed. v1's own
> commit touches only the four in-scope files, and two of the four are final as written there.
>
> | File | Take | Reason |
> |---|---|---|
> | `internal/lower/ops.go` ~565 | **v1** | Present tense, keeps every old fact, and keeps the `step 3.5` provenance, which is live house style (`ops.go:592`, `slices.go:13`, `order.go:63`) rather than stale future wording |
> | `internal/lower/expr.go` ~668 | **v2** | Only version true of *every* call shape and that says why each shape is named elsewhere; v1's "Every call … goes through here" repeats the overclaim being fixed; v3 misnames a method as "named qualified already" |
> | `cmd/gocpp/build.go` ~1035 | **v1**, one clause corrected | Its block comment is the only one naming all three sources in `ResolveMode`'s order and pointing at it; v2 drops the legacy spelling, v3 names no switch. Correct "when the interop mode is typed" → also true under `auto` + clang++ |
> | `runtime/include/gocpp/testing.hpp` ~5 | **v3**, causal clause scoped | Naming the packages the natives *replace* is the accurate, PORT.md-checkable framing; but "a native is not there because a package is missing" is false for `libCPUName` |
> | `testing.hpp` bullet 1 ~16 | **v3** | Drops the false "A test binary has no `time` and no `sync` yet"; its cheaper-int64-clock / one-lock reasons match `PORT.md:15-16` |
> | `testing.hpp` `lookup_env` bullet ~27 | **v3** | v1 leaves "for T.Setenv" stale; `native.gp:11-12` says `T.Setenv` uses `os` since step 9.10 and the three C++ functions have no caller |
> | test evidence | combine | g++ focused set, then `-cxx clang++`, plus `TestTestingNatives` and `TestTestingNativesDefined` by name |
>
> Dropped: v2's `testing.hpp` list (drops `flag`, which Go's `testing` imports), v1's blanket
> "are real packages now" (false for `fmt` and `runtime/trace`), v1's `expr.go` universal, v3's
> `expr.go`, v2's and v3's `build.go`, and v2's/v3's `ops.go` reflows.

**Manager verification of the PM's remaining novel claims.** Each was re-checked on `dev`:

| Claim | Check | Result |
|---|---|---|
| v3's "not there because a package is missing" is false for `libCPUName` | `PORT.md:21` maps `libCPUName` → `internal/sysinfo`; `stdlib/internal/sysinfo` is **absent** | **confirmed** |
| v1's `lookup_env` bullet is stale | `testing.hpp:25` says "for T.Setenv"; `native.gp:11-12` says `T.Setenv` uses `os` since 9.10; callers are only `runtime/test/testing_natives_test.cpp:110,227,331,594,704` | **confirmed** |
| "when the interop mode is typed" is incomplete | `mode.go:52` `DefaultMode = Auto`; `mode.go:190` returns `Decision{On: true}` for auto with clang++ | **confirmed** |
| v1's `expr.go` universal is false | `expr.go:549-553`: `sig.Variadic()` and tuple-arg calls bypass `funcCallName` | **confirmed** |
| v1's base is `d2f99ef5`, not `1a1224be` (PM's correction of the manager) | `git merge-base HEAD dev` in the v1 worktree | **confirmed — the manager had said otherwise** |

### Merge handoffs (build phase)

Recorded from the coders' own reports; the manager has not re-run them.

- **c3 `d3423e13`**: all four sites, plus two neighbouring `testing.hpp` sentences it judged in scope
  (the `time`/`sync` bullet and the `lookup_env` bullet). Its g++ `TestCXX` was killed under load at
  ~97 (four groups re-run alone green, 270.9s); its **clang++ focused run was fully green** (482
  TestCXX passed).
- **c1 `c676f80c`**: all four sites, `build.go` moved to a block comment above the call, and the
  `time`/`sync` bullet reworded. Its `TestCXX/g++/O0` re-run alone at load ~5 green (363.7s); the O2
  half was **not** re-run. `TestTestingNatives` green under both g++ and clang++.
- **c1 correctly caught a manager error:** the manager's wind-down message to it cited "your commit
  `4ff62002`" and `.scratch/focused-final.log`, which belong to **c2**, not c1 — the manager mixed up
  the two coders. c1 reported its own commit and did not "reconcile", which is the right response.

### Merge result

**Commit `ac9fe281`** on `job/dev.stalehints` in the canonical worktree (on top of v1 `c676f80c`).
Four files, exactly the plan's scope, and **comment-only**: the diff's one non-comment line is the
`attachTyped` call with its trailing comment removed (`-d.attachTyped(in, pkg) // …` →
`+d.attachTyped(in, pkg)`) — the same statement, per the plan's instruction not to duplicate the
comment. `go build` + `go vet` on `./cmd/gocpp ./internal/lower` clean, gofmt clean, tree clean.

**Manager read every merged comment and checked its claims:**

| File | Merged text | Check |
|---|---|---|
| `ops.go` | present tense, keeps `step 3.5` and `sliceConversion` | matches plan |
| `expr.go` | "This is for a direct call of a function of the package…" — v2's accurate quantifier | matches the measured `expr.go:549-553` bypass |
| `build.go` | "resolves to typed: -interop, else GOCPP_INTEROP, else the legacy GOCPP_TYPED_INTEROP, else the default auto, which is typed when clang++ is usable" | **the clause correction landed** — names auto+clang++, cites `ResolveMode`/`DecideFor` |
| `testing.hpp` | v3's reframing, with the causal clause scoped and three exceptions named: `libCPUName` (no `internal/sysinfo`), `libAllocCounting`/`libAllocs` (no allocator counters), `libFrames` (no Go frame names) | **all three verified**: `stdlib/internal/sysinfo` is absent; `PORT.md:23` says "the runtime has no allocator counters"; `PORT.md:17` scopes `libFrames` |
| `testing.hpp` bullets | the `time`/`sync` bullet now gives the cheaper-clock/one-lock reasons; `lookup_env` says "`T.Setenv` has used os itself since step 9.10, so no Go code calls them, only their tests" | matches `native.gp:11-12` |

**Manager's own comment-only proof, independent of the coder's.** For each of the four files, the
worktree file and `git show dev:<file>` were both stripped of `//` comments and of blank lines, then
hashed:

| File | stripped hash `dev` | stripped hash `ac9fe281` | |
|---|---|---|---|
| `internal/lower/ops.go` | `38675331e1eb` | `38675331e1eb` | same |
| `internal/lower/expr.go` | `27d72d550c59` | `27d72d550c59` | same |
| `cmd/gocpp/build.go` | `a363baf33a8c` | `a363baf33a8c` | same |
| `runtime/include/gocpp/testing.hpp` | `054a7b4c8c69` | `054a7b4c8c69` | same |

Four identical hashes is a stronger statement than "one non-comment line": **no executable line differs
in any file**. The trailing comment on `d.attachTyped(in, pkg)` was deleted, not duplicated.

**The two gaps the merging coder left, settled by the manager:**

1. **`-cxx clang++` focused half never run.** Unnecessary here and shown so rather than assumed: a
   header whose comments changed cannot alter what either compiler parses, so the manager compiled
   `runtime/include/gocpp/testing.hpp` standalone under **both** compilers
   (`clang++ -std=c++20 -fsyntax-only -Iruntime/include` and the same with `g++`) — both `OK`. A full
   clang++ matrix for a comment-only diff would have cost 30 minutes of a shared box for no
   information.
2. **The `TestCXX/g++/(O0|O2)` `signal: killed`.** Three compiles (`unit.cpp`, `interop_test.cpp`,
   `map_test.cpp`) killed inside a `MemoryMax=6G` scope on a box at load 12-15. That is the cap and
   the load, not the diff — and the diff cannot reach a compile at all (hashes above). Recorded as
   **excused, not green**: if this job is ever asked for a runtime matrix it must be re-run at
   `-mem 16G` on a quiet box.

**Manager intervened twice on scope.** The merging coder restarted the full `./runtime` `TestCXX`
O0/O2 sweep (16G, 30 min) after the 6G run was killed, and again after the manager killed the first
restart. Both processes were stopped. A comments-only job does not get a runtime sweep, both because
it cannot show anything and because WORKFLOW forbids worker-run full phase/runtime sweeps. The
manager's instruction and the coder's compliance ("I stopped them, confirmed no process of mine
remains") are on the record so a later reader does not read the missing matrix as an omission.

**Departures from the plan the manager accepted, all evidence-backed:**

1. `build.go` needed more than "correct one clause": the honest sentence is "when the mode *resolves*
   to typed", which is only true if the `auto` default is named. Backed by `mode.go:52`
   (`DefaultMode = Auto`), `mode.go:187` (`Decision{On: true}` on the clang++-usable path) and
   `cmd/gocpp/env.go:160`.
2. `testing.hpp` replaced the plan's "a native is not there because a package is missing" with the
   capability statement, naming three natives and their missing capability. Manager re-checked all
   three in the rows above.
3. The package list is v3's (`sync, time, os, runtime, strconv, io, ...`), which describes the packages
   the *natives replace* and so avoids `flag` (real, `tools/stubgen/main.go:138`), `fmt` (not ported,
   `stdlib/fmt/PORT.md:3`) and `runtime/trace` (no `stdlib` copy) all at once. This is the correction
   the manager got wrong the first time — v1's blanket "are real packages now" is what it blessed.

**Out-of-scope, filed not fixed:** `stdlib/testing/native.gp:7-8` still carries the same stale
"declaration stubs until later steps of Phase 09" phrasing this job removed from `testing.hpp`. Same
defect, different file, outside the ticket — recorded below in Out of scope.

### Rounds

| Round | Fresh reviewer # | Commit (state hash) | Manager test run | Verdict |
|---|---|---|---|---|
| R1 | fresh reviewer | `ac9fe281` | manager: stripped-code hashes identical to `dev` for all four files; standalone `clang++`/`g++` compile of `testing.hpp`; `go build`/`go vet` clean | **FINDINGS (1 MAJOR, 2 MINOR)** |
| R1 fix | (merging coder 1) | `2ca8df06` | manager re-ran the stripped-code hashes on the **fix** (`expr.go` `4ad43c5b48b2`, `testing.hpp` `49d04960a389` — both identical to `dev`, so still comment-only) and checked every new claim: `find stdlib -type d -name testdeps` → only `stdlib/testing/internal/testdeps`; `stdlib/internal/sysinfo` absent; `runtime/src/testing.cpp:133-135` uses `shown_frames`+`resolve_pcs`; `ReadMemStats` (`runtime.gp:872-884`) never sets `Mallocs`; 30 `//gocpp:native` lines, so "two capabilities … three natives" is a minority | awaiting a fresh R2 reviewer |

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| H1 | R1 | MAJOR | `runtime/include/gocpp/testing.hpp:11` | "libFrames (runtime lacks the Go frame names)" is **false**, and it is text **this job added** (the base comment had no `libFrames` clause). A reader is told the native exists because the runtime cannot do it, when the runtime does exactly that. | WORKFLOW §8 / the ticket's Goal: "make all four comments true today" — this is the defect class the job exists to remove | TBD | **CONFIRMED — and it is the manager's own error.** I verified `libFrames` as one of three named exceptions and blessed this clause. The evidence against it is in three places I did not read: `stdlib/runtime/PORT.md:22-24` says names **are** Go's (`main.f`, `main.(*T).M`, `main.main.func1`, `main.Id[...]`); `runtime/src/testing.cpp:127-148` `frames()` calls the very same `gp::detail::shown_frames` + `resolve_pcs(..., true, ...)` that `runtimepkg.cpp:283,309` (`callers`, `pc_infos`) use; and `stdlib/testing/PORT.md:17`'s "can replace it **when runtime has the frames of the Go names**" is the stale sentence the wrong clause evidently came from. The reviewer's repro (`CallersFrames` printing `main.(*T).PtrM`, `main.Generic[...]`, `main.FuncLit.func1`) is the decisive kind: it observes the names. FIX: drop `libFrames` from the capability list and say what is actually true. | FIX (`2ca8df06`) | **FIXED-PENDING R2.** `libFrames` is out of the capability list; the new text says it reads the frames with the runtime's own reader (`shown_frames` and `resolve_pcs` of `frames.cpp`) — the one `runtime.Callers`/`CallersFrames` use too — so it is a later cleanup, not a gap. Manager re-checked each clause: `testing.cpp:133-135` is that pair, `runtimepkg.cpp:302,318` is the other, and `stdlib/runtime/PORT.md:22-24` states the names are Go's. The list now reads "Two capabilities are missing and give three natives" and counts exactly the three it names, so the universal that broke H1 is gone; the 30 `//gocpp:native` lines make "Most natives …" in the next sentence true. | `FIXED-PENDING` |
| H2 | R1 | MINOR | `runtime/include/gocpp/testing.hpp:38` | "This header is for stdlib/testing and stdlib/internal/testdeps" names a directory that does not exist. The package is `stdlib/testing/internal/testdeps` (`find stdlib -type d -name testdeps` returns only that). | reviewer procedure 3 "cross-references resolve" | TBD | **CONFIRMED — manager checked**: `stdlib/testing/internal/testdeps` exists, `stdlib/internal/testdeps` does not. **Pre-existing** (identical at base line 31), so it is an older issue in the same comment block: in scope per the ticket's "check the other names … fix only what is actually wrong". | FIX (`2ca8df06`) | **FIXED-PENDING R2.** Now `stdlib/testing/internal/testdeps`. Manager re-ran `find stdlib -type d -name testdeps` on the fix: one hit, the right one. | `FIXED-PENDING` |
| H3 | R1 | MINOR | `internal/lower/expr.go:668` | "This is for a direct call of a function of the package" overstates the function's domain. A variadic direct call and a tuple-argument direct call of a package function are qualified **without** `funcCallName` (`expr.go:548-554`: `case sig.Variadic(): callee = fg.g.ns + "::" + callee`, and the `funcCallName` call is only the fallthrough of `len(c.Args) != 1 \|\| !isTuple(...)`; the tuple path at `expr.go:601-604` qualifies itself "as funcCallName would"). | WORKFLOW §8 / ticket Goal "must not omit a case that makes it misleading" | TBD | **CONFIRMED — manager read the code**: both branches qualify before `funcCallName` is reached. The reviewer's emit shows `gpkg_main::variadic(...)` and `std::apply(gpkg_main::add, pair())`. Note the old text ("Every call … must go through here") was *flatly* false; the new text is only overstated. Both are wrong in the same direction, and the fix is to name the two bypasses. This is the exact bypass the manager had already identified earlier in this job and still let through in the merged wording. | FIX (`2ca8df06`) | **FIXED-PENDING R2.** Both bypasses are now named inline: the variadic call ("its last argument is a `gp::slice`, which would find `gp::append`") and `f(g())` with several results ("`std::apply` names the function without calling it, so ADL doesn't apply"). Manager re-read `expr.go:548-556` and `596-608` against them: the variadic branch qualifies at `:550`, the tuple branch at `:601-604` and `std::apply` is emitted at `:626`. Neither clause is new prose beyond the code. | `FIXED-PENDING` |

**Manager note.** H1 and H3 are both cases where I verified the *premise* (the native's map, the bypass code) and then approved wording that did not follow from it. H1 in particular: I checked `stdlib/internal/sysinfo` absent and read `PORT.md:23`, and then took `PORT.md:17`'s conditional clause as if it asserted a fact. The reviewer's habit of reproducing at the observation level — running the program and reading the printed names — caught what reading the code twice did not. Recorded because the same shortcut is available to every later round.

**Manager re-checked H1's premise at the observation level, and it holds — with a caveat.** Running a
`runtime.Callers`+`FuncForPC` program through `scripts/gp-repro` in this worktree: gocpp prints
`main.(*T).PtrM`, `main.main`; Go prints `runtime.Callers`, `main.main`,
`internal/runtime/atomic.(*Uint32).Load`, `runtime.goexit`. So the **names** are Go's, which is what
the header now claims. The **frame set** differs (2 vs 4: gocpp does not include its own `Callers`
frame or the atomic/goexit tail). That is a difference in what `Callers` returns, not in how a frame
is named, so the new wording ("they give Go's names, so they can replace it in a later cleanup") is
defensible — but a later cleanup that swaps `libFrames` for `runtime.Callers` will meet the frame-set
difference, and the header's "can replace it in a later cleanup" should not be read as "is a drop-in
replacement". Worth a PORT.md line when someone does that cleanup; not this job.

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| – | – | – | – | – | – | – | – | – |

### Out of scope

| Round | Note | Filed in |
|---|---|---|
| manager | `stdlib/testing/native.gp:7-8` repeats the stale "declaration stubs until later steps of Phase 09" wording that this job removed from `testing.hpp` | **already filed** — TODO.md:171-176 ("two stale stdlib prose sites"), recorded by the stalehints coders before the merge |
| manager | `internal/lower/expr.go:673`'s trailing comment and `cname`'s qualifier comment (`expr.go:267`) read redundantly beside the new block but are accurate | nothing — no defect |

---

## Job dev.staleflag: two LSP completion nits

**Status:** `[x]` done — CLEAN in R3, squash-merged as `57f3fa45` · **Merging coder:** 3 · **Canonical worktree:** ~/worktrees/GoToCPP/dev.staleflag (removed after the merge)

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

### Merge result

**Commit `143bf165`** on `job/dev.staleflag` · worktree now `~/worktrees/GoToCPP/dev.staleflag`.

One file, 8 insertions / 6 deletions: `internal/lsp/server/completion_test.go`. It swaps v3's
vacuous `F!(y)` row for v1's `F!(1)` row (byte-identical to `c65e0044`'s), rewrites the row comment
to name both constructs, and adds v1's fixture rationale. `completion.go` needed no change: v3's
version is byte-identical to v1's, and the blob is `6dec15da…` in both `143bf165` and `c65e0044`.

Tags kept for every version: `staleflag-v1` = `c65e0044`, `staleflag-v2` = `ad28e422`,
`staleflag-v3` = `8129beef`.

**Manager re-measured the row table on the merged commit** (the same two reverts as before):

| Revert | `F!(1)` row | `macro m()` row |
|---|---|---|
| nothing | pass | pass |
| only `Mode: cache.ParseFull` removed from the struct literal | **FAIL** | **FAIL** |
| only the fixture's `parser.ParseFile` mode → `ParseComments\|AllErrors` | pass | **FAIL** (`got=""`) |

Matches the PM's table exactly, so no row needed fixing in the merge.

**Manager process error (§7.3), corrected.** §7.3 says the manager renames the merging coder's
branch and worktree to the canonical `job/<job>` name *before* sending the merge plan. The manager
did not, so the merging coder built the merge in `dev.staleflag-c3`. Corrected afterwards: the
v1-only canonical worktree (untracked `.scratch/` only, all content tagged) was removed, its branch
deleted, and the c3 worktree moved and renamed with `git worktree move` + `git branch -m`. The
canonical path now holds `143bf165` and re-runs green. Lesson for the remaining jobs: do the rename
in the same step as dispatching the merging coder, not after the handoff.

**Also noted, harmless:** the merge commit's parent `8129beef` has a commit message that still
mentions the dropped `F!(y)` row. The coder declined to amend history, which is right; the merge
commit supersedes it.

### Rounds

| Round | Fresh reviewer # | Commit (state hash) | Manager test run | Verdict |
|---|---|---|---|---|
| R1 | fresh reviewer | `143bf165` | manager re-ran `^TestFixDangling$` ok 0.036s in the canonical worktree after the §7.3 rename; manager re-measured both revert rows (see the merge result above) | `FINDINGS (1)` — 1 NIT |
| R2 | fresh reviewer | `1b891f68` | manager: comment-only diff, gofmt clean, `^TestFixDangling$` ok 0.041s; reviewer re-derived the whole revert matrix in three disposable copies and left the worktree untouched at `1b891f68` | `FINDINGS (1)` — 1 NIT |
| R3 | – | `ce9c582d` (fix) | manager: `git diff 1b891f68 ce9c582d` is one hunk of comments; the new claim re-checked against `go/parser` (`a:3:1: expected declaration, found macro`); coder's `gp-focused` green, 1214 passed | – |

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| S1 | R1 | NIT | `internal/lsp/server/completion_test.go:924-928` | The mode comment explains the *old* fixture's failure ("in mode 0 `F!(1)` is a syntax error whose recovery swallows the second `x.`", "a macro file … loses everything"), but the fixture below always parses with `cache.ParseFull`. A reader at that line cannot reproduce the failure the comment describes without inventing the revert. | §8 "comments explain what is not obvious" | `FIX` (`1b891f68`) | **FIX-VERIFIED by the R2 reviewer**: it re-derived the full revert matrix in disposable copies — (a) `Mode` only removed → both rows fail; (b) fixture parse reverted → macro row only; (c) both → both — matching the comment and the merge commit exactly, and it confirmed neither surviving row is vacuous. The original defect (a reader could not reproduce the described failure) is closed; the residual wording error is tracked separately as S2. | `CLOSED-FIXED` |
| S2 | R2 | NIT | `internal/lsp/server/completion_test.go:926-930` | The rewritten comment says the macro file is "a syntax error **from its first line**". It is not: with `parser.ParseComments\|parser.AllErrors` (mode 36) the first error is `p.gp:3:1: expected declaration, found macro` — line 3, at the `macro` keyword. `"package p\n"` alone parses with `err=<nil>`. Only the "got `\"\"`" half is right. | §8 "comments explain what is not obvious" — the round-1 NIT's own substance: a reader at this line must be able to check the claim | `FIX` (`ce9c582d`) | **Manager verified the fix** (comment-only hunk; the new claim re-checked against `go/parser`, whose mode-36 error for the macro fixture is `a:3:1: expected declaration, found macro`; the coder also measured it through the tree's forked parser and confirmed `got ""` by calling `fixDangling`). Per the rule in this table, `CLOSED-FIXED` needs a **fresh reviewer's** `FIX-VERIFIED`, so the state stays `FIXED-PENDING` until R3 reports. | `CLOSED-FIXED` |

**R3 CLEAN.** The fresh reviewer confirmed S2 `FIX-VERIFIED` — `git diff 1b891f68 ce9c582d` is seven
lines and none is free of `//`; the tree's own forked parser reports `p.gp:3:1: expected declaration,
found macro` for the macro fixture under `parser.ParseComments|parser.AllErrors`, and line 3 of the
fixture is `macro m() {` at column 1, so the new clause is checkable where it sits. It re-derived the
revert matrix independently in four `go test -overlay` variants and got the same four outcomes the
comment and earlier rounds record, and it independently closed item 1 again (marshalling a
`protocol.CompletionItem` in both layouts gives byte-identical JSON — `additionalTextEdits` is
`omitempty`). `go test -timeout 40m -count=1 ./internal/lsp/server` → ok 302.745s; `gp-focused -x
-cpus 6 -mem 6G` green, 1214 passed, 0 skipped. Worktree left at `ce9c582d` with only `?? .scratch/`.

### Completion

- [x] CLEAN round: **R3**, fresh reviewer (`ce9c582d`)
- [x] Commit (state hash) of that round: `ce9c582d`
- [x] Final manager test run on that hash: `go vet ./internal/lsp/server` clean; `gofmt -l` empty;
      `^TestFixDangling$` ok 0.030s — run on `dev` **after** applying the squash, not on the branch
- [x] Every finding `CLOSED-FIXED` / `CLOSED-REFUTED`: S1 `CLOSED-FIXED` (R2 FIX-VERIFIED),
      S2 `CLOSED-FIXED` (R3 FIX-VERIFIED)
- [x] Out-of-scope notes filed: `repair_test.go:14-17`'s missing-`Mode` fixture is a latent trap
      (harmless today: `fixDangling`, `completion.go:253`, is the only `pf.Mode` reader) — filed as a
      TODO line
- [x] Squash-merged into dev (`57f3fa45`), TODO.md claims closed, step marked `[x]`
- Re-entered by: –

**Fix round 1.** Merging coder 3 answered S1 with `FIX` in **`1b891f68`** (on top of `143bf165`, not
amended). The comment now names both reverts and which row each fails:

> These rows are a ParseFull file and fail when the fixture below is not. With Mode unset the passes
> parse in mode 0, where `F!(1)` is a syntax error whose recovery swallows the second `x.`; with the
> source parsed in `parser.ParseComments|parser.AllErrors` instead, a macro file is a syntax error
> from its first line and nothing is fixed (`got ""`). The macro row fails under either revert, the
> `F!(1)` row under the first only.

**Manager verified the fix.** Comment-only (`git show -U0` has no non-`//` changed line), `gofmt -l`
clean, `^TestFixDangling$` ok 0.041s, tree clean. The claim the comment makes is exactly the table the
manager measured before the merge — macro row under either revert, `F!(1)` under the struct half only
— so the prose and the behaviour now agree and the failure is reproducible at the line. No test can
fail differently from a comment edit, which the coder stated.

**Manager note on the R1 finding.** The reviewer reached it adversarially and its own evidence
confirms the merge is behaviourally correct: it reproduced all three reverts independently
(Mode-only → both rows fail; fixture-parse-only → macro row fails; the deleted `F!(y)` row re-added
on a Mode-reverted copy still **passes**, confirming it was vacuous), and it verified `cache.ParseFull`
is exactly what the loader stores (`internal/lsp/cache/parse.go:31,73`) and what `completer` passes
(`completion.go:178`). So this is a comment-wording NIT and nothing else. Default is `FIX`; the
merging coder repairs the wording and no behaviour changes.

**Reviewer's OUT OF SCOPE, for the record (does not block):** `internal/lsp/server/repair_test.go:14-18`
has the same missing-`Mode` fixture shape. It hides nothing today (`repairText` reparses with a
literal `cache.ParseFull`, `repair.go:297,326`, never `pf.Mode`), so it is a latent trap worth its own
TODO line, not a patch here.

**R2's own out-of-scope confirmations (recorded, no action):**
- `repair_test.go:14-17` — same reasoning as above, re-derived by R2: `fixDangling` is the **only**
  reader of `pf.Mode` in the tree (`completion.go:253`, grep-confirmed), so the trap is real but
  dormant.
- `8129beef`'s commit message still describes the dropped `F!(y)` row. History is not amended; R1
  already accepted this as harmless.
- The `.scratch/` `.bak` copies the reviewer listed **are** in the job worktree, at
  `.scratch/x/` (`completion_test.go.mine`, `completion.go.bak`, `ct.now`,
  `probe-staleflag_test.go.bak`, `zz_probe2_test.go.bak`). The manager's first check looked for them
  at `.scratch/*.bak` and found nothing, concluded they were the reviewer's own scratch, and was
  **wrong** — they are one directory deeper. Removed; `git status --short` is now `?? .scratch/`
  only. This was coder scratch from an earlier round, all untracked, so nothing was ever committed.

**R2 also independently re-checked item 1's cosiness** — the blank-line removal — by marshalling a
`protocol.CompletionItem` in both layouts and getting byte-identical JSON. That is the proof the
"cosmetic" claim needed, and it means neither half of the job is a behavioural change.


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

**Manager §7.3 correction (same miss as staleflag).** The §7.3 rename was again not done before the
merge, so `d940d4b4` lives in `~/worktrees/GoToCPP/dev.whynotes-c3` on `job/dev.whynotes-c3`,
while the canonical path `~/worktrees/GoToCPP/dev.whynotes` still holds **v1** `3a0d06bf`. The R1
reviewer had been pointed at the job without the hash, so the manager messaged it to name its
worktree and `git rev-parse HEAD`, and to base every finding on `d940d4b4`. The worktree move and
branch rename are **deferred until that round reports**, so the reviewer's cwd is not yanked
mid-round. All four states are tagged so nothing can be lost:
`whynotes-v1` = `3a0d06bf`, `whynotes-v2` = `424c78df`, `whynotes-v3` = `2576f0e1` (coder 3's build),
`whynotes-merged` = `d940d4b4`.

**Manager re-verified the merge on `d940d4b4`** (in the c3 worktree, tree clean):
`TestLocalClosureHeadline` + `TestAnnotateGolden` + `TestExplainTopicsGolden` green
(`internal/lower` 0.026s, `cmd/gocpp` 1.234s). Negative check reproduced by disabling only the
`case len(caps) == 0` arm: `TestLocalClosureHeadline` FAILS, printing
`… (no gp::func, no allocation) that captures by reference` for a literal with an empty capture list
— the exact false sentence the ticket is about. The non-local branch has its own correct
`len(caps) == 0` case ("it captures nothing, and … is wrapped in gp::func"). Eight files changed vs
base, all in the plan's scope.

### Rounds

| Round | Fresh reviewer # | Commit (state hash) | Manager test run | Verdict |
|---|---|---|---|---|
| R1 | fresh reviewer | `d940d4b4` | manager: `TestLocalClosureHeadline`, goldens, negative check (recorded above) — all green | **FINDINGS (1 MAJOR, 2 MINOR, 2 NIT)** |
| R1 fix | (merging coder 3) | `f50e67a1` | manager: `internal/lower` `ok 12.061s`, `internal/explain` `ok 0.250s`, `TestAnnotate` `ok 4.701s`; negative check reproduced **FAILS on 8 assertions** with the pre-fix sentences | awaiting a fresh R2 reviewer |

**§7.3 now done.** The reviewer confirmed it reviewed `d940d4b4` in `dev.whynotes-c3` and never
`dev.whynotes` (v1), so the round stands. The deferred move was then performed: the v1 worktree and
`job/dev.whynotes` were removed (v1 preserved as tag `whynotes-v1`; it is **not** an ancestor of the
merge, so the tag is what keeps it), and `dev.whynotes-c3` was moved to `~/worktrees/GoToCPP/dev.whynotes`
with `job/dev.whynotes` as the branch. `git worktree list` now shows the job at the canonical path.

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| W1 | R1 | MAJOR | `internal/lower/whynotes.go:158-161` | The new zero-capture **non-local** headline says "a lambda that is used as a func value is wrapped in gp::func", but four shapes emit no `gp::func`: `defer func(){...}()` and `go func(){...}()` (→ `gp_df.gp_push_call([…])` / `gp::go_call`), a macro closure parameter (`rawLits`, `closures.go:177`, → raw lambda), and an immediately-called literal that calls `recover` (→ `gp::aware`). | WORKFLOW §8 "a reason is about the C++ of its own chunk" — on a line this job authored | TBD | **CONFIRMED — manager reproduced.** `defer func() { fmt.Println("bye") }()` in a `main` with a defer emits `gp_df.gp_push_call([]<int = 2>() mutable GP_TRACE_FRAME -> void {…})` under the headline claiming `gp::func`. The by-value `func()` parameter case **is** wrapped, so the clause is true for the common shape and false for the four above — which is why this is MAJOR, not BLOCKER. FIX: make the clause say what is true for each, or state the rule without the unconditional wrapper claim. | FIX (`f50e67a1`) | **FIXED-PENDING R2.** The unconditional claim is gone from every `noteClosure` headline; a comment at the top of the function lists the four no-`gp::func` shapes and where each is decided (`closures.go: funcValue, callValue, deferValue`), so it cannot be re-added silently. A new `TestAnnotateTruth` rule (`annotate_test.go:471`) forbids `wrapped in gp::func` in the chunk of any `is a C++ lambda (the stack trace names it func` note while requiring `<int = `; the coder reports it failing on 9 programs pre-fix, and the manager's own negative run fails `whynotes_test.go:264` with `the headline of a lambda that captures nothing claims a gp::func`. | `FIXED-PENDING` |
| W2 | R1 | MINOR | `internal/lower/whynotes.go:163-171` | Per-variable note says "by copy into the lambda" for a variable the emitted lambda captures by reference: a literal passed to a macro's **ref-style** closure parameter is `[&n]` (`closures.go:296` `byRef := fg.refLits[fl] \|\| local`, `refLits` set at `closures.go:213`), but the loop only branches on `inBox`/`local`, so `local` false prints "by copy". | WORKFLOW §8; the job's own `topics.go:102` now says "the annotation says which one happened for each captured name" | TBD | **CONFIRMED — manager reproduced**, and it is **pre-existing** (`1a1224be` too, per the reviewer). `#Later(fmt.Sprint("n = ", n))` with `macro Later(msg lazy string)` prints `captures n by copy into the lambda` directly above `[&n]`. The ticket said these notes were already correct and must not change; the reviewer shows the ticket was wrong on this one shape. FIX (in scope: same function, same feature, and the job's prose now advertises it). | FIX (`f50e67a1`) | **FIXED-PENDING R2.** The rule is factored to one place, `byRefLit` (`closures.go:301`), called by both `captures()` (`:293`) and `noteClosure` (`whynotes.go:129`) — so the annotation cannot say "by copy" where the lambda has `[&`. Manager confirmed only one spelling of `refLits \|\| local` exists in the tree. Negative: `whynotes_test.go:224` fails pre-fix with `the note of n says no reference capture, though the lambda is [&n]`. | `FIXED-PENDING` |
| W3 | R1 | MINOR | `internal/lower/whynotes.go:149-151` | "how it captures each variable is noted where the variable is used" is absolute; the note lands at the **ident's** position inside the lambda body, which for a multi-line literal is a different chunk from the headline, and for a capture used only in the header lands below the use. | ticket's "prefer the option that cannot go stale" | TBD | **PARTLY CONFIRMED — manager reproduced the displacement but not the ticket's rule.** `boxmulti.gp` puts the note at the `arr` ident in the body (line 9) while the headline is at line 8; that is exactly what "at the ident" means, and it is what makes the note findable. The absolute "where the variable is used" is looser than the truth. FIX as wording only — do not move the note. | FIX (`f50e67a1`) | **FIXED-PENDING R2.** Wording only, position untouched: now "how it captures each variable is noted at the first use of its name". The coder declined to add a machine-checked rule for it and said why — the note's own text is `captures X by …`, so "the chunk contains a use of X" can never fail; a vacuous rule was correctly not added. | `FIXED-PENDING` |
| W4 | R1 | NIT | `internal/explain/examples/closures.gp:27` | The example exercises only the boxed case, never boxed + by-reference in one literal, so the neutral headline's "each variable" is not covered by a golden. | test-coverage gap | TBD | **CONFIRMED as a coverage gap, but the PM forbade exactly this shape** ("never put a by-reference and a boxed capture in one example literal — the `[&` truth rule"). Adding it requires a `TestAnnotateTruth` rule for a mixed literal; that is more than a NIT and does not belong in this round. Fold into W1's fix only if the coder keeps it small, else file it. | OPEN (deferred) |
| W5 | R1 | NIT | `internal/explain/topics.go:102` | "Such a closure captures by reference a variable that stays on the C++ stack" — "Such" refers to the local literal of the previous sentence, but a **non-local** literal (macro ref parameter) also captures by reference. | prose accuracy | TBD | **CONFIRMED — same evidence as W2** (`[&n]` on a non-local `gp_inst_…_Later` argument). Fold into W2's fix, since both are the same wrong assumption that by-reference implies local. | FIX (`f50e67a1`) | **FIXED-PENDING R2.** "Such a closure…" replaced by "A local closure captures by reference…" plus a new paragraph for the non-local ref case (a macro's lazy parameter or `#body()`), which names `[&` and the no-boxing rule. Manager read `topics.go:101-105` and it reads correctly against `byRefLit`. | `FIXED-PENDING` |

### Out of scope

| Round | Note | Filed in |
|---|---|---|
| R1 | `TestGuideExamples` fails on `dev` (`docs/guides/explain.md` predates `mutable GP_TRACE_FRAME`) | already TODO.md, pre-existing |
| R1 | `docs/guides/explain.md:214-215` embeds the old long non-local headline; regenerating it is blocked by the same `TestGuideExamples` failure, and the new short sentence will appear when that is fixed | nothing — follows from the line above |
| R1 | `internal/explain/examples/macros/main.gp:113` prints two contradictory notes for one name on one line (`captures b by copy` and `captures b by reference`); pre-existing on `1a1224be`, a different mechanism (two literals' notes colocated) | TODO.md, new line |
| R1 | the reviewer's own `.scratch/` shows as untracked because `.scratch/` is in no `.gitignore` in this worktree; the coder scratch is expected, the reviewer left it in place | nothing — `.scratch/` is never committed |

### Fix round 1

Dispatched to the merging coder 3 in the canonical worktree on `job/dev.whynotes` (`d940d4b4`).
Scope: **W1, W2, W3, W5** as code/wording, **W4 deferred** (it needs a new `TestAnnotateTruth` rule
and the PM forbade the mixed-capture example without one).

**Result: `f50e67a1`** (parent `d940d4b4`, one new commit), 18 files, +207/−56. Manager re-ran and
independently re-proved the negative:

| Check | Result |
|---|---|
| `go test ./internal/lower` | `ok 12.061s` |
| `go test ./internal/explain` | `ok 0.250s` |
| `go test -run TestAnnotate ./cmd/gocpp` | `ok 4.701s` |
| **negative**: `git checkout d940d4b4 -- whynotes.go closures.go`, re-run the headline tests | **FAILS on 8 assertions**, printing the exact wrong sentences (below) |
| `byRefLit` is the only spelling of `refLits \|\| local` | confirmed — `closures.go:301` defines it; `captures()` (`closures.go:293`) and `noteClosure` (`whynotes.go:129`) both call it |
| `wrapped in gp::func` left in product text | see the coder-claim correction below |

**Pre-fix output the manager saw itself** (not quoted from the handoff):

```
func literal #4 is a C++ lambda (the stack trace names it func4); what it captures is copied into
  the lambda (a box is shared), and a lambda that is used as a func value is wrapped in gp::func, …
captures n by copy into the lambda: it never changes after the closure exists and is at most 128 bytes
```
with `whynotes_test.go:224` reporting `the note of n says no reference capture, though the lambda is
[&n]` — W2 exactly — and `whynotes_test.go:264` reporting `the headline of a lambda that captures
nothing claims a gp::func` — W1 exactly.

**Manager correction of a coder claim.** The handoff says the stale sentence in
`docs/guides/explain.md:216` is "the last copy in the repo (`rg "wrapped in gp::func"` shows only it
and my test comment)". **That is wrong**: `internal/explain/topics.go:108` and
`cmd/gocpp/testdata/explain/closures.golden:20` both still carry "wrapped in gp::func" — correctly,
since those sentences are about the general shape ("Any other func literal, used as a value, is a
C++ lambda wrapped in gp::func"), not about a specific literal. So the coder's `rg` missed two files
and the "only it" claim does not hold; the sentence itself is right in those two places and needs no
change. What matters, and is true: the *annotation* headline no longer makes the claim, and the guide
line will drop it when `TestGuideExamples` is fixed.

**Coder's out-of-scope flags, manager-checked:**
- `internal/explain/examples/macros/main.gp` (the path in TODO.md:171) **does not exist**. The real
  file is `examples/macros/main.gp` — confirmed with `find . -name '*.gp' -path '*macros*'`:
  `./examples/macros/main.gp`, and nothing under `internal/explain/examples/`. The TODO line's path
  must be corrected when that line is closed. (The manager had recorded the same path; it was wrong.)
- The coder's claim that a macro lazy parameter of type `func()` gets a `gp::func` nil-check claim is
  a different function (`noteCallValue`) — left alone, correctly out of scope.


