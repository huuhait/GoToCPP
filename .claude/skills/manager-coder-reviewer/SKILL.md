---
name: manager-coder-reviewer
description: Run the gocpp 3-coder / PM-merge / review-until-clean loop. Use when starting, building, merging or reviewing a job (a plan step or dev.* item), when asked for the coder/reviewer loop, fresh-eyes review, or "review until clean". The main agent is the manager and never codes; three coders build the same job in parallel worktrees; a read-only PM writes the merge plan and names the merging coder; a fresh reviewer reviews every round; disputed findings need coder refutation + reviewer confirmation; a job is done at its first CLEAN review, then gocpp-merge.
---

# 3 coders → PM merge → review until clean

You, the main agent, are the **manager**. The canonical spec is `plans/2026-10-13-gocpp/WORKFLOW.md` (version 2). Read it before the first job of a session. If this skill and WORKFLOW.md disagree, follow the stricter rule and tell the user. Templates are in WORKFLOW.md §7; records go in the phase `REVIEW.md` (from `plans/2026-10-13-gocpp/REVIEW-TEMPLATE.md`).

## Roles
Each role has a definition file under `.claude/agents/`; spawn it with `Agent` and `subagent_type` set to that name.
- **Manager (you):** pick the job, write one ticket, spawn coders, PM and reviewers, route findings, **re-run `scripts/gp-focused -x` yourself**, keep STATE.md / TODO.md / REVIEW.md, merge with the `gocpp-merge` skill. You **never** write feature code, merge versions, judge which version is better, close a finding on your own authority, downgrade a severity or declare CLEAN.
- **Techleads: 3 per performance-shaped job** (`techlead`), spawned in **one message**, read-only. Each finds the bottleneck independently, measures it and returns a techlead brief. You merge the three briefs into the ticket the coders get; you never pick a winner by preference, only by evidence. **This stage runs before the coders and counts as three workers**, so the coders start when it releases the slots.
- **Coders: exactly 3 per job** (`coder`), same ticket, spawned in **one message**, each in its own worktree `~/worktrees/GoToCPP/<job>-cN` on branch `job/<job>-cN` from `dev`. They commit, run the `gocpp-focused-test` skill, and return a factual handoff (no "this is clean").
- **PM: 1 fresh read-only agent per job** (`pm`): compares v1/v2/v3, returns the merge plan and the merging coder.
- **Merging coder:** the coder the PM names; continue it with `SendMessage` (load via `ToolSearch select:SendMessage`) for the merge and every fix round. The other two are retired; you remove their worktrees and branches (literal paths).
- **Reviewer: 1 per round, ALWAYS a fresh spawn** (`reviewer`). Never resume one. It follows the `gocpp-review` skill checklist. If it edits the subject, the round is void.

Worker limit: five active workers total (HANDOFF.md); three coders count as three, three techleads too. Keep machine load near the core count.

## Procedure
0. **Research** (performance-shaped jobs only). If the job's point is speed, memory or size (a performance TODO line, a benchmark row over budget, a `15.x` optimization, the perf stage of a Runtime line), spawn **three techleads** on the same target and baseline commit (`Agent`, all in one message, each told only its own target), collect the three briefs, and merge the evidence into the ticket. Agreement between independent localizations is what justifies acting; a single unproven claim goes in labelled `HYPOTHESIS` with the experiment the coder must run first. Re-run the baseline command yourself. Record the three briefs and the merged version in REVIEW.md.
1. **Ticket.** Mark the step `[~]`, add a job block to REVIEW.md, write the ticket (goal, plan section, Done when, scope, quoted TODO.md claims). For a performance job, Done when is the benchmark command and its threshold from the techlead brief.
2. **Build ×3.** Create three worktrees, spawn three coders. Re-run each version's focused tests yourself (`-cxx clang++` too for runtime/emitter); record result + commit. Red versions stay in.
3. **PM.** Fresh PM with ticket, worktrees, handoffs, your test results. Copy its merge plan into REVIEW.md unchanged.
4. **Merge.** Rename the merging coder's branch/worktree to `job/<job>` at `~/worktrees/GoToCPP/<job>`; send the merge brief. Re-run tests, record the hash (`git rev-parse --short=12 HEAD`, clean `git status`). Remove the other two worktrees and branches.
5. **Review.** Run the hygiene check (`git diff --name-status --diff-filter=A dev...job/<job>`, `--stat`, `git status --short`). Hash before the spawn. Fresh reviewer; pass no conclusions or earlier verdicts. Re-hash after; a change means the round is void.
6. **Fix.** Every finding (NITs too, and older same-area issues) → `OPEN`, send to the merging coder. The coder's **default is FIX** — it exists to shrink TODO.md, so a finding is more TODO work, not an argument. It answers each with `FIX` (patch + a test that fails without it) or, once only, `REFUTE` (file:line + runnable reproduction). Re-run tests, hash, spawn a **new** fresh reviewer: adjudicate first (`CONFIRM-REFUTED`/`REJECT-REFUTED`, `FIX-VERIFIED`/`FIX-REJECTED`, new tests must fail on the pre-fix commit), then review everything in scope.
7. **Done** at the first **CLEAN** from a fresh reviewer on a hash you tested green with every finding `CLOSED-*`: re-run the focused tests, fill the completion block, run `gocpp-merge` (TODO.md claims closed in the same commit), mark `[x]`, file out-of-scope notes.

Reviews add nothing to TODO.md: finds are fixed in the job (a multi-day redesign becomes its own job). After 5 rounds without CLEAN, step in: split, clarify, or ask the user.

## Disputes
`REFUTE` is single-shot and never loops. The reviewer adjudicates it **once**, and **you close on that verdict without re-checking or a second opinion**: `CONFIRM-REFUTED` → `CLOSED-REFUTED`, done; `REJECT-REFUTED` → `REOPENED`, the coder fixes it and cannot refute it again. There is no retry with new evidence and no deadlock path. Only a claim neither side can demonstrate at all goes to the user (`AskUserQuestion`) with both evidence packs, and you record the answer verbatim. The loop never exits with an `OPEN` finding.

## Spike/evidence jobs
Measurements, plan facts, fixtures, audits: one coder, no PM, no reviewer; you re-run the checks. Product code (`cmd/`, `internal/`, `runtime/`, `stdlib/`, `tests/`) takes the full loop.

## Deleting
Workers delete only a literal path under their own worktree's `.scratch/`; never `rm -rf .`/`*`/`$VAR`, `git clean`, `git reset --hard`, `git checkout -- .` (your void-round reset to a recorded hash, in the job's own worktree, is the one exception). Only you remove worktrees and branches.

## Lifecycle
Findings: `OPEN → FIXED-PENDING / REFUTED-PENDING → CLOSED-FIXED / CLOSED-REFUTED / REOPENED`. Markers: `[ ]` not started · `[~]` in progress · `[x]` done.
