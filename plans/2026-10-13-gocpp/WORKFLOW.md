# How work is done: 3 coders → PM merge → review until clean

> **Version 2, adopted 2026-10-26** (replaces v1, the single-coder manager → coder → reviewer loop;
> v1 is in git history). Same loop as `~/GoLLVMCmake` (its WORKFLOW.md v2), adapted to gocpp's
> worktrees, `dev` squash merges and TODO.md claims.
> **Applies to:** every job and every step of every phase (the `[ ]` lines in `phase-XX-*/STATE.md`).
> **Skill:** [`.claude/skills/manager-coder-reviewer/SKILL.md`](../../.claude/skills/manager-coder-reviewer/SKILL.md) runs the loop.
> **Techlead:** [`.claude/agents/techlead.md`](../../.claude/agents/techlead.md) is the performance researcher; three run before a performance-shaped job (§1, step 0).
> **Records:** one `REVIEW.md` per phase folder (and `plans/2026-10-13-gocpp/REVIEW-dev.md` for `dev.*` jobs), made from [REVIEW-TEMPLATE.md](REVIEW-TEMPLATE.md).
> Legend: `[ ]` not started · `[~]` in progress · `[x]` done.

If the skill and this file disagree, the stricter rule wins and the mismatch is fixed.

| | |
|---|---|
| Roles | manager (the orchestrator), 3 techleads for performance-shaped jobs, 3 coders, 1 PM, 1 fresh reviewer per round |
| Done means | the **first CLEAN** review from a fresh reviewer on a commit the manager tested green (§4) |

---

## 1. Roles

| Role | Who | Does | Never |
|---|---|---|---|
| **Manager** | the main agent | picks the job, writes the brief, spawns the coders, the PM and the reviewers, routes findings, **re-runs the focused tests itself**, keeps STATE.md / TODO.md / REVIEW.md, merges (gocpp-merge skill), asks the user when a decision is theirs | writes feature code · merges versions · judges which version is better · closes a finding on its own authority · downgrades a severity · declares anything CLEAN |
| **Techlead** | **3 per performance-shaped job** (`.claude/agents/techlead.md`), read-only, spawned in one message | each finds the bottleneck **independently**, measures it, and returns a **techlead brief** (§3 step 0): baseline command + numbers, where the cost is, root cause or `HYPOTHESIS`, the smallest optimization, the ceiling, the guardrails | writes product code or plan files · edits the job branch · guesses without labelling it · broadens past the one target |
| **Coders** | exactly **3** subagents per job, same brief | each builds the whole job independently in its own worktree, with tests, runs `scripts/gp-focused -x`, returns a **factual** handoff (files changed, commands + results, decisions, limits, TODO.md claims: `fixed` / `already done (commit)` / `skipped (reason)`) | looks at another coder's worktree · says "this is correct / clean" · edits plan files · merges or pushes |
| **PM** | **1** fresh read-only subagent per job | compares the 3 versions with the brief, writes the **merge plan** and names the **merging coder** | writes code · edits any file · reviews the merged result |
| **Merging coder** | the coder the PM names (continued, not respawned) | builds the merged result on the job branch, then answers every finding: **FIX by default**, or one `REFUTE` with evidence | departs from the merge plan without saying so · refutes a finding twice · refuses a finding it could fix |
| **Reviewer** | exactly **1** per round, **always a fresh spawn**, read-only | reviews the merged result, adjudicates refutations, verifies fixes, returns `FINDINGS` or `CLEAN` | edits the subject (then the round is void, §5.3) · is resumed · is told earlier verdicts or the coders' / PM's conclusions |

- The two coders the PM does not name are retired after the merge; their worktrees and branches are
  removed by the manager. A new job starts 3 new coders and a new PM.
- **Every review round is a brand-new agent.** A resumed reviewer anchors on its old findings.
- With Claude Code the manager spawns coders and the PM with `Agent` (coders `general-purpose`, PM
  `Plan`), continues the merging coder with `SendMessage`, and spawns reviewers with `Plan` (no edit tools).
- Jobs that do not depend on each other may run in parallel. **Worker limit:** the user's limit is
  five active workers total (HANDOFF.md); a job's three build-phase coders count as three. Keep the
  machine load near the core count (HANDOFF.md, load warning): when load is high, run fewer jobs at once.
- **Techlead stage (performance-shaped jobs only).** When the job's point is to make something
  faster, cheaper in memory or smaller (a TODO.md performance line, a benchmark row over budget, a
  `15.x` optimization, the perf stage of a Runtime line, a "why is this slow" question), the manager
  runs step 0 **before** the coders: three **techleads** research the same target independently,
  and the manager merges their briefs into the one ticket the three coders get. The techleads count
  as three workers while they run, so the coders start when they release the slots. This is the same
  independence principle as the three coders: three separate localizations of one bottleneck.
  A single-measurement or spike job still uses the one-coder spike rule (§6).

## 2. Terms

| Term | Meaning |
|---|---|
| **Job** | One reviewable unit of work, normally one plan step or a `dev.*` item; big steps split (`01-1.8a`). |
| **Version** | One coder's complete result: branch `job/<job>-c1`, `-c2`, `-c3`, each in `~/worktrees/GoToCPP/<job>-cN`. Coders commit on their branch (uncommitted work does not count). |
| **Canonical branch** | `job/<job>` at `~/worktrees/GoToCPP/<job>`: the merged result; it is what gets reviewed and squash-merged into `dev`. |
| **Round** | One reviewer spawn and its verdict: R1, R2, … |
| **Techlead brief** | One techlead's performance finding: baseline command and numbers, the layer that owns the cost, the share, root cause or `HYPOTHESIS`, the smallest optimization, the ceiling and the guardrails (`.claude/agents/techlead.md` §4). Three are merged into the ticket before the coders. |
| **State hash** | `git rev-parse --short=12 HEAD` of the canonical branch, with `git status --short` empty. Equal hash = unchanged. (This replaces the rsync snapshots of the GoLLVMCmake loop: commits are the snapshots.) |
| **Scope** | What a reviewer judges: the job's acceptance list, the diff `dev...job/<job>`, the plan lines that state its results, and the TODO.md claims the brief names. |

## 3. The loop

```text
 ┌─ 0 RESEARCH ────── (performance-shaped jobs) manager: 3 techleads on one target, independent
 │                    → 3 briefs → manager merges the evidence into the ticket
 │
 ├─ 1 BRIEF ───────── manager: pick job, mark [~], one brief (ticket) for all three coders
 │
 ├─ 2 BUILD ×3 ────── coders 1-3 in parallel, own worktree each → 3 factual handoffs
 │                    manager: re-run each version's focused tests; record results
 │
 ├─ 3 PM MERGE PLAN ─ fresh PM (read-only): compare v1/v2/v3 → merge plan + merging coder
 │
 ├─ 4 MERGE ───────── merging coder: build job/<job> from the plan → handoff
 │                    manager: focused tests, state hash
 │
 ├─ 5 REVIEW ──────── fresh reviewer: FINDINGS or CLEAN (scoped to the job)
 │
 ├─ 6 FIX ─────────── any finding → merging coder: FIX by default (+ test), or one REFUTE (+ repro)
 │                    manager: tests, hash; back to 5 with a NEW fresh reviewer, which
 │                    adjudicates each refutation ONCE (final) — the manager closes on that
 │                    verdict — then reviews everything in scope
 │
 └─ 7 DONE ────────── first CLEAN on a hash tested green → gocpp-merge, mark [x]
```

### Step 0: Research (performance-shaped jobs)
1. Decide the job is performance-shaped by §1's test, mark the step `[~]`, and spawn **three
   techleads in one message** (`.claude/agents/techlead.md`) on the same target at the same baseline
   commit. They read the shared checkout read-only and write only under their own
   `.scratch/techlead-<n>/`; each gets its own build cache (`GOCPP_*_CACHE`, `CCACHE_DIR`) so three
   concurrent measurements do not perturb each other. None is told about the others.
2. Each returns a **techlead brief** (baseline command + numbers, layer, share, root cause or
   `HYPOTHESIS`, proposed change, expected recovery, ceiling, guardrails, risks).
3. **The manager merges the three briefs into the ticket.** Agreement between two or three
   independent localizations is what justifies acting; a claim made by only one and not proven is
   labelled `HYPOTHESIS` in the ticket and the ticket says what the coder must measure to confirm it
   before optimizing. The manager never picks a winner by preference, only by evidence.
4. Record the three briefs and the merged version in the job block of REVIEW.md. The manager
   re-runs the baseline command itself before the coders start; a baseline it cannot reproduce is
   reported to the techleads, not passed on.
5. Non-performance jobs skip step 0 and go straight to step 1.

### Step 1: Brief
1. Pick one job in dependency order (README §8); split big steps into small jobs.
2. Mark the step `[~]` in the phase STATE.md; open a job block in the phase `REVIEW.md`.
3. Write **one** ticket (§7.1): goal, plan sections, "Done when" tests, scope, and the TODO.md claims the
   job is meant to fix (quote the line). Copy it into REVIEW.md.

### Step 2: Build ×3
1. Create three worktrees from `dev` (`git worktree add -b job/<job>-cN ~/worktrees/GoToCPP/<job>-cN dev`).
   Spawn the 3 coders **in one message**. Each gets the same ticket plus its own worktree; none is told
   more than "two other coders do the same task independently".
2. Each coder checks every TODO.md claim is still open on `dev` before coding it (`rg`, `git log -S`,
   `scripts/gp-repro`); a stale claim is reported, not worked on. Each commits and returns a handoff.
3. **The manager re-runs each version's focused tests itself** (`scripts/gp-focused -x`, plus
   `-cxx clang++` when the runtime or emitter changed) and records command, result and commit in the
   Versions table. A red version stays in the comparison.

### Step 3: PM merge plan
1. Spawn a **fresh PM** with the ticket, the 3 worktrees, the 3 handoffs and the manager's test results.
2. It returns a **merge plan**: per acceptance item / file, which version to take (`v1`/`v2`/`v3`/combine)
   with a one-line reason; every conflict between versions and how to resolve it; what to drop and why;
   and the **merging coder** (the best base, so the merge moves the fewest parts). "Take one version
   whole" is valid.
3. The manager copies the plan into REVIEW.md unchanged and does not second-guess it.

### Step 4: Merge
1. The manager renames the merging coder's branch and worktree to the canonical ones
   (`git branch -m job/<job>-cN job/<job>`, `git worktree move ... ~/worktrees/GoToCPP/<job>`), then sends the merge plan (§7.3). The coder merges parts from
   the other worktrees as the plan says, commits, runs focused tests and lists every departure.
2. The manager re-runs focused tests, records the hash, then removes the two other worktrees and
   branches (literal paths only, §6).

### Step 5: Review
1. Spawn a **fresh reviewer** (§7.4) with the ticket, the plan sections, the worktree and `dev...job/<job>`.
   It gets **no** conclusions of coders or PM, no earlier verdicts, no round count (open findings
   under adjudication are given with their evidence).
2. One review covers: correctness + Go behavior (the checklist in §8), tests + edge cases + determinism,
   simplicity + safety.
3. **Scope:** only what is in scope (§2). Anything wrong outside it goes under OUT OF SCOPE; it does not
   block. Exception (user, 2026-10-23): an older issue in the **same area** is in scope and is fixed in
   the job.
4. It returns exactly `FINDINGS` (numbered; severity §5; `file:line`; failure scenario; violated rule;
   **a NIT counts**) or the single word `CLEAN`.
5. The manager checks the hash after the round; a changed hash voids it (§5.3).

### Step 6: Fix
1. Record every finding `OPEN`; send the list to the merging coder (§7.5). Its standing bias is
   **FIX**: the coder's job is to close TODO.md `[ ]` lines, so a finding is treated as more TODO
   work, not as an argument to win. It re-checks each, then:

| Response | Requires | Finding moves to |
|---|---|---|
| `FIX` (default) | the patch **and** a test that **fails without the patch** | `FIXED-PENDING` |
| `REFUTE` (exception, single-shot) | `file:line` plus a runnable reproduction (`scripts/gp-repro` against real Go where it applies); never refuted twice | `REFUTED-PENDING` |

   "I disagree" without proof is no response, and the default is still to fix it. For
   non-behavioural findings (a comment, a doc line) the coder says in one line that no test can fail;
   the adjudicating reviewer must agree.
2. The manager re-runs the tests, records the hash, spawns a **new fresh reviewer**, who works in order:
   refutations → `CONFIRM-REFUTED` / `REJECT-REFUTED` (one decision, final); fixes → `FIX-VERIFIED`
   (new test fails on the pre-fix commit) / `FIX-REJECTED`; then a full review in scope.
3. **The manager closes on the reviewer's word alone.** `CONFIRM-REFUTED` → `CLOSED-REFUTED`; no
   re-check, no second reviewer, no judgement call. That is the whole point of keeping refutations
   single-shot: a rejection costs one adjudication and then disappears.

| Before | Reviewer says | After |
|---|---|---|
| `REFUTED-PENDING` | `CONFIRM-REFUTED` | `CLOSED-REFUTED` |
| `REFUTED-PENDING` | `REJECT-REFUTED` | `REOPENED` |
| `FIXED-PENDING` | `FIX-VERIFIED` | `CLOSED-FIXED` |
| `FIXED-PENDING` | `FIX-REJECTED` | `REOPENED` |

### Step 7: Done
At the **first CLEAN** from a fresh reviewer, on a hash the manager tested green, with every finding
`CLOSED-*`, the manager: re-runs the focused tests on that hash; fills the completion block; merges
with the gocpp-merge skill (squash into `dev`, TODO.md claims closed in the same commit, push, remove
worktree and branch); marks the step `[x]` in the phase and root STATE.md; files out-of-scope notes.

## 4. Exit rule
1. One `CLEAN` from a fresh reviewer ends the job if: the manager's focused run is green on that hash;
   no finding is outside `CLOSED-*`; the round was not void.
2. Any finding at any severity means another fix round and another fresh reviewer.
3. A clean verdict on a hash that later changes does not count.
4. Green CI alone is not approval. Never `make ci` or full sweeps for a job; focused tests only (gocpp-focused-test).

## 5. Severities, mechanics, voids

### 5.1 Severities
All four block the exit; severity sets the order of work.

| Severity | Use for |
|---|---|
| `BLOCKER` | wrong Go behavior, crash, UB, data loss, a required/acceptance test missing or failing, a plan rule or speed budget broken |
| `MAJOR` | wrong behavior in a realistic case or on an error path, a plan deviation, a test that cannot fail, a TODO.md claim marked fixed that is not |
| `MINOR` | wrong behavior in an unlikely case, code that invites bugs, user-visible behavior without docs, a fixed claim left open in TODO.md |
| `NIT` | style, names, comments, a stale number |

The manager never downgrades a severity.

### 5.2 State hash and hygiene
Take the hash (§2) after every coder turn and right before and right after every review round.
Before the merge, the manager runs the hygiene check: `git diff --name-status --diff-filter=A dev...job/<job>`,
`git diff --stat`, `git status --short`. Binaries, `.scratch` content, helper scripts, logs, files over
~1 MB, edits outside the scope, uncommitted work: findings.

### 5.3 Void rounds
A round is **void** when the hash after it differs from before, when `git status` shows reviewer edits
in the worktree, or when the reviewer was given conclusions or an earlier verdict. Reset the branch to
the pre-round hash (`git reset --hard <hash>` is allowed here only inside the job's own worktree and only
to a hash the manager recorded; otherwise recreate the worktree), confirm the hash, log `VOID`, spawn a new reviewer.
Reviewers may run commands and make copies under their own `.scratch/` or the scratchpad.

## 6. Rules for the loop

- **Deleting (user, 2026-10-22: an agent once wiped the home folder):** a worker deletes only inside
  its own worktree's `.scratch/`, and only with a literal path that starts with that `.scratch/` folder:
  `rm -rf "$W/.scratch/devsrc2"`, never `rm -rf .`, `rm -rf *`, `rm -rf "$VAR"` with an unchecked
  variable, `rm -rf ~...` or `/...` outside the worktree. Never `git clean`, `git reset --hard`,
  `git checkout -- .` (the one exception is the manager's void-round reset, §5.3). When in doubt, don't
  delete: leave it and say so in the report. Only the manager removes worktrees and branches.
- **Disputes (single pass, user 2026-10-26).** A coder's job is to **close TODO.md `[ ]` lines**, so
  the default answer to every finding is `FIX`; refusing is the exception. A `REFUTE` is
  **single-shot**: it never loops back to the coder. The next fresh reviewer adjudicates it — one
  reviewer, one decision. `CONFIRM-REFUTED` → the manager marks it `CLOSED-REFUTED` and moves on,
  with no second opinion, no escalation and no debate. `REJECT-REFUTED` → the finding is `REOPENED`,
  the coder fixes it in that round, and may not refute it again. There is no retry-with-new-evidence
  and no deadlock path: the reviewer's adjudication is final in both directions. Only a claim neither
  side can demonstrate at all goes to the user, with both evidence packs. **The loop never exits with
  an `OPEN` finding.**
- **Review finds are fixed in the job (user, 2026-10-23):** every issue a review finds, in the ticket's
  scope, an older one in the same area, or one unrelated, goes back to the merging coder as a finding of
  the next round and is fixed fully (no workaround, no "document it" when a real fix is possible).
  **The coder exists to shrink TODO.md** (user, 2026-10-26): its default is to fix, not to refuse, so a
  finding is one more `[ ]` line closed. Reviews add nothing to TODO.md. Only a find that needs a
  multi-day redesign becomes its own job, chosen by the manager; a job merges only when it closes more
  TODO.md lines than it adds. Cosmetic notes go in the merge commit message.
- **Too many rounds:** after 5 rounds without CLEAN, the manager steps in: split the job, clarify the
  ticket, or take the question to the user.
- **When the plan is wrong:** coders, PM and reviewers report it; the manager updates the phase README
  and STATE.md (with the reason) before the job goes on. Design decisions the plan does not settle go to
  the user, with a small code example they can edit and send back.
- **Small jobs:** one job is one reviewable change, ideally a few hundred lines.
- **Spike/evidence jobs** (measurements, plan facts, fixtures, audits; the ticket says "spike"): **one
  coder, no PM, no reviewer** (the GoLLVMCmake rule, WORKFLOW §10, for work that is not product code).
  The manager re-runs the checks itself; green → `[x]`, red → back to the coder. Product code in
  `cmd/`, `internal/`, `runtime/`, `stdlib/`, `tests/` always takes the full loop.

## 7. Prompt templates (fill the brackets; keep the rest)

### 7.0 Techlead brief (same text for all three, performance-shaped jobs only)
> You are TECHLEAD [1|2|3] of three in a build/merge/review loop (see `plans/2026-10-13-gocpp/WORKFLOW.md` §3 step 0). Two other techleads research the same target independently; do not look at their work. Follow `.claude/agents/techlead.md`. Target: [the benchmark row / TODO line / function / regression]. Baseline commit: [hash]. Worktree: [path]; copy what you need under your own `.scratch/` (a literal path inside it). Read-only otherwise: do not edit product code, plan files or the job branch; never `make ci`, `verify.sh`, full sweeps or sanitizer campaigns. Measure on a quiet machine and record the load. Keep workload, build flavor, binary identity and run order identical when comparing. Return the TECHLEAD BRIEF of `.claude/agents/techlead.md` §4, filled in, with the exact command behind every number; label anything unproven `HYPOTHESIS` and say the experiment that would confirm it. Do not write the fix.

### 7.1 Ticket / coder brief (same text for all three)
> You are CODER [1|2|3] of three in a build/merge/review loop (see `plans/2026-10-13-gocpp/WORKFLOW.md`). Two other coders do the same job independently; do not look at their worktrees. Your worktree: [~/worktrees/GoToCPP/<job>-cN] on branch [job/<job>-cN]. Job: [name + title]. Plan: [phase README section]. Goal: [...]. Done when: [tests]. Scope: [dirs]. TODO.md claims to fix: [quoted lines]. Check each claim is still open on dev before coding it. Implement the whole job, add tests, run `scripts/gp-focused -x` (and `-cxx clang++` for runtime/emitter changes), commit on your branch, and return a factual handoff: files changed, commands + results, decisions, limits, claim by claim `fixed` / `already done (commit)` / `skipped (reason)`. Do not claim the work is correct or clean. Do not edit plan/STATE/TODO files. Delete only a literal path under your own `.scratch/`; never merge, push or remove worktrees.

### 7.2 PM brief
> You are the PM for job [name] (see `plans/2026-10-13-gocpp/WORKFLOW.md` §3 step 3). Read-only: edit nothing. Three coders built the same job independently: v1 at [path], v2 at [path], v3 at [path]; their handoffs and the manager's test results are below. Ticket: [ticket]. Compare the three against the ticket and "Done when". Return a MERGE PLAN: per item / file, which version to take (v1/v2/v3/combine) and a one-line reason; every conflict between versions and how to resolve it; what to drop and why; and the MERGING CODER (1, 2 or 3): the best base. Judge the work, not the handoffs' claims: open the files and run things under `.scratch/` where it helps.

### 7.3 Merge brief (to the merging coder)
> You are the MERGING CODER for job [name]. The PM's merge plan is below. Your worktree is now [~/worktrees/GoToCPP/<job>] on branch [job/<job>]. Build the merged result there, taking parts from the other versions at [paths] as the plan says. Run the focused tests, commit, and return a factual handoff: files changed, commands + results, every departure from the plan with the reason. You will also answer the reviewers' findings. Do not edit plan/STATE/TODO files.

### 7.4 Reviewer brief
> You are a FRESH-EYES REVIEWER (new instance). Read-only: edit nothing in [worktree]; run things under the scratchpad or `.scratch/` only. Review job [name] (`git diff dev...job/<job>` in [worktree]) against [ticket + plan sections]. Cover Go behavior (prove edge cases with `scripts/gp-repro`), tests + edge cases + determinism, simplicity + safety, added-file hygiene, and the TODO.md claims (each claimed fix is really fixed; none forgotten). Scope: the acceptance list, the diff, the plan lines that state its results, and older issues in the same area; list anything else under OUT OF SCOPE. Assume bugs exist; be adversarial. Run `scripts/gp-focused -x`. Return FINDINGS (numbered; severity BLOCKER|MAJOR|MINOR|NIT; file:line; failure scenario; violated rule) or the single word CLEAN only if nothing in scope is wrong. [If adjudicating:] first adjudicate the attached refutations (CONFIRM-REFUTED / REJECT-REFUTED with evidence) and verify the fixes (FIX-VERIFIED / FIX-REJECTED), including whether new tests fail on the pre-fix commit [hash].

### 7.5 Coder response brief
> You are the MERGING CODER. These are reviewer findings: [list]. Your job is to **close TODO.md `[ ]` lines**, so the default answer to every finding is `FIX`, even one you think is minor: treat it as more TODO work, not as an argument to win. Re-check each against the code and the plan first. Per finding: `FIX` (patch + a test that fails without it) — the default — or `REFUTE` **once**, with `file:line` and a reproduction that settles it, and never a second refutation of the same finding. Commit, run the focused tests, return the handoff. A fresh reviewer adjudicates each refutation in a single pass and a full review follows; the manager closes a confirmed refutation without a second look, so a refutation is your only cost-of-refusal decision.

## 8. What the reviewer checks (gocpp checklist)

- **Go behavior**: output matches real `go run` (a differential test) wherever Go has the same feature,
  or a documented divergence (Phase 00 §2.2) applies.
- **Tests**: new behavior has run tests, error paths have error tests (`// ERROR`), golden files change
  only on purpose, and every new test fails without the change.
- **Generated C++ / runtime**: compiles with GCC and Clang using the plan's flags, no undefined behavior
  (targeted sanitizers), readable, `#line` points at the right `.gp` lines; nothing outside
  `runtime/src/os/` calls the operating system directly (the stdlib's per-OS files are the other allowed
  place); Linux and Windows both stay targets.
- **Front-end**: no new false errors in the interop corpus; messages read like Go's.
- **Plan conformance**: the code does what the phase README says; differences are reported, never hidden.
- **Speed**: language server budgets (Phase 11) and build times (Phase 06) do not get worse.
  For a performance job, the ticket's threshold must actually be met, measured with the command
  the techlead brief named, on the same flavor and workload; an "expected" recovery that did not
  materialize is a finding, and the coder's numbers are re-measured by the reviewer.
- **TODO.md**: claims the job fixes are listed in the report and closed at the merge (§9.1); a fixed
  claim the report forgets, or marked fixed but still reproducible, is a finding.
- **Scope and quality**: only the ticket's change; Go code gofmt- and vet-clean; C++ follows
  `.clang-format`; comments explain what is not obvious.

## 9. Updating the plan files

| When | The manager updates |
|---|---|
| a performance job starts | run step 0 first: three techlead briefs and the merged ticket in REVIEW.md, before the coders |
| a job starts | the step to `[~]` in the phase STATE.md (and the root STATE.md phase line on the phase's first job); a job block in REVIEW.md |
| each phase of the loop | REVIEW.md: versions, merge plan, rounds, findings (the manager is the only writer) |
| a CLEAN review | the step to `[x]`, plus a one-line note ("1.2 clean after 2 rounds") |
| the plan changed | the phase README, plus a dated line in "Decisions / notes" |
| all exit criteria of a phase are `[x]` | the phase to `[x]` in both STATE files, the milestone row and "Current focus" |
| after every job | TODO.md "Now" shows the next jobs |
| a job merges | every TODO.md claim fixed is ticked `[x] (done: <commit>)` or deleted, in the same commit as the merge |

**Phase gate:** a phase is `[x]` only when every job in its `REVIEW.md` has a CLEAN review.

### 9.1 TODO.md must not go stale
TODO.md holds open claims only: a "lows" or "follow-ups" bullet is a list of separate claims, each closed
on its own. The ticket lists the claims; each coder checks they are still open and reports claim by claim.
At the merge the manager ticks `[x] (done: <short commit>)` when the bullet still has open claims, or
deletes the whole bullet when all are done; an obsolete claim is deleted with a note in the commit
message. Priority words go with their claim. Once per phase (and before each release step) the manager
audits a sample of open claims against the code; one that cannot be reproduced is deleted. The last full
audit is AUDIT-2026-10-22.md.

**Commits:** one squash commit per clean job (gocpp-merge), e.g. `phase 01, step 1.2: parser patch 1
(keywords after .)`; a split job uses the job name (`phase 01, step 1.8a: …`). Coder commits on `job/*`
branches are not pushed. The manager merges and pushes only once the user has said commits are OK.

## 10. Transition from v1
Jobs already in flight under v1 (one coder, one reviewer) finish under v1's exit rule: their next round
uses a fresh reviewer, and they are merged as before. Every job started after 2026-10-26 uses this loop.
