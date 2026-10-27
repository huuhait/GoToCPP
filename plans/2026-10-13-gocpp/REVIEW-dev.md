# dev REVIEW: jobs against `dev` that are not a phase step

> Maintained by the **manager only**, per [WORKFLOW.md](WORKFLOW.md) (v2: 3 coders → PM merge → review until one CLEAN). The manager is the manager. Coders, the PM and reviewers never edit this file.
> Legend: `[ ]` not_started · `[~]` in_progress · `[x]` completed.
> Finding lifecycle: `OPEN` → `FIXED-PENDING` / `REFUTED-PENDING` → `CLOSED-FIXED` / `CLOSED-REFUTED` / `REOPENED`.
> Every review covers correctness + spec parity, tests + edge cases + determinism, and simplicity + safety, scoped to the job.
> Performance-shaped jobs run three techleads before the coders (§3 step 0); their briefs are recorded under "Techlead research".

## Jobs

| Status | Job | Steps | Rounds | Clean | Completed |
|---|---|---|---|---|---|
| `[x]` | `dev.evalorder` — map key read before the appended value's argument | – | 5 / 3 | 1 (R5) | `ca29ca14` |
| `[~]` | `dev.probecached` — `TestDeclsDeterministic` fails when the probe cache is on (TODO 137) | – | 7 / 3 | 0 | **STOPPED at R7 — awaits user decision** (`9bae04e4`; code verified correct, 4 comment NITs/MINOR open) |
| `[~]` | `dev.stalehints` — four stale comments in the compiler (TODO 145) | – | 6 / 3 | 0 | – |
| `[x]` | `dev.staleflag` — two LSP completion nits (TODO 184) | – | 3 / 3 | 1 (R3) | `57f3fa45` |
| `[~]` | `dev.whynotes` — `emit -annotate` headline "captures by reference" for a boxed capture (TODO 144) | – | 5 / 3 | 0 | – |
| `[~]` | `dev.rtbugs` — nine Runtime claims in one batch (TODO 103-109, 120, 126 first sub-claim) | – | 0 / 1 | 0 | – |

The `/ 3` is the target, not a ceiling: WORKFLOW.md:239 lets a job run to **5** rounds before the
manager must split it, clarify the ticket, or take it to the user. `dev.probecached` reached 5 on
2026-10-26 with two NITs, both in the single comment block its last fix added and both verified by
the manager running the function. **Manager decision: clarify, not split.** The two findings are two
sentences of a doc comment — a split would mean a second worktree, a second coder and another review
round for a change smaller than the ticket that spawned it — so the next round is scoped to exactly
those two sentences, with the replacement text measured against the real `dropNativeArch` before it
is written. **That round is R6, on `74f5e52f`, and it came back with three findings, so the job did not close.**
Its fix changed code as well as comments, because P10 was a real hole in `dropNativeArch` and not only
a false sentence; R6 confirmed the code half (`FIX-VERIFIED`) and found only comment text left.

**Manager decision at the ceiling, recorded because it departs from "split":** the remaining work is
one clause of one doc comment plus one caveat, and the reviewer and I both ran the code and found the
*code* correct. Splitting would create a second worktree and a second coder to rewrite a paragraph. So
the round is **clarified, not split**: round 7 is bounded to `dropNativeArch`'s and
`scrubNativeArch`'s doc comments, the replacement text is prescribed clause by clause, and the coder
must measure every clause by running the function before writing it. The root cause of six rounds here
is now diagnosed and named: each fix replaced one universal with a narrower universal, and the comment
keeps asserting a rule over a fixed option list instead of saying what the list is. The prescribed
text states no universal at all. **If R7 is not CLEAN, the job stops and goes to the user.** `dev.stalehints` is likewise past its target (5 rounds) and its fix
rounds have stayed comment-only; its R5 sweep was scoped to make the round terminal.

### probecached: STOPPED at round 7, per the rule above (2026-10-26)

**R7 on `9bae04e4` came back with 1 MINOR + 3 NIT, so the pre-committed stop applies and the job awaits
the user's decision.** Every one of the four findings was reproduced by the manager with the real
`dropNativeArch`/`build.NativeArch`, and all four sit in text **this job added** (`git blame`:
`614c8bf8`, `69e5d819`; `git merge-base --is-ancestor` says neither is on `dev`), so they are in-area
and not out-of-scope noise.

**The failure of my own prediction is the important part of the record.** I told the coder "after your
amend, the file's prose has no universal left for a reviewer to find." That was false. I prescribed
replacement text for **one** of the file's two doc comments (`dropNativeArch`, `scrubNativeArch`) and
never audited `TestDropNativeArch`'s comment 48 lines below, which carries the *same* defect the P11
fix was written to remove. The diagnosis "each fix replaces one universal with a narrower universal"
was right, and I then applied it to one block and declared the file clean. A sweep proves what its
pattern covers; I stated the conclusion wider than the evidence — the identical mistake I recorded
against myself for H15 earlier the same session.

**What is verified and not in dispute:** the ticket's behaviour works (R7 re-derived it by overlay);
the scrub is load-bearing (disabling only its body passes in the default shell and fails
`TestDeclsDeterministic/cache` in the native env); the invariant holds over 200,000 random `KeyEnv`
shapes; `scripts/gp-focused` is green at 683 passed / 2 skipped; and both the reviewer and the manager
ran the code and found it correct. **The code has been approved by two independent reviewers and
verified by me; only comment prose is open.**

**Options as the manager sees them, for the user to choose:**
1. **One more round (R8), fully prescribed.** The four replacements are already measured clause by
   clause and can be written without new investigation — roughly one coder pass and one review.
   Cost: an eighth round on prose, against a documented pattern of each round finding one more.
2. **Merge the verified code and file the four comment sentences as a follow-up ticket**, with the
   measured corrections attached so the follow-up is mechanical. This keeps a correct fix out of
   limbo and stops spending rounds on documentation.
3. **Keep going round after round** until a reviewer happens to say CLEAN on prose. Not recommended:
   nothing in the last four rounds suggests the rate improves.

The manager's recommendation is **2**, with 1 as the immediate fallback if the user prefers the job to
close on its own terms. The manager will not merge on its own authority — the exit rule needs a
reviewer CLEAN, and the state rule forbids the manager closing a job unilaterally.

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

**Status:** `[x]` **CLOSED — merged `ca29ca14` (squash, 2026-10-26)** · **Merging coder:** 2 · **Canonical worktree:** removed

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
| R1 | fresh reviewer | `023cea66` | manager re-ran the reviewer's boundary repro itself: depth 100 `SAME`, **101 and 150 `DIFFERENT`** (`map[5:[7]]` vs Go `map[1:[7]]`); manager re-derived the 16/23 revert figure from the failure log; manager built HEAD-stub vs `dev`-stub and compared `emit` on the interface case (`gp::ref` on HEAD, `gp::set` on `dev`); gc's own `-W` IR shows `CONVIFACE main.A`, never `CONVNOP` | **FINDINGS (2 MINOR, 2 NIT)** |
| R1 fix | (merging coder 2) | `20cdd28e` | manager built `20cdd28e` and `023cea66` and ran the boundary itself: **`base023` N=100 (99 conversions) → `map[1:[7]]`, N=101 (100) → `map[5:[7]]`; `20cdd28e` → `map[1:[7]]` at every depth tested (100/101/150/500 by the coder; 100/101/150 by the manager).** Manager re-ran the whole diff program on both binaries against Go: `base023` differs on the `deep conversion chain` row only; the **fixed** binary matches Go on all 24 rows; with `internal/lower/maps.go` reverted to `dev` the same program diverges on 20 rows, so the rows are live. Negative check reproduced: the new `deep conversion chain` and `non-empty to non-empty with the same methods` subtests **fail on `base023`** and pass on `20cdd28e`. gc cross-check: `assignOp` rule 2 gives `OCONVNOP` only for an empty source interface or identical types, matching the new predicate; `-W` IR confirms `CONVIFACE main.A` / `CONVNOP main.E`. `go vet` clean, `gofmt -l` empty | awaiting a fresh R2 reviewer |
| R2 | fresh reviewer | `20cdd28e` | reviewer: `scripts/gp-focused -x -cpus 6 -mem 6G` green (gofmt, vet, `internal/lower` ok 26.155s 406 passed, `TestGolden` 41 passed 1 skipped, `TestDiff/maps_append_key_expr` 3 passed); `-cxx clang++` on the diff test ok 7.470s; adversarial probes (parentheses, named conversions, uintptr/unsafe.Pointer, string `+`, the ticket repro) all SAME vs Go. E1/E2/E3 all `FIX-VERIFIED`. **Manager reproduced both new findings itself** (below): the `float64 + float64` row is vacuous — confirmed twice, with a rounds-guard mutant and with a float-`+`-rejecting mutant — and the `convNopExpr` doc comment inverts gc's rule, confirmed against gc's own source comment | **FINDINGS (1 MAJOR, 1 NIT)** — both confirmed, one with a correction to the suggested fix |
| R2 fix | (merging coder 3) | `17fc288d` | manager verified independently: `gofmt -l` empty and `vet` clean; `TestMapAppendShape` green at HEAD and caught by **different rows** under the two mutants the manager rebuilt from source on this commit (mutant A → only `float conversion` fails; mutant B → only `float addition` fails), so the new pin is live and not vacuous; E7's new wording matches `assignOp`'s `src.IsEmptyInterface()` branch at `subr.go:274` and agrees with the code two lines under it; the coder's own "pre-existing" label on line 190 was corrected by the manager (this job created the file) but is not a finding (`.gp` files carry no 100-column rule) | awaiting a fresh R3 reviewer |

Verdicts: `FINDINGS (n)` · `CLEAN` · `VOID (reason)`. Every reviewer number is a new instance.

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| E1 | R1 | MINOR | `internal/lower/maps.go:798` (`stripConvNop`, `for range 100`) | A silent correctness cliff: 101+ nested no-op conversions are not stripped, so `sameSafe` rejects both keys and the statement falls to the slow path, which reads the key **after** `f`. 100 nested conversions gives Go's order; 101 gives the old wrong order. gc has no cap (`ir/expr.go:1044`: `for l.Op() == OCONVNOP { l = … }`), so the bound is this diff's alone. | WORKFLOW §8 "Go behavior … or a documented divergence"; ticket item 2 ("must equal `go run`") | `FIX` in `20cdd28e` — `for range 100` → `for`, with a comment on why the loop terminates (each turn replaces `e` with a strictly smaller subtree) | **CONFIRMED — manager reproduced the boundary itself.** Generated `m[int(…N…(k))] = append(m[k], f())` for N=100,101,150 and ran each through `scripts/gp-repro`: **100 `SAME`; 101 and 150 `DIFFERENT`**, gocpp `map[5:[7]]` vs Go `map[1:[7]]`. Direction: `dev` was wrong at *all* depths, so this is not a regression — HEAD fixes ≤100 and leaves ≥101 at `dev`'s behaviour — but the cap is new code inside the predicate this ticket exists to make exact. FIX: drop the cap (the loop is bounded by the finite AST, as `ast.Unparen` + one level per iteration guarantees), or if a cap stays it must not change the answer at the boundary. **Manager re-measured on the fix:** boundary is 99 conversions (safe) / 100 (wrong) on `base023`; `20cdd28e` gives Go's answer at 100, 101 and 150. | **FIX-VERIFIED by R2**, independently: the new subtest fails on `023cea66`'s `maps.go` through an overlay (`deep conversion chain: mapAppend = false, want true`) and passes at HEAD; the unbounded `for` was re-checked for termination (each turn replaces `e` with `c.Args[0]`, a strictly smaller subtree); 150/5000/30000-deep chains all SAME vs Go. | `CLOSED-FIXED` |
| E2 | R1 | MINOR | `internal/lower/maps.go:829,838` (`convNopExpr`) | `convNopExpr` returns true for a conversion between two **named non-empty interfaces with identical method sets**, because `convNop` (`panicid.go:156`) matches `types.Identical(from.Underlying(), to.Underlying())` first. gc does not make that an `OCONVNOP`: `convertOp` → `assignOp` rule 3 (`subr.go:296-315`, `dst.IsInterface()`) gives `OCONVIFACE`, and `ir.SameSafeExpr` has no such case. So HEAD takes the fast path where gc takes the slow path — the only direction the ticket names as able to silently reorder — and the doc comment's "these are the rest of gc's set" is false for interface operands. | WORKFLOW §8 / ticket's "must equal `go run`" | `FIX` in `20cdd28e` — an interface check placed **before** `convNop`, returning `types.Identical(from, to) || types.Identical(from.Underlying(), to.Underlying()) && isEmptyInterface(from)`, plus `isEmptyInterface` (`Complete().NumMethods() == 0`) | **PREDICATE CONFIRMED, OBSERVABLE EFFECT NOT — reported as such, and the manager reached the same place independently.** Confirmed: `gocpp emit` on `mx[A(xv)] = append(mx[A(xv)], change())` with `A`/`I` identical interfaces gives `gp::ref` on HEAD and `gp::set` on a `dev`-stub build; and gc's own IR (`go build -gcflags='-W'`) prints `CONVIFACE main.A` for both sides, never `CONVNOP`. **Not demonstrated:** neither the reviewer (6 probes) nor the manager (4 probes: key changed in `f`, `delete` of the held key inside `f`, nil map, concrete-vs-interface source) could produce wrong output — an interface key is always slow for gc and its slow path copies the key before the RHS arguments, so the orders coincide. FIX: refuse nop-stripping when either side's underlying type is an interface, matching gc's `OCONVIFACE`. This is worth doing on the predicate even though the behaviour is currently unobservable, because the predicate is what a later change will lean on. **Manager re-checked the fix against gc's source:** `assignOp` returns `OCONVNOP` under rule 2 only when `src.IsEmptyInterface()` (a non-empty source recomputes the itab), which is exactly the new condition; `-W` IR on this Go prints `CONVNOP main.E` for `E(ev)`, `CONVNOP any` for `any(ev)`, `CONVNOP main.N` for an interface embedding `E`, and `CONVIFACE main.A` / `CONVIFACE main.E` otherwise — the predicate agrees on every one, and `emit` on the fixed build takes the slow path for the `A(xv)` case as gc does. | **FIX-VERIFIED by R2**, independently, and from gc's source again rather than from the ticket: `convertOp` returns after `assignOp` for any interface operand, and its only `OCONVNOP` exits are `types.Identical` plus `src.IsEmptyInterface()` — the new predicate exactly. The subtest fails pre-fix (`non-empty to non-empty with the same methods: mapAppend = true, want false`). The R1 caveat stands unchanged: the effect is latent, because an interface key is slow in gc either way. | `CLOSED-FIXED` |
| E3 | R1 | NIT | `tests/diff/maps_append_key_expr.gp:144,148` | The `int -> int64` and `int -> int8` rows write the **same source node** on both sides (`m64[int64(ki)] = append(m64[int64(ki)], …)`), so they pass whether or not conversions are stripped and pin nothing — the exact defect `023cea66` fixed for the named-int row beside them, still present two rows later. | ticket item 1 (the test must pin the rule); WORKFLOW §8 "every new test fails without the change" | `FIX` in `20cdd28e`, with a deviation the manager verified: the ticket's prescribed row `m64[int64(ki)] = append(m64[ki], …)` **does not compile** (an `int` is not an `int64` map key), so both int rows use a plain name of the map's own key type (`k8 int8`) on the right. Also live-ified the three other identical-both-sides `+` rows | **CONFIRMED — manager ran the revert itself.** With `internal/lower/maps.go` replaced by `dev`'s, the two rows print `int -> int64: map[1:[7]]` and `int -> int8: map[1:[7]]` under both g++ and clang++ — identical to Go, so they pass. FIX: make one side a plain name (`m64[int64(ki)] = append(m64[ki], change())`), which does distinguish (`gp::ref` on HEAD, `gp::set` on `dev`). **Manager verified the deviation and the new rows:** copied the fixed diff program and ran all 24 rows under Go, under `base023`, and under `20cdd28e` — Go and `20cdd28e` agree on **every** row; `base023` differs only on `deep conversion chain` (the pre-existing `for range 100` cap, not on E3's rows); with `maps.go` reverted to `dev` 20 rows diverge, which is the live-ness the ticket wanted. The coder's two claimed non-strengthenable rows (float64 `+`, `float conversion split`) match the manager's reading: a same-kind float conversion is an `OCONV` and never reaches `mapAppend`, so it pins the negative only. | **FIX-VERIFIED by R2**, independently: the old shape passes with `dev`'s `maps.go` (`map[1:[7]]`, pinned nothing), the new shape DIFFERENT there (`map[5:[7]]` vs Go `map[1:[7]]`), SAME at HEAD; whole-program live-ness re-measured — 24/24 SAME on HEAD, 22 of 24 differ with `maps.go` reverted. R2 also confirmed the coder's deviation is forced (`m64[ki]` is an `int` key, a compile error). | `CLOSED-FIXED` |
| E5 | (manager, during E1's fix) | NIT | `internal/lower/panicid.go:36` (`staticConst`, `for range 100`) | **The same cap E1 fixes exists in the sibling loop, and it is already biting: the chain stops being followed at 99, not 100** — `panic(aN)` in a chain `a0 := 42; a1 := a0; …` emits `gp::panic_constant(int64_t(42))` up to N=98 and plain `gp::panic(aN)` from N=99. Pre-existing on `dev`, not caused by this job. | WORKFLOW §8 (a cap that silently changes the answer) | — | **NOT OBSERVABLE IN THE MANAGER'S REPRO, SO NOT A FIX REQUEST — recorded so the next person knows.** The cap's cost is that `staticConst` stops seeing the constant, which by its own doc comment means "such a chain prints both panics" — the documented divergence gc-follows/chains-gocpp-doesn't, which the comment at `panicid.go:28-31` already declares. Manager tried to observe the `panic_identity` difference gc's constant gets for `-1` (the `panic: -1 … [recovered, repanicked]` case) at N=97/98/99 with a recover-and-repanic wrapper: **all three print `panic: -1 [recovered, repanicked]` on both gocpp and Go**, so the cap has no user-visible effect in that shape. Out of scope for this job (pre-existing, different function); filed as its own TODO line rather than folded into E1's fix. If someone does widen it, drop the cap rather than raising it — the shift from 100 to 99 is the loop's per-iteration bookkeeping, which is exactly the kind of off-by-N a larger constant hides. **The fix-round coder independently measured the same boundary (N=98 constant, N=99 and N=100 not) and reproduced the stderr divergence, so the NIT is now confirmed by two workers.** | OUT-OF-SCOPE |
| E4 | R1 | NIT | `.scratch/` in the worktree | 120 MB untracked: 109 coder-era probe files, a 40 MB `.scratch/bin`, an 80 MB reviewer `.scratch/r1`, and `.scratch` is not in `.gitignore` (`git check-ignore` exits 1). Untracked so it cannot merge, but §5.2 lists it as a pre-merge hygiene item. | WORKFLOW §5.2 | — | **FIXED by the manager** — `rm -rf .scratch` in `~/worktrees/GoToCPP/dev.evalorder`, recreated empty (a literal path, as §5.2 requires). `git status --short` now prints nothing; disk went 74G → 73G. The work is committed on `job/dev.evalorder`, so nothing was lost. | CLOSED |

| E6 | R2 | MAJOR | `tests/diff/maps_append_key_expr.gp:86` (printed at `:87`) | The `float64 + float64` row **cannot fail**, which is the same defect class E3 was opened for, in a row the ticket names explicitly in its "Done when" list ("Add the neighbouring forms in the same program — `int + int`, `uint32 + uint32`, `float64 + float64` … the test must pin the whole rule"). The mutation it exists to catch is a plausible one: drop `&& !rounds(from)` at `internal/lower/maps.go:857`, so a same-kind float conversion counts as an `OCONVNOP`. That genuinely flips the code path (`emit`: HEAD `gp::set(mf, …)`, mutant `gp::ref(mf, …)`) but **not the output** — the map starts empty and both keys come from the same value. | ticket item 1 ("the test must pin the whole rule"); WORKFLOW §8 "every new test fails without the change"; §5.1 "a test that cannot fail" = MAJOR | TBD | **CONFIRMED — manager built both mutants and ran them.** (a) Rounds-guard mutant (`!rounds(from)` removed, one-line diff verified): running the whole program on HEAD vs the mutant differs on **exactly one line, `float conversion split`** — the `float64 + float64` row prints `map[2:[7]]` on both, and Go prints `map[2:[7]]` too. (b) Float-`+`-rejecting mutant (`sameSafe`'s BinaryExpr rejecting `rounds(ta.Type)`): the whole program's output is **byte-identical** to HEAD, so nothing in the program pins float `+` at all. **Manager correction of the suggested fix.** The reviewer's primary proposal (`hold := map[float64][]int{2:{1}, 6:{99}}; hold[kf64+1] = append(hold[float64(kf64)+1], change())`) **does not work** — I built it and ran it: HEAD, mutant and Go all print `[1 7] [99]`, because HEAD already emits `gp_t0 = kf64+1.0; gp_t1 = kf64+1.0; gp_t2 = change()` and the slow path reads both keys before the call as well. Its *secondary* proposal does work, but pins a different rule: `hold[kf64] = append(hold[float64(kf64)], change())` gives HEAD `[1] [1 7]`, mutant `[1] [99 7]`, Go `[1] [1 7]` — that is the conversion rule (already covered by the live `float conversion split` row), **not** float `+`. I tried eight further float-`+` shapes (seeded/empty, `setkf` vs `change`, conversion on either side, `+0`) and **could not construct one that distinguishes**, which suggests the row may be live-able only by a row that pins the `+` *predicate* through `emit`/a unit test rather than through printed output. FIX: make the row live, or pin the float-`+` acceptance in `maps_test.go` as a `TestMapAppendShape` case and note in the diff program that the printed row pins the *output* only — as `dev.evalorder`'s R1 fix already did for the two rows it documented as non-strengthenable. | **FIXED-PENDING R3** — fix commit `17fc288d`, route 1 (unit test) plus the documenting comment. **Manager rebuilt both mutants on this commit and ran the new rows itself, not trusting the handoff:** at HEAD `TestMapAppendShape` passes; with the rounds guard removed (**mutant A**) exactly one row fails — `float conversion: mapAppend = true, want false`; with `sameSafe`'s `BinaryExpr` rejecting `rounds(ta.Type)` (**mutant B**) exactly one row fails — `float addition: mapAppend = false, want true`. Different rows, so the pair covers both halves of the float rule, and neither row is vacuous. The coder's route-2-impossible argument checks out against the emitted C++ I saw in both mutant failures: the key is a `BinaryExpr`, so both paths materialise it (`gp_t0 = fl + 1.0; gp_t1 = fl + 1.0`) **before** the appended argument — hence no printed map can distinguish. Route 1 was the only live option, and it is live. **R3 `FIX-VERIFIED` independently** — it rebuilt both mutants from HEAD source and got the same two distinct failing rows, and additionally confirmed gc agrees with both `want` values. | `CLOSED-FIXED` |
| E7 | R2 | NIT | `internal/lower/maps.go:837-838` (the `convNopExpr` doc comment) | The parenthetical is attached to "except where assignOp's own rules make it one", so it presents "rule 2, **a non-empty interface to another with the same methods**, recomputes the itab" as an example of an `OCONVNOP`. That case is precisely where gc does **not** produce one — which is what the check two lines below returns false for, and why E2 exists. A reader implementing from the comment would re-open E2. | WORKFLOW §8 comment accuracy | TBD | **CONFIRMED — manager read gc's own comment.** `$(go env GOROOT)/src/cmd/compile/internal/typecheck/subr.go:269-276`: rule 2 gives a nop only for "either src or dst is not a named type, or both are empty interface types", followed by "For assignable but different non-empty interface types, we want to recompute the itab". So the prose says the opposite of the code under it. FIX: "(rule 1, identical types; and rule 2 only when the source is an empty interface — a non-empty interface to another with the same methods recomputes the itab, so it is not one)". **R3 manager re-check of the fix:** the new sentence now reads "rule 1, identical types, and rule 2 when the source is an empty interface (a non-empty interface to another with the same methods recomputes the itab, so it is not one)", which is rule 2's `if src.IsEmptyInterface()` branch verbatim, and the sentence directly below it ("gc's assignOp … rule 1, and rule 2 when the source is an empty interface") states the same condition as the code it annotates. **R3 `FIX-VERIFIED`** — it read `subr.go:264-296` and checked the predicate against gc on the three interface cases that matter (`E` source, `N interface{ E }` source, same-method `I`): gc and gocpp agree on all three. | `CLOSED-FIXED` |
| R2 fix | (merging coder 3) | `17fc288d` (3 files, +9/−2) | **Manager re-ran independently:** `gofmt -l` empty, `vet` clean; `TestMapAppendShape` green at HEAD and red under both mutants (rows as above); E7's wording matches gc's `assignOp` rule 2 branch at `subr.go:274`; the diff program's added comment is 3 lines, max 94 columns. **Manager correction of the coder's own out-of-scope note:** it reported `tests/diff/maps_append_key_expr.gp:190` (102 columns) as "pre-existing, unrelated to this job" — but `git log --diff-filter=A` shows **this job created that file** (`741c2633`) and the line was 102 columns at its creation, so it is not pre-existing on `dev`. It is still **not a finding**: `.gp` test files are not held to `.clang-format`'s `ColumnLimit: 100` (which applies to C++), 84 lines across `tests/diff/*.gp` exceed 100 columns, and the sibling `maps_append_key.gp` carries a 109-column line. Recorded because a wrong "pre-existing" label is how an in-job defect escapes a later round. | awaiting a fresh R3 reviewer |

| R3 | fresh reviewer | `17fc288d` | reviewer: focused suite green (`internal/lower` ok 10.914s 406 passed, `TestGolden` 41+1, `TestDiff/maps_append_key_expr` 3 passed; `-cxx clang++` ok 8.240s); `gp-repro` on the whole 24-row program SAME vs Go; compared `sameSafe` against gc on 62 key shapes with **zero mismatches**; re-measured the E6 mutants and confirmed both rows live; verified `maps.go`'s only delta from `20cdd28e` is the E7 comment (comment-only) | **FINDINGS (1 NIT)** — E6/E7 `FIX-VERIFIED`, one new finding, **partly confirmed, its decisive evidence fabricated** |
| R3 fix | (merging coder 3) | `344cb538` | manager on the fix: `git diff --stat 17fc288d..HEAD` → `maps.go \| 4 ++--` (2 insertions, 2 deletions, both `//` lines); comment-only proved with my own `go/parser` strip tool built as a binary — 47820 B and 47847 B strip to **851 identical lines**. **A first attempt printed a false "identical" from two empty outputs** (the tool failed to compile with the two `.go` files in one directory); recorded because that is exactly the failure mode a reviewer must not repeat. Both claims the finding rested on re-checked: `assignOp`'s SimType attribution gone, "integer" kept. The coder's focused run green (`internal/lower` 406 passed, `TestGolden` 41+1, `TestDiff/maps_append_key_expr` 3 passed) | awaiting a fresh R4 reviewer |
| R4 | fresh reviewer #4 | `344cb538` | reviewer: comment-only re-proved with **its own** `go build`'d stripper (851 non-empty lines each side, byte-identical) **plus a negative control** — the same tool reports `DIFFERS` on `4444578e`↔`023cea66`, so the identity is trustworthy; `scripts/gp-focused -x` green (`internal/lower` 406 passed, `TestGolden` 41+1, `TestDiff/maps_append_key_expr` 3), `-cxx clang++` ok; `gp-repro` on the 24-row program SAME; E1/E2/E3 liveness re-measured through overlays of each pre-fix commit; E6 mutants rebuilt and both rows distinct; **built its own gc oracle** — ~950 generated key-shape pairs across 18 types, classifying gc fast/slow from `-gcflags=-S` mapaccess/mapassign counts — **0 mismatches** except one class (E9). **E1/E2/E3/E6/E7/E8 all `FIX-VERIFIED`**, E4 `CLOSED`, E5 out of scope. **It independently reproduced the manager's R3 refutation:** `grep -rn '\bFAST\b\|\bSLOW\b'` over gc's `typecheck/` and `walk/` → 0 hits, its `-W` output has no such vocabulary, so that evidence was fabricated and refusing the reviewer's replacement sentence was right | **FINDINGS (1 MINOR, 2 NIT)** — round 4, not CLEAN |
| E9 | R4 | MINOR | `internal/lower/maps.go:721-723` (`sameSafe`'s `*ast.IndexExpr` case) | gc typechecks `x[i]` through `implicitstar`, wrapping a pointer-to-array operand in an implicit `ODEREF`, so `SameSafeExpr` sees the same chain for `parr[i]` and `(*parr)[i]`. gocpp compares source nodes, so a `StarExpr` base and an `Ident` base never match: it declines the fast path and takes the slow one, reading the key **after** the appended argument — the very reordering this ticket exists to prevent. | WORKFLOW §8 Go behaviour / the ticket's "check the whole set against `ir/expr.go:1040`", read with the diff's own doc claim at `maps.go:675` that `x[i]` is covered | TBD | **CONFIRMED — manager reproduced the divergence and gc's decision independently.** `scripts/gp-repro` on the reviewer's shape → **DIFFERENT**: gocpp prints `map[5:[7]] map[5:[7]]`, Go prints `map[1:[7]] map[5:[7]]` — the first row, the mixed-spelling one. I then built the same program for gc with `//go:noinline` on the mutator and counted: **0** `CALL runtime.mapaccess`, **2** `CALL runtime.mapassign` — so gc takes the **fast** path, exactly as the reviewer said, and gocpp is the one that differs. Pre-existing (the reviewer confirmed `dev`'s `maps.go` prints the same wrong answer via overlay), but it is the predicate this job changed, and the job's own doc comment claims `x[i]` is covered. FIX: normalize a pointer-to-array base to its implicit deref before comparing, with a unit row and a diff-program row that both fail on `344cb538`. | OPEN |
| E9 fix | (merging coder) | `b3b7154c` | **Manager verified E9 independently, three ways.** (1) Negative unit check: `344cb538`'s `maps.go` substituted into the **current** test file by overlay → exactly the two intended rows fail (`star on the left: mapAppend = false, want true`, same for the right); the "different indexes" over-match guard passes both ways. (2) `scripts/gp-repro tests/diff/maps_append_key_expr.gp` → **SAME**, and I checked it is **not vacuous**: 26 lines each side, both new rows present, both print `map[1:[7]]`. (3) Negative end-to-end: the same fixture against the old `maps.go` by overlay → **DIFFERENT** on exactly those two rows (gocpp `map[5:[7]]` vs Go `map[1:[7]]`), so the fixture is load-bearing. Added lines all ≤ 100 cols; gofmt empty; vet clean. | `E9 FIX-VERIFIED` |
| E10 fix | (merging coder) | `b3b7154c` | **The fix replaced one false claim with another — found by the manager, not a reviewer.** New text: "gc's convertOp tests Sym() == nil in rules 2 and 4". I read Go 1.25.7 `typecheck/subr.go`: **convertOp's rules 2 and 4 contain no `Sym()` test at all** (rule 2 is `IdenticalIgnoreTags`, rule 4 is the numeric rule). convertOp's only `Sym() == nil` is **rule 3**, the unnamed-pointer rule — one of the two rules `isUnnamed` actually feeds (`:889`); the other two sites are `assignOp` rules 2 and 4, reached via convertOp's own rule 1. The same paragraph also said basic-type conversions are decided "before it reaches those tests", but convertOp rule 4 (numeric) comes **after** rule 3. So E10 recurred inside the fix for E10 — the third round on this file with the same failure mode as probecached's P7→P11. | `FIXED-PENDING R5` — sent back with clause-by-clause replacement text, every fact pre-verified (`defBasic` does `typ.obj = obj`, `universe.go:64-71`; `isUnnamed` has exactly two call sites, `:889` and `:898`, so a basic type cannot arrive) |
| E16 | R5 fix (manager) | NIT | `internal/lower/maps.go:923-924` (`isUnnamed` doc, final sentence) | **FOUND BY THE MANAGER in the text the manager itself prescribed.** "a basic-type conversion is decided by gc's numeric rule or the identical-underlying check, never by these two" is a universal and false: `int -> string` is **convertOp rule 6** (`ORUNESTR`, `subr.go:439-442`) and `[]byte -> string` is rule 6 too (`OBYTES2STR`) — neither rule 4 nor rule 2. I wrote the exact defect class I had just sent the coder back for, one message earlier. | WORKFLOW §8 comment accuracy | TBD | **CONFIRMED by the manager.** The clause also explains the wrong mechanism: the divergence never decides because **neither side's guard admits a basic type at that test** — gocpp calls `isUnnamed` only after the `*types.Pointer`/`*types.Chan` assertions (`:889`, `:896`), and gc's `Sym()` tests sit behind `src.IsPtr() && dst.IsPtr()` (convertOp rule 3) and `src.IsChan() && dst.IsChan()` (assignOp rule 4). FIX sent: replace only that final sentence with the guard-based explanation. | `FIXED-PENDING R5` |
| E10 fix | (merging coder) | `cddb4253` | Amend of `b3b7154c`, `isUnnamed` doc paragraph only. **Manager verified:** zero non-`//` changed lines across `b3b7154c..cddb4253`, no line over 100 cols, gofmt empty, text is the prescribed wording. The citation defect is closed — the coder re-derived gc's numbering from source and reproduced my finding (`convertOp` rule 3 at `:410` is the only `Sym()` test there; `assignOp` rules 2 and 4 at `:283`/`:334`; `convertOp` reaches `assignOp` via its own rule 1 at `:389`). | `FIX-VERIFIED` (citation half); see E16 for the residual sentence |
| E16 fix | (merging coder) | `ca29ca14` | Amend of `cddb4253`, final sentence of `isUnnamed`'s doc only. **Manager verified:** zero non-`//` changed lines across the amend, no line over 100 columns, and I re-checked **both guard claims in the new sentence against gc's own source** — `convertOp` rule 3 is `src.IsPtr() && dst.IsPtr() && src.Sym() == nil && dst.Sym() == nil` (`subr.go:410`), `assignOp` rule 4 is `src.IsChan() && src.ChanDir() == types.Cboth && dst.IsChan()` with the `Sym()` test inside (`subr.go:333-334`) — and both gocpp call sites (`:888-889` pointer, `:895-897` channel) assert the type before calling `isUnnamed`. Every clause of the paragraph now holds. | `FIX-VERIFIED` |
| R5 | fresh reviewer #5 | `ca29ca14` | brief asked specifically for an adversarial probe of `indexBase`/`isPtrToArray` (pointer-to-**slice**, `**[N]T`, defined array types, non-deref `StarExpr`, differing index spellings) with any "gocpp fast / gc slow" case to be filed as **BLOCKER**, and told the reviewer the `isUnnamed` paragraph is manager-authored and has already been wrong once. Two vacuous-pass traps restated. Reviewer: gofmt clean, `go vet` clean, `gp-focused -x -cpus 6 -mem 6G -base 4444578e` → `internal/lower` **406 passed**, `TestGolden` **41 + 1 skipped**, `TestDiff/maps_append_key_expr` **3 passed**; same counts under `clang++`; no collateral damage on the five neighbouring `TestDiff/maps_*` rows. Fixture proved load-bearing three ways (overlay of `344cb538`'s `maps.go` → differs on exactly the two new rows; overlay of `dev`'s → 24 of 26 rows differ; unit overlay → exactly the two intended rows fail). **BLOCKER hunt came back empty:** its own gc oracle (`-gcflags='-N -S'`, counting `CALL runtime.mapaccess`/`mapassign`) over **1323 compared pairs** (22 base expressions × 22 × 3 index spellings) with **zero** gocpp-FAST/gc-SLOW cases, plus `**[N]T` nesting, key-type sweep, compound bases, conversion/field-chain grids and end-to-end `gp-repro` with rebinding mutators. Pointer-to-slice proved **unexpressible** in Go itself. `isUnnamed` paragraph checked clause by clause against Go 1.25.7 and holds (both the `:889`/`:898` call-site guards and gc's `convertOp` rule 3 / `assignOp` rule 4 guards). | **CLEAN** — the job's exit condition |
| R5 fix | — | — | none needed; CLEAN ends the job per WORKFLOW.md:182 | **CLOSED** |
| E9 adversarial | (manager) | `ca29ca14` | Probing the over-match risk of `indexBase` — the only way E9 could become a **BLOCKER** (gocpp fast where gc is slow ⇒ wrong order). **Pointer-to-slice is unexpressible:** `psl[0]` on a `*[]int` is rejected by Go itself ("cannot index psl (variable of type *[]int)"), so `isPtrToArray`'s guard excludes a case that cannot be written. Cases run through `scripts/gp-repro` against real Go, **3 non-empty lines each side, SAME**: defined array type behind the pointer (`MyArr`) → `map[1:[7]]`; `**[2]int` (`(**ppa)[0]` vs `(*parr)[0]`) → `map[5:[7]]` on **both** (correctly slow: `indexBase` strips one star each side, leaving `(*ppa)` vs `parr`, which differ); `(*st.b)[0]` vs `st.b[0]` through a struct field → `map[1:[7]]`. The asymmetry that must stay slow (`parr[0]` vs `arr[0]`) is already a fixture row and still slow. No case found where gocpp goes fast and gc does not. | `no over-match found` |
| E10 | R4 | NIT | `internal/lower/maps.go:892-894` (`isUnnamed`'s doc comment) | "gc's `Sym() == nil` (a basic type, …)" is false: gc's predeclared basic types **do** carry a `Sym` (`defTypeName` sets `typ.obj`). `Sym() == nil` is true only for compound types built with no object. | WORKFLOW §8 comment accuracy | TBD | **CONFIRMED — manager checked gc's `universe.go`.** `defBasic` does `typ.obj = defTypeName(...)`, so `Types[TINT].Sym()` is the `int` symbol, not nil. The reviewer also correctly says the **code is right anyway** (routes 3 and assignOp rule 4 need conditions a basic type cannot satisfy), which its 950-case oracle confirms. Comment-only fix. | OPEN |
| E11 | R4 | NIT | `.scratch/` at the worktree root | Untracked, not in `.gitignore` (`git check-ignore` exits 1), so every round recreates it. Cannot merge. | WORKFLOW §5.2 hygiene (pre-merge manager item) | TBD | **CONFIRMED, and it stays a manager item, not a coder one.** E4 was closed on exactly this basis. Whether to add `.scratch/` to `.gitignore` is a repo-policy question wider than this job; I am not deciding it inside a review round. **Manager clears `.scratch` before the merge**, as §5.2 requires. | `CLOSED` (manager action) |
| E8 | R3 | NIT | `internal/lower/maps.go:823` (the `convNopExpr` doc comment, one clause) | "convNop (panicid.go) covers the numeric rules (**convertOp's rule 4, which is assignOp's integer SimType rule**)" misattributes the rule: `assignOp` has **no** SimType rule at all, and `convertOp`'s rule 4 is not integer-only. | WORKFLOW §8 comment accuracy | TBD | **PARTLY CONFIRMED — the clause is wrong, but the reviewer's evidence for it is fabricated and its suggested replacement rests on the same false premise.** What I measured: `grep -rn '\bFAST\b\|\bSLOW\b' $(go env GOROOT)/src/cmd/compile/internal/typecheck/ .../walk/` → **0 hits**, so gc's `-W` has no FAST/SLOW vocabulary; the reviewer's "`gcflags=-W` gives **FAST** for `m[float64(f32)]`" and "**SLOW** for `m[float64(fl)]`" cannot have come from any command. I ran its exact cases: `float64(f32)` and `float64(fl)` both print `CONV float64` in the IR, and `INDEXMAP Assigned` appears identically for the fast and slow map shapes, so `-W` does not distinguish them either. `SimType` occurs **only** in `convertOp` (rules 4 and 5, `subr.go:416-430`); `assignOp` (`:256-358`) never mentions it — so "assignOp's … SimType rule" is wrong, and that much is a genuine defect **in text this job added** (`git log -S"assignOp's integer SimType rule"` → `741c2633`; not on `dev`). **But "integer" is not a defect:** rule 4's float half (`src.IsFloat() && dst.IsFloat() && same SimType`) can never survive `tcConv`, which demotes every same-kind float/complex conversion to `OCONV` ("Floating point casts imply rounding and so the conversion must be kept", `expr.go:364-372`) — so `convNop`'s integer-only branch loses nothing, and the reviewer's "a reader would add a `rounds` check in the wrong place" is speculation against correct code. FIX: drop the false attribution only — "convertOp's rule 4, for the integers (a same-kind float conversion never survives `tcConv`)". **Do not adopt the reviewer's suggested sentence**, which says the float case "fall[s] to the identical-underlying branch below" as though that were a live path. | **FIXED-PENDING R4** — fix commit `344cb538`, the manager's wording used verbatim: "numeric rule (convertOp's rule 4, for the integers: a same-kind float conversion never survives tcConv)". **Manager verified it is comment-only with a real build, after discarding a vacuous first attempt:** my first strip run compiled both `.go` files into one directory, the build failed on mixed packages, and `cmp` of two **empty** outputs printed a misleading "identical" — I rebuilt the tool as a standalone binary (`go build`) and re-ran: `before.go` 47820 B and `head.go` 47847 B strip to **851 identical lines** each. `git diff --stat 17fc288d..HEAD` → `maps.go | 4 ++--`, 2 insertions 2 deletions, both `//` lines. `assignOp`'s SimType claim is gone and "integer" survives. | awaiting a fresh R4 reviewer |

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
| R2 fix | (merging coder 2) | `f8d02388` | manager on the fix, all three environments run itself: default `ok 2.491s`, `NIX_CFLAGS_COMPILE="-O2 -march=native"` `ok 2.557s`, surgical (`-O2 -isystem /usr/include -march=native`) `ok 2.659s`; **load-bearing re-proved in the native env**: mutating the `cache` row to `"off"` now **FAILS** (`run 2: cached false, want true`) where the same mutation on `ac9c8975` **PASSES** (`ok 3.179s`) — that pass is the lost coverage F1 named, so the regression is closed and measured. `git status` clean after each | awaiting a fresh R3 reviewer |
| R3 | fresh reviewer | `f8d02388` | reviewer: P1 re-measured in three environments (`ok 2.468s` / `2.426s` / `2.640s`) plus the ticket repro, all green; F1 and F2 `FIX-VERIFIED`; load-bearing of the scrub proved by an overlay disabling only its body; 681 passed 2 skipped in `scripts/gp-focused -x`; `-count=5`, `-race`, `-shuffle=on` stable; disclosed it briefly copied a scratch test into the package and removed it within the same minute, hash never differed from a committed state, so the round is not void. Manager verified findings 1 and 3 by reproducing them, and finding 2 by counting the failures itself | **FINDINGS (3 MINOR)** — all 3 confirmed, all environment-triggered, none reachable by `make ci` |
| F1 | R2 | MINOR | `internal/typedinterop/probe/decls_more_test.go:21-23` | The `ac9c8975` comment and commit message say "the environment can ask for that (build.KeyEnv) **and a test can't take it back**". That is false, and the fix pays for it with real coverage: in a native environment `hostCPU` is true, so the `cache` subtest asserts nothing and is byte-identical to `no_cache`. A test can scrub it — `t.Setenv(name, "")` for each `build.KeyEnv()` entry with `build.NativeArch`, before the runs — and then asserts `tc.cached && i == 1` unconditionally. | Ticket "Done when … without weakening what it proves"; WORKFLOW §8 "every new test fails without the change"; §5.1 MINOR | TBD | **CONFIRMED — manager proved it with its own test, not by reading the reviewer's.** `zz_f1_test.go` (overlay, outside the worktree) under `NIX_CFLAGS_COMPILE="-O2 -march=native"`: it scrubs every `NativeArch` `KeyEnv` entry, then `build.NativeArch(build.KeyEnv()...)` is **false** and the two runs give `run 1: cached=false`, `run 2: cached=true` — the assertion stays live and the test passes (`ok 0.933s`). So the premise in the comment is wrong and the unconditional form is reachable in exactly the environment P1 was filed about. FIX: scrub the native `KeyEnv` entries in the subtest, drop `hostCPU` from the expectation. | FIX (`f8d02388`) | **FIXED-PENDING R3.** `scrubNativeArch` drops only the native-arch *options* from each `build.KeyEnv` variable, keeping the rest (it filters `strings.Fields` tokens rather than blanking the variable, so a Nix `-isystem` path survives). `hostCPU` and the copied `diskFor` predicate are deleted; the assertion is `tc.cached && i == 1`. Manager measured the whole point: in the native environment the mutated-folder check **fails on the fix** and **passes on `ac9c8975`** — the lost coverage is restored, not merely argued. | `FIXED-PENDING` |
| F2 | R2 | MINOR | `internal/typedinterop/probe/decls_more_test.go:22,24` | The copied predicate restates two of `diskFor`'s three refusal reasons and omits `units.go:497`'s `selfID() == ""`, while the comment claims "the rows take their expectation from the same rule". With `dir` set and `selfID()==""`, `diskFor` returns `("", false)`, both runs are cold, and the test fails with the misleading `run 2: cached false, want true`. | WORKFLOW §5.1 MINOR / §8 comment accuracy | TBD | **CONFIRMED — manager reproduced with an overlay stubbing `selfID`.** `zz_f2_test.go` sets `selfID = func() string { return "" }`, `hostCPU=false`, and both runs report `cached=false` while the test's own rule wants `true` for run 2 — it would fail on a third reason it never consults. Note the reviewer's own drift experiment cuts the other way and is worth keeping: deleting `diskFor`'s `NativeArch` line makes the current test fail loudly, so the copy is not a silent second source of truth in that direction; calling `diskFor` directly instead was tested and is worse (tautological). FIX: fixed by F1's shape — with the scrub and an unconditional expectation there is no copied predicate left to be unfaithful. | FIX (`f8d02388`) | **FIXED-PENDING R3.** The copy is gone: no `hostCPU`, and the doc comment no longer restates `diskFor`'s rule (it now says only "the second runner is then served from it"). The one remaining `diskFor` mention is in `scrubNativeArch`'s comment and claims exactly one thing — that native-arch is a reason results stay in memory — which is true (`units.go:500`). Manager accepts the coder's reasoned limit: **`selfID() == ""` is still unconsulted, but nothing is consulted now**, so the test no longer claims to follow `diskFor`'s rule; a stubbed `selfID` makes both runs cold and the test fails with `run 2: cached false, want true`. **Manager correction (R3):** the first version of this cell claimed the failure reads "the disk cache is disabled"; no such message exists anywhere in `internal/typedinterop/probe` (`grep -rn "disk cache is disabled"` → nothing) and the failure text is unchanged. The closure stands on F2's own text — no copied predicate, no false-fidelity claim — but the justification sentence was invented by the manager and is corrected here. That is a different (correct) outcome from F2's misleading `run 2: cached false, want true` under a false claim of fidelity. | `FIXED-PENDING` |
| R3 fix | (merging coder 2) | `614c8bf8` | manager re-ran every environment itself: `env 'NIX_CFLAGS_COMPILE-march=native'` → **PASS 2.59s** (was FAIL); `-isystem .../-march=native/inc` → PASS 2.73s with healthy tables; `NIX_CFLAGS_COMPILE="-O2 -march=native"` → `ok 2.631s`; `-nostdinc` → **fails loudly** with the new message ("the environment broke the compiler, the table proves nothing"), which is the vacuous-pass case now caught; cache assertion still load-bearing (an overlay with the `cache` row at `"off"` fails `run 2: cached false, want true`); `TestDecls|TestScrubNativeArch|TestDropNativeArch` `ok 28.685s`; gofmt empty, vet clean. **Manager checked the coder's central claim** — that no marker can sit in a name because every marker contains `=` — and it is right: `NativeArch` matches only `-march=native`/`-mtune=native`/`-mcpu=native`, all of which contain `=`, while a name is the bytes before the first `=`. So the ticket's "skip" was unreachable, and the straddle-and-empty shape is correct | awaiting a fresh R4 reviewer |
| R4 | fresh reviewer | `614c8bf8` | reviewer: `scripts/gp-focused -x -cpus 6 -mem 6G` **683 passed, 2 skipped, 0 FAIL**; `-count=5 -shuffle=on`, `-count=3` determinism and `-race` green; P4 re-ran under six environment shapes plus `scripts/in-devenv`, all `ok`; P6 proved load-bearing **both ways** by an overlay deleting the pair-drop (`the environment broke the compiler, the table proves nothing`) against the same overlay on `f8d02388` passing; both new tests fail without their fix; TODO:166's 25 names re-measured and exact. Manager verified the one finding by running `dropNativeArch` itself | **FINDINGS (1 NIT)** — confirmed |

| P4 | R3 | MINOR | `internal/typedinterop/probe/decls_more_test.go:71-73` | `scrubNativeArch` gates on the variable's **value** (`build.NativeArch(value)` after `strings.Cut`), but `diskFor` (`units.go:500`) calls `build.NativeArch(build.KeyEnv()...)` on the whole `NAME=value`. `keyEnv` matches by prefix (`NIX_CFLAGS_COMPILE*`, `compile.go:411`), so a variable *named* `NIX_CFLAGS_COMPILE-march=native` is in `KeyEnv`, `diskFor` refuses the folder, and the scrub skips it — then the now-unconditional assertion fails. The comment's "The host's CPU is then no longer a reason" is a universal the code does not meet. | WORKFLOW §8 comment accuracy / §5.1 MINOR — same class as F2 | TBD | **CONFIRMED — manager reproduced the exact command.** `env 'NIX_CFLAGS_COMPILE-march=native' go test ./internal/typedinterop/probe -run '^TestDeclsDeterministic$'` → `decls_more_test.go:47: run 2: cached false, want true`, FAIL. The mechanism is mechanical, not a guess: `keyEnv` is `strings.HasPrefix(name, "NIX_CFLAGS_COMPILE")`, and `NativeArch` is a `strings.Contains` over each part, so the name alone trips `diskFor`. Trigger name is contrived (no real shell sets it), but the finding is correct and cheap to fix. FIX: gate on the whole `kv` and scrub the whole entry, or word the comment to what the code does. `FIX` in `614c8bf8` — gates on the whole `kv`, and empties the entry when the marker straddles the `=`. | **FIX-VERIFIED by R4** independently: the pre-fix file `f8d02388` fails `env 'NIX_CFLAGS_COMPILE-march=native' …` with `run 2: cached false, want true` and `614c8bf8` passes, plus five more environment shapes and a run inside `scripts/in-devenv`. | `CLOSED-FIXED` |
| P5 | R3 | MINOR | `plans/2026-10-13-gocpp/TODO.md:166` | The "Half done" note says two tests are affected; under `NIX_CFLAGS_COMPILE="-O2 -march=native"` the whole cache family fails, so "the remaining half of this line" understates the job's real scope by an order of magnitude. | WORKFLOW §8 "a claim marked fixed but still reproducible is a finding", read with "an older issue in the same area must be reported" | — | **CONFIRMED — manager counted them itself: 25 failures** in `-run 'TestCache|TestLocate'` (the reviewer listed 25 by name and said 24 in prose; my own `grep -c '^--- FAIL'` gives **25**), plus a `TestPCH` group. **Pre-existing on `dev`**: the manager ran two of them at `/home/huuhait/GoToCPP` (`4444578e`) and got the same failures, so this is not caused by the diff. It is a TODO-accuracy defect the manager introduced with the "Half done" wording. **Fixed by the manager** (TODO line rewritten — see the note below). | CLOSED |
| P6 | R3 | MINOR | `internal/typedinterop/probe/decls_more_test.go:76-82` | The scrub removes tokens positionally, so when the marker is an *argument* (`-isystem /path/-march=native/inc`) it deletes the path and leaves `-isystem` dangling; the probes then fail to find `cstddef`, both runs produce identical all-error tables, and the determinism half of the test **passes** on two degenerate tables. `strings.Contains(out[0], "\"x\"")` does not catch it because the two surviving records still carry `"x"`. | WORKFLOW §8 "every new test fails without the change" / §5.1 MINOR | TBD | **CONFIRMED — manager reproduced and instrumented it.** The reviewer's command passes (`ok 0.635s`), and a manager overlay mirroring the real test order under the same environment prints `run 1: cached=false runs=5 records=2 headerErrors=5` / `run 2: cached=true runs=0 records=2 headerErrors=5`, with all five errors `'cstddef' file not found`. So the comparison is `5 error entries vs the same 5` and the test is green while measuring nothing. FIX: drop the option together with its argument when the argument matched, and assert the probes actually produced records (e.g. `len(tab.HeaderErrors) == 0`) so a broken environment cannot pass as determinism. `FIX` in `614c8bf8` — `dropNativeArch` with a 23-entry `nextWordOpts` list, plus a `HeaderErrors`/`Errors`/`Records` assertion on every run. | **FIX-VERIFIED by R4** independently, and by the sharpest test available: with the pair-drop deleted by an overlay and `NIX_CFLAGS_COMPILE=-isystem /tmp/x/-march=native/inc`, the fix **FAILS loudly** with the new message while the same overlay on `f8d02388` **passes** — the vacuous pass P6 described, closed and measured. `-nostdinc` also fails loudly; no false positive found in the real Nix dev shell, `-Werror`, `-pedantic` or `-std=c++98`. | `CLOSED-FIXED` |
| P7 | R4 | NIT | `internal/typedinterop/probe/decls_more_test.go:78-79` (the `scrubNativeArch` doc comment) | The comment says the scrub keeps "the rest … such as the -isystem folders a Nix shell needs", but for the three colon-separated `KeyEnv` variables (`CPATH`, `CPLUS_INCLUDE_PATH`, `C_INCLUDE_PATH`, `compile.go:404`) the whole value is **one whitespace word**, so a marker anywhere in it deletes every path, not the marker-bearing element. End-to-end benign — the reviewer measured `CPATH=<needed>:/tmp/x/-march=native/inc` through the dev shell green with healthy tables — so this is comment accuracy only. | WORKFLOW §8 comment accuracy, read with the F2/P4 precedent (a stated universal the code does not meet) | TBD | **CONFIRMED — manager ran `dropNativeArch` itself.** `"/needed/include:/tmp/-march=native/inc"` → `""` and `"/a:/b/-mtune=native/c:/d"` → `""`, while the advertised `"-O2 -isystem /tmp/x/-march=native/inc"` → `"-O2"` keeps its rest. So the quantifier is wider than the code, exactly as reported. **Manager chose the comment fix over the code fix:** teaching `dropNativeArch` about `:` needs to know *which* variables are colon-separated (the `NIX_CFLAGS_COMPILE` family is space-separated, the three path variables are not), which is new special-casing in a test helper at the job's round limit for an effect nobody can observe. The honest cheap fix is to say the value is split on spaces, so a colon-separated list is one word, and that emptying `CPATH` cannot hide a broken environment because the new `HeaderErrors` assertion fails loudly. FIX carried out in `d11998e3` (comment-only, widths 87-98). | **FIXED-PENDING → but R5 shows the fix text still carries the defect in a narrower form; see P8.** R5 re-judged P7 from scratch and found the *new* sentence repeats a false universal ("split on spaces … one word … empties the whole list") that `strings.Fields` does not honour. So P7's fix is incomplete rather than wrong, and P8 is where it is recorded. | `FIXED-PENDING` |
| P8 | R5 | NIT | `internal/typedinterop/probe/decls_more_test.go:79-81` | The P7 fix says the value "is split on spaces, so a colon-separated path list (CPATH, CPLUS_INCLUDE_PATH, C_INCLUDE_PATH) is one word, and a marker anywhere in it empties the whole list, folders and all". False whenever a folder in the list holds white space. | WORKFLOW §8 comment accuracy / §5.1 NIT | TBD | **CONFIRMED — manager ran the real function, not the reviewer's excerpt.** `dropNativeArch` on `"/home/u/My Docs/include:/tmp/-march=native/inc"` → **`"/home/u/My"`** (a truncated non-empty prefix, not `""`); on `"/needed\t/tmp/-march=native/inc"` → **`"/needed"`** (the code uses `strings.Fields`, which splits on **all** white space, so "split on spaces" is wrong twice over); the documented no-space case `"/needed/include:/tmp/-march=native/inc"` → `""` is correct. Downstream is still loud — an emptied or mangled `CPATH` surfaces in `HeaderErrors` — so this is comment accuracy, NIT not MINOR. FIX: say "split on white space (`strings.Fields`)" and that a colon list is one word only when its folders hold none; otherwise it is truncated at the first one, which the `HeaderErrors` guard catches. | OPEN |
| P9 | R5 | NIT | `internal/typedinterop/probe/decls_more_test.go:78-79` | "the words that ask removed and the other words kept, such as the -isystem folders a Nix shell needs" — the option word is **not** kept when its argument is the marker. | WORKFLOW §8 comment accuracy / §5.1 NIT | TBD | **CONFIRMED — manager ran it.** `dropNativeArch("-O2 -isystem /tmp/x/-march=native/inc")` → **`"-O2"`**: `-isystem` is dropped *with* its argument, which is exactly what `nextWordOpts` (lines 108-113) exists for. An ordinary `-isystem` folder is kept (line 165 asserts it), so the comment as written invites a reader to conclude a dangling `-isystem` could survive the scrub — the breakage P6 was filed for. FIX: append "and the option before such an argument with it (see `dropNativeArch`)". | OPEN |
| R5 | fresh reviewer | `d11998e3` | reviewer: comment-strip diff of HEAD vs `614c8bf8` **identical** (round genuinely comment-only); `scripts/gp-focused -x -cpus 6 -mem 6G -base 4444578e` → **683 passed, 2 skipped, 0 FAIL** (378 s); ticket repro green with and without `GOCPP_NOPROBECACHE`; pre-fix failure reproduced by overlay (only `"cached":true` differs); proved the scrub load-bearing by neutralising it under the **arch-scoped** Nix variable `NIX_CFLAGS_COMPILE_x86_64_unknown_linux_gnu`; ~25 hostile-but-plausible environments and 14 adversarial `KeyEnv` shapes with no false positive; `HeaderErrors` claim proved empirically. Reported 2 NITs, both in the one comment block the fix added | **FINDINGS (2 NIT)** — both confirmed by the manager running `dropNativeArch` |
| R5 fix | (merging coder 2) | `69e5d819` | coder rewrote the P8/P9 sentences; manager re-measured **all eleven** of its `dropNativeArch` values through an overlay test (they reproduce exactly, including both kept-`-isystem` cases) and confirmed comments-only by the coder's own strip tool reporting the same 12470 stripped bytes. **But the manager then falsified the fix's own new universal** — see P10 — so this round is not closed by the coder's text. | not closed — P10 open |
| P10 fix | (merging coder 2) | `74f5e52f` | manager on the fix, both halves re-derived: code is the iterative tail pop, proved by an overlay probe of my own (`-isystem -include MARKER`, `-include -isystem MARKER`, `-isystem -isystem MARKER`, `-Xclang -isystem MARKER`, the three-option chain → all `""`; `-O2 -isystem -isystem /a/MARKER -O3` → `-O2 -O3`; `-O2 -isystem /nix/store/x/include` unchanged; `-Werror -O2 -isystem /a/MARKER` → `-Werror -O2`), and the negative check rebuilt from the parent's loop body **inside the current test file** (an old-file overlay would drop the new rows and pass vacuously) → exactly six rows fail, the two guard rows and eleven pre-existing rows pass. The coder's own report that its first proposed universal also failed (`"-isystem -isystem"` → `"-isystem -isystem"`) reproduces. Focused run green: 683 passed, 2 skipped | awaiting a fresh R6 reviewer. **This is round 6: past the 5-round ceiling. If it is not CLEAN the job splits** (see the note at the top of this file) |
| R6 | fresh reviewer #3 | `74f5e52f` | reviewer: `scripts/gp-focused -x -cpus 6 -mem 6G -base 4444578e` → **683 passed, 2 skipped, 0 FAIL**; re-derived P10's negative check by its own method — old single-pop body substituted into the **current** test file → exactly six rows fail, guards and pre-existing rows pass; brute-forced the scrub invariant over **4000** random combinations with **0 violations**; ten scrubbed shapes end to end through the real compiler; `-count=2 -shuffle=on` and `-race` green; shell permutations (`GOCPP_NOPROBECACHE`, `GOCPP_PROBE_CACHE=off`, relative path) green; `scripts/in-devenv` green. **P10 `FIX-VERIFIED`.** Reported 2 MINOR + 1 NIT, none in the code P10 fixed | **FINDINGS (2 MINOR, 1 NIT)** — round 6, not CLEAN |
| P13 | R6 | NIT | `plans/2026-10-13-gocpp/TODO.md:166` | "The other 24 + the `TestPCH` group are untouched" undercounts by one: the 25-name list in the same line does **not** contain `TestDeclsDeterministic`, so the remaining set is 25, not 24. | WORKFLOW §8 TODO.md accuracy / §5.1 "a stale number" | TBD | **CONFIRMED — manager parsed the line and re-ran the pattern.** The line's own list holds exactly **25** test names and `TestDeclsDeterministic` is not among them; `go test -list 'TestCache|TestLocate'` matches **0** `TestDeclsDeterministic`. So the "Half done" sentence's "other 24" contradicts the line's own 25 and should read 25. This one is mine (manager-written TODO text), not a coder's. **FIXED by the manager** — TODO.md:166 now reads "The other 25" (`grep -c` → 1). | `CLOSED-FIXED` (manager action, no reviewer needed) |
| P12 | R6 | MINOR | `internal/typedinterop/probe/decls_more_test.go:126-127` | "Only options before a dropped word are popped, so one at the end of the value stays as it is" is **true of the code**; the finding is that it stops short of the caveat, that the survivor can be a dangling option — the exact shape the neighbouring comments call out as breaking the compile. | WORKFLOW §8 comment accuracy / §5.1 MINOR | TBD | **CONFIRMED, at NIT rather than MINOR.** I measured the survivor: `dropNativeArch("-isystem -isystem")` → `"-isystem -isystem"` (the sentence is accurate), and `NIX_CFLAGS_COMPILE_x86_64_unknown_linux_gnu="-isystem"` never enters the loop at all because `build.NativeArch` is false for it, so the scrub cannot remove a bare trailing option by construction. I did **not** reproduce the reviewer's `hdr=10` break for `-include`/`-D` at the tail through the real test: `clang++ -isystem -fsyntax-only` on a `<cstddef>` include exits **0**, so a dangling `-isystem` is benign with this compiler. The gap is a missing caveat in a comment whose own neighbours describe the hazard, which is a wording nit, not wrong behaviour. FIX: fold the caveat into whichever sentence the P11 fix rewrites, rather than adding a sentence. | **FIXED-PENDING R7** — folded into the P11 paragraph as instructed: "an option with nothing after it is left where it is and may be left without its argument", with the measured example `"-isystem /tmp/x/-march=native/inc -O2 -isystem"` → `"-O2 -isystem"`, which I re-ran and confirmed. States the behaviour of the helper, claims nothing about the environment. | awaiting a fresh R7 reviewer |
| P11 | R6 | MINOR | `internal/typedinterop/probe/decls_more_test.go:123-124` (the `dropNativeArch` doc comment) | The comment states "a word that is the argument of the option before it takes that option with it" as a universal, but it is true only for options in `nextWordOpts`: `-target`, `-resource-dir`, `-arch` and `-stdlib` all take an argument and are not in the list. | WORKFLOW §8 comment accuracy, the F2/P4/P7/P9/P10 precedent | TBD | **CONFIRMED — manager ran `dropNativeArch` itself and reproduced all four values exactly:** `"-target /tmp/-march=native/x -O2"` → `"-target -O2"`, `"-resource-dir …"` → `"-resource-dir -O2"`, `"-arch …"` → `"-arch -O2"`, `"-stdlib …"` → `"-stdlib -O2"`, against `"-isystem …"` → `"-O2"`. **Manager correction of the severity rationale:** the reviewer justified MINOR by claiming the dropped value "reproduced a broken environment end to end (`hdr=5`, all `'stddef.h' file not found`)". I could not reproduce that with this clang++: `clang++ -resource-dir -O2 -fsyntax-only` on a file including `<cstddef>` exits **0**, and a bogus `-resource-dir /tmp/nonexistent` also exits 0, because the Nix wrapper supplies its own resource root. What *does* break is compiler-dependent: dangling `-target` and `-arch` exit 1 on **both** clang++ and g++, and `-resource-dir`/`-stdlib` exit 1 on **g++** only. So the finding stands as a false universal — MINOR is right — but the dramatic end-to-end evidence did not reproduce as described. FIX: the root cause of five rounds on this one comment is that it keeps stating universals over a fixed list. Say what the function does over `nextWordOpts` and stop there. | **FIXED-PENDING R7** — `31990c77` rewrites the doc comment to name `nextWordOpts` as "one list" and to give measured outcomes instead of a rule. **Manager re-ran all ten examples quoted in the new text** (plus the two P11 shapes) and every one reproduces exactly, including the negative case the comment exists to admit: `"-target /tmp/-march=native/x -O2"` → `"-target -O2"`. The coder also caught a universal in its own draft ("the loop consults nextWordOpts and nothing else" — false, the loop also calls `build.NativeArch`) and rewrote it before committing. | awaiting a fresh R7 reviewer |
| P10 | R5-fix (manager) | MINOR | `internal/typedinterop/probe/decls_more_test.go:79-81` and `:117-119` (`dropNativeArch`'s own comment) | The P8/P9 fix added "so no -isystem is left without a folder". **False**, and the defect it denies is the live one: the pair-drop pops **one** option, so when an option sits in the argument slot it becomes the dangling option. `dropNativeArch("-isystem -include /tmp/-march=native/inc")` → **`"-isystem"`**; `("-isystem -isystem /tmp/-march=native/inc")` → `"-isystem"`; `("-include -isystem /tmp/-march=native/inc")` → `"-include"`; `("-Xclang -isystem /tmp/-march=native/inc")` → `"-Xclang"`. A dangling `-isystem` is exactly the degenerate-environment shape **P6** was filed for: it can leave the probes compiling nothing and let two identical broken tables pass as determinism. So the fix's comment overstates the code *and* the code has the hole the comment denies. | WORKFLOW §8 (a stated universal the code does not meet — the F2/P4/P7 precedent) read with P6's own rule | TBD | **FOUND BY THE MANAGER while verifying the coder's fix; not raised by any reviewer, and it is in code this job wrote.** The same loose phrasing the coder flagged out of scope is at `:117-119` ("The words are split on spaces", `strings.Fields` splits on all white space) and `:141` ("the options around the dropped ones survive"). **FIX (both halves):** (a) code — pop options iteratively, so a chain of option-words before the dropped argument goes too: `for { if n == 0 or kept[n-1] is not an option: break; kept = kept[:n-1] }`. Manager simulated it: the four dangling cases above all become `""`, and every legitimate input is unchanged (`"-O2 -isystem /a/-march=native/x -O3"` → `"-O2 -O3"`, `"-O2 -isystem /nix/store/x/include"` → unchanged, `"-O2 -march=native -isystem /a -mcpu=native"` → `"-O2 -isystem /a"`, `"-D -march=native FOO"` → `"FOO"`, `"-isystem"` → `"-isystem"`). (b) comments — all three sites say white space and describe the chain. | **FIXED-PENDING R6** — fix commit `74f5e52f` (one file, +25/−7 over `69e5d819`). **Manager verified both halves independently.** (a) Code: the body is the iterative tail pop (`for n := len(kept); n > 0 && slices.Contains(nextWordOpts, kept[n-1]); n = len(kept)`). I built my own probe as an overlay and ran it: all four P10 shapes and the three-option chain give `""`, `"-O2 -isystem -isystem /a/-march=native/x -O3"` → `"-O2 -O3"`, and the legitimate inputs hold — including two I chose, not the coder's: `"-Werror -O2 -isystem /a/-march=native/x"` → `"-Werror -O2"` and `"-O2 -isystem /nix/store/x/include"` unchanged. (b) Negative check: I rebuilt the pre-fix body by substituting `69e5d819`'s `if n := …` block into the **current** test file (not an old-file overlay, which would drop the new rows and pass vacuously) and ran it — exactly **six** rows fail, the two guard rows and all eleven pre-existing rows pass. **The coder's own assessment that its first universal failed too** ("so no option is left without its argument": `"-isystem -isystem"` → `"-isystem -isystem"`) reproduces, and the comment now claims only what was measured. | awaiting a fresh R6 reviewer |
| P11+P12 fix | (merging coder 2) | `41d69d92` | amends `31990c77` into one commit over `74f5e52f`: +18/−11, **all `//` lines** — I confirmed `git diff -U0 HEAD~1 HEAD` has zero non-comment added lines, so this round is genuinely comment-only and no test can fail on it. **Manager re-measured every claim in the new `TestScrubNativeArch` paragraph** through my own overlay probe of the real `dropNativeArch`: the four cited values all reproduce exactly (`"-O2 -isystem /a/-march=native/x"`→`"-O2"`, `"-target /a/-march=native/x -O2"`→`"-target -O2"`, `"-isystem -include -Xclang /a/-march=native/x"`→`""`, `"-D FOO -isystem …"`→`"-D FOO"`), and **so do the two claims the coder did not cite** — the P12 caveat `"-isystem /tmp/x/-march=native/inc -O2 -isystem"`→`"-O2 -isystem"` (a trailing option stays and is left without its argument) and the doc comment's chain `"-isystem -include /tmp/-march=native/inc"`→`""`. gofmt empty, vet clean, no changed line over 100 columns. | **`FIX-VERIFIED` by the manager, awaiting R7** |
| R6 fix | (merging coder 2) | `41d69d92` | Covers P11 and P12 only. Two NITs the coder flagged itself and did not fix are now P14/P15 below; I sent both back rather than let a fresh reviewer spend a round on them. | not closed |
| P14 | R6 self-noticed | NIT | `internal/typedinterop/probe/decls_more_test.go:89-90` | Ragged line break left by an earlier edit: line 89 is **45 columns** and its sentence continues on line 90. Cosmetic, inside a block five rounds have already been spent on, so a fresh reviewer will report it. | WORKFLOW §5.1 NIT (style) | TBD | **CONFIRMED by the manager** — measured `:89` = 45 cols against 88–100 for its neighbours. FIX: rejoin and re-wrap the paragraph, words unchanged, every line ≤ 100. | `FIXED-PENDING R7` |
| P15 | R6 self-noticed | NIT | `internal/typedinterop/probe/decls_more_test.go:22-25` | "each row scrubs **those options** out of the variables of `build.KeyEnv`" — no antecedent for "those options" (the preceding clause names none), and the scrub removes *words*, not options. | WORKFLOW §8 comment accuracy | TBD | **CONFIRMED by the manager** — read the paragraph: the clause runs "the environment can, in a variable of `build.KeyEnv` (NIX_CFLAGS_COMPILE is one). So each row scrubs those options out…" with no option named. Clumsy rather than false (it sits just above `TestDeclsDeterministic`, not `dropNativeArch`, so it is outside the block P11/P12 live in). FIX: name the words, not the options. | `FIXED-PENDING R7` |
| P14+P15 fix | (merging coder 2) | `9bae04e4` | amends `41d69d92`. **Manager verified:** `git diff -U0 HEAD~1 HEAD` has zero non-comment added lines; no changed line over 100 columns; gofmt empty; vet clean; the 45-column ragged break at `:89` is gone. The coder's structural claim reproduces exactly — I diffed the file's **word stream** (all `//` tokens removed) across `41d69d92..9bae04e4` and got **exactly one** non-equal opcode for the whole file: `those options` → `the words that ask for the host's CPU`; the other ~40 lines of movement are pure re-wrapping, 2975 → 2981 words. It also declined my "re-flow the whole paragraph" option on a good ground (a full reflow splits the quoted `"/a/My Docs/include:/tmp/-march=native/inc"` example across lines, in a comment about exact strings). | `FIX-VERIFIED`, R7 reviewer dispatched |
| R7 | fresh reviewer #4 | `9bae04e4` | dispatched with the round explicitly bounded and the two vacuous-pass traps (empty-output strip, `println`→stderr) restated, plus instructions to re-derive the ticket behaviour rather than trust prior rounds | **awaiting** — **final round: not CLEAN ⇒ job stops and goes to the user** |
| R7 | fresh reviewer #4 | `9bae04e4` | reviewer: `scripts/gp-focused -x -cpus 6 -mem 6G -base 4444578e` → **683 passed, 2 skipped**; re-derived the ticket's failing-without by overlay (two marshalled tables differ only by `"cached":true`); proved the scrub load-bearing by disabling only its body (passes in the default shell, fails `TestDeclsDeterministic/cache` in the native env); brute-forced the scrub invariant over **200,000** random `KeyEnv` shapes → 0 violations; `-count=3 -shuffle=on` and `-race` green; proved comment-only with non-empty 12729-byte stripped artifacts. Adjudicated P11/P12/P14/P15 all `FIX-VERIFIED` | **FINDINGS (1 MINOR, 3 NIT)** — round 7, not CLEAN |
| P16 | R7 | MINOR | `decls_more_test.go:176-177` (`TestDropNativeArch` doc) | Two universals, both false, refuted 48 lines above in the same file: "takes an option with its argument when the argument is what asks" (false outside `nextWordOpts`) and "keeps the options around the one it drops" (false when an option sits in the argument slot). | WORKFLOW §8 comment accuracy; the F2/P4/P11 precedent | TBD | **CONFIRMED — manager measured all of it with the real `dropNativeArch`.** `"-target /tmp/-march=native/x -O2"`→`"-target -O2"`, `-arch`→`"-arch -O2"`, `"-stdlib …"`→`"-stdlib -O2"`, `"-resource-dir …"`→`"-resource-dir -O2"`, against `-isystem`→`"-O2"`; and `"-O2 -isystem -isystem /a/-march=native/x -O3"`→**`"-O2 -O3"`** (both surrounding `-isystem` popped, not kept). This is the P11 defect surviving in the **sibling** comment block — I prescribed replacement text for only one of the two blocks and then wrote "the file's prose has no universal left", which was false. | `OPEN` |
| P17 | R7 | NIT | `decls_more_test.go:87-88` | "the words after the space survive when they hold no marker" is false for any option word in `nextWordOpts`. | WORKFLOW §8 comment accuracy | TBD | **CONFIRMED — manager measured:** `dropNativeArch("/a/My Docs/include -isystem /tmp/-march=native/inc")` → **`"/a/My Docs/include"`** — `-isystem` holds no marker yet does not survive; same for `-include`. It survives only when not an option word: `"/a/My Docs/include -O2 -march=native"` → `"/a/My Docs/include -O2"`. Benign end to end (the pop removes the option together with the marker), hence NIT. | `OPEN` |
| P18 | R7 | NIT | `decls_more_test.go:106` | "only an empty value leaves this entry without it" is false: any value not beginning with `native` also clears it. | WORKFLOW §8 comment accuracy | TBD | **CONFIRMED — manager measured `build.NativeArch` directly:** `"NIX_CFLAGS_COMPILE-march="+"native"` → **true**, `+"Xnative"` → **false**, `+"foo"` → **false**, `+""` → **false**. The two false non-empty values are exactly the counterexamples. Code is correct (it writes `""`); wording only. | `OPEN` |
| P19 | R7 | NIT | `decls_more_test.go:177` **and `:113`** | "(a dangling -isystem breaks the compile)" does not hold on this toolchain — and the same claim appears twice, both naming `-isystem`. | WORKFLOW §8 comment accuracy; the unmeasured-rationale precedent | TBD | **CONFIRMED, and the manager measured it per-option rather than at the reviewer's single example.** Dangling option, `clang++ -fsyntax-only` / `g++ -fsyntax-only` exit codes: **break (1/1)** — `-include`, `-D`, `-U`, `-imacros`, `-Xclang`; **benign (0/0)** — `-isystem`, `-I`, `-L`, `-iquote`, `-isysroot`, `-Xlinker`, `-F`. So the claim is true for the list and false for the option both sites name. The pair-drop is still worth having (it avoids a mangled flags string); only the *stated reason* is wrong. `:113` was not raised by the reviewer — I found it while checking whether the claim recurred. | `OPEN` |
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
| R2 | fresh reviewer | `2ca8df06` | reviewer: stripped-code hashes still identical to `dev` on all four files, no added files, `go build`/`vet`/`gofmt` clean, `go test ./internal/lower` ok 11.159s; both compilers accept `testing.hpp` standalone. Manager verified each of the five findings separately (below): **1 and 3 confirmed, 2 refuted by running the code, 4 narrowed to one clause, 5 is a manager action** | **FINDINGS (1 MAJOR, 3 MINOR, 1 NIT) — 2 confirmed, 1 refuted, 1 narrowed, 1 manager action** |

| R3 fix | (merging coder 1) | `c43692c2` | manager re-verified both itself: line 16 is now **97 + 6** columns and `clang-format --dry-run --Werror runtime/include/gocpp/testing.hpp` exits **0** on `c43692c2` while `git show 484afff2:…` still reports `16:98` (exit 1), so H9 is closed by the tool that raised it; the two files' comment-stripped code hashes are identical to `dev` with **my own** stripper (`expr.go` `bc16f9bd73e7e202`, `testing.hpp` `5c5ebc2f68332c10`, both), so the job's central invariant survives round 3; `lazy.hpp` and `test/hresult_test.cpp` still fail `clang-format` **on unmodified `dev`** (verified at `/home/huuhait/GoToCPP`), so `TestClangFormat`'s remaining red is the filed L308.2 item and not this branch | awaiting a fresh R4 reviewer |

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
| H4 | R2 | MAJOR | `runtime/include/gocpp/testing.hpp:12` | "**the one** `runtime.Callers` and `CallersFrames` use too" asserts uniqueness and is false: `grep -rn "shown_frames\|resolve_pcs" runtime/src/*.cpp` gives **four** consumers, not two — `testing.cpp:133,135` (the native this sentence is about), `runtimepkg.cpp:270,276` (`runtime.Caller`, singular), `runtimepkg.cpp:302,318` (`Callers`/`pc_infos`), and `trace.cpp:2639` (the "created by" goroutine trace). So `Caller` (singular) uses the same pair and `trace.cpp` uses `resolve_pcs` too. This is H1's defect class one clause over: a uniqueness claim in text this job added that the code does not support. | WORKFLOW §8 / ticket Goal "make all four comments true today" | `FIX` in `484afff2` — the sentence now names all four users and asserts no uniqueness | **CONFIRMED by the manager's own grep** — the four consumers are exactly as reported (`runtimepkg.cpp:266 caller`, `:302 callers`, `testing.cpp:133 frames`, `trace.cpp:2639`). The reviewer is right that this is not a near-synonym for "uses too": the definite singular is the defect, and dropping it is a two-word fix. FIX: name the users without a superlative. | OPEN |
| H5 | R2 | MINOR | `runtime/include/gocpp/testing.hpp:9-10` | "Two capabilities are missing and give three natives" is not exhaustive: the reviewer argues `libParseFloat` is a fourth gap under the sentence's own test, because `stdlib/testing/PORT.md:22` gives its "Until" as **strconv**. | WORKFLOW §8 (a count the code does not support) | TBD | **REFUTED BY THE MANAGER RUNNING THE CODE — the premise is false.** `strconv.ParseFloat` works **today**, not "until later": `stdlib/strconv/atof.gp:696` is the real ported function, and `gocpp run` on the fix build prints Go's answers for `3.25`, `1e300`, `-0.0`, `Inf` and the `ErrSyntax` message (`NotANumber -> 0 strconv.ParseFloat: parsing "NotANumber": invalid syntax`) — the same on `dev`. So `strconv` is not a missing capability; `PORT.md:22`'s "Until: strconv" is a **stale row** (a separate, already-filed PORT.md cleanup), not evidence that a capability is absent. The three named natives stay correct: `find stdlib -type d -name sysinfo` → nothing, and `ReadMemStats` (`runtime.gp:872-884`) still never sets `Mallocs`/`Frees`. The reviewer's fallback concern — that the sentence with a count reads as exhaustive — is a style point, not a falsehood, and "Most natives exist because …" immediately after already scopes it. **No fix requested.** Manager refutation — needs a reviewer `CONFIRM-REFUTED` before it closes. **R3: `CONFIRM-REFUTED`** — the reviewer re-ran `strconv.ParseFloat` itself through `scripts/gp-repro` (`SAME` vs Go, Go's answers for all five inputs) and re-derived the count: `sysinfo` absent, `ReadMemStats` sets no `Mallocs`, `totalAlloc` documented as counting nothing; of 30 distinct natives only `libCPUName` and the `libAllocCounting`/`libAllocs` pair are gap-driven. | `CLOSED-REFUTED` |
| H6 | R2 | MINOR | `runtime/include/gocpp/testing.hpp:14` | "PORT.md says which Go function **each** native replaces" is false: `native.gp` declares 31 `//gocpp:native` lines across two files, `PORT.md` has 11 table rows, and the `libRunningAdd`/`libRunningDelete`/`libRunningList` group (`native.gp:100-107`) has **zero** mentions, as does `testdeps.exit`. | WORKFLOW §8 / reviewer procedure 3 (a cross-reference must resolve) | TBD | `FIX` in `484afff2` — "each" → "the main" | **CONFIRMED — manager reproduced both counts.** `grep -c libRunning stdlib/testing/PORT.md` → **0**; `grep gocpp:native` over `native.gp` + `internal/testdeps/native.gp` → **31**. And `git log -S "PORT.md says which Go function"` → `ac9fe281`, so the sentence is new in this job, not pre-existing. FIX: "each" → "the main" (or add the three rows plus `exit`). Cheaper and more honest: drop the universal. | OPEN |
| H7 | R2 | MINOR | `internal/lower/expr.go:671-674` | "except **the two shapes** that qualify the callee themselves" reads as a complete carve-out; the reviewer names `defer`/`go` of a package function (`defer.go:410,426` → `cdf.gp_push(...)`/`gp::go_fn(...)`) and a stored generic instance (`gpkg_main::gp_inst_…`). | WORKFLOW §8 / H3's rule | TBD | **PARTLY CONFIRMED — the emit reproduces, but the finding's scope is wrong.** The manager built the fix and emitted both: `defer copy1([]int{1})` → `gp_df.gp_push(copy1, …)`, `go copy1([]int{1})` → `gp::go_fn(copy1, …)`, `F[int](3)` → `gpkg_main::gp_inst_17381b1a_F(3)`. **However**, neither of the first two is a case `funcCallName` would ever be asked about — `funcCallName` has exactly two call sites (`expr.go:553` and the tuple fallthrough) and `defer`/`go` calls do not reach either, so naming them as exceptions to *this function's* prose would be wrong in the other direction. The generic-instance case is different: the prose's own later clause ("an instance of a generic function is qualified too") already states it, so it is not missing so much as placed after the word "except". The reviewer's own suggestion ("two of the shapes") is a wording tweak with no factual content behind it for `defer`/`go`. **Verdict: no fix for `defer`/`go`; not a false statement about `funcCallName`.** A reviewer's over-broad finding is still a finding, recorded here so R3 or the user can disagree with the manager. **R3: `CONFIRM-REFUTED`** — the reviewer traced `funcCallName` to its single call site (`expr.go:553`, the `*ast.Ident` case), showed `defer`/`go` take `deferredCall` (`defer.go:392-445`) and never re-enter `call()`, and additionally compiled `gp_df.gp_push(len, …)` and a slice-arg `gp_push` to prove no latent ADL bug exists on that path. The reviewer adds a *narrower* real defect from the same sentence — the count "two" — filed as H10. | `CLOSED-REFUTED` |
| **Manager correction of its own ticket.** My H4 fix instruction to the coder said to write "the same **pair** that `runtime.Caller`, `runtime.Callers` (and so `CallersFrames`) and the panic trace's 'created by' line use". The coder refused that sentence and was right: `trace.cpp:2639` calls `resolve_pcs` **only** — it takes `create_pc` directly and never calls `shown_frames` (verified at `trace.cpp:2616-2660`). Writing "the pair … and the panic trace" would have put a second false claim into a job whose whole purpose is removing false claims, one round after H4 was raised about exactly that kind of clause. The coder split the sentence instead ("which `runtime.Caller` and `runtime.Callers` … read with too, and whose `resolve_pcs` also names the panic trace's 'created by' line"), which is true of every consumer. Manager reproduced the split's premise with `grep shown_frames(`/`grep resolve_pcs(` and confirmed the trace is in the second list only. **Recorded because the manager's suggested wording was the defect this round, and a coder overriding it was correct.** | — |
| R2 fix | (merging coder 1) | `484afff2` | manager re-verified independently: comment-strip hash of `testing.hpp` **identical across `dev`, `2ca8df06` and `484afff2`** (`d029e4e7…`, my own strip script, not the coder's); `grep shown_frames(` outside `frames.cpp` → 3 hits, `grep resolve_pcs(` → 4, and the new text names exactly those users; `gcc` and `clang++` `-fsyntax-only` on the header both exit 0; `gofmt`/`go build ./...`/`go vet` clean | awaiting a fresh R3 reviewer |
| R3 | fresh reviewer | `484afff2` | reviewer: comment-only re-proved with its own Go AST printer + `go/scanner` token stream (identical hashes for all three Go files against `dev`); stripped header identical; both compilers accept the header standalone; `go build`/`vet`/`gofmt` clean; `scripts/gp-focused -x` 5 groups green, the `TestCXX` `signal: killed`s read as the known 6G cap. **Both manager refutations `CONFIRM-REFUTED`.** Manager verified both new findings itself: line 16 is **101 columns** (`.clang-format` `ColumnLimit: 100`) and `clang-format --dry-run --Werror` reports it, with `dev`→0, `2ca8df06`→0, `484afff2`→**1**, so this job introduced it; the generic-instance carve-out at `expr.go:558-562` bypasses `funcCallName` exactly as claimed. `TestClangFormat` at `runtime/portable_test.go:72` would fail on that file once clang-format is present | **FINDINGS (1 MINOR, 1 NIT)** — both confirmed |
| H9 | R3 | MINOR | `runtime/include/gocpp/testing.hpp:16` | The line the H6 fix added is **101 columns**, over `.clang-format`'s `ColumnLimit: 100`, so `TestClangFormat` (`runtime/portable_test.go:72`) fails for this file. The job introduced it: `dev` and `2ca8df06` both format clean, `484afff2` does not. The other files `TestClangFormat` complains about (`lazy.hpp`, `test/hresult_test.cpp`) are pre-existing and are the filed L308.2 item; this one is not. | WORKFLOW §8 "C++ follows `.clang-format`"; `docs/CONTRIBUTING.md:270` | TBD | **CONFIRMED — manager measured it three ways.** `awk` gives line 16 as 101 characters; `clang-format --dry-run --Werror` names `16:98` with `-Wclang-format-violations`; formatting the header blob from each commit gives **dev 0, `2ca8df06` 0, `484afff2` 1**. The fix is a rewrap, which is comment-only, so it does not threaten the job's central invariant. FIX: rewrap or shorten line 16. | `FIX` in `c43692c2` — line 16 wrapped onto two lines (97 + 6 columns), using `clang-format`'s own reflow so the sentence is unchanged. | **FIXED-PENDING R4.** Manager measured it: `clang-format --dry-run --Werror` exits **0** on `c43692c2`, and the pre-fix blob still fails at `16:98` — the check that raised the finding now passes, which for a formatting finding is the whole proof. Manager notes the wrap is ugly (`// it:` alone on line 17, 6 columns) and that a one-line rewrite was available at ≤94 columns (e.g. "… replace, and which could: "), but chose not to churn it: `// it:` is a normal continuation line in this repo (40 such short comment lines, `internal/load/cxxrecipe.go:17` is literally `// it:`), and the defect was the width, not the shape. | `FIXED-PENDING` |
| H10 | R3 | NIT | `internal/lower/expr.go:670` | "except the **two** shapes that qualify the callee themselves" is a miscount: there are three. A direct call of a generic instance (`F[int](3)`) is qualified by its own `IndexExpr` branch (`expr.go:558-562`) and never reaches `funcCallName`, exactly like the variadic and tuple shapes. The following clause does name the generic instance, so no reader is misled about *how* it is named — which is why this is a NIT, not H4/H6's class — but the number in text this job added is not supported by the code. | WORKFLOW §8 / ticket Goal | TBD | **CONFIRMED — manager read `expr.go:545-568`.** The `*ast.IndexExpr, *ast.IndexListExpr` case `break`s with `callee` already set by `instanceName`, so `funcCallName` is skipped for a generic instance as well. The reviewer's refutation of H7 (which asked for `defer`/`go` to be added) stands, and this is the *correctly narrower* version of the same complaint: not a missing category, a wrong count. FIX: "the shapes that qualify the callee themselves (a variadic call, `f(g())` with several results, an instance of a generic function)" or drop the count. | `FIX` in `c43692c2` — dropped the count; both existing explanations kept byte-for-byte, and the next sentence already names the generic instance. | **FIXED-PENDING R4.** Manager read the new line: `expr.go:669` is 92 columns and now reads "except the shapes that qualify the callee themselves", with no number left to be wrong. The coder also cited `expr.go:488-492` (`callee` set from `instanceName` when `inst != nil`) as the earlier, stronger reason the generic case never arrives — the same conclusion I reached from the `IndexExpr` branch, by a different path. No `defer`/`go` clause was added, per H7's confirmed refutation. | `FIXED-PENDING` |

| H8 | R2 | NIT | `.scratch/` in the worktree | 43 MB untracked. | WORKFLOW §5.2 | — | **Manager action before merge**, as §5.2 says; not a coder defect. Deferred to the pre-merge step so the remaining rounds can use it. | OPEN |
| H11 | R4 | NIT | `stdlib/testing/util.gp:9` | "package strings is a declaration stub until step 9.4 ports it" is false today: `stdlib/strings/` holds 8 `.gp` files and **116** `func` declarations across them (61 of them in `strings.gp`) with **zero** bodyless ones, and `tools/stubgen/main.go:130` lists `{Path: "strings", Phase: 9, Real: true}`. **Pre-existing** — `git diff dev -- stdlib/testing/util.gp` is empty — but it is the same stale-comment class this job exists to remove, in the same package, so it is in scope per WORKFLOW §6 "an older one in the same area". | WORKFLOW §8 ("comments explain what is not obvious") / ticket Goal extended to the same block | TBD | **CONFIRMED — manager checked the premises and the "nothing else" claim.** `grep -rh '^func ' stdlib/strings/*.gp \| wc -l` → **116** (per file: 1, 6, 12, 3, 21, 1, 11, 61); no bodyless `func` in any `stdlib/strings/*.gp`; `stdlib/testing/*.gp` never imports `strings`, so the *helpers* are still needed — only the "is a declaration stub" clause is wrong. Swept all of `stdlib/testing/*.gp` for `declaration stub\|stub until\|until (step\|later)\|Phase 09\|step 9\.` and it returns exactly this line and H12's two lines, so a fix round now can be terminal. FIX: drop the stub clause and say why the helpers stay (byte-level work is simpler than the ported `strings`). FIX: drop the stub clause and say why the helpers stay (byte-level work is simpler than the ported `strings`). | **FIXED-PENDING R5** — fix commit `0b766dee`: "Small string helpers: package strings is a real package now, but testing imports it nowhere, so these stay. They work on bytes, like the standard functions do." **Manager re-derived every premise itself:** `grep -rh '^func ' stdlib/strings/*.gp | wc -l` → **116**; `grep '"strings"' stdlib/testing/*.gp` → **NONE** (so "imports it nowhere" is true and the helpers are still needed); the comment is free-floating (`go/ast` `Doc=false`), so it cannot reach generated C++. The coder dropped two draft clauses it could not prove — (a) "simpler than the strings calls", false because `strings.HasPrefix` delegates to an `internal/stringslite` body character-identical to the local helper, and (b) "over the bytes" as a blanket file-wide claim. Both calls are right, and neither surviving clause overclaims. | awaiting a fresh R5 reviewer |
| H12 | R4 | NIT | `stdlib/testing/native.gp:7-9` | "the packages that Go's testing imports for them (sync, time, os, runtime) are declaration stubs until later steps of Phase 09" is the **same sentence this job deleted from `testing.hpp`**, left standing 20 lines away in the package that header documents. All four are `Real: true` (`tools/stubgen/main.go:115,122,133,136`). Byte-identical to `dev` (pre-existing). | WORKFLOW §8 / ticket Goal "make all four comments true today" | TBD | **CONFIRMED, and the dedupe is right.** `git diff dev -- stdlib/testing/native.gp` is empty, and the exact wording is already filed at TODO.md:182-188 ("two stale stdlib prose sites"). The R4 reviewer flagged it because its brief said to report same-area older issues, and noted the manager could close it against the existing line rather than reopen it. **Manager decision: fix it here, not against the TODO line.** WORKFLOW §6 sends every in-area finding back to the merging coder, the file is one the job already edited, and the sentence is literally the one this job removed elsewhere — leaving it means the job's own neighbours still contradict it. TODO:182-188 then keeps only its `stdlib/README.md:53` half, which I will narrow when this closes. FIX: adopt the header's wording. FIX: adopt the header's wording. | **FIXED-PENDING R5** — fix commit `0b766dee` adopts the header's sentence and keeps this file's own subject: "The packages whose functions these natives replace (sync, time, os, runtime, strconv, io, ...) are real packages now." **Manager verified all seven are `Real: true`** in `tools/stubgen/main.go` (`runtime`:115, `io`:121, `time`:122, `strconv`:129, `strings`:130, `sync`:133, `os`:136) and that the `...` hides no stub — the only remaining stubs are `weak`, `embed` and `fmt`. The coder deliberately did **not** copy the header's "Most natives exist because…" clause, because PORT.md's "Until" column shows many natives in *this* file are wrappers with a planned replacement; that is the right discrimination, not a copy-paste. | awaiting a fresh R5 reviewer |
| R4 | fresh reviewer | `c43692c2` | reviewer proved the comment-only invariant three independent ways (Go `go/scanner` token-stream SHA-256 and `g++ -fpreprocessed -dD -E -P` both identical to `dev` for all four files); verified every changed site's claim at the observation level (`gp-repro` on rune/surrogate/string/bool conversions, `sync`+`time`+`strings`, `os`+`runtime`; `gocpp emit` on four `funcCallName` fixtures) — all TRUE; confirmed H9/H10 fixes present; `clang-format --dry-run --Werror` exits 0 on the header where `484afff2` failed at `16:98`. Focused selection green except `TestCXX/g++/(O0\|O2)` **`g++: signal: killed`** inside the 6G `MemoryMax` (`map_test.cpp`, `unit.cpp`, `interop_test.cpp`), which it flagged honestly as *excused, not green* | **FINDINGS (2 NIT)** — both pre-existing, both in-area, both go to the merging coder |
| R4 fix | (merging coder 1) | `0b766dee` | manager on the fix: `git diff 4444578e...HEAD --name-status` → exactly six `M` files, no additions; every changed line is a `//` comment except one — the trailing comment on `d.attachTyped(in, pkg)` in `cmd/gocpp/build.go` became the block above it, so the code line itself is unchanged. Manager re-derived every factual claim in the new prose: `stdlib/strings` **116** funcs with none bodyless; `stdlib/testing/*.gp` imports no `strings`; all of `runtime`, `io`, `time`, `strconv`, `strings`, `sync`, `os` are `Real: true` in `tools/stubgen/main.go`; `ResolveMode` (`mode.go:115`) and `DecideFor` (`:174`) exist and order `-interop` → `GOCPP_INTEROP` → `GOCPP_TYPED_INTEROP` exactly as the new comment says. **Sweep re-run:** the brief's stale-claim regex over `stdlib/testing/*.gp` returns only `native.gp:11` ("since step 9.10", which is accurate and about `T.Setenv`), so this round can be terminal. Coder's focused run: 4 of 5 groups green; the runtime `TestCXX` group's failures are all 6G `MemoryMax` `signal: killed` on `g++ -O2`, zero compiler errors — **excused by the cap, not green**, and it says so | awaiting a fresh R5 reviewer |
| R5 | fresh reviewer #4 | `0b766dee` | reviewer: comment-only invariant re-proved with its own `go/scanner` token-stream SHA-256 **with a token-count assertion** on all six files (and `g++ -fpreprocessed` on the header) — no executable token differs anywhere; `clang-format --dry-run --Werror` on the header still **rc=0**; H11/H12 premises re-derived; cross-references the job asserts (`ResolveMode`/`DecideFor` order, `funcCallName` call sites, `shown_frames`/`resolve_pcs` users) all checked and right; `gp-repro` differentials on `ParseFloat`, `os.Getwd/Mkdir/Chdir`, `parseDuration`; `scripts/gp-focused -x -cpus 6 -mem 6G` → `internal/lower` 404 passed, `TestGolden` 41+1, `TestRun` 5, `TestRuntimeObjectsHaveNoStaticInit` 5; `TestCXX` failures all 6G `MemoryMax` `signal: killed` with **zero** `: error:` lines. **H11 and H12 `FIX-VERIFIED`.** Reported 4 MINOR + 1 NIT. **It also falsified the manager's terminal-round premise**, correctly: the sweep pattern that the manager declared clean was too narrow — see H15 | **FINDINGS (4 MINOR, 1 NIT)** — none of them in the code, but H13 is a real behaviour divergence |
| H13 | R5 | MINOR | `stdlib/testing/util.gp:10` (sentence this round wrote) + `util.gp:76-85` (`trimSpace`) | "They work on bytes, like the standard functions do" is false for `trimSpace`, and the consequence is user-visible: `trimSpace` trims only the six ASCII bytes of `isSpaceByte`, while Go's `strings.TrimSpace` is rune-based. `example.gp:50-52,90-91` uses `trimSpace` to compare an example's captured stdout against its `// Output:` block, where Go's `example.go:62-63` calls `strings.TrimSpace` — so an example whose output ends in non-ASCII white space fails under `gocpp test` and passes under `go test`. | WORKFLOW §8 Go behaviour / no documented divergence; ticket Goal on `util.gp:10` | TBD | **CONFIRMED — manager reproduced it three ways, and it is the first finding in this job that is not about wording.** (a) Function level via `scripts/gp-repro` on a copy of the helper: `"3\u00a0"` → local `"3\u00a0"`, std `"3"`, `equal=false` — and real Go agrees, so the divergence is between the two helpers, not between gocpp and Go. **My first attempt at this printed a vacuous `SAME`** because I used `println`, which writes to stderr: both captured files were 0 bytes and the harness compared nothing. Re-ran with `fmt.Printf` → the real difference above. (b) End to end: I copied `tests/gotest/testdata/examples`, added `ExampleAdd_nbsp` (`fmt.Print("3\u00a0")`, `// Output: 3`) and `ExampleAdd_tab` (`fmt.Print("3\t")`), then `GOCPP_GATE_PHASE=09 gocpp test .` → **`--- FAIL: ExampleMgrNBSP`** with `got: 3<NBSP>` / `want: 3`. (c) Controls: the ASCII-tab row **passes** (`ok`), isolating the cause to the non-ASCII byte; and the same fixture as `.go` under real `go test` → **`ok`**. So gocpp fails where Go passes. Pre-existing (`trimSpace` from `f1d24edd`, no fixture covers NBSP), but this round's sentence asserts the opposite, which is what brings it in. **FIX: make `trimSpace` rune-aware** (ASCII fast path then `unicode.IsSpace`, and `unicode` is `Real: true` at `tools/stubgen/main.go:118`) with a fixture row. This widens the round from comments to behaviour, deliberately. | OPEN |
| H14 | R5 | MINOR | `stdlib/testing/native.gp:7-8` | "The packages whose functions these natives replace … are real packages now" makes the natives read as gap-driven, but `libParseFloat` is a wrapper over a working `strconv` (`Real: true`), not a gap: `fuzz.gp:362` is its only caller. A reader concludes the native exists because `strconv` is not real. | WORKFLOW §8 comment accuracy | TBD | **CONFIRMED — manager checked both premises.** `strconv` is `{Path: "strconv", Phase: 9, Real: true}` (`tools/stubgen/main.go:129`) and `libParseFloat`'s only non-declaration caller is `stdlib/testing/fuzz.gp:362`. So the sentence's framing does not fit that row. Fair, and the fix is one clause. | OPEN |
| H15 | R5 | MINOR | `stdlib/testing/benchmark.gp:13` | "package time is ported, but testing does not use it yet" is false: the file **imports** `"time"` and uses `time.Duration` in executable code, and `testing.gp` imports it too (`time.Duration`, `time.Second`, `time.ParseDuration`). Same stale-Phase-09 class as H12, in the package this job edits. | WORKFLOW §6 in-area older issue / §8 | TBD | **CONFIRMED, and the manager's own terminal-round claim is what missed it.** `benchmark.gp` imports `"time"` at line 8 and uses `time.Duration` at 292, 298, 299, 446, 785; `git diff 4444578e -- stdlib/testing/benchmark.gp` is empty, so it is pre-existing. My sweep used `declaration stub|stub until|until (step|later)|Phase 09|step 9\.`, which does not match "does not use it yet"; the reviewer's `does not use|not use it|not yet|is ported, but` does. **The lesson is recorded rather than excused: a class sweep proves what its pattern covers, and I stated the conclusion wider than the evidence.** Not filed in TODO.md. | OPEN |
| H16 | R5 | MINOR | `stdlib/testing/PORT.md:19` and `:21` | Two rows the job's new text now points readers at: `:19` names `libLookupEnv`, `libSetEnv`, `libUnsetEnv`, whose declarations were deleted (`git grep` finds them only in this row — the real names are the C++ `lookup_env`/`set_env`/`unset_env`); `:21` maps `libCPUName` to `internal/sysinfo`, which does not exist (`find stdlib -type d -name sysinfo` → nothing) and which the header itself now contradicts ("no internal/sysinfo"). | WORKFLOW §8 / reviewer procedure 3 (a cross-reference must resolve) | TBD | **CONFIRMED — manager re-ran both checks.** `git grep -n libLookupEnv HEAD` → exactly one hit, `PORT.md:19` itself; `find stdlib -type d -name sysinfo` → nothing. R2/R3 had called both rows stale but kept them out of scope; what changes the call now is that `native.gp:7-9` and `testing.hpp:16-17` both name PORT.md as the authority, so the job's own text establishes a cross-reference that does not resolve. | OPEN |
| H17 | R5 | NIT | `stdlib/testing/native.gp:11` | 102 columns, over the 100 this job just enforced on `testing.hpp` (H9). No tool enforces it for `.gp`. | WORKFLOW §8 style consistency | TBD | **CONFIRMED — `awk 'NR==11{print length}'` → 102.** Five other `.gp` lines in this package are over 100 too (61: 103), all pre-existing, so this is a rewrap that keeps the round tidy, not a rule the job broke. | OPEN |

| R5 fix (H13–H17) | (merging coder) | `ccae15a7` | `git diff 0b766dee HEAD` = 5 files, +63/−16. **Manager verified the one executable change and the prose claims independently.** H13: I extracted gocpp's real `trimSpace`+`isSpaceByte` and ran **21 cases** through `scripts/gp-repro` against Go's `strings.TrimSpace` — ASCII, U+00A0, U+2003, U+0085, U+2028, U+3000, U+200B, a 4-byte emoji, **invalid UTF-8** (`\xff\xfe`), all-space and tab/NBSP mixes → **0 mismatches**, non-vacuous (`cases=21 mismatches=0`). Failing-without: the two **new fixture strings** run through `0b766dee`'s `trimSpace` return the untrimmed bytes — `od -c` gives `[3 \302\240]` and `[\342\200\203 3 \302\240]` against `// Output: 3` — so the new rows really do fail pre-fix, at byte level, not merely in a coder's log. H16: `git grep libLookupEnv\|libSetEnv\|libUnsetEnv` at HEAD → **0 hits**; the C++ `lookup_env`/`set_env`/`unset_env` exist (`testing.hpp:109-111`, `testing_os.cpp:80,89,101`); `internal/sysinfo` is absent from the tree; `T.Setenv` does call `os.LookupEnv`/`os.Setenv` (`testing.gp:879-884`). Only over-100 lines in the diff are the two PORT.md table rows (markdown tables cannot wrap; pre-existing pattern). | `FIX-VERIFIED`, R6 reviewer dispatched |
| R6 | fresh reviewer #6 | `ccae15a7` | reviewer: full `dev...HEAD` diff read; **H13 attacked exhaustively** — algorithm reimplemented in real Go over all strings ≤ 4 from a 21-symbol nasty alphabet (204,205 cases, 0 mismatches), every code point × 5 wrap shapes (5,560,320 cases, 0 mismatches), 300,000-case xorshift fuzz with an FNV checksum so agreement cannot be vacuous, 22 targeted invalid-UTF-8 edges; H13 failing-without reproduced **end to end** with a pre-fix `gocpp` binary (both rows FAIL); H16 re-derived (0 hits for the deleted names, C++ functions exist, `internal/sysinfo` absent, `T.Setenv` uses `os`); H14/H17 checked against the files; `scripts/gp-focused` → `internal/lower` 404/0, `TestGolden` 41/1, `TestCXX` 476 passed + 3 killed at the 6G cap with **0** `: error:` lines | **FINDINGS (1 MAJOR, 3 MINOR, 1 NIT)** — round 6, not CLEAN |
| S1 | R6 | **MAJOR** | `stdlib/testing/benchmark.gp:326` (`ReportMetric`) and `stdlib/testing/testing.gp:946` (`Attr`) | Both loop `isSpaceByte` over raw bytes; Go uses `strings.IndexFunc(unit, unicode.IsSpace)` (`testing/benchmark.go:385`) and `strings.ContainsFunc(key, unicode.IsSpace)` (`testing/testing.go:1510`). Every non-ASCII space (U+00A0, U+2003, U+0085, U+3000) is missed by gocpp and rejected by Go. | WORKFLOW §8 Go behaviour, no documented divergence (`PORT.md` mentions `Attr`/`ReportMetric` **0** times) | TBD | **CONFIRMED END TO END — the manager built binaries and ran both halves, not just read source.** `ReportMetric(1.0, " x y")`: gocpp **PASS** (`BenchmarkMetricEMSpace-64 … ok example.com/m`), real Go **`panic: metric unit must not contain whitespace`**. `t.Attr("a b", "v")`: gocpp **`--- PASS`** with `=== ATTR  TestAttrNBSP a b v`, real Go **`disallowed whitespace in attribute key "a b"`**. Source agrees: gocpp loops bytes, Go calls `IndexFunc`/`ContainsFunc`. Pre-existing, but in the package this job edits, and made actionable by H13 — `util.gp` now holds both the corrected (`trimSpace`) and uncorrected (`isSpaceByte` callers) form of one predicate. | `FIX-PENDING` (fix round dispatched) |
| S2 | R6 | MINOR | `stdlib/testing/util.gp:13-15` | The new sentence reads as a file-wide "by rune" claim, but `isSpaceByte` stays byte-based and governs the two S1 checks. | WORKFLOW §8 comment accuracy; ticket Goal "make the comments true today" | TBD | **CONFIRMED by the manager** — `isSpaceByte` is at `util.gp:77`, and its two non-`trimSpace` consumers are exactly the S1 sites. Re-word after S1 lands so the claim matches the code. | `FIX-PENDING` |
| S3 | R6 | MINOR | `stdlib/testing/benchmark.gp:14` | "imported for its types (time.Duration, time.Second)" — this file never uses `time.Second`. | WORKFLOW §8 comment accuracy (H15's defect re-created inside H15's own fix) | TBD | **CONFIRMED — `grep -n 'time\.Second' stdlib/testing/benchmark.gp` → exactly one hit, line 14, the comment itself.** `time.Duration` → 6 hits; `time.Second` lives in `testing.gp:76,83`. | `FIX-PENDING` |
| S4 | R6 | MINOR | `stdlib/testing/PORT.md:19` and `stdlib/testing/native.gp:12` | "have no caller" is false: the three env natives have **63** call sites in `runtime/test/testing_natives_test.cpp`. | WORKFLOW §8 cross-reference accuracy | TBD | **CONFIRMED — `grep -c` → 63.** The correct wording already exists one file away at `runtime/include/gocpp/testing.hpp:36-37`: "T.Setenv has used os itself since step 9.10, so no Go code calls them, only their tests." Fix = adopt it verbatim. | `FIX-PENDING` |
| S5 | R6 | NIT | `stdlib/testing/native.gp:62` | 103 columns — the file's only remaining over-100 line, left behind by H17's fix; H17's stated "five other lines over 100" is now a stale number. | WORKFLOW §5.1 NIT (style, stale number) | TBD | **CONFIRMED — `awk 'length>100'` returns exactly `62: 103`.** | `FIX-PENDING` |

### Manager traps hit this session (recorded so reviewers and future rounds stop repeating them)

1. **Empty-output strip (evalorder R3).** A strip tool whose source and target sat in one directory failed to build ("found packages main and lower"), so both outputs were 0 bytes and `cmp` printed a confident *identical*. Fix: build the tool as a standalone binary, then confirm both files are non-empty. Generalised rule now in every reviewer brief: **print line counts before trusting a comparison.**
2. **`println` writes to stderr (stalehints).** A `gp-repro` differential of two `println` programs compared two empty stdout files and printed SAME. Fix: use `fmt.Printf`, confirm output is non-empty.
3. **Truncated `tail` hid the failing subtest (stalehints, manager).** I ran `go test -v … | tail -25` and reported "a FAIL" without the subtest name — a log I had truncated myself. Rule: when a suite can fail, capture the whole log to a file and grep it.
4. **Unmatched wait pattern (stalehints, manager).** A `pgrep -f` pattern that did not match the running command made an `until` loop print "FINISHED" while the test was still going, and the `0 passes` line it printed was an artifact. Rule: wait on a PID (`kill -0`), not on a pattern you just guessed.
5. **Test run outside the dev shell (stalehints, manager).** Running `go test ./tests/gotest` directly made `TestGoTest/asan_parallel` fail with `'sanitizer/lsan_interface.h' file not found` — clang's include path lacks the ASAN headers outside the shell. **The same subtest passes in `scripts/in-devenv --` (89.9 s, `ok`).** Not a code defect, and not an H13 regression: the rest of the suite was green in the same run (83 passes, that one failure). Rule: **run suites through `scripts/in-devenv`**, as `gp-focused` does.
6. **Over-subscription (manager).** Four reviewers plus a fifth full suite on 64 cores took load to **60.68**. The heavy clang++ work was the reviewers' own `gp-focused` runs. Rule: do not launch a full sweep while three or more reviewers are mid-run; check `uptime` first.

**Note on R4's two findings being pre-existing.** Neither is introduced by this job, and a reviewer
could reasonably have filed both as out-of-scope notes. Two things make them findings here: the user's
rule (WORKFLOW §6, 2026-10-23) that "every issue a review finds, in the ticket's scope, **an older one
in the same area**, or one unrelated, goes back to the merging coder", and this job's own ticket Goal
("make all four comments true today") applied to a package the job already edits. The round ceiling
(WORKFLOW §6, "after 5 rounds without CLEAN … split, clarify, or take it to the user") is the reason
I swept the whole `stdlib/testing/*.gp` comment space myself before dispatching: R4 is round 4, and a
terminal fix round beats discovering a fresh crop of the same class in R5.

### Out of scope

| Round | Note | Filed in |
|---|---|---|
| manager | `stdlib/testing/native.gp:7-8` repeats the stale "declaration stubs until later steps of Phase 09" wording that this job removed from `testing.hpp` | **now H12** — R4 raised it; the manager fixes it in this job rather than against the existing TODO line (see H12) |
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
| R2 fix | (merging coder 3) | `f47fd51d` | manager re-ran the focused suite itself: **`scripts/gp-focused -x -cpus 6 -mem 6G` green** — `cmd/gocpp` annotate group `ok 369.079s` **189 passed 0 skipped**, `internal/explain` `ok 0.242s` 20 passed, `internal/lower` `ok 10.764s` **408 passed**, `tests` `TestGolden` 41 passed 1 skipped (escape), `gofmt -l` empty and `go vet` clean on all three packages. Manager also re-proved every negative itself: the two new tests **fail on `f50e67a1`** through an overlay of the parent's `whynotes.go` (6 assertions printing the old `(the stack trace names it func1)` sentences); the `n`→`n+7` mutation **fails `TestAnnotateTruth`** on 8+ chunks where it was green before W8's fix; `TestGuideExamples` **was red on `f50e67a1`** (verified by running it in an extracted tree of that commit) and is green at HEAD; and the wrap-macro trace levels were measured on the HEAD binary (0 macros → `main.work.func1`, 1 → `.1`, 2 → `.1.1`), which is the evidence W6's drop-the-number decision rests on | awaiting a fresh R3 reviewer |
| R2 | fresh reviewer | `f50e67a1` | reviewer: `scripts/gp-focused -x -cxx clang++` green (cmd/gocpp `ok 378.658s` 189 passed, internal/explain 20, internal/lower 406, TestGolden 41/1 skipped); `-update-annotate` in a scratch copy leaves `cmd/gocpp/testdata` byte-identical, so the 12 regenerated goldens hide no regression; `gocpp emit` identical pre-fix vs HEAD on all 14 `truth/*.gp` and all `internal/explain/examples/*.gp` (annotation-only); W1's and W2's fixes re-proved by negative checks. Manager verified all three findings itself (below) | **FINDINGS (1 MAJOR, 1 MINOR, 1 NIT)** — all 3 confirmed |
| R3 | fresh reviewer | `f47fd51d` | reviewer: focused suite green (189/20/408/41+1); real golden regeneration in a `git archive` copy leaves `cmd/gocpp/testdata` md5-identical and `docs/guides/explain.md` unchanged; ported `whynotes_test.go` onto a clean `f50e67a1` tree and both new tests FAIL there; measured trace numbering for 8 shapes; emitted 7 recover shapes and matched them to W7's comment clause by clause; re-ran the `n+7` mutation. **Manager adjudicated all five itself, not by reading:** W9 reproduced (see below); W10 reproduced by mutation; W11 **REFUTED by the manager**; W12 reproduced against `go run`; W13 reproduced | **FINDINGS (3 MAJOR, 2 MINOR)** — 4 confirmed, 1 refuted |

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

| W6 | R2 | MAJOR | `internal/lower/whynotes.go:167` (`noteClosure`, the non-local headline) | `(the stack trace names it func%d)` prints the literal's ordinal **within its immediate parent**, so it is wrong whenever a literal sits inside another literal or inside a wrap-macro (`#body()`) body. The real trace name of the inner literal is `main.main.funcN.1`, but the note says `funcN`. The commit's own new comment (`whynotes.go:122-127`) declares this clause part of "what holds for every shape", making the universal explicit. | WORKFLOW §8 / the ticket's headline-truth requirement | TBD | **CONFIRMED — manager reproduced it and the repository contradicted the annotation for its own case.** A literal nested in a literal annotates `func1` and the program prints `main.main.func1.1()`. `tests/run/closures_trace.gp` itself asserts `main\.main\.func1\.1\(` at line 2 **for a literal whose annotation says `func1`** — the repo's own test and the repo's own annotation disagree about the same program. Pre-existing (`d940d4b4` and `dev`), so not a regression, but it is a false sentence in the headline this job owns, it survived the round whose brief was "true for EVERY shape", and finding 3 shows why the suite could not catch it. FIX: drop the number, or make it relational ("names it after its parent: funcN, or funcN.M inside another literal"). | `FIX` in `f47fd51d` — the number is gone: the headline is `func literal #%d is a C++ lambda` plus the unchanged capture suffix, with no trace claim at all. The coder measured the relational alternative too and rejected it with evidence I reproduced: **a wrap macro adds a level per macro** — my own run of a `#VM()`/`#VM2()` stack prints `main.work.func1.1.1()`, one macro prints `main.work.func1.1()`, no macro prints `main.work.func1()`. A "funcN or funcN.M" wording would lie at three macros, so dropping the number is the only shape that cannot go stale. | **FIXED-PENDING R3.** Manager verified at the observation level, not by reading: `emit -annotate` on a nested literal now prints `func literal #1 is a C++ lambda; it captures nothing` with no trace number; the new `TestNestedClosureHeadline` and `TestDeferredLiteralTraceNote` **FAIL on `f50e67a1`** (I ran them through an overlay of the parent's `whynotes.go`: 6 assertion failures printing the old `(the stack trace names it func1)` sentences) and pass on HEAD. The repo's own `tests/run/closures_trace.gp` case is unaffected: the trace still prints `main.main.func1.1`. | `FIXED-PENDING` |
| W7 | R2 | MINOR | `internal/lower/whynotes.go:122-125` (new comment) | The comment says "a literal that calls recover is inside `gp::aware` (closures.go: funcValue, callValue, deferValue)". False at two of the three sites it names: `callValue` never builds `gp::aware` (an immediately-called literal that calls recover emits a bare lambda), and `deferValue` emits `gp_df.gp_push_call_aware(...)`, not `gp::aware`. | WORKFLOW §8 comment accuracy | TBD | **CONFIRMED — manager emitted both programs.** `callValue`: `gp::any r = []<int = 1>() mutable -> gp::any { … gp::recover(gp_dc); }();` — no `gp::aware` anywhere. `deferValue`: `gp_df.gp_push_call_aware([…])`. `gp::aware` is produced only at `closures.go:181,187` (`funcValue`) and `methods.go:667`. So the comment reintroduces a false mechanism claim one line under the one W1 removed — the same defect class, in the fix for it. FIX: name what each site actually emits. | `FIX` in `f47fd51d` — the comment now names `callValue`, `gp::go_call`/`gp_push_call`, `refArgs`, `gp::func<Sig>(gp::aware(lambda))` with `closures.go: funcValue, methods.go: methodValue`, `gp_df.gp_push_call_aware`, and "an immediately-called or spawned one stays a bare lambda", plus a second paragraph on how the trace numbers a literal within its parent. | **FIXED-PENDING R3.** Manager emitted four recover-literal shapes at HEAD and matched each clause: `valued()` → `gp::func<void()>(gp::aware([]<int = 1>() ...))`; `called()` → a bare `[]<int = 1>() ... .operator()()` containing `gp::take_defer_ctx`; `deferred()` → `gp_df.gp_push_call_aware([...])`; and a lazy macro argument → a bare lambda passed to `gp_inst_…_Later(...)`. All four match the new text. The spawned shape cannot be emitted (go statements are gated Phase 08), so that clause rests on `pushCall`'s code path: `deferValue` sets `gp_push_call_aware` when the body is aware, and `pushCall` rewrites any `gp_push_call*` name to `gp::go_call` under `spawn` — a bare lambda. The sibling `howLiteral` sentences for `defer`/`go` carried the identical false "names it func1, func2…" and are fixed in the same commit; I confirmed the old text was there (`git show f50e67a1:internal/lower/whynotes.go`) and that the new `TestDeferredLiteralTraceNote` fails on it. | `FIXED-PENDING` |
| W8 | R2 | NIT | `cmd/gocpp/annotate_test.go:471` | The `TestAnnotateTruth` rule `{msg: "~is a C++ lambda (the stack trace names it func", need: []string{"<int = "}}` does not pin the number it names — any lambda in the chunk satisfies `<int = `. | test that cannot fail on the defect it appears to cover | TBD | **CONFIRMED — manager ran the mutation.** Changing the format from `n` to `n+7` in an overlay leaves `TestAnnotateTruth` **green** (`ok 0.819s`) while `TestAnnotateGolden` catches it. That laxness is exactly why W6 passes the whole focused suite. FIX: require the literal's own number, or say in the rule's comment that the number is pinned by the goldens only. | `FIX` in `f47fd51d` — `truthRule` gains a `num bool`, the two lambda rules set it, and `litNum` (`func literal #(\d+)`) ties the reason's number to `<int = N>` in the chunk. | **FIXED-PENDING R3.** Manager re-ran the exact mutation the finding was filed on: format `n` → `n+7` through an overlay leaves `TestAnnotateTruth` **failing on 8+ chunks** at HEAD (`the reason says the literal is #8, but its chunk has no <int = 8>`) where it was green pre-fix. So the rule now bites on the defect it appears to cover. | `FIXED-PENDING` |
| W9 | R3 | MAJOR | `internal/lower/whynotes.go:178,188` (the headline branches this job added) | **This job re-creates its own bug in a new shape:** a func literal that is the argument of a macro's `lazy`/ref closure parameter is lowered **twice** (`noteClosure` runs once with `rawLits`/`refLits` set and once with neither), and before `f50e67a1` the two texts were byte-identical so `Why.add`'s `seen` map deduped them. R1's per-branch suffixes make them different strings, so **one literal gets two contradictory headlines** and its captured variable is called both by-reference and by-copy. | ticket Goal "a reader of `gocpp emit -annotate` must not see … a contradiction two lines later"; WORKFLOW §8 | TBD | **CONFIRMED — manager ran it and attributed it to the job.** `#Later(func() string { return fmt.Sprint("n=", n) })` with `macro Later(f lazy func() string)` on HEAD prints at col 9 both "func literal #1 is a C++ lambda that captures by reference, because the macro only calls it" **and** "func literal #1 is a C++ lambda; what it captures is copied into the lambda (a box is shared)", and at col 49 both "captures n by reference" and "captures n by copy into the lambda" — while the emission is `int64_t n = 3;` (not boxed) and a single lambda. **Attribution measured on three binaries I built from extracted trees:** `dev` → **1** headline at col 9, `f50e67a1` → **2**, HEAD → **2** (and HEAD's pair is the contradictory one). So R1's fix created the split and R2's dropped number did not close it. Nothing catches it: `TestEmitCorpusIdentical` compares C++, not notes, and `tests/run` output is not golden-pinned. FIX: emit one reason for a literal lowered twice — memoise `noteClosure` per `*ast.FuncLit`, or lower the macro argument once; do **not** merely re-dedupe by message, since the two messages now differ by design. | **FIX-VERIFIED R4** — the reviewer disabled only the skip (`hiddenLit` → `return false`) by overlay: `TestMacroLazyLiteralOneNote` fails with two headlines and the contradictory `captures n` pair at one line; at HEAD it passes, and the real binary prints one headline and one note. Reviewer also checked the lazy `HiddenLits` map init (`internal/load/macros.go`) and the nil guards in `closures.go:260` — no nil-write path. | `CLOSED-FIXED` |
| W10 | R3 | MAJOR | `cmd/gocpp/annotate_test.go:574-580` (the new `num` rule) | The `num` half of W8's fix checks only that `<int = N>` occurs **somewhere** in the chunk, so when one chunk holds two func literals a wrong number passes. | WORKFLOW §5.1 "a test that cannot fail"; §8 "every new test fails without the change" | TBD | **CONFIRMED — manager reproduced the reviewer's mutation, and narrowed the reason.** Mutating `closures.go:274` to `lit(strconv.Itoa(3 - n))` swaps every literal's number. Running **only** the two-literal subtest on a program I added (`a, b := func() int { return 1 }, func() int { return 2 }`): `--- PASS: TestAnnotateTruth/truth-twolit`, and the emission is `[]<int = 2>` for `a` and `[]<int = 1>` for `b` while the reasons say `#1` and `#2`. **The reason is chunk-continuation, not line-joining** (the reviewer said "one line"; my measurement is sharper): the test concatenates every `Continued` chunk of the same `.gp` line into one `cpp` string, so both lambdas' tags land in one string and `strings.Contains` finds the other one's. The suite does fail the mutation, but only through *other* programs with one literal per chunk — the two-literal shape itself is unpinned. FIX: match the ordered literal numbers against the ordered tags in the chunk, not containment of each. | **FIX-VERIFIED R4** — reviewer re-ran the `3 - n` mutation itself rather than trusting the coder: at HEAD `TestAnnotateTruth/truth-twolits` **fails** (`reasons ["1" "2"]`, tags `[["<int = 2>"] ["<int = 1>"]]`); with `f47fd51d`'s test file overlaid the same mutation **passes**. So the new ordered check converts this subtest from pass to fail on the defect. **The fix is right and is itself the source of W14.** | `CLOSED-FIXED` (defect closed; its replacement rule is W14) |
| W11 | R3 | MAJOR | `cmd/gocpp/annotate_test.go:560-580` | Reviewer claim: the `num`/`need` rules **fail on correct output** for a func literal in a **package-variable initializer**, because the reason attaches to the `constinit …{}` chunk while the literal lives in `gp_var_init()`. | WORKFLOW §8 (a truth rule that rejects correct output) | TBD | **REFUTED BY THE MANAGER — the premise is false for every shape the reviewer named.** I added the reviewer's own programs to `testdata/truth` in an extracted HEAD tree and instrumented the rule to print the chunk it actually sees. The chunk **continues** into `gp_var_init()` (the same-.gp-line `Continued` join again), so `<int = 1>` is present: `var f = func() int { return 7 }` → `cpp="constinit gp::keep<gp::func<int64_t()>> gp_keep_f{};\nvoid gp_var_init() {\n    f = gp::func<int64_t()>([]<int = 1>() …"`; `var v = func() int { panic("x") }()` → `… constinit int64_t v{};\nvoid gp_var_init() {\n    v = []<int = 1>() …`; `var g = func(x int) int {…}` → same with the tag. All three **PASS**. The reviewer's raw `gocpp emit -annotate` output shows only the first chunk line, which is why it concluded the tag was absent. **No fix requested.** Manager refutation — needs a reviewer `CONFIRM-REFUTED` before it closes, per the single-pass rule. | **REJECT-REFUTED in R4, and the manager's refutation was WRONG.** I concluded the chunk continues into `gp_var_init()` so the tag is present, from a program where it did. It does not when a *different-line* chunk sits between the `constinit` chunk and the continued assignment chunk — a second package variable gives exactly that. My own minimal repro (`var a = func() int { return 1 }` then `var x = 5`) emits the reason at col 9 beside `constinit gp::keep<gp::func<int64_t()>> gp_keep_a{};` and the `<int = 1>` six lines later at `gp_var_init`, so the rule sees a chunk with no tag and **fails on correct output**. The reviewer's real-program evidence is `tests/run/closures_pkgvars.gp` lines 47/21/25 (corpus index 0069). **My error is the one I had already recorded twice this session: I generalised from one program.** I proved the tag was present *in the shapes I chose*, and wrote the conclusion as "the premise is false for every shape the reviewer named" — a universal from three samples. The reviewer's refutation stands as **W15/W16**. | `CLOSED-REFUTED-WRONG` → defect filed as W16 |
| W12 | R3 | MINOR | `internal/lower/whynotes.go:287,298` (the new `howLiteral` note for `go`/`defer`) | "the stack trace names the literal as Go does, numbered within the function or literal that holds it" is false for a literal written directly in a **range-over-func body**: gocpp drops the frame entirely and Go prints the range-body frame. | WORKFLOW §8; the note is the divergence's own documentation | TBD | **CONFIRMED — manager ran both sides.** A panicking local closure inside `for v := range seq`: gocpp prints `main.seq(...)` / `main.main()`; `go run` prints `main.main.seq.main-range1.func2(...)` / `main.main-range1(...)` / `main.seq(...)` / `main.main()` — four frames against two, with the literal's frame **absent**, not renamed. `docs/spec/language.md:1063` documents the divergence but says the literal "is `main.f.func1`", which is also not what this shape does. FIX: add "not one written in a range-over-func body, whose frame is left out (language.md)" to both `howLiteral` strings, and correct `language.md:1063`. | **FIX-VERIFIED R4** — reviewer reverted `whynotes.go` to `f47fd51d` by overlay: `TestRangeBodyLiteralTraceNote` fails twice quoting both old `howLiteral` strings. Behaviour re-measured: gocpp shows `main.seq` then `main.main`; go1.25.7 shows `main.main.seq.main-range1.func2`, `main.main-range1`, `main.seq`, `main.main`. Substantive claim right. **`language.md:1063` is still wrong — that is W18.** | `CLOSED-FIXED` (the two notes); W18 open |
| W13 | R3 | MINOR | `internal/lower/whynotes.go:190` (the `default` branch) | The headline appends "; what it captures is copied into the lambda **(a box is shared)**" even when nothing is boxed. | ticket Goal (headline accuracy) | TBD | **CONFIRMED — manager emitted the shape.** `#Twice(func() { fmt.Println("n =", n) })` with `macro Twice(f func())` emits `int64_t n = 1;` and `gp::func<void()>([n]<int = 1>() …` — the capture is a plain by-value copy and no box exists anywhere — yet the headline says "(a box is shared)". The code shows why: `whynotes.go:190`'s `default` branch appends the box clause unconditionally. Older than this job in substance (the reviewer reports `directfunc.gp` identical on `dev`), but the sentence is in the headline this job rewrote, so it is in scope. FIX: append the box clause only when a capture is actually boxed. | **FIX-VERIFIED R4** — reviewer re-ran the revert: `TestNoBoxClaimWithoutBox` fails with the false "(a box is shared)". Real binary: an unboxed never-changing capture prints `[n]<int = 1>` with no box clause; a changing `x` keeps it. `whynotes.go:201-203` appends the clause only under `if boxed`. | `CLOSED-FIXED` |
| W14 | R4 | MAJOR | `cmd/gocpp/annotate_test.go:455` (`orderedTags`, this round's W10 fix) | **The fix for W10 rejects correct output.** `orderedTags` assumes a chunk's `<int = N>` tags occur in source order; they do not — when a temporary is materialised before an earlier source expression the tags come out swapped. `tests/run/iface_eq_structs.gp:75` emits `auto gp_t7 = …([]<int = 14>…)` then `(*a) = …([]<int = 13>…)` while the reasons are numbered 13 then 14 by source order. | WORKFLOW §8 (a test must accept correct output); the TODO claim that the ordered rule "accepts each reason's literal" | TBD | **CONFIRMED — manager reproduced on the real program.** `emit -annotate tests/run/iface_eq_structs.gp` line 495: `{ auto gp_t7 = …([]<int = 14>() …); (*a) = …([]<int = 13>() …); (*b) = gp_t7; }` with the reasons at col 9 `#13` and col 20 `#14`. Reviewer's corpus attribution: over 1106 injected programs `f47fd51d` failed 92, HEAD fails 93, and the one added failure is exactly this program through `orderedTags`. Two smaller shapes reproduce too (a two-assign tuple, and a two-value `return` where only one literal's tag is in the chunk). | OPEN |
| W15 | R4 | MINOR | `cmd/gocpp/annotate_test.go:494` (`~is a C++ lambda` truth rule) | The `need: ["<int = "]` rule fires on the `constinit` chunk of a package-variable func-literal initializer, which legitimately carries no tag (the tag is on a later `gp_var_init` chunk). **This is W11 as a defect rather than as an adjudication.** | WORKFLOW §8 (rejects correct output) | TBD | **CONFIRMED — manager's own minimal repro fails.** `var a = func() int { return 1 }` then `var x = 5`: reason at col 9 on `constinit gp::keep<gp::func<int64_t()>> gp_keep_a{};`, tag at `a = gp::func<int64_t()>([]<int = 1>() …)` inside `gp_var_init`. Reviewer adds `tests/run/closures_pkgvars.gp:47,21,25` and the struct/map package-var shapes. | OPEN |
| W16 | R4 | MINOR | `internal/lower/whynotes.go:214,218` | A **local** closure inside a lazy macro argument's literal gets two contradictory capture notes for one variable at one place. The W9 fix covers the synthesised thunk, not locals inside the user's own literal. | WORKFLOW §8 (same-area older issue, in scope per §6) | TBD | **CONFIRMED — manager ran the reviewer's `dupnote.gp`.** At col 32, both `captures n by copy into the lambda: it never changes after the closure exists` **and** `captures n by reference: it stays on the C++ stack and the closure runs within the function`, while the emission is a single `[&n]<int = 1>` lambda — so the by-copy note is false. Identical on `dev` (4444578e) and `f47fd51d`: pre-existing, same function. `lazyLitSrc` has no inner local, which is why no test catches it. | OPEN |
| W17 | R4 | MINOR | `cmd/gocpp/annotate_test.go:480,488,490` | Older truth rules reject correct output on ~92 of 1106 repo programs: `~is a local closure` `forbid: ["gp::func<"]` fires when the lambda's **return type** is `gp::func<…>`; `method expression` `need: ["[](auto&& gp_r", "> gp_r)"]` vs the actual `[]<int = 0>(refOuter gp_r …)`; `&T{...} allocates a new` `need: ["gp::new_<"]` vs `gp::new_array_keyed<…>`. | WORKFLOW §8 (the suite must accept correct output) | TBD | **CONFIRMED — manager spot-checked two of the three.** `emit tests/run/iface_mv_places.gp:121` → `gp::func<int64_t()> cl = [&i]<int = 2>() mutable -> gp::func<int64_t()> {`, which the `forbid` rejects on correct output. `emit tests/run/arrays_large.gp:39,48` → `gp::new_array_keyed<…>` where the rule demands `gp::new_<`. Present on `dev` (same rule text), so older; the truth suite is this job's area. | OPEN |
| W18 | R4 | NIT | `docs/spec/language.md:1063` | The row says Go names a func literal in a range body `main.main.f-range1.func1`. Measured on go1.25.7: `main.main.seq.main-range1.func2` (default), `main.main.func1` (prevent inlining), `main.f.func1` (non-inlined `seq` + prevent inlining). No configuration produced the name the doc gives. Substantive claim (Go shows the range frame plus the literal) is right; the literal's name is wrong. | plan/doc accuracy | TBD | **CONFIRMED — manager accepts the reviewer's measurement.** It is the doc half of W12, left open because W12's fix was scoped to the two `howLiteral` strings. | OPEN |
| R3 fix | (merging coder 3) | `1d02a073` | 19 files. **Manager verified all four fixes by disabling the new mechanism and watching the new tests fail — not by reading the diff.** (a) W9: with `g.hiddenLit(fl)` short-circuited to `false` (mechanism kept, skip disabled), `TestMacroLazyLiteralOneNote` fails with exactly the contradictory pair the finding named — 2 headlines at col 9 for one literal, "captures by reference … the macro only calls it" beside "captures by copy", and 2 notes for `n`. (b) W13: reverting `whynotes.go` alone to `f47fd51d` fails `TestNoBoxClaimWithoutBox` with the false "(a box is shared)" on an unboxed capture. (c) W12: same revert fails `TestRangeBodyLiteralTraceNote` twice, quoting both old `howLiteral` strings. (d) W10: the coder's own evidence is that the `3-n` mutation now fails the two-literal subtest while the pre-fix test file **passes** it — the reviewer must re-run this one, I did not. The coder also reports two reverted design attempts (a broader "mark every macro-written literal" HiddenLits, and W12 wording using "frame" which an existing truth rule forbids in `go:` notes), and the emission is byte-identical (`TestEmitByteIdenticalWithLog`, `TestEmitCorpusIdentical` green). W11 untouched, per the refutation | reviewed in R4 |
| R4 | fresh reviewer #4 | `1d02a073` | reviewer: baseline `scripts/gp-focused -x -cpus 6 -mem 6G` green (`cmd/gocpp` 190 passed, `internal/explain` 20, `internal/load` 1109, `internal/lower` 411, `TestGolden` 41+1), gofmt empty, vet clean; **emission byte-identical** — 2322 `.gp` programs through dev (4444578e) and HEAD binaries under `GOCPP_GATE_PHASE=15`, `compared=2322 diffs=0`, and the 17 legitimately empty outputs have stderr identical 17/17; goldens honest (real `-update-annotate` regeneration in a `git archive` copy leaves `cmd/gocpp/testdata` and `docs/guides/explain.md` byte-identical). It reports having first run the comparison under the wrong variable (`GOCPP_TEST_PHASE`), got 1195 identical empties, and re-ran it under the real gate variable — the vacuous-pass trap caught and disclosed by the reviewer itself. **W9, W10, W12, W13 all `FIX-VERIFIED`**, each by an overlay that disables only the fix. **W11 `REJECT-REFUTED`** and the manager's refutation was wrong (see W11) | **FINDINGS (1 MAJOR, 3 MINOR, 1 NIT)** — W14-W18, all manager-confirmed |


### Out of scope

| Round | Note | Filed in |
|---|---|---|
| R1 | `TestGuideExamples` fails on `dev` (`docs/guides/explain.md` predates `mutable GP_TRACE_FRAME`) | already TODO.md, pre-existing |
| R1 | `docs/guides/explain.md:214-215` embeds the old long non-local headline; regenerating it is blocked by the same `TestGuideExamples` failure, and the new short sentence will appear when that is fixed | nothing — follows from the line above |
| R1 | `internal/explain/examples/macros/main.gp:113` prints two contradictory notes for one name on one line (`captures b by copy` and `captures b by reference`); pre-existing on `1a1224be`, a different mechanism (two literals' notes colocated) | TODO.md, new line |
| R2 | `internal/explain/topics.go:108` says the `<int = N>` "numbers the literal for the stack trace (main.f.func1, as Go names it)" — the same flat-only over-claim W6 removed from the headline. **Manager verified it is pre-existing** (`git diff dev -- internal/explain/topics.go` shows only lines 102-105 changed by this job) and **verified it is false**: a nested literal's `#1` is the trace's `func1.1`. The coder flagged it and correctly left it alone (outside the ticket's files; changing it would put a claim in prose the ticket did not send anyone to check). | **TODO.md, new line** — "`explain` topic prose states the trace number for nested literals" |
| R2 | the `howLiteral` note for `defer`/`go` carried the same false trace sentence two lines above W6's headline; the coder fixed it in the same commit and added `TestDeferredLiteralTraceNote` rather than leaving it for the next round | **in scope and done** — manager verified the old text existed at `f50e67a1` and that the new test fails on it |
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
