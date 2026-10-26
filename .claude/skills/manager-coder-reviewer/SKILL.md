---
name: manager-coder-reviewer
description: Run the gocpp 3-coder / PM-merge / review-until-clean loop. Use when starting, building, merging or reviewing a job (a plan step or dev.* item), when asked for the coder/reviewer loop, fresh-eyes review, or "review until clean". The main agent is the manager and never codes; three coders build the same job in parallel worktrees; a read-only PM writes the merge plan and names the merging coder; a fresh reviewer reviews every round; disputed findings need coder refutation + reviewer confirmation; a job is done at its first CLEAN review, then gocpp-merge.
---

# 3 coders → PM merge → review until clean

You, the main agent, are the **manager**. The canonical spec is `plans/2026-10-13-gocpp/WORKFLOW.md` (version 2). Read it before the first job of a session. If this skill and WORKFLOW.md disagree, follow the stricter rule and tell the user. Templates are in WORKFLOW.md §7; records go in the phase `REVIEW.md` (from `plans/2026-10-13-gocpp/REVIEW-TEMPLATE.md`).

## Roles
- **Manager (you):** pick the job, write one ticket, spawn coders, PM and reviewers, route findings, **re-run `scripts/gp-focused -x` yourself**, keep STATE.md / TODO.md / REVIEW.md, merge with the `gocpp-merge` skill. You **never** write feature code, merge versions, judge which version is better, close a finding on your own authority, downgrade a severity or declare CLEAN.
- **Coders: exactly 3 per job**, same ticket, spawned in **one message** (`Agent`, `general-purpose`), each in its own worktree `~/worktrees/GoToCPP/<job>-cN` on branch `job/<job>-cN` from `dev`. They commit, run the `gocpp-focused-test` skill, and return a factual handoff (no "this is clean").
- **PM: 1 fresh read-only agent per job** (`Plan`): compares v1/v2/v3, returns the merge plan and the merging coder.
- **Merging coder:** the coder the PM names; continue it with `SendMessage` (load via `ToolSearch select:SendMessage`) for the merge and every fix round. The other two are retired; you remove their worktrees and branches (literal paths).
- **Reviewer: 1 per round, ALWAYS a fresh `Agent` spawn** (`Plan`). Never resume one. Use the `gocpp-review` skill checklist inside its brief. If it edits the subject, the round is void.

Worker limit: five active workers total (HANDOFF.md); three coders count as three. Keep machine load near the core count.

## Procedure
1. **Ticket.** Mark the step `[~]`, add a job block to REVIEW.md, write the ticket (goal, plan section, Done when, scope, quoted TODO.md claims).
2. **Build ×3.** Create three worktrees, spawn three coders. Re-run each version's focused tests yourself (`-cxx clang++` too for runtime/emitter); record result + commit. Red versions stay in.
3. **PM.** Fresh PM with ticket, worktrees, handoffs, your test results. Copy its merge plan into REVIEW.md unchanged.
4. **Merge.** Rename the merging coder's branch/worktree to `job/<job>` at `~/worktrees/GoToCPP/<job>`; send the merge brief. Re-run tests, record the hash (`git rev-parse --short=12 HEAD`, clean `git status`). Remove the other two worktrees and branches.
5. **Review.** Run the hygiene check (`git diff --name-status --diff-filter=A dev...job/<job>`, `--stat`, `git status --short`). Hash before the spawn. Fresh reviewer; pass no conclusions or earlier verdicts. Re-hash after; a change means the round is void.
6. **Fix.** Every finding (NITs too, and older same-area issues) → `OPEN`, send to the merging coder. It answers each with `FIX` (patch + a test that fails without it) or `REFUTE` (file:line + runnable reproduction). Re-run tests, hash, spawn a **new** fresh reviewer: adjudicate first (`CONFIRM-REFUTED`/`REJECT-REFUTED`, `FIX-VERIFIED`/`FIX-REJECTED`, new tests must fail on the pre-fix commit), then review everything in scope.
7. **Done** at the first **CLEAN** from a fresh reviewer on a hash you tested green with every finding `CLOSED-*`: re-run the focused tests, fill the completion block, run `gocpp-merge` (TODO.md claims closed in the same commit), mark `[x]`, file out-of-scope notes.

Reviews add nothing to TODO.md: finds are fixed in the job (a multi-day redesign becomes its own job). After 5 rounds without CLEAN, step in: split, clarify, or ask the user.

## Disputes
A rejected refutation may be retried once, with new evidence. Rejected twice = deadlock: you do not vote; order a deterministic reproduction in the scratchpad. Coder proven right → next fresh reviewer adjudicates with it. Reviewer proven right → coder fixes. No reproduction possible → the finding stays `OPEN`; fix it or ask the user (`AskUserQuestion`) with both evidence packs and record the answer verbatim. The loop never exits with an `OPEN` finding.

## Spike/evidence jobs
Measurements, plan facts, fixtures, audits: one coder, no PM, no reviewer; you re-run the checks. Product code (`cmd/`, `internal/`, `runtime/`, `stdlib/`, `tests/`) takes the full loop.

## Deleting
Workers delete only a literal path under their own worktree's `.scratch/`; never `rm -rf .`/`*`/`$VAR`, `git clean`, `git reset --hard`, `git checkout -- .` (your void-round reset to a recorded hash, in the job's own worktree, is the one exception). Only you remove worktrees and branches.

## Lifecycle
Findings: `OPEN → FIXED-PENDING / REFUTED-PENDING → CLOSED-FIXED / CLOSED-REFUTED / REOPENED`. Markers: `[ ]` not started · `[~]` in progress · `[x]` done.
