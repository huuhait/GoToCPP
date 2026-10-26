# Phase NN REVIEW: {title}

> Maintained by the **manager only**, per [WORKFLOW.md](../WORKFLOW.md) (v2: 3 coders → PM merge → review until one CLEAN). The manager is the manager. Coders, the PM and reviewers never edit this file.
> Legend: `[ ]` not_started · `[~]` in_progress · `[x]` completed.
> Finding lifecycle: `OPEN` → `FIXED-PENDING` / `REFUTED-PENDING` → `CLOSED-FIXED` / `CLOSED-REFUTED` / `REOPENED`.
> Every review covers correctness + spec parity, tests + edge cases + determinism, and simplicity + safety, scoped to the job.

## Jobs

| Status | Job | Steps | Rounds | Clean | Completed |
|---|---|---|---|---|---|
| `[ ]` | – | – | – | 0 / 1 | – |

<!--
Copy the block below once per job, under "## Job <ID>". Delete nothing; leave "–" in empty cells.
-->

---

## Job {JOB_ID}: {JOB_TITLE}

**Status:** `[ ]` not_started · **Merging coder:** – · **Canonical worktree:** ~/worktrees/GoToCPP/{job}

### Brief

> (The exact coder brief from WORKFLOW.md §7.1, filled in; the same text went to all three coders.)

- **Goal:**
- **Reference / plan:**
- **Acceptance tests:**
- **Test commands the manager re-runs:**

### Versions

| Version | Coder | Worktree / branch | Manager test run | Commit (state hash) |
|---|---|---|---|---|
| v1 | coder 1 | | | |
| v2 | coder 2 | | | |
| v3 | coder 3 | | | |

### PM merge plan

**PM:** {agent} (fresh, read-only) · **Merging coder:** {1|2|3}

> (The PM's merge plan, copied unchanged: per item/file/section the version taken and why; conflicts and their resolution; what was dropped.)

**Merge result:** {manager test run → result} · **Commit (state hash):** {hash} · **Departures from the plan (from the merge handoff):** {list or "none"}

### Rounds

| Round | Fresh reviewer # | Commit (state hash) | Manager test run | Verdict |
|---|---|---|---|---|
| R1 | | | | |

Verdicts: `FINDINGS (n)` · `CLEAN` · `VOID (reason)`. Every reviewer number is a new instance.

### Findings

| # | Round | Severity | file:line | Failure scenario | Violated rule | Coder response | Adjudication | State |
|---|---|---|---|---|---|---|---|---|
| F1 | | | | | | | | `OPEN` |

- **Coder response:** `FIX` (patch + failing-without test, named) · `REFUTE` (evidence: file:line + reproduction) · `RETRY` (2nd refutation; new evidence only).
- **Adjudication:** `CONFIRM-REFUTED` / `REJECT-REFUTED` / `FIX-VERIFIED (test fails pre-fix: yes)` / `FIX-REJECTED`, with the reviewer # and evidence.
- **State:** never set to `CLOSED-*` by the manager alone. `CLOSED-REFUTED` needs a coder refutation **and** a reviewer `CONFIRM-REFUTED`.

### Disputes

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

- [ ] CLEAN round: {R?}, fresh reviewer {#?}
- [ ] Commit (state hash) of that round: {hash}
- [ ] Final manager test run on that hash: {command → result}
- [ ] Every finding `CLOSED-FIXED` / `CLOSED-REFUTED`: {count}
- [ ] Out-of-scope notes filed: {count}
- [ ] Squash-merged into dev (commit), TODO.md claims closed, step marked `[x]` in the phase `STATE.md`: {date}
- Re-entered by: –
