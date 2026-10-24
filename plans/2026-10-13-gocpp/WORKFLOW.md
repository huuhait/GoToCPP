# How work is done: manager → coder → reviewer

> Every plan step is built by three roles in a loop. The **manager** assigns a job, the
> **coder** builds it, the **reviewer** checks it and gives the findings back, and the coder fixes
> them, round after round, **until the review is clean**. Only then does the step count as done.

| | |
|---|---|
| Applies to | every step of every phase (the `[ ]` lines in `phase-XX-*/STATE.md`) |
| Roles | manager, coder, reviewer: separate agents or people; nobody reviews their own code |
| Done means | the review is clean **and** the step's tests pass (§4) |

---

## 1. The loop

```
                 job ticket
   MANAGER ─────────────────────────► CODER ◄─────────────────────┐
 (plan, STATE,                          │                         │
  TODO, decisions)                      │ change + report          │ findings
      ▲                                 ▼                         │ (not clean)
      │        clean                                              │
      └──────────────────────────── REVIEWER ─────────────────────┘
       verify, mark [x], next job
```

1. The **manager** picks the next step, writes a job ticket (§3.1) and marks the step `[~]`.
2. The **coder** builds it, writes its tests, runs them, and hands over the change with a short
   report (§3.2).
3. The **reviewer** reviews the whole change against the ticket and the plan (§5) and returns a
   review (§3.3) with a verdict: **changes requested** or **clean**.
4. **Changes requested** → the coder answers every finding (fixed, or disagree with a reason) and
   hands over the next round → back to 3.
5. **Clean** → the manager checks it (§4), marks the step `[x]`, updates the plan files (§7) and
   assigns the next job.

## 2. Roles

| Role | Does | Doesn't |
|---|---|---|
| **Manager** | picks steps in dependency order (README §8), splits big steps into small jobs, writes tickets, settles disagreements, keeps STATE.md, TODO.md and the "Decisions / notes" current, asks the user when a decision is theirs | write the feature code, or mark a step done before the review is clean |
| **Coder** | builds exactly what the ticket asks, with tests; runs `make ci`; reports honestly (what changed, how it was tested, what is unsure, where the plan looked wrong) | change the plan's design on their own (they report it), mark steps done, skip or weaken failing tests |
| **Reviewer** | reads the ticket, the plan sections and the whole diff; runs the tests; reports each finding with a severity, a place, a reason and a suggested fix; gives the verdict | fix the code themselves (then nobody reviews that fix), approve while a blocker is open, review their own code |

With Claude Code, the manager is the main session, and the coder and the reviewer are separate
agents. The reviewer starts with a fresh context, so the review is independent of how the code
was written. Several coder/reviewer pairs can work at the same time on jobs that don't depend on
each other (README §8), for example the language server's v0 next to Phase 02.

## 3. Templates

### 3.1 Job ticket (manager → coder)

```markdown
Job 01-1.2: parser patch 1 (Go keywords after `.`, same line only)
Plan: phase-01-frontend/README.md §2.2 "Patch 1", step 1.2
Goal: `std.map[...]`, `node.type()` and `posix.select(...)` parse; nothing else changes.
Done when:
- [ ] tests: `std.map`, `x.type()`, `posix.select`, `x.(type)` unaffected, semicolon after `.range`
- [ ] the upstream go/scanner + go/parser tests pass, incl. `TestIncompleteSelection`
- [ ] `make ci` is green (GCC + Clang)
Scope: `internal/syntax/` only.
Notes: spikes/2026-10-13/frontend/parsecheck shows the inputs that fail today.
```

### 3.2 Coder report (coder → reviewer, every round)

```markdown
Job 01-1.2, round 1
Changed: internal/syntax/scanner/scanner.go (keyword-after-dot rule), scanner_test.go (new tests)
Tested: `make ci` green; upstream tests unchanged
Unsure: a keyword after `.` at the end of a line, followed by a comment
Plan issues: none
```

From round 2 on, the report starts with one line per finding of the last review:
`1 fixed: <how>` or `2 disagree: <why>`.

### 3.3 Review (reviewer → coder)

```markdown
Review of job 01-1.2, round 1: changes requested
1. [blocker] scanner.go:412: a keyword on the NEXT line after `.` also becomes a name, but the
   plan says same line only (`TestIncompleteSelection` covers the editor case). Fix: compare the
   line of the `.` with the line of the keyword.
2. [should-fix] scanner_test.go: no test for `.range` followed by a newline (semicolon insertion).
3. [nit] scanner.go:405: the comment says "after a dot"; the code checks the previous token.
Checked: the whole diff, plan §2.2, `make ci` (green), the upstream tests.
```

**Severities**
- **blocker**: wrong behavior, a required test missing or failing, a plan rule broken, undefined
  behavior, a speed budget broken;
- **should-fix**: a real problem that isn't wrong yet (a missing edge-case test, code that invites
  bugs, user-visible behavior without docs);
- **nit**: style, names, comments. Optional: the coder may decline with a reason.

## 4. When is it clean?

A step is done only when **all** of these hold:
- the last review says **clean**: no blocker and no should-fix finding is left (nits may stay);
- every box in the ticket's "Done when" is ticked, and `make ci` is green on both compilers, plus
  the sanitizer jobs that apply (ASan/UBSan for runtime code, TSan for concurrency);
- the phase README still describes what was built. If the build differs from the plan, the
  manager accepted that and wrote it into the phase STATE.md "Decisions / notes";
- the manager checked these points themselves: they re-run the tests instead of only reading the
  report.

## 5. What the reviewer checks (gocpp checklist)

- **Go behavior**: the output matches real `go run` (a differential test) wherever Go has the same
  feature, or a documented divergence (Phase 00 §2.2) applies.
- **Tests**: new behavior has run tests, error paths have error tests (`// ERROR`), and golden
  files changed only on purpose.
- **Generated C++**: compiles with GCC and Clang using the plan's flags, has no undefined behavior
  (sanitizers), stays readable, and its `#line` directives point at the right `.gp` lines.
- **Front-end**: no new false errors in the interop corpus; messages read like Go's.
- **Plan conformance**: the code does what the phase README says; anything different is reported,
  never hidden.
- **Speed**: the language server budgets (Phase 11) and build times (Phase 06) don't get worse.
- **Platforms**: in the runtime, nothing outside `runtime/src/os/` calls the operating system
  directly (that folder also holds the Phase 08 thread layer `gp::os`); the standard library's
  own per-OS files (Phase 09, Phase 14) are the other allowed place. Linux and Windows are both
  targets.
- **TODO.md**: the TODO.md claims the job fixes are listed in the report and will be closed at
  the merge (7.1); a fixed claim the report forgets, or a claim marked fixed that is still
  reproducible, is a finding.
- **Scope and quality**: only the ticket's change; Go code is gofmt- and vet-clean; C++ follows
  `.clang-format`; comments explain what isn't obvious.

## 6. Rules for the loop

- **Deleting (user, 2026-10-22: an agent once wiped the home folder):** a worker deletes only inside
  its own worktree's `.scratch/`, and only with a literal path that starts with that `.scratch/`
  folder: `rm -rf "$W/.scratch/devsrc2"`, never `rm -rf .`, `rm -rf *`, `rm -rf "$VAR"` with an
  unchecked variable, `rm -rf ~...` or `/...` outside the worktree. Never `git clean`, `git reset
  --hard`, `git checkout -- .`. When in doubt, don't delete: leave it and say so in the report.
- **Every finding gets an answer** in the next round: fixed (how) or disagree (why). No answer
  means not fixed.
- **The reviewer re-reviews the whole change** every round, not only the fixes, because a fix can
  break something else.
- **Disagreements** go to the manager, who decides. When it is really a design decision the plan
  doesn't settle, the manager asks the user, with a small code example the user can edit and send
  back.
- **Too many rounds**: after 5 rounds without a clean review, the manager steps in: split the
  job, make the ticket clearer, or take the question to the user.
- **When the plan is wrong**: if the coder or the reviewer finds that the plan itself doesn't
  work, they report it. The manager updates the phase README and STATE.md (with the reason) before
  the job goes on.
- **Small jobs**: one job is one reviewable change, ideally a few hundred lines. A big step becomes
  several jobs (`01-1.8a`, `01-1.8b`).

## 7. Updating the plan files

| When | The manager updates |
|---|---|
| a job starts | the step to `[~]` in the phase STATE.md, and the phase line in the root STATE.md if it's the phase's first job |
| a review is clean | the step (and its sub-steps) to `[x]`, plus a one-line note when something notable happened ("1.2 clean after 2 rounds; needed the line of the `.` token") |
| the plan changed | the phase README, plus a dated line in "Decisions / notes" |
| all exit criteria of a phase are `[x]` | the phase to `[x]` in both STATE files, the milestone row and "Current focus" |
| after every job | TODO.md "Now" shows the next jobs |
| a job merges | every TODO.md claim the job fixed is ticked `[x]` with `(done: <commit>)` or deleted, in the same commit as the merge (see 7.1) |

### 7.1 TODO.md must not go stale

TODO.md holds open claims only: a "lows" or "follow-ups" bullet is a list of separate claims,
and each claim is closed on its own.

- **Ticket:** the coder's ticket lists the TODO.md claims the job is meant to fix (quote the
  line or its first words), and the coder's report says, claim by claim, `fixed`, `already done
  (commit)`, `skipped (reason)`. Before coding a claim, check it is still open on dev
  (`rg`, `git log -S`, a one-minute repro): a stale claim is reported, not worked on.
- **Merge:** the manager closes the fixed claims in the same commit as the merge: the claim is
  ticked `[x]` with `(done: <short commit>)` when the bullet still has open claims, or the whole
  bullet is deleted when all its claims are done. A claim the job found obsolete is deleted with
  a one-line note in the commit message. Priority words (`PRIORITY`) go with the claim they
  belong to.
- **Review:** the reviewer checks this: a merge that fixes a claim of TODO.md and leaves it open
  is a finding (severity: should-fix), and so is a claim the report says is fixed that is not.
- **Review finds are fixed in the job (user, 2026-10-23):** every issue a review finds, in the
  ticket's scope, an older one in the same area, or one unrelated to the ticket, goes back to the
  coder as a finding of the next round and is fixed there, fully (no workaround, no "document it"
  when a real fix is possible). Reviews add nothing to TODO.md. Only a find that needs a multi-day
  redesign becomes its own job, chosen by the manager; a job merges only when it closes more TODO.md
  lines than it adds. Cosmetic notes go in the merge commit message, not TODO.md.
- **Audit:** once per phase (and before each release step) the manager checks a sample of the
  open claims against the code; a claim that cannot be reproduced is deleted. The last full
  audit is in AUDIT-2026-10-22.md.

**Commits:** one commit per clean job, with a message like
`phase 01, step 1.2: parser patch 1 (keywords after .)`; a job split from a step uses the job
name (`phase 01, step 1.8a: …`). The manager makes them only once the user
has said commits are OK; until then, nothing is committed.
