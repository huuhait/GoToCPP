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
| R1 | fresh reviewer | `2154f0e2` | manager: merged file with cache `ok 2.585s`, no env `ok 2.547s`; three-way revert matrix above (assertion-only removal passes; zeroing removal fails in the `cache` subtest); gofmt clean | – |

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| – | – | – | – | – | – | – | – | – |

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
| R2 | – | `1b891f68` | manager: comment-only diff, gofmt clean, `^TestFixDangling$` ok 0.041s | – |

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| S1 | R1 | NIT | `internal/lsp/server/completion_test.go:924-928` | The mode comment explains the *old* fixture's failure ("in mode 0 `F!(1)` is a syntax error whose recovery swallows the second `x.`", "a macro file … loses everything"), but the fixture below always parses with `cache.ParseFull`. A reader at that line cannot reproduce the failure the comment describes without inventing the revert. | §8 "comments explain what is not obvious" | `FIX` (`1b891f68`) | – | `FIXED-PENDING` |

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
| R1 | – | – | – | – |

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| – | – | – | – | – | – | – | – | – |
